# FBLink Node Agent

`fblink-node-agent` runs on each VPN VPS and exposes a small local management API for the main VDS. The API is intended to be reachable only through the VLESS/Reality management path or a private interface. Do not publish it directly on the internet.

## API

- `GET /health` returns version, commit, uptime, node id, and Docker client availability.
- `GET /snapshot` returns `tar.gz` with allowlisted Docker config files and a `manifest.json`.
- `POST /updates/apply` accepts `{"image_digest":"repo/name@sha256:..."}`.
- `GET /updates/status` returns the last update state.
- `POST /updates/rollback` applies the recorded previous digest.

All endpoints except `/health` require Ed25519 signed requests from the main VDS.

## Signing

The main VDS signs this canonical payload:

```text
METHOD
/path
RFC3339_TIMESTAMP
NONCE
SHA256_HEX_BODY
```

Headers:

- `X-FBLink-Timestamp`
- `X-FBLink-Nonce`
- `X-FBLink-Signature` as base64 Ed25519 signature

The agent receives only `AGENT_VERIFY_PUBLIC_KEY`, a base64 raw Ed25519 public key. The private signing key stays on the main VDS secret storage.

## Environment

- `AGENT_ADDR`, default `127.0.0.1:9090`
- `AGENT_NODE_ID`
- `AGENT_VERIFY_PUBLIC_KEY`
- `AGENT_ALLOWED_SKEW_SECONDS`, default `300`
- `AGENT_DOCKER_BIN`, default `docker`
- `AGENT_XRAY_CONTAINER`, default `amnezia-xray`
- `AGENT_AWG_CONTAINER`, default `amnezia-awg2`
- `AGENT_AWG_INTERFACE`, default `awg0`
- `AGENT_PIHOLE_CONTAINER`, default `pihole`
- `AGENT_UPDATE_COMMAND`
- `AGENT_ROLLBACK_COMMAND`
- `AGENT_STATE_PATH`, default `/var/lib/fblink-node-agent/update-state.json`

`AGENT_UPDATE_COMMAND` and `AGENT_ROLLBACK_COMMAND` are host-managed executables. The agent passes the verified immutable digest in `FBLINK_TARGET_IMAGE_DIGEST`; this keeps production deployment arguments outside the HTTP API and avoids generic remote shell behavior. The update command should restart the container/service and return non-zero if its healthcheck fails; the agent will then call the rollback command with the previous digest when one is recorded.
