# Self-Hosted XRay Setup

This folder now contains a one-shot bootstrap script for deploying `amnezia-xray`
on a fresh VPS in a layout that the FBLink backend can auto-discover over SSH.

## What it does

- builds the `amnezia-xray` docker image from the local `Dockerfile`
- generates and persists:
  - `server.json`
  - `xray_uuid.key`
  - `xray_short_id.key` (совместимость, всегда == первый элемент из `xray_short_ids.txt`)
  - `xray_short_ids.txt` (8 short IDs, по строке на ID)
  - `xray_public.key` / `xray_private.key` (Reality X25519 keypair)
- recreates the docker container with `--restart always`
- publishes the chosen TCP port
- writes VLESS + TLS + XHTTP over `packet-up`
- enables inbound sniffing and blocks private IP / BitTorrent server-side
- opens the local host firewall when possible
- prints the final connection parameters and verification commands

## Files expected by backend

The backend VIP XRay auto-discovery reads these files over SSH:

- `/opt/amnezia/xray/server.json`
- `/opt/amnezia/xray/xray_uuid.key`
- `/opt/amnezia/xray/xray_short_id.key`
- `/opt/amnezia/xray/xray_short_ids.txt`
- `/opt/amnezia/xray/xray_public.key`
- `/opt/amnezia/xray/xray_private.key`

## Recommended defaults

- container: `amnezia-xray`
- config dir: `/opt/amnezia/xray`
- port: `443`
- XHTTP path: `/assets/7d91f0e4`
- XHTTP mode: `packet-up`
- TLS: generated self-signed origin certificate for CDN/origin tests
- uTLS fingerprint: `chrome`

SNI is required and intentionally has no default. Choose a real host that fits
your deployment target.

## Quick start on a new VPS

Copy the `xray` folder to the server and run:

```bash
chmod +x ./install_selfhosted.sh
./install_selfhosted.sh --port 443 --sni example.com
```

If you want a fixed public address in the printed summary:

```bash
./install_selfhosted.sh \
  --port 443 \
  --sni example.com \
  --xhttp-path /assets/7d91f0e4 \
  --public-host 138.124.101.69
```

## Verification

On the VPS:

```bash
docker ps --format 'table {{.Names}}\t{{.Ports}}\t{{.Status}}'
docker exec amnezia-xray sh -lc 'nc -z 127.0.0.1 443 && echo XRay is listening'
```

From Windows:

```powershell
Test-NetConnection <server-ip> -Port 443
```

Expected:

- `docker ps` shows `0.0.0.0:443->443/tcp`
- `nc -z` succeeds inside the container
- `TcpTestSucceeded : True` from Windows

## Important notes

- If external TCP reachability is still false, open the port in the provider firewall.
- Re-running the script keeps existing UUID/keys by default.
- Use `--force-regenerate` only when you intentionally want to rotate credentials.
- Use `--rebuild-image` when you want to rebuild the docker image from the current repo files.
- Keep AmneziaWG as fallback while testing this XHTTP profile.

## Backend integration

After the server is up:

1. add or update the server record in FBLink admin/backend
2. ensure SSH host/user/password are correct
3. make sure the server host/IP in backend matches the public address
4. refresh client config or re-login

The backend will then auto-read the XRay runtime files from the server.
