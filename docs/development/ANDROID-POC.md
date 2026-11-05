# Android 合成短信接收 / 持久化 / 离线补传 PoC

2026-11-05 新机更新：用户选择 JDK 25，Gradle wrapper 升至 9.1.0。新机 SDK、宿主测试、APK 和 lint 已恢复；2026-11-03 的旧机记录保留在下文。环境启用见 [ANDROID-ENVIRONMENT.md](ANDROID-ENVIRONMENT.md)。

同日 UI 更新：主屏改为收件箱，最近 50 条只读展示实际 Room 合成事件；原保存、同步、权限和暂停操作保留。Room/JVM、debug APK 和 lint 已重新通过。所连小米手机的安装尝试被系统拒绝，真机界面尚未验证；详见 [设备 UI 记录](DEVICE-UI-2026-11-05.md)。

用户随后提供移动端截图，首屏重做为浅色留白、顶部双标签与底部悬浮栏，原合成验证操作移入菜单；界面不再提供短信权限申请按钮。手机确认 USB 安装后，更新 APK 成功安装并在该小米设备启动；空态、菜单、两条按钮保存的 Room 合成事件均有实际截图。短信权限保持未授权；构建和 lint 再次通过。见 [设备 UI 记录](DEVICE-UI-2026-11-05.md)。这些结果不覆盖短信广播、WorkManager 或第二台设备。

## 中文操作说明（0.0.2-poc）

用户反馈已安装旧版 APK，但英文操作页难以理解。新版将应用名称、按钮、权限状态、数量和同步/错误提示统一改为中文，资源集中在 `res/values/strings.xml`；协议文本和内部诊断码保持英文。

1. 点“保存这条测试数据”，保存当前编号的合成事件；这一步无需短信权限。
2. 测试多条时，先点“换一个测试编号”，再保存；同一编号重复保存不会重复计数。
3. 与电脑测试服务连接后，点“请求同步到电脑”。只安装 APK 不会自动连接云服务器。
4. 长按安装 ID 可复制到网页进行查询。

版本号升为 `0.0.2-poc`（versionCode 2），包名和数据库格式保持原样。用户安装反馈不是短信广播或后台同步已通过的证据；机型、系统 build、权限与设备测试记录仍为 Needs Verification。

新版实际验证：主 APK、测试 APK 和 lint 构建成功（3m03s；29 个任务执行、89 个已是最新）；lint 为 0 errors / 8 warnings。
使用 apksigner 验证新 APK 签名有效，证书与上一版一致；aapt2 确认包名为 `org.openalc.relay.poc`、versionCode 2、应用名称“Relay 合成验证”。
可覆盖安装的独立文件位于忽略目录 `bin/openalc-relay-0.0.2-poc-zh.apk`；构建日志为 `bin/android-tooling/android-chinese-build.log`。
本次为界面文案变更，未修改队列或同步协议，未连接手机或代用户安装。

## 2026-11-03 Android 实际构建结果

Web 联调增量：最小控件增加可选择复制的 Installation ID，宿主 harness 输出合成 ID 清单供浏览器逐条核对。
增量构建 `assembleDebug`、`assembleDebugAndroidTest`、`lintDebug` 再次成功（1m18s；14 个任务执行、104 个已是最新），lint 仍为 0 errors / 8 warnings。
新主 APK 为 16,062,829 字节；本段以下大小和 118 个任务全部执行的记录为首次构建历史。
链路范围与宿主/真机边界见 [Web PoC](WEB-POC.md)。

执行 `:app:assembleDebug :app:assembleDebugAndroidTest :app:lintDebug --max-workers=1 --no-watch-fs`，结果 **BUILD SUCCESSFUL**，118 个任务执行完成。

- 主 APK：`apps/android/app/build/outputs/apk/debug/app-debug.apk`，16,036,134 字节。
- 测试 APK：`apps/android/app/build/outputs/apk/androidTest/debug/app-debug-androidTest.apk`，359,037 字节；测试源码已编译，尚未在设备执行。
- lint：0 errors、8 warnings（依赖新版本提示 6 项、缺少 PoC 应用图标 1 项、UseKtx 建议 1 项）。未禁用 lint、未增加忽略 baseline。
- 报告：`apps/android/app/build/reports/lint-results-debug.html` / `.xml`；原始构建日志在忽略的 `bin/android-tooling/android-build.log`。
- Android Room 变体、应用 Kotlin 源码、Manifest、资源、dex 与 debug 签名打包均通过。SDK XML v4/旧 AGP 解析警告保留记录，未导致失败。
- 未连接或安装到手机，未运行 instrumentation，未发送短信；系统广播/HyperOS/Doze 等待验证项保持不变。

日期：2026-11-02（Asia/Hong_Kong）。范围为用户授权的合成数据验证，不是正式短信转发服务。

## 已实现的链路

