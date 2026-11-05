# Codex handoff — 2026-11-05

Read `AGENTS.md` first. This is the durable project state for a fresh chat on a new
computer. The export-time historical verification section was performed on the
old Windows machine; destination results are recorded separately below.

The owner selected the MIT license and authorized publication of the repository
on 2026-11-05. Earlier statements below about an unselected license or no Git
remote describe historical states.

## Proposed server and Web management MVP — 2026-11-05

The owner requested a concrete server plan after the Android/Web UI and synthetic
device-to-browser check. [RFC-0002](../architecture/RFC-0002-server-mvp.md)
defines staged delivery: local owner bootstrap and login; explicit device
pairing and revocation; encrypted, owner-scoped event ingest with commit-before-
ACK; authenticated Web inbox/device management; Rocky Linux HTTPS deployment,
backup and restore; then synthetic long-running device checks. Remote SMS send,
real messages, push and HarmonyOS business integration remain later gates. This
RFC remains a staged plan. Local key generation, one-time owner bootstrap,
revocable Web sessions and the `/v1/session` contract are implemented; pairing,
device credentials and event APIs remain pending. See
[`SERVER-IMPLEMENTATION-2026-11-05.md`](SERVER-IMPLEMENTATION-2026-11-05.md).

## Xiaomi synthetic sync to browser — 2026-11-05

The connected Xiaomi's two persisted `injected` fixtures crossed a USB loopback
tunnel into the Go collector and appeared in the built Web UI on mobile and
desktop Chromium. With first-ACK loss injected, the collector had 2 records while
Room still had 2 pending; after the owner unlocked the phone, the explicit
foreground sync action produced Room 0 pending / 2 synced with the collector
still at 2. IDs matched and a repeated sync did not add records. The Android
sync button now attempts immediately; WorkManager remains the retry/background
path. Android assemble/lint and Web build passed. See
[`DEVICE-SYNC-2026-11-05.md`](DEVICE-SYNC-2026-11-05.md). HyperOS background
scheduling and real SMS behavior remain **Needs Verification**.

## HarmonyOS Stage project and emulator result — 2026-11-05

The owner installed DevEco Studio and HarmonyOS 7 phone/tablet SDK images. The
existing ArkUI page was incorporated into a generated API 26 Stage project in
`apps/harmony/`. `@deveco/deveco-cli@1.3.0-stable` built an unsigned HAP and
installed/launched it on phone and tablet HarmonyOS 7.0.0 emulators. The phone
menu and device tab were exercised, and screenshots on both form factors were
inspected. Tablet content now has a centered width limit. The app is still a UI
shell with no SMS, account, server or Push Kit connection. Physical HarmonyOS
device behavior remains **Needs Verification**. See
[`HARMONY-UI-2026-11-05.md`](HARMONY-UI-2026-11-05.md). The `check lint` command
reported zero files checked, so it is not counted as source lint validation.

The two earlier sections below preserve the sequence of the UI correction and
Xiaomi result; their statements that HarmonyOS was source-only describe the
state before this Stage project was created.

## Screenshot correction and Xiaomi UI result — 2026-11-05

The owner supplied an actual ChatGPT Android screenshot and rejected the first
card-heavy design. Android, Web, and HarmonyOS UI source now use its structural
cues: near-white empty inbox, round menu and Relay controls, centered segmented
inbox/device tabs, and a floating bottom dock. See the revised
[`RELAY-UI.zh-CN.md`](../design/RELAY-UI.zh-CN.md). No ChatGPT name, voice
feature, or imaginary SMS conversation was copied into Relay.

The owner confirmed the phone-side USB installation prompt. The JDK 25 / Gradle
9.1 debug APK installed on Xiaomi `25113PN0EC` (Android 17 / HyperOS build
`OS4.0.5.0.XPCCNXM`). The app launched; device screenshots verify empty inbox,
drawer, and two button-saved synthetic Room rows. `RECEIVE_SMS` stayed denied.
After an app process force-stop and restart, both rows were still visible.
See [device evidence](DEVICE-UI-2026-11-05.md). This does not verify SMS
broadcasts, WorkManager upload, restart recovery on phone, or the second Xiaomi.

Android assemble/lint passed after the correction. The full Web script passed
100 offline/recovered fixtures, collector restart, and 8 browser tests again;
ignored evidence: `bin/poc-integration/5343448531b54971b8d6161bdf8d3d2e/`.
HarmonyOS remains source-only, without DevEco compilation or device testing.

## UI work and device gate — 2026-11-05

