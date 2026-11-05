# Android synthetic SMS PoC

The debug screen uses Chinese labels, instructions, permission states and
human-readable sync/error messages as of version `0.0.2-poc` (version code 2).
The 2026-11-05 UI correction follows the owner's screenshot: a quiet inbox,
segmented header, and floating bottom bar. The most recent 50 persisted
synthetic events appear as simple rows; test controls are in the drawer. See the
[shared UI guide](../../docs/design/RELAY-UI.zh-CN.md).
The explicit sync button now runs a foreground attempt immediately; failed
attempts retain the Room queue and enqueue WorkManager retry. Background repair
continues to use WorkManager and remains subject to device scheduling limits.
Protocol fixture strings and internal diagnostic codes remain unchanged. Upgrade
the existing app only with an APK signed by the same certificate; clearing app
data is not required. The current workstation's new debug key differs from the
exported APK. The owner abandoned the old signature; the connected Xiaomi had
no old Relay package, and the new APK installed successfully on it.

Local environment update (2026-11-05): SDK 36/build-tools 35.0.0/platform-tools are
installed under ignored `bin/android-tooling/sdk`. The pinned Gradle 9.1.0 runs on
project-local JDK 25. In PowerShell,
dot-source `../../scripts/use-android-env.ps1` from this directory before building.
No registry or machine-wide JAVA_HOME changes are needed.

This debug-only prototype receives **strictly allowlisted synthetic SMS**, persists
fixture metadata in Room, and uploads it through WorkManager to a separate local
Go collector. There is no production UI, user/device authentication, SMS sending,
missed-call support, vendor push, or general-purpose SMS forwarding.

Only this complete body is accepted (lowercase canonical UUID, no trailing text):

```text
OPENALC_RELAY_POC:v1:00000000-0000-4000-8000-000000000002
```

The original body, sender number and raw PDU are discarded. Only fixture/event ID,
installation ID, source and capture time are persisted/uploaded. The Android
receiver deliberately records subscription as **unknown/null**: cross-ROM SIM
mapping is not verified. A unique fixture UUID represents one test input;
replaying it is deduplicated locally. This is not a production algorithm for
deduplicating arbitrary SMS.

## Modules and correctness

- `app`: minimal Compose controls, protected `SMS_RECEIVED` receiver, bounded
  `goAsync` capture, WorkManager network constraints/backoff and repair scan.
- `poc-core`: actual Room entities/DAO, strict fixture parser, bounded batches,
  typed protocol, ACK validation and loopback transport. Kotlin Multiplatform is
  used only to run the **same Room schema/DAO on Android and JVM**; native
  HarmonyOS remains separate.
- `server/cmd/poc-collector`: separate local executable/database, transactional
  persistence and installation-scoped deduplication; not the production server.

Capture commits before upload is enqueued. Failed requests keep the same IDs.
Only `persisted`/`duplicate` ACKs mark records synced. Missing/malformed ACKs leave
records pending; explicit rejections are retained and not automatically retried.
`Retry-After` deadlines are stored in Room and survive restart. The collector
commits before returning ACK.

WorkManager uses connected-network constraints and exponential retry starting at
30 seconds. API 31+ expedited work can fall back to ordinary work; older Android
uses ordinary work without a foreground service. A 15-minute **minimum** periodic
repair scan covers a crash between commit and enqueue; Android may defer it.
There is no fixed polling SLA or permanent socket. Pending capacity is 10,000;
full/storage failures produce a visible capture failure. Synced/rejected synthetic
records are retained for this short experiment; purge is not implemented.
Uninstalling or clearing app data destroys the local queue.

## Host tests without Android SDK

Requires JDK 25 for the pinned Gradle 9.1.0, Go 1.26+, and access
to Google Maven/Maven Central. The wrapper pins the distribution SHA-256.

```sh
cd apps/android
bash ./gradlew -PpocJvmOnly=true :poc-core:jvmTest
```

PowerShell: use `.\gradlew.bat`. If a global Gradle init script injects incompatible
repositories, use an isolated `GRADLE_USER_HOME`; do not edit global settings or
disable TLS certificate verification.

JVM tests use bundled SQLite with real Room disk files, not a fake queue. They
cover 100-fixture offline recovery, database reopen, lost/partial/invalid ACKs,
rejections, Retry-After, strict filtering, HTTP parsing, redirects and size limits.
They cannot prove Android broadcast delivery or OEM background scheduling.

From the repository root, PowerShell runs the multi-process Kotlin-to-Go test:

```powershell
./scripts/test-android-poc.ps1
```

It runs JVM tests, seeds 100 records, tries upload while the collector is offline,
starts the collector with an ACK-drop fault, retries the same disk queue and
verifies exactly 100 committed events. Data/logs stay in ignored
`bin/poc-integration/`. No phone or carrier SMS is used.

The [Web PoC](../web/README.md) adds a same-origin read-only verification page.
The Android debug screen now shows a selectable Installation ID to paste there.
Run `scripts/test-web-poc.ps1` from the repository root for Kotlin → Go → real
browser verification, including offline/ACK-loss recovery and collector restart.

## Android debug build

For a new checkout, install SDK platform 36 and build-tools 35.0.0, accepting the
required SDK terms during authorized setup. Set `ANDROID_HOME` or untracked
`local.properties` (`sdk.dir=...`). On this workstation the owner-authorized local
setup has already installed these packages; build commands do not reinstall them.

```sh
bash ./gradlew :app:assembleDebug :app:assembleDebugAndroidTest :app:lintDebug
```

Output: `app/build/outputs/apk/debug/app-debug.apk`. No release variant is enabled.
Use `:app:connectedDebugAndroidTest` for the device database test on an explicitly
selected emulator/test phone; obtain device-owner authorization before deployment.

## Manual emulator / dedicated test phone

1. Host, from `server/`: `go run ./cmd/poc-collector` (port 8787), optionally with
   `-drop-first-ack` to recreate commit followed by response loss.
2. Set `adb -s <serial> reverse tcp:8787 tcp:8787`. Both sides use loopback. Debug
   plaintext traffic is permitted only to `127.0.0.1`; redirects are blocked.
3. Open the app: queue counts, status, pause, fixture ID, injection, permission
   request and sync request are the only controls. Injection tests persistence
   and sync, **not** SMS broadcast delivery.
4. For actual system delivery, explicitly grant RECEIVE_SMS and use the dedicated
   emulator: `adb -s <serial> emu sms send 15555215554 "OPENALC_RELAY_POC:v1:<new-uuid>"`.
   This is an emulator command, not a carrier send. Do not pretend `am broadcast`
   exercises the protected system SMS path.
5. Stop the collector/remove the tunnel, inject fixtures, verify pending counts
   after reopening, restore connectivity and request sync. Compare the collector's
   `GET http://127.0.0.1:8787/poc/v1/stats`.
6. Follow [the device checklist](../../docs/development/ANDROID-POC.md) for reclaim,
   locks, Doze, permissions and restart. Restore test settings afterward.

A USB tunnel does not satisfy WorkManager's connected-network requirement by
itself. Connected Wi-Fi also does not prove collector reachability. Neither case
may delete pending data. Pause prevents new sync runs; an already running sync
(at most four batches / 100 records) may finish.

This synthetic-metadata harness has no encryption or pairing. Do not generalize
it to arbitrary SMS or expose it publicly before implementing production security.
See the separate [PoC OpenAPI](../../packages/openapi/poc-collector.json).
