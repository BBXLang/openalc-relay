# Bootstrap deployment: local validation only

This stage runs health checks, SQLite migrations, and an optional locally
bootstrapped owner session. Device pairing, encrypted communication storage,
event APIs, push, backup tooling and public TLS setup are not implemented.
The examples have not been validated on Rocky
Linux 9.5. Run one instance per database on a local filesystem.

## Local binary

From `server/`:

```sh
go build -o ../bin/relay ./cmd/relay
../bin/relay
```

For a Linux amd64 binary from a POSIX shell:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o ../bin/relay-linux-amd64 ./cmd/relay
```

PowerShell equivalent (from `server/`):

```powershell
$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
go build -trimpath -o ../bin/relay-linux-amd64 ./cmd/relay
Remove-Item Env:GOOS
Remove-Item Env:GOARCH
```

Set `RELAY_HTTP_ADDR`, `RELAY_DATABASE_PATH`, and `RELAY_LOG_LEVEL` as needed.
`.env.example` is a reference; the binary does not read it. Defaults are loopback,
`data/relay.db` relative to the working directory, and `info` logs. Startup applies
migrations before opening the listener. New directories/files request 0700/0600
on Unix; check existing directories and Windows ACLs separately.

## Docker Compose

Requires Docker Engine and Compose v2. From the repository root:

```sh
docker compose -f deploy/docker/compose.yaml up --build -d
curl --fail http://127.0.0.1:8080/readyz
docker compose -f deploy/docker/compose.yaml logs relay
docker compose -f deploy/docker/compose.yaml down
```

The named volume persists across `down`. Do not remove it unless intentionally
discarding data. The container runs as UID/GID 65532 with a read-only root
filesystem; only its data volume and temporary filesystem are writable. The port
is published to host loopback only. The image contains no shell/curl; probe from
the host. A healthcheck is not fabricated with tools absent from the runtime.
Container build, volume ownership and shutdown remain Needs Verification because
Docker is not installed in the current environment.

For a Rocky Linux bind mount in place of the named volume, verify ownership and
SELinux labeling (for example, a dedicated appropriately labeled directory),
without disabling SELinux. Never share the SQLite files between replicas or place
WAL storage on an unverified network filesystem.

## systemd alternative

The example [`relay.service`](../../deploy/systemd/relay.service) expects the
binary at `/usr/local/bin/openalc-relay` and a pre-created system user/group
`openalc-relay`. It uses `StateDirectory=openalc-relay` for writable persistent
storage, a strict umask, and a loopback listener. Optional settings are read from
`/etc/openalc-relay/relay.env` via systemd, not by the Go process itself.

Review and install the service on the target server using normal administrative
procedures. Verify the user, binary permissions, unit hardening and SELinux
policy first; no installation or privileged server modification was performed by
this task. Do not run Compose and this unit against the same database.

## Public deployment and recovery gates

Public service must use a valid HTTPS reverse proxy and firewall off direct
access to the HTTP listener. A container proxy would require an explicitly
configured internal network/TLS setup; this Compose example does not provide it.
Verify request limits, trusted-proxy boundaries, clock sync, disk capacity and
log redaction. Device authentication and encrypted communication payload storage
must land before storing or exposing real communication data. The owner-session
slice does not make the current deployment examples production-ready.

Before upgrades, take a consistent backup. Use SQLite's online backup facilities,
or stop the process cleanly and copy a consistent complete state; copying only an
active WAL database's main file is unsafe. Backup automation is not implemented.
Keep future encryption keys separate from encrypted backups and test restores.
Unknown/newer or edited migration history must block startup; do not delete ledger
rows to force a downgrade. Restore a compatible binary/backup instead.

Future command restoration must invalidate sync cursors and suspend unfinished
send intents until reconciliation. Never resume possibly executed commands from
an old snapshot. SIGINT/SIGTERM triggers a bounded HTTP shutdown before database
close; graceful shutdown needs a Linux/systemd real-host check.
