# FBLink Node Agent Operations

The production management path is now hybrid:

1. Main VDS stores `AGENT_SIGNING_PRIVATE_KEY` as a secret for commands sent to agents.
2. Each VPN VPS runs `fblink-node-agent` on a Docker-private interface.
3. Each agent has its own `AGENT_PUSH_PRIVATE_KEY`; the backend stores only `agent_push_public_key`.
4. Agents push heartbeat and config snapshots to the public backend API over HTTPS:
   - `POST /api/v1/node-agent/heartbeat`
   - `POST /api/v1/node-agent/snapshot`
5. VLESS/Reality remains the management path for manual/admin actions:
   - `POST /api/v1/admin/servers/:id/agent/snapshot`
   - `POST /api/v1/admin/servers/:id/agent/update`
   - `POST /api/v1/admin/servers/:id/agent/rollback`
   - `GET /api/v1/admin/servers/:id/agent/status`

Do not expose the agent port directly to the public internet. Do not deploy production updates through `git pull`; use immutable Docker image digests such as `registry.example/fblink-node-agent@sha256:...`.

Automatic config sync is push-based: the agent sends snapshots when the content hash changes, and periodically refreshes the snapshot even if unchanged. Manual snapshot requests remain available through the admin endpoint for immediate sync or diagnostics.

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
2. Generates an agent push key pair and stores only the public key in `vpn_servers.agent_push_public_key`.
3. Starts `fblink-node-agent` on the private `fblink-mgmt` Docker network.
4. Starts `fblink-mgmt-xray` as a separate VLESS/Reality sidecar on the VPS.
5. Starts `fblink-mgmt-client-<server_id>` on the main VDS with a local HTTP proxy listener.
6. Saves `agent_url` as `http://fblink-node-agent:19090` and uses the local proxy for management calls.
7. Probes `/health` through the management tunnel.

Requirements:

- Docker must be available on the VPS and on the main VDS host running `vpn-backend`.
- `vpn-backend` needs access to the local Docker CLI/socket if it should create the main-VDS tunnel container.
- The selected `management_port` must be reachable from the main VDS to the VPS.
- The selected `local_port` must be free on the main VDS.
