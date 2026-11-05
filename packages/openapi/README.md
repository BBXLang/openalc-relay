# API contract

[`openapi.json`](openapi.json) is the OpenAPI 3.1 contract for the implemented
server. It describes liveness/readiness (GET and HEAD) and the locally bootstrapped
owner session (GET, POST and DELETE). The session route requires a valid owner,
keyring and HTTPS public origin configuration; it is not a complete Web console.
JSON lets Go tests check the contract without a YAML dependency. Changes to
documented responses must pass the HTTP and session tests.

Planned authenticated business endpoints and resource boundaries are in
[RFC-0001](../../docs/architecture/RFC-0001-system-architecture.md#7-api-与同步契约).
They are not implemented and are not advertised as available in OpenAPI.

[`poc-collector.json`](poc-collector.json) separately describes the implemented
loopback-only synthetic test collector. It is a different executable/database,
accepts no original SMS body or telephone number, and is not a production API.
It includes the implemented installation-scoped paginated GET used by the
[minimal Web verification page](../../apps/web/README.md), as well as the Android
metadata upload and aggregate stats. Browser writes and cross-origin reads remain
forbidden. The production health contract remains available.
