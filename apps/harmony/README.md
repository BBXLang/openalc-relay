# HarmonyOS UI shell

This is a native ArkTS/ArkUI Stage project targeting HarmonyOS API 26. Its
inbox-first screen follows the shared [Relay UI design](../../docs/design/RELAY-UI.zh-CN.md):
a near-white empty canvas, segmented inbox/device navigation, and a bottom dock.
The content width is capped on tablets. All visible copy remains Chinese.

The app is a UI shell. It has no account, service connection, SMS or call access,
pairing, local event store, or Huawei Push Kit integration. The yellow dock
control is deliberately inactive; the dock says `尚未连接服务`. The generated
launcher icon is still a placeholder.

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
**emulators** (API 26). The phone menu and device tab were exercised; screenshots
were inspected on both form factors. See the
[verification record](../../docs/development/HARMONY-UI-2026-11-05.md).
Physical HarmonyOS device installation and behavior remain **Needs Verification**.
