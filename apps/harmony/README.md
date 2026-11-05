# HarmonyOS synthetic exchange client

This is a native ArkTS/ArkUI Stage project targeting HarmonyOS API 26. Its
inbox-first screen follows the shared [Relay UI design](../../docs/design/RELAY-UI.zh-CN.md):
a near-white empty canvas, segmented inbox/device navigation, and a bottom dock.
The content width is capped on tablets. All visible copy remains Chinese.

The app now pairs with the local Relay server and exchanges fixed synthetic
metadata. It keeps a pending event ID and ping acknowledgement ID in ArkData
preferences, and keeps its device credential only in process memory. It cannot
recover that credential after process death; pairing again is required. It has
no SMS or call access, real message data, production HTTPS transport, or Huawei
Push Kit integration. The generated launcher icon is still a placeholder.

## Build

Open this directory as a project in DevEco Studio with the API 26 SDK, or use
the official `@deveco/deveco-cli@1.3.0-stable` from this directory:

```powershell
devecocli build
```

The local CLI used for verification is installed under Git-ignored
`bin/harmony-tooling` at the repository root. The build creates
`entry/build/default/outputs/default/entry-default-unsigned.hap`. No signing
profile or credentials are checked in. The project has no runtime package
dependencies. The generated Stage template uses the DevEco/Hvigor toolchain
provided by the installed IDE.

On 2026-11-05, the HAP built and launched on HarmonyOS 7.0.0 phone and tablet
**emulators** (API 26). The complete local synthetic exchange passed on both.
Install the unsigned HAP on a running emulator, then run
`./scripts/test-harmony-emulator.ps1` from the repository root. The script
starts an isolated local server, creates a pairing, and uses `hdc rport` to
connect the emulator's loopback port to the host. It never sends SMS or a real
message. See the [exchange record](../../docs/development/HARMONY-EXCHANGE-2026-11-05.md)
and earlier [UI record](../../docs/development/HARMONY-UI-2026-11-05.md).
Physical HarmonyOS device installation and behavior remain **Needs Verification**.
