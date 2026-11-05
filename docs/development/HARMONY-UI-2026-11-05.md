# HarmonyOS UI build and emulator evidence — 2026-11-05

## Scope and environment

- Host: destination Windows workstation. This is new-machine evidence, not a replay of old-machine results.
- IDE: DevEco Studio `26.0.0.865` at `D:\ide\Huawei\DevEco Studio` (local `product-info.json`). Its default HarmonyOS SDK is API 26, package `26.0.0.105 Release`.
- User-provided phone/tablet system images: `D:\ide\Huawei\HarmonyOS SDK\system-image\HarmonyOS-7.0.0`. The IDE emulator manager reports both images downloaded.
- CLI: official MIT-licensed `@deveco/deveco-cli@1.3.0-stable`, installed only in ignored repository-local `bin/harmony-tooling` for this verification. Project build uses the IDE-provided Hvigor toolchain.
- Stage project: `apps/harmony`; bundle `org.openalc.relay.harmony`; target and compatible SDK `26.0.0`; device types `phone`, `tablet`. No runtime package dependencies or checked-in signing credentials.

## Performed checks

1. Generated an API 26 Stage project using `devecocli create`; retained the pre-existing `entry/src/main/ets/pages/Index.ets` UI page while adding project configuration and resources.
2. Ran `devecocli build` in `apps/harmony`. ArkTS compilation and HAP packaging passed. Artifact: `entry/build/default/outputs/default/entry-default-unsigned.hap` (146,420 bytes after the final tablet-width change). The CLI warns that no signing configuration is present.
3. Created and started a phone emulator. `devecocli run --device 127.0.0.1:5555` built, installed and launched the HAP. Its software version reported `emulator 7.0.0.107(SP8DEVC00E999R4P11)`. Visually checked the empty inbox, menu, and device tab; captured a phone screenshot.
4. Created and started a tablet emulator. The same `devecocli run` command built, installed and launched the HAP. Its software version reported `emulator 7.0.0.107(SP8DEVC00E16R1P1)`. The first layout stretched across the full tablet width; added centered maximum widths for the top navigation, content area, and dock; rebuilt, relaunched and captured the corrected tablet screenshot. The final screenshot shows no horizontal overflow.
5. Ran `devecocli check lint`. It exited successfully but reported `Files checked: 0`; this is **not** counted as a lint pass on ArkTS source.

Raw local screenshots are excluded from Git under `bin/harmony-evidence/2026-11-05/relay-phone.jpeg` and `relay-tablet.jpeg`. The phone and tablet emulator runs are separate checks; neither is physical-device evidence.

## Boundaries and remaining verification

The UI is a shell with no service endpoint, account/session, SMS or call permissions, storage, Huawei Push Kit, or real data. The bottom dock states `尚未连接服务`; its yellow control has no sending behavior. The emulator accepted the unsigned HAP, but physical HarmonyOS installation, signing, performance, accessibility, keyboard/insets, and system behavior are **Needs Verification**. No real HarmonyOS phone or tablet was attached for this check. No Git commit, push, publication, or real communication occurred.
