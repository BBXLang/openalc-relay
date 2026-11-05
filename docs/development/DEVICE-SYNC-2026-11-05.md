# Xiaomi synthetic sync to Web — 2026-11-05

This record extends the [UI-only device check](DEVICE-UI-2026-11-05.md). It verifies a local synthetic-data path on the connected Xiaomi. It is not an SMS broadcast, background-delivery, cloud, or production-security result.

| Item | Observation |
| --- | --- |
| Device | Xiaomi `25113PN0EC` (`pudding`), Android 17 / API 37, HyperOS `OS4.0.5.0.XPCCNXM` |
| App | `org.openalc.relay.poc`, target SDK 36, debug APK installed with `adb install -r` over the existing app; Room data preserved |
| Permission | `RECEIVE_SMS` remained denied; no SMS was sent or received for this test |
| Transport | USB `adb reverse tcp:8787 tcp:8787` to a Go collector bound to host `127.0.0.1:8787`; the phone also had a connected mobile network. No cloud endpoint or public listener was used |
| Samples | Two button-created `injected` fixtures retained from the earlier UI check; no new or real messages |
| Fault | Collector started with `-drop-first-ack`; it committed 2 records while the phone Room snapshot still showed 2 pending records. The two event ID sets matched |
| Recovery | After the owner unlocked the phone and opened Relay, the new explicit foreground sync action changed Room to 0 pending / 2 synced / 0 rejected. The collector still held 2 records, and IDs matched. A repeated sync left the collector count at 2 |
| Browser | Collector restarted without fault injection using the same SQLite database and serving the built Web assets. Chromium at 390×844 and 1280×900 each displayed 2/2 rows from that installation |

The Android change makes the explicit sync button call `SyncEngine.synchronize()` immediately from an IO coroutine. Failed attempts retain the Room queue and schedule WorkManager retry; background capture and repair still use WorkManager. The drawer was given a bounded width and inspected after its open animation completed. JDK 25 / Gradle 9.1.0 `:app:assembleDebug :app:lintDebug` and `:poc-core:jvmTest`, plus Web `npm run build`, passed.

While the phone was locked, `dumpsys jobscheduler` listed Relay work as frozen. The initial collector commit happened, but a timely background retry was not established in that state. Unlocking and explicit foreground sync proved recovery; it does **not** prove HyperOS background scheduling, process reclaim, reboot recovery, or reliable delivery without user action. Those remain **Needs Verification**. The second Xiaomi and HarmonyOS physical devices were not tested.

Raw local evidence is Git-ignored under `bin/device-evidence/2026-11-05-sync/`: phone Room snapshot files, collector SQLite, a settled drawer screenshot, and Web mobile/desktop screenshots. The recorded repository document contains counts and environment facts only, not installation or event IDs. No real communication data, Git commit, or push was involved.