The owner selected an inbox/conversation-list first screen and requested a
ChatGPT-mobile-inspired visual direction across Android, HarmonyOS, and Web.
The shared colors, spacing, scope, and data boundaries are in
[`docs/design/RELAY-UI.zh-CN.md`](../design/RELAY-UI.zh-CN.md). Android now reads
the most recent 50 persisted synthetic Room events into the inbox while retaining
the PoC actions. Web uses a mobile-first inbox list over the same read-only
collector API. HarmonyOS has a native ArkUI source page with honest empty states;
no DevEco build or device test has run. The official local
`@deveco/deveco-cli@1.3.0-stable` (MIT) could not scaffold or lint without a
DevEco Studio installation; both commands reported that the IDE was not found.

After these UI changes, Room/JVM tests and Android debug APK/lint passed. The
complete Web host script again passed 100 offline/recovered events, collector
restart, and 8 Chromium tests; ignored evidence is in
`bin/poc-integration/1b68b7b0d3d54999a4c420e9c443ad17/`.

One Xiaomi `25113PN0EC` with Android 17 / HyperOS build
`OS4.0.5.0.XPCCNXM` is connected over USB. Its ADB authorization succeeded,
but APK installation returned `INSTALL_FAILED_USER_RESTRICTED`; no app launch,
SMS permission, or phone runtime result is claimed. See
[`DEVICE-UI-2026-11-05.md`](DEVICE-UI-2026-11-05.md). The owner chose to stop
using the old debug signing key; this connected device had no existing Relay
package before the failed install.

## Destination recovery — 2026-11-05

The 104-file migration manifest passed before source edits. On the destination
Windows machine, Go 1.26.0 (auto-downloaded toolchain) passed `go test ./...`,
`go vet ./...`, and `go build ./cmd/relay`; Node 24.15.0 / npm 11.12.1 passed
`npm ci` and Web build. `CGO_ENABLED=0`, so a local Go race test was not run.

At the owner's request, Android now uses project-local Microsoft JDK 25.0.5.0.2
and the Gradle 9.1.0 wrapper. AGP 8.13.2 and Kotlin 2.3.21 remained pinned.
Room/JVM: 14 tests passed, zero failures. `scripts/test-web-poc.ps1` passed with
100 offline pending, 100 recovered synced, 100 collector records, zero rejected,
collector restart, and 8 Chromium tests. Raw synthetic evidence is ignored under
`bin/poc-integration/fb5d2b0b4d544feba87a5dba2dfa9ea6/`.

SDK platform 36, build-tools 35.0.0, platform-tools and command-line tools were
installed locally. Main/test debug APK assembly and lint passed (0 errors,
8 warnings). A project-local debug key was generated: its certificate SHA-256
`d90f871b40d6ffb0cec4782c4fa439b320bc323648aa5fc01d3e3e1db774d23b`
differs from the exported APK's
`07a3624e17f4e37a83b08a7fb36863bcd05911e197bdc72e01e7c4db5cb43bca`.
Do not install the new APK over the old app without the original private key;
do not uninstall or clear app data to bypass this. No phone, real SMS, or cloud
server was used. Device and HyperOS behavior remain **Needs Verification**.

An empty local `main` Git repository was initialized for workspace tooling.
There are no commits or remotes; nothing was pushed or published.

The export-time destination steps below are retained as historical instructions.
For current commands, use [Android environment](ANDROID-ENVIRONMENT.md).

## Export-time objective and owner decisions (2026-11-03; later UI decision above)

- Self-hosted management of the owner's two Xiaomi Android phones, SMS and missed
  calls. Server target: Rocky Linux 9.5. Mobile/desktop clients: Web first.
- Android: Kotlin, Compose, Room, WorkManager. Server: Go modular monolith,
  SQLite first. Web: Vue 3 + TypeScript. Native HarmonyOS and Huawei Push deferred.
- Owner has a domain, server and SSL; no connection details have been provided,
  no cloud deployment has happened. Formal UI awaits the owner's HTML prototype.
- Use Chinese for user-facing explanations and Android controls. Current Android
  APK is `0.0.2-poc`, versionCode 2, application ID `org.openalc.relay.poc`.
- Current task: migrate to a more capable computer, restore environment, then
  continue feasibility/device validation. Do not invent a new product scope.

## Implementation map

- `server/cmd/relay`: health endpoints and, after local bootstrap plus key/origin
  configuration, owner Web sessions; SQLite migrations and graceful shutdown.
- `server/cmd/relay-admin`: local key generation and one-time owner bootstrap.
- `server/cmd/poc-collector`: separate synthetic-only local test executable and
  database. Commit before per-event ACK; deduplication by installation/event ID;
  optional first-ACK drop. Optional `-web-dir` serves built Web assets.
