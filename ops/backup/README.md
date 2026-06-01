# FBLink encrypted backups

This setup sends encrypted daily backups from the production VDS to
`srv.frakebit.com`. The receiver stores encrypted restic repository data only;
it does not have the repository password and cannot read restored files.

## What is backed up

- A consistent SQLite snapshot from the `vpn-backend` container.
- `/app/data/downloads` from the backend data volume.
- Deployment files from the VDS, such as `.env`, `docker-compose.yml`, Caddy
  config, certificates, and any extra paths listed in `backup.env`.

Redis is intentionally excluded because it is treated as cache/rate-limit state.

## Receiver server

1. Create an `A`/`AAAA` record for `srv.frakebit.com` pointing to the backup
   server.
2. Copy `receiver/docker-compose.yml` and `receiver/Caddyfile.example` to
   `srv.frakebit.com`.
3. Create `receiver/auth.htpasswd` with a strong username/password.
4. Start the receiver with `docker compose up -d`.
5. Put Caddy or another TLS reverse proxy in front of `rest-server` using the
   `srv.frakebit.com` hostname.
6. Restrict access to the production VDS IP or to a private management VPN.

The receiver should not contain `RESTIC_PASSWORD` or the repository password
file. Keep those on the VDS and in a separate offline recovery vault.

## VDS source server

1. Install `restic`, `docker`, `flock`, and `sqlite3` on the VDS host.
2. Copy `source/fblink-backup.sh` to `/usr/local/sbin/fblink-backup`.
3. Copy `source/fblink-backup.env.example` to `/etc/fblink-backup/backup.env`
   and fill in the real paths and receiver credentials.
4. Store the restic repository password in
   `/etc/fblink-backup/restic-password` with mode `0600`.
5. Store the receiver password from `auth.htpasswd` in
   `/etc/fblink-backup/rest-server-password` with mode `0600`.
6. Run `/usr/local/sbin/fblink-backup --validate-config` before the first
   backup. This rejects insecure `http://` receivers, unexpected receiver
   hosts, credentials embedded in `RESTIC_REPOSITORY`, and unreadable secret
   files.
7. Run `/usr/local/sbin/fblink-backup` manually once.
8. Copy `source/fblink-backup.service` and `source/fblink-backup.timer` to
   `/etc/systemd/system/`, then run `systemctl enable --now fblink-backup.timer`.

## Restore check

Run `restore/fblink-restore-latest.sh /tmp/fblink-restore-test` on a trusted
machine with restic access. The script restores the latest snapshot and runs
`PRAGMA integrity_check` against the restored SQLite file when `sqlite3` is
available.

## Retention

The receiver is append-only. Do not run `restic forget --prune` from the VDS.
Run retention and prune manually from a separate trusted maintenance machine
after a successful restore test.