```text
受保护的 SMS_RECEIVED → goAsync / 短时采集
                     → 严格合成 fixture allowlist
                     → Room 事务提交
                     → WorkManager 有网络时上传
                     → 本机 Go collector 事务提交 / 去重
                     → 逐条 ACK → Room 标记 synced
```

原始号码、正文和 PDU 不落盘、不上传、不记日志。只接受完整 `OPENALC_RELAY_POC:v1:<小写 UUID>`；本地注入同样只生成 fixture 元数据。这个 UUID 就是合成事件 ID，重复接收保留首次记录和时间；不能将该规则解释为任意真实短信的生产去重方案。

Android app 只保留默认 Compose 控件：权限、fixture、注入、暂停上传、请求同步、队列计数和通用状态。没有正式 UI、发送权限、短信发送、未接来电、厂商推送、配对或业务数据加密。SIM 暂为 null，等待真实系统 API/ROM 证据，不从槽位猜订阅。

`poc-core` 使用 Room Kotlin Multiplatform 的 Android/JVM 两个目标，让自动测试与手机适配器共享实际实体、DAO 和同步逻辑。JVM 使用 bundled SQLite 和真实文件，不以假队列替代持久化。Android 的 Room 适配和 WorkManager 仍需要单独验证。

## 2026-11-05 新机宿主检查

Windows 11/amd64；Go 自动下载的 1.26.0 工具链、Node 24.15.0、Microsoft JDK 25.0.5.0.2、Gradle 9.1.0、AGP 8.13.2、Kotlin 2.3.21、Room 2.8.5。迁移清单 104 个文件在修改前校验通过。Go test/vet/build、Web build、Room/JVM 14 项测试（0 failures）、三端宿主脚本（100 pending → 100 synced，collector 100，rejected 0）、重启恢复和 8 项 Chromium 测试均通过。主 APK、AndroidTest APK 与 lint 构建通过；lint 为 0 errors、8 warnings。原始合成证据在忽略的 `bin/poc-integration/fb5d2b0b4d544feba87a5dba2dfa9ea6/`。Go 本机 CGO=0，未运行 race 测试。

新机 APK 与旧包签名证书不同，不能直接覆盖已安装版本。没有连接真机、安装新 APK、发送真实短信或部署服务；HyperOS 行为仍是 **Needs Verification**。

## 2026-11-03 旧机宿主检查（历史记录）

环境：Windows/amd64；Go 1.27.3（CGO=0）；项目本地 Temurin 21.0.13+9；Gradle 8.13；Kotlin 2.3.21；Room 2.8.5。JDK 只下载/校验至忽略的 `bin/`，未改系统 Java、全局 Gradle init、注册表或证书校验。

| 检查 | 结果与证据边界 |
| --- | --- |
| Room/JVM 编译与测试 | 14 项通过、0 failures、0 errors；两份 JUnit XML 在 `poc-core/build/test-results/jvmTest/` |
| 真实 Room 文件重开 | 安装 ID、事件 ID、首次 payload、pending 和 Retry-After 截止时间保留 |
| 100 条离线事件 | 上传失败保持 100 条 pending，恢复后 100 条 synced |
| ACK | 丢失/缺少 ACK 保留未确认记录；错误安装、未知 ID、重复 ID、非法状态拒绝整个 receipt；明确 rejected 保留并停止自动重试 |
| HTTP transport | 实际本机 HTTP、429 Retry-After、拒绝重定向、限制响应大小、拒绝非 loopback URL |
| Go collector | 全套 `go test ./...` 和 `go vet ./...` 通过；覆盖 100 次并发去重、落盘重开、冲突、事务回滚和提交后断开响应 |
| Kotlin → Go 独立进程集成 | `scripts/test-android-poc.ps1` 通过：offline_pending=100、recovered_synced=100、collector_event_count=100、rejected=0；开启首次 ACK 丢失注入 |

集成证据在忽略的 `bin/poc-integration/<run-id>/result.json` 与同目录日志。每次 JVM 命令是新进程，验证干净退出/重开的恢复；不是 Android 进程强杀、掉电或系统调度测试。Go 事务故障使用受控 SQL trigger；ACK 故障在 commit 后关闭 socket。

本机初次构建受全局 init 的仓库注入、暂时性 Java TLS 握手、Gradle Windows 缓存移动问题影响。使用隔离 GRADLE_USER_HOME、限制构建并发并重试后完成宿主测试，未放宽公网 TLS。不要把下载失败当作应用测试失败或为它关闭证书验证。

## 2026-11-02 历史记录：当时的 SDK 缺失 gate（已解除）

已实际尝试 `:app:assembleDebug :app:assembleDebugAndroidTest :app:lintDebug`。Gradle 配置完成，但在任务依赖解析阶段因 **SDK location not found** 停止：本机没有已发现的 Android SDK，也未配置 ANDROID_HOME/local.properties。

