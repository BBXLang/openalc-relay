# 本机 Android 开发环境

新机配置日期：2026-11-05。工具放在 Git 忽略的 `bin/android-tooling/`，不修改注册表、系统 PATH 或机器级 `JAVA_HOME`。

## JDK 选择

用户选择 JDK 25。新机下载 Microsoft OpenJDK 25.0.5.0.2 到 `bin/android-tooling/jdk-25.0.5.0.2+1`，SHA-256 为 `5da7525c8268a10a66bd76a39f4e9af24d4ebc974d42800a609852c498a68460`。使用本机 MotrixNext 的 aria2 多线程下载，并用 Git 的 CA bundle 保持 TLS 证书校验。[微软下载页](https://learn.microsoft.com/en-us/java/openjdk/download)
迁移清单原先要求的 JDK 21 文件仍在忽略目录中，当前环境脚本不会选用它；本次自动审批拒绝清理该目录。

Gradle wrapper 已从 8.13 升到 9.1.0，distribution SHA-256 为 `a17ddd85a26b6a7f5ddb71ff8b05fc5104c0202c6e64782429790c933686c806`；wrapper JAR 的官方 SHA-256 为 `76805e32c009c0cf0dd5d206bddc9fb22ea42e84db904b764f3047de095493f3`。现有 Android Gradle Plugin 8.13.2、Kotlin 2.3.21 在本机完成 JVM 测试、APK 及 lint。Gradle 9.1 是首个正式支持以 JDK 25 运行的版本。[兼容表](https://docs.gradle.org/current/userguide/compatibility.html) / [Gradle 校验值](https://gradle.org/release-checksums/)

JDK 用于运行 Gradle，许可为 [GPLv2 + Classpath Exception](https://learn.microsoft.com/java/openjdk/faq)；Gradle 用于 Android 构建，许可为 [Apache 2.0](https://docs.gradle.org/current/userguide/licenses.html)。没有新增应用运行时依赖。

## SDK

- 路径：`bin/android-tooling/sdk`。
- 已安装：`platforms;android-36`、`build-tools;35.0.0`、`platform-tools`。
- 官方 Windows command-line tools 包：15859902；下载后已验证 SHA-256：`90ae805d20434428bffcb699c290860f19bb5f66a67e6b330067e3de801fb04a`。[官方来源](https://developer.android.com/studio/)
- 新机已接受 SDK 许可并安装所需组件；常规构建不重新安装 SDK。
- 当前 cmdline-tools 与 AGP 8.13.2 组合会提示 SDK XML v4 警告；APK 和 lint 实际通过。

## 每个终端启用

在仓库根目录的 PowerShell 中：

```powershell
. ./scripts/use-android-env.ps1
Set-Location apps/android
./gradlew.bat :app:assembleDebug :app:assembleDebugAndroidTest :app:lintDebug --max-workers=1 --console=plain
```

脚本设置当前 PowerShell 进程的 `JAVA_HOME`、`ANDROID_HOME`、`GRADLE_USER_HOME` 和 `ANDROID_USER_HOME`。默认选项目内 JDK 25；可传 `-JavaHome` 或 `-SdkRoot`。Gradle 缓存和新调试签名留在项目忽略目录，不使用全局 Gradle init。

新机生成的 APK 签名 SHA-256 为 `d90f871b40d6ffb0cec4782c4fa439b320bc323648aa5fc01d3e3e1db774d23b`；迁移包旧 APK 为 `07a3624e17f4e37a83b08a7fb36863bcd05911e197bdc72e01e7c4db5cb43bca`。二者不同，新 APK 不能直接覆盖已安装的旧应用。旧私钥未迁移；若另一台手机仍安装旧应用，卸载前必须单独确认其本地队列的数据处置。

2026-11-05 用户决定不再使用旧调试签名。当前通过 USB 连接的手机未安装 `org.openalc.relay.poc`，但首轮新 APK 安装被 HyperOS 拒绝（`INSTALL_FAILED_USER_RESTRICTED`）。其他手机若仍安装旧签名版本，不能静默卸载或清除其本地队列；需分别核对。见 [设备 UI 记录](DEVICE-UI-2026-11-05.md)。

只构建，不自动部署到手机，也不发送短信。APK 构建、lint、instrumentation 构建与设备执行结果应分别登记在 [Android PoC 记录](ANDROID-POC.md)，不能互相替代。
