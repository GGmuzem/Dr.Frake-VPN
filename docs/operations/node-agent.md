# FBLink Node Agent Operations

The production management path is:

1. Main VDS stores `AGENT_SIGNING_PRIVATE_KEY` as a secret.
2. Each VPN VPS runs `fblink-node-agent` on `127.0.0.1:9090` or a private interface.
3. VLESS/Reality exposes the management path from the main VDS to the agent URL.
4. The backend calls admin-only endpoints:
   - `POST /api/v1/admin/servers/:id/agent/snapshot`
   - `POST /api/v1/admin/servers/:id/agent/update`
   - `POST /api/v1/admin/servers/:id/agent/rollback`
   - `GET /api/v1/admin/servers/:id/agent/status`

Do not expose the agent port directly to the public internet. Do not deploy production updates through `git pull`; use immutable Docker image digests such as `registry.example/fblink-node-agent@sha256:...`.

The agent update command is intentionally external to the HTTP API. Install a host-side executable, point `AGENT_UPDATE_COMMAND` and `AGENT_ROLLBACK_COMMAND` to it, and have that executable perform the service/container restart plus healthcheck. The agent only passes `FBLINK_TARGET_IMAGE_DIGEST`.

## Full Bootstrap From Main VDS

Use this admin endpoint to install both the VPS agent and the VLESS/Reality management sidecar:

```http
POST /api/v1/admin/servers/:id/agent/bootstrap
Content-Type: application/json

{
  "agent_image_digest": "registry.example/fblink-node-agent@sha256:...",
  "xray_image_digest": "ghcr.io/xtls/xray-core@sha256:...",
  "management_port": 39001,
  "local_port": 19001,
  "node_id": "vps-1"
}
```

The backend uses the existing SSH credentials only for bootstrap. It then:

1. Pulls exact Docker image digests on the VPS.
2. Starts `fblink-node-agent` bound to `127.0.0.1:9090`.
3. Starts `fblink-mgmt-xray` as a separate VLESS/Reality sidecar on the VPS.
4. Starts `fblink-mgmt-client-<server_id>` on the main VDS with a local `dokodemo-door` listener.
5. Saves `agent_url` as `http://127.0.0.1:<local_port>`.
6. Probes `/health` through the management tunnel.

Requirements:

- Docker must be available on the VPS and on the main VDS host running `vpn-backend`.
- `vpn-backend` needs access to the local Docker CLI/socket if it should create the main-VDS tunnel container.
- The selected `management_port` must be reachable from the main VDS to the VPS.
- The selected `local_port` must be free on the main VDS.
