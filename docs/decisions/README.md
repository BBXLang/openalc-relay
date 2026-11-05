# Architecture decision register

Current proposals are grouped in [RFC-0001](../architecture/RFC-0001-system-architecture.md).
This is not a record of owner-approved production behavior or verified phone capabilities.

| Item | Current position | Status |
| --- | --- | --- |
| Server shape | Go modular monolith, SQLite, one instance | User-directed baseline; bootstrap implemented |
| Transport | Event-driven HTTPS, offline-first, best-effort push | User-directed; protocol design proposed, phone PoC pending |
| SMS reliability | Durable attempts, fenced start, no uncertain automatic resend | Required invariant; design only |
| Primary client | Vue 3 Web on mobile/desktop; native HarmonyOS deferred | Owner decision, 2026-11-03; [ADR-0003](0003-web-first.md) |
| Sensitive storage | Authenticated envelope encryption, no E2EE claim | Required; not implemented |
| Project license | Evaluate Apache-2.0 and MIT | Owner decision pending; no LICENSE grant added |
| Dependency notices | Pin modules, inventory linked licenses before release | Initial graph recorded; full release audit pending |
| Synthetic Android PoC | Shared Android/JVM Room tests, separate loopback collector | [ADR-0002](0002-synthetic-sms-poc.md); host evidence is not phone evidence |

Create a focused ADR when a significant choice is accepted or changed. Include
context, alternatives, consequences, verification evidence, and superseded decisions.
