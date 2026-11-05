# Contributing

Relay is in its architecture and feasibility stage. Read [RFC-0001](docs/architecture/RFC-0001-system-architecture.md),
the [PoC plan](docs/development/POC-PLAN.md), and [AGENTS.md](AGENTS.md) first.
The project is licensed under [MIT](LICENSE). By contributing, you agree that
your contribution is submitted under the same license.

Keep changes small and reviewable. Describe the concrete problem, resulting
behavior, relevant checks, and limitations. Use English identifiers and commit
messages (for example, `feat(server): add readiness probe`). Discussion and design
documents may use Chinese; the primary README uses English.

Run `gofmt`, `go test ./...`, `go vet ./...`, and `go build ./cmd/relay` from
`server/`. Run the race suite on Linux when appropriate. Update OpenAPI only for
implemented endpoints and update the RFC if the design changes. Do not edit an
applied SQL migration: append a new forward migration and test failure recovery.

New dependencies need a clear purpose, a fixed version, license review, and
committed go.sum updates. Avoid preemptive frameworks, queues, and abstractions.
Business changes must test authorization, deduplication, crash windows, stale
leases, and unknown SMS outcomes. Only real-device evidence can establish platform
compatibility; mocks and emulators must be labeled.

Never commit communication data, credentials, raw device logs, databases, or
backups. Use synthetic fixtures. Do not run carrier SMS tests, incur external
costs, register vendor services, publish, or push remotely without the owner's
explicit authorization. Security reports follow [SECURITY.md](SECURITY.md).
