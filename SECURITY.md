# Security policy

The bootstrap is not a production communication service. It has local owner
bootstrap and a revocable Web session endpoint, but no device authentication,
communication-event API, encrypted business payload storage, or vendor integration.
Do not put real SMS, OTPs, telephone numbers, push tokens, or secrets into it.
No released version currently carries a security support promise.

The target model requires HTTPS, independent revocable device credentials,
owner/device authorization, explicit single-recipient send actions, rate limits,
audit metadata, authenticated encryption at rest, and notifications without
sensitive content. At-rest encryption is not end-to-end encryption; the server
will be able to decrypt payloads. These are requirements, not implemented claims.

Never log SMS bodies, OTPs, full telephone numbers, authorization headers, pairing
challenges, private keys, or raw request bodies. Ignore local databases, `.env`
files, backups, signing keys and PoC evidence. Protect the data directory and
backups with OS permissions; Windows ACLs need separate verification because Unix
mode bits do not establish equivalent protections there.

Public connections must terminate TLS at a trusted reverse proxy, with no direct
public access to the plaintext listener. Trust forwarded headers only from an
explicit proxy allowlist when that feature is implemented. Do not disable
certificate validation. The bootstrap has no need to trust forwarded headers.

Do not open a public issue containing a vulnerability's sensitive evidence. Use
the repository's [private vulnerability reporting form](https://github.com/BBXLang/openalc-relay/security/advisories/new)
when available. If that form is unavailable, request a private reporting channel
without posting exploit details or secrets. No security email address or response
SLA is asserted here.

Backup automation, key rotation, retention policy and disaster recovery still
need implementation and a restore drill. Restoring old command state must pause
remote sending; an uncertain SMS must never be replayed automatically.
