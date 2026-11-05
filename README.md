# OpenALC Relay

Self-hosted communication management for your own Android phones and SIM cards,
with a Web client prioritized for mobile and desktop browsers. The owner
resumed Android, Web, and HarmonyOS UI work on 2026-11-05; HarmonyOS business
integration and Push Kit remain deferred.

[简体中文](README.zh-CN.md) · [Architecture RFC](docs/architecture/RFC-0001-system-architecture.md) · [PoC plan](docs/development/POC-PLAN.md)

The [server and Web MVP plan](docs/architecture/RFC-0002-server-mvp.md) defines
the next security, sync, management, deployment and continuous-test gates. It is
a staged plan; only the owner bootstrap and session slice is implemented so far.

Moving to another computer: [migration checklist](docs/development/MIGRATION.zh-CN.md)
and [Codex handoff](docs/development/CODEX-HANDOFF.md). The export script preserves
uncommitted source, includes a per-file integrity manifest and excludes credentials/caches.

**Status: architecture and feasibility stage.** The Go server contains health
checks, SQLite migrations, structured logs and graceful shutdown. Local owner
bootstrap and an authenticated Web session endpoint are now implemented, but
device pairing and business event APIs are not. The server does not receive or
send SMS, encrypt stored communication payloads, or integrate push.
An Android debug PoC now captures only synthetic fixture metadata with Room and
WorkManager; its separate local collector is described below. No production
mobile application or business UI is implemented. Mobile-first Android, Web and
HarmonyOS inbox shells are available, with synthetic data only on Android/Web.
The [HarmonyOS Stage project](apps/harmony/README.md) builds and launches on
phone and tablet emulators; physical HarmonyOS behavior is unverified.
Do not connect real communication
data to this bootstrap or treat it as a production Relay service.

## Design direction

- Go modular monolith, SQLite first; PostgreSQL support is planned.
- Offline-first event capture and durable queues, event-driven HTTPS sync.
- Push-assisted command discovery; adaptive background work instead of permanent sockets.
- Explicit device/SIM selection, fenced command execution, no automatic resend of uncertain SMS.
- Independent device identities, encrypted payload storage, and privacy-preserving notifications are requirements for business development.
- Android: Kotlin/Compose. HarmonyOS: native ArkTS/ArkUI. Web: Vue 3/TypeScript.

Platform capabilities remain **Needs Verification**, including HyperOS background
behavior, OTP visibility, call-log permissions, Xiaomi wakeup feasibility, and
HarmonyOS 7 Push Kit eligibility/delivery. See the [official-source register](docs/development/PLATFORM-VERIFICATION.md).
Actual local checks and unverified environments are recorded in the
[initialization verification report](docs/development/VERIFICATION.md).

The [Android synthetic SMS PoC](apps/android/README.md) has a strict fixture
allowlist, Room persistence, ACK/retry logic, WorkManager adapters and minimal
validation controls. It never stores/uploads original SMS bodies or sender
numbers. The [PoC guide and evidence](docs/development/ANDROID-POC.md) distinguish
JVM verification, APK build and actual phone validation.

The [Web verification page](apps/web/README.md) reads committed metadata from that
same local collector. The [Web PoC guide](docs/development/WEB-POC.md) describes the
Kotlin/Room → Go/SQLite → browser host test and its actual-device limits.
The [Xiaomi sync record](docs/development/DEVICE-SYNC-2026-11-05.md) also verifies
two button-created synthetic events from the phone through the local collector
to mobile and desktop browser views, including recovery after a lost ACK.

The [shared UI design](docs/design/RELAY-UI.zh-CN.md) follows the owner's mobile
reference screenshot. The [HarmonyOS emulator record](docs/development/HARMONY-UI-2026-11-05.md)
separates build and emulator results from physical-device validation.

## Run the bootstrap

Requires Go 1.26 or newer. No external database, CGO, Node, Android SDK, or
HarmonyOS SDK is required for the server.

```sh
cd server
go run ./cmd/relay
```

From another terminal:

```sh
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
```

On PowerShell, use `Invoke-RestMethod http://127.0.0.1:8080/readyz`.
Both return `{"status":"ok"}` when ready. `/healthz` checks process liveness;
`/readyz` checks database connectivity and returns 503 without private error
details when unavailable. Before owner bootstrap, unknown paths return 404.
After bootstrap and secure configuration, `/v1/session` supports owner login,
session lookup and logout. The implemented routes are documented in
[OpenAPI](packages/openapi/openapi.json).

The binary reads process environment variables; it **does not auto-load `.env`**.
[`.env.example`](.env.example) lists the supported settings:

| Variable | Default | Meaning |
| --- | --- | --- |
| `RELAY_HTTP_ADDR` | `127.0.0.1:8080` | Literal IP and port; loopback by default |
| `RELAY_DATABASE_PATH` | `data/relay.db` | Filesystem path, relative to the working directory |
| `RELAY_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |
| `RELAY_MASTER_KEY_FILE` | unset | Required after owner bootstrap; keep outside Git and backups |
| `RELAY_PUBLIC_ORIGIN` | unset | Required after owner bootstrap; exact HTTPS browser origin |

Startup creates the data directory, opens SQLite with WAL/foreign keys, and
transactionally applies embedded migrations. The schema currently includes the
migration ledger, singleton owner and revocable Web sessions. The server rejects
changed migration checksums and unknown history. Only one process may own a
database. After owner bootstrap, the HTTP listener must remain on loopback
behind a correctly configured HTTPS proxy. See the
[implementation record](docs/development/SERVER-IMPLEMENTATION-2026-11-05.md).

## Development

Run from `server/`:

```sh
go test ./...
go vet ./...
go build ./cmd/...
```

On Linux with CGO and a C compiler available, also run `go test -race ./...`.
The SQLite driver itself requires no CGO. CI defines Linux/Windows checks and a
Linux race check. See the [GitHub Actions runs](https://github.com/BBXLang/openalc-relay/actions)
for the actual hosted results.

Deployment examples: [local Docker Compose and Linux binary](docs/deployment/BOOTSTRAP.md).
The examples do not configure a public domain, TLS, encrypted communication
payload storage, or backups on your behalf.

## Repository map

```text
apps/android/          Debug synthetic PoC and JVM-testable Room queue
apps/web/              Minimal Vue/TypeScript synthetic verification page
apps/harmony/          ArkUI Stage UI project; no business connection
server/                Go module, owner session, storage, embedded migrations
packages/openapi/      Implemented OpenAPI 3.1 contract
packages/protocol/     Proposed reliability invariants
deploy/docker/         Local bootstrap container example
deploy/systemd/        Linux single-instance service example
docs/architecture/     RFC-0001
docs/development/      PoC plan, platform evidence, local verification
docs/deployment/       Bootstrap operation and safety boundaries
docs/decisions/        Decision register
```

Read [AGENTS.md](AGENTS.md), [CONTRIBUTING.md](CONTRIBUTING.md), and
[SECURITY.md](SECURITY.md) before changing the project. Real phone permission and
OS behavior remain separate validation gates.

## License

OpenALC Relay is licensed under [MIT](LICENSE), copyright 2026 BBXLang.
Dependencies retain their own licenses; see
[dependency notes](docs/development/DEPENDENCIES.md).
