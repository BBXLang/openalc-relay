# Bootstrap dependency decisions

Recorded 2026-11-02. The server module declares Go 1.26.0 as its minimum language
version; this is a baseline, not a recommendation to install an old patch.
Use a maintained patch release in that minor line or newer. The local toolchain
was Go 1.27.3 on Windows/amd64 with CGO disabled.

| Dependency | Fixed version | Purpose | License / evidence |
| --- | --- | --- | --- |
| Go standard library | Installed toolchain | HTTP, configuration, JSON, logs, signals, hashing, SQL API | Go's distribution license |
| modernc.org/sqlite | v1.61.1 | Pure-Go SQLite, no C compiler for server builds | BSD-3-Clause; [official package](https://pkg.go.dev/modernc.org/sqlite) |
| golang.org/x/crypto | v0.58.0 | Argon2id owner password verifier for local bootstrap and login | BSD-3-Clause; [official module](https://pkg.go.dev/golang.org/x/crypto/argon2) |
| actions/checkout | v7.0.1, full commit SHA in CI | Read-only source checkout without persisted credentials | [upstream](https://github.com/actions/checkout) |
| actions/setup-go | v7.0.0, full commit SHA in CI | Install Go and cache modules | [upstream](https://github.com/actions/setup-go) |

`go.mod` and `go.sum` pin the actual graph, including modernc.org/libc at the
SQLite driver's required version. Do not independently upgrade libc without
checking the driver's compatibility guidance. `go mod verify` checks the local
module cache against recorded checksums; it is not a vulnerability or license
audit.

The driver includes third-party components beyond its own license. Review its
`LICENSE-3RD-PARTY.md` and SBOM plus the complete linked module graph before public
binary/container distribution, preserving required notices. A full release
license inventory and vulnerability scan are pending. HarmonyOS, WebAuthn,
crypto-storage and vendor SDK dependencies have not been chosen yet.

## Android synthetic PoC additions

| Dependency | Version | Purpose / license |
| --- | --- | --- |
| Gradle distribution | 8.13, SHA-256 pinned in wrapper properties | Reproducible build bootstrap; Apache-2.0 |
| Kotlin + Compose/serialization compiler plugins | 2.3.21 | Android/JVM Room test module; Apache-2.0 |
| Android Gradle Plugin | 8.13.2 | SDK 36 Android debug build; Apache-2.0 |
| KSP | 2.3.12 | Room code generation; Apache-2.0 |
| Room | 2.8.5 | Same DAO/schema tested on disk under JVM and Android; Apache-2.0 |
| AndroidX SQLite bundled driver | 2.6.2 | SQLite runtime for Room; AndroidX Apache-2.0 plus bundled SQLite notices |
| kotlinx.coroutines / serialization-json | 1.10.2 / 1.9.0 | Suspend storage/sync and typed strict JSON; Apache-2.0 |
| Compose BOM / Activity Compose / Lifecycle | 2025.10.01 / 1.11.0 / 2.9.4 | Minimal debug validation controls; Apache-2.0 |
| WorkManager | 2.11.2 | Connected-network work, backoff, periodic repair; Apache-2.0 |
| AndroidX test runner/core / ext-junit | 1.7.0 / 1.3.0 | Android device database test; Apache-2.0 |

The pinned AGP supports Kotlin 2.3 and SDK 36; these are compatibility baselines,
not claims to be the latest releases. Room uses only Android/JVM targets to test
the actual persistent implementation without an Android SDK; HarmonyOS remains
native ArkTS. The synthetic-only transport uses JDK HTTP APIs, avoiding another
network library. JVM tests use Kotlin's JUnit adapter; test-only licenses are not
an app runtime license grant. Android artifacts and full release notices still
require the SDK build and release inventory.

Sources: [AGP compatibility](https://developer.android.com/build/releases/agp-8-13-0-release-notes),
[Room](https://developer.android.com/jetpack/androidx/releases/room),
[Room KMP](https://developer.android.com/kotlin/multiplatform/room),
[WorkManager](https://developer.android.com/jetpack/androidx/releases/work).

## Web synthetic PoC additions (2026-11-03)

| Dependency | Fixed version | Purpose / license |
| --- | --- | --- |
| Vue | 3.5.44 | Minimal reactive verification page; MIT |
| Vite / official Vue plugin | 8.3.5 / 6.0.10 | Build local static assets, no runtime CDN; MIT |
| TypeScript / vue-tsc | 5.9.3 / 3.3.13 | Type and Vue template checks; Apache-2.0 / MIT |
| @playwright/test | 1.64.1 | Real Chromium desktop/mobile-sized browser verification, test-only; Apache-2.0 |
| actions/setup-node | v6, full commit SHA in CI | Node 26 for integrated host tests; MIT |

Exact versions and transitive integrity hashes are in `apps/web/package-lock.json`.
Node 26.12.1/npm 11.20.0 were used locally. The project `.npmrc` selects the official
npm registry without changing global npm settings. Playwright's Chromium download
is a test tool, not shipped in the app; preserve applicable notices if distributed.
Versions were resolved from npm metadata and checked by an actual install/build;
these are not claims of being the latest releases. No UI library, router, state
framework or additional Go dependency was introduced.

Sources: [Vue build setup](https://vuejs.org/guide/quick-start.html),
[Vite guide](https://vite.dev/guide/), and pinned packages' upstream license metadata.

The Docker builder uses the official `golang:1.26.10-bookworm` tag, checked against
the [official image listing](https://hub.docker.com/_/golang). Runtime is
`gcr.io/distroless/static-debian12:nonroot`. Tags are mutable; release automation
must resolve tested image digests and preserve an SBOM before publishing. Docker
is unavailable in the current workspace, so the container image has not been built.