截至 11 月 2 日未生成 APK、未完成 Android 编译或 lint；该构建限制已于 11 月 3 日解除，见上方新结果。instrumentation、模拟器与真机执行仍未进行。工程固定 compile/target SDK 36、min SDK 26，不表示 Android 17 OTP 行为已验证。

现在可使用项目环境脚本按 [Android 工程 README](../../apps/android/README.md)构建。CI 尚未远程运行；未部署到任何手机、未发送运营商短信、未接入真实验证码。

## 仍需设备验证（Needs Verification）

| 项目 | 操作与必须记录的证据 |
| --- | --- |
| 权限与安装来源 | RECEIVE_SMS 的安装器 allowlist、运行时授予、撤销后行为；权限拒绝不得宣称接收成功 |
| 真实系统广播 | 专用模拟器 `emu sms send` 的合成内容 → receiver → Room；再在用户明确授权的专用 SIM 场景复核。按钮注入不算这条证据 |
| Receiver 执行窗口 | 冷启动/锁屏时本地提交及 PendingResult.finish 的耗时；持久化失败/容量满显示状态，不能静默丢弃或超时保活 |
| 后台状态 | 前后台、锁屏 5/30/120 分钟、Doze/App Standby、低电量、HyperOS 默认/允许后台设置分别记录广播、落盘、上传时间 |
| 回收、重启和漏 enqueue | 本地 commit 后、WorkManager enqueue 前回收；恢复后启动扫描/系统允许的周期修复能补传。重启首次解锁前后单列 |
| 强制停止 | 与普通回收区分；需要用户重新打开的限制必须如实记录 |
| 网络 | Wi-Fi、移动网络、切换、无网、USB reverse 中断、collector 不可用；恢复后原 ID 补传，无重复记录 |
| SIM / 多段 | 当前 SIM=null；不声明双卡识别完成。PDU 数组顺序、不同格式/多段重组必须实测，普通 fixture 本身很短 |
| Android OTP 限制 | 按真实 OS/build/target/合法角色验证可见性；当前只处理无真实验证码的合成 marker，不证明 OTP 实时可读 |
| 调度与功耗 | WorkManager 配额、expedited 降级、网络约束、退避、periodic 延迟、HyperOS 进程管理以及 24h 功耗，均未实测 |

每项登记机型、完整 OS/HyperOS build、安全补丁、安装方式、target SDK、权限/角色、后台设置、SIM 别名、网络、电源、样本数、结果与脱敏证据。沿用 [PoC 计划](POC-PLAN.md) 的 Pass/Fail/Blocked/Inconclusive 模板。

## 运行与安全边界

快速宿主测试（从 `apps/android/`）：

```powershell
.\gradlew.bat -PpocJvmOnly=true :poc-core:jvmTest
```

完整本机集成（仓库根目录，JAVA_HOME 指向支持的 JDK）：

```powershell
.\scripts\test-android-poc.ps1
```

正式 Relay 的 `openapi.json` 仍只有健康检查。独立 collector 的接口在 [poc-collector.json](../../packages/openapi/poc-collector.json)，数据使用独立文件；默认只监听 127.0.0.1:8787，拒绝非 loopback 地址/对端/Host、跨来源读取及浏览器 POST；同源浏览器 GET 为 Web PoC 放行。body/phone 等字段直接拒绝且不回显。

手机只通过 `adb reverse` 访问自己的 127.0.0.1。调试网络策略仅开放该地址的 HTTP；transport 禁止代理和重定向，release 变体禁用。所有公网流量仍必须 HTTPS。这个例外仅用于受控本机合成验证，不能作为正式传输、安全配对或加密存储实现。

ACK 前从不删除队列。永久拒绝以 rejected 留存；其人工修复/重新提交和 synced 的清理暂未实现。暂停只阻止新同步，已经运行的同步最多完成 4 批/100 条；清除应用数据/卸载会销毁队列。后续若要存储真实正文或号码，必须先实现认证、授权、密钥管理和认证加密。

官方依据（核查 2026-11-02）：[SMS intents](https://developer.android.com/reference/android/provider/Telephony.Sms.Intents)、[RECEIVE_SMS 受限权限](https://developer.android.com/reference/android/Manifest.permission#RECEIVE_SMS)、[Receiver goAsync](https://developer.android.com/reference/android/content/BroadcastReceiver#goAsync())、[Room KMP](https://developer.android.com/kotlin/multiplatform/room)、[Room 测试](https://developer.android.com/training/data-storage/room/testing-db)、[WorkManager](https://developer.android.com/develop/background-work/background-tasks/persistent)、[Doze](https://developer.android.com/training/monitoring-device-state/doze-standby)、[调试网络安全配置](https://developer.android.com/privacy-and-security/security-config)。
