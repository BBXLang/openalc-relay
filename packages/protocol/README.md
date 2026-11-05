# Relay protocol design boundary

Status: proposed; no generated client SDK or working business protocol yet.

The implemented synthetic-only test protocol is separately documented in
[the PoC guide](../../docs/development/ANDROID-POC.md) and
[its OpenAPI](../openapi/poc-collector.json). Its fixture format and unauthenticated
loopback collector must not be treated as the production device protocol.

The authoritative design is [RFC-0001](../../docs/architecture/RFC-0001-system-architecture.md).
Implement schemas alongside their first vertical slice, then add them to OpenAPI.

- Events use a stable `(device_id, installation_id, client_event_id)` identity.
- ACK means a committed server transaction, never merely receipt of a request.
- Command creation uses an owner-scoped idempotency key and a server expiry.
- Claim and start are separate, fenced transitions; credentials are checked again at start.
- A provider acceptance is not a wakeup, an execution, or an SMS delivery receipt.
- A possibly invoked SMS remains `unknown` until the same attempt's evidence resolves it.
- An uncertain or partially sent SMS must never be automatically executed again.
- `sent` and per-segment recipient delivery are distinct facts.
- Sync cursors are owner-bound and advance only after local application succeeds.

Do not generate client types from the proposed tables or endpoint list and imply
they are already interoperable. Protocol versioning and golden fixtures belong
to the first implemented business slice.
