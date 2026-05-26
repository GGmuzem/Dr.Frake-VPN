# Self-hosted Hysteria2 for VIP

This script installs Hysteria2 as the primary VIP transport for Happ
subscriptions. It writes `/etc/hysteria/config.yaml`, starts
`hysteria-server.service`, and prints the admin fields needed by the backend.

Example:

```bash
sudo bash install_selfhosted.sh \
  --port 443 \
  --sni your-domain.example \
  --force-regenerate
```

Open UDP on the provider firewall for the chosen port. The script opens local
iptables/ufw/firewalld rules where those tools are available.

The generated Happ link uses:

```text
hy2://<password>@<server>:<port>/?sni=<sni>&insecure=1&obfs=salamander&obfs-password=<password>
```