- PoC API: POST `/poc/v1/events`, GET `/poc/v1/events?installation_id=...` with UUID
  keyset pagination, GET `/poc/v1/stats`. Literal-loopback listener, peer and Host;
  same-origin browser reads only; browser POSTs denied. Not a deployable business API.
- Android accepts only complete `OPENALC_RELAY_POC:v1:<canonical lowercase UUID>`
  fixtures, never stores original SMS bodies/senders/PDUs. Fixture ID = event ID;
  this is a test rule, not general SMS deduplication. SIM metadata is null.
- `poc-core`: actual Room schema/DAO and sync engine shared between JVM and Android.
  Disk outbox, stable installation/event IDs, pending/synced/rejected states,
  validated ACKs, bounded retries/Retry-After, loopback-only transport.
- Android adapters: protected receiver, bounded `goAsync`, WorkManager network
  constraints/backoff/repair; minimal Chinese validation controls. No formal UI.
- Web: read-only Chinese verification page, installation filter, manual refresh,
  pagination, loading/empty/error states, no browser-generated fake events/polling.

## Historical verification (2026-11-03)

- Go `test ./...`, `vet ./...`, `build ./cmd/relay`: passed. Local CGO disabled;
  Go race was NOT run. Linux race and Windows integration CI are defined, not run remotely.
- Room/JVM: 14 tests passed, actual SQLite files and multi-process reopen.
- `scripts/test-web-poc.ps1`: offline pending 100, recovered synced 100, server
  count 100, rejected 0, through deliberate ACK loss and collector restart.
- Browser: 8 Playwright tests passed in desktop and mobile-sized Chromium.
  All 100 UUIDs matched Kotlin seed output. Mobile viewport != HarmonyOS device.
- Chinese Android main/test APK assembly and lint passed: 0 errors, 8 warnings.
  Old and new APK signing certificates matched. Raw logs/DBs are not in the package.
- Owner reported installing the old app, then requested Chinese text. Do not infer
  successful SMS delivery, WorkManager execution or Chinese-version installation.
- No device has been connected by the agent, no real SMS sent, no cloud deployed.

## Destination steps

1. Read [migration checklist](MIGRATION.zh-CN.md), then validate the package with
   `scripts/verify-migration.ps1`. Do not trust old absolute paths or a still-running
   collector at `127.0.0.1:8787`; processes do not migrate.
2. Inspect local free tools; install missing Go >=1.26, Node >=22.12 (old machine
   used 26.12.1), JDK 21. Gradle 8.13 wrapper is included; do not use JDK 25 to run it.
3. For Android builds, install SDK platform 36, build-tools 35.0.0, platform-tools
   and command-line tools. Use a project-specific GRADLE_USER_HOME to avoid global
   repository injection. `scripts/use-android-env.ps1` accepts explicit paths.
4. Run checks serially, using `--max-workers=1` for APK build/lint. Verify Go first,
   Web build next, Room/JVM and complete host integration next, Android assembly last.
5. Before rebuilding for an existing installed app, resolve debug signing: the old
   private debug keystore is not in this package. The checklist has its old location
   and public certificate fingerprint. Do not overwrite a destination keystore or
   silently uninstall the phone app to bypass mismatched signing.
6. For phone work, have the owner select/authorize the test phone and wireless ADB
   pairing on a trusted network. Then test `adb reverse` while keeping the collector
   bound to loopback. Do not change it to LAN/public HTTP as a shortcut.

## Boundaries and outstanding work

Owner login exists; device authorization, encrypted business payloads, pairing,
real SMS sending, missed calls and vendor push are not implemented. No E2EE or
production-ready claim. Formal APIs need real implementation, matching OpenAPI
and scope/security tests. Unknown send outcomes must never auto-send again.

HyperOS permissions/install allowlists, actual SMS broadcasts, Android Room runtime,
WorkManager/Doze/lock/reboot/force-stop and dual-SIM behavior remain **Needs Verification**.
Record device model, OS build, target SDK, install source, permissions, network,
sample count and raw evidence location for any actual device claim.

No commits or Git remotes existed at export time. All source files were untracked;
the export includes them. The package omits `.git`; `git init -b main` on the new
machine is enough for local repository tooling. Do not commit/push/publish without
owner instruction. The project has no selected LICENSE; do not select one silently.
Optional global Codex skills/plugins are not required to understand or build this
repository; do not copy the entire `.codex` directory or login/session databases.
