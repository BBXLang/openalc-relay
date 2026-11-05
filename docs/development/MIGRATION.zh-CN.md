# 换电脑迁移清单

整理日期：2026-11-05。面向 Windows 新电脑；解压目录可以改变，不必沿用旧电脑用户名或盘符。

**接手后更新（2026-11-05）：**新机已按用户后续决定改用 JDK 25 和 Gradle 9.1.0，完成构建与宿主验证。下列 JDK 21 / Gradle 8.13 命令是迁移 ZIP 导出时的历史清单；当前环境与签名状态见 [Android 环境记录](ANDROID-ENVIRONMENT.md) 和 [接手状态](CODEX-HANDOFF.md)。

## 最快接手方式

1. 把本轮生成的迁移 ZIP 和旁边的 `.sha256` 文件传到新电脑。
2. 解压到例如 `C:\dev\openalc-relay` 的父目录；ZIP 自带一层 `openalc-relay/`。
3. 在新电脑的 Codex 中添加/打开**解压后的项目文件夹**，将它作为当前工作目录。
4. 在新对话粘贴本文最后的提示词。新 Codex 先读仓库里的 `AGENTS.md` 和接手文档，就能恢复项目状态。

同账号登录不等于项目文件已经迁移。官方说明将“对话同步”和“项目文件访问”分开；同步对话不会把整个本地目录复制到另一台电脑。
因此以实际解压文件和仓库接手文档为准，不依赖旧聊天是否出现。[OpenAI 官方说明](https://learn.chatgpt.com/docs/projects#use-local-projects-for-folders-and-codebases)

## 包内与包外

| 内容 | 处理方式 |
| --- | --- |
| 全部非忽略源码、文档、CI、迁移 SQL、OpenAPI、测试 | 包内；包括当前尚未提交到 Git 的文件 |
| Gradle wrapper JAR/脚本、Go sums、npm lockfile | 包内，保持固定版本 |
| 中文 APK `0.0.2-poc` | 完整包放在 `bin/openalc-relay-0.0.2-poc-zh.apk`，方便直接安装 |
| 每个文件的 SHA-256/大小清单 | `MIGRATION-MANIFEST.json`；另有 ZIP 的外部 `.sha256` |
| Git 历史 | 当前没有任何提交或 remote；不打包 `.git`，新机可重新初始化 |
| SDK/JDK、Go/Node、Gradle/npm 缓存、node_modules、build/dist | 不打包；新机复用已有安装或重新下载 |
| `.env`、真实数据、数据库、日志、原始测试证据、浏览器/Codex登录态、ADB密钥 | 不打包 |
| Android 调试签名私钥 | 不打包；见下面的单独迁移事项 |

目前所有功能源码都在包内；旧的合成数据可用脚本重新生成，原始数据库不影响开发接手。
旧电脑的 `127.0.0.1:8787` 进程、内存状态和浏览器已输入的 ID 不会随 ZIP 转移。

## 签名：继续覆盖手机旧应用前处理

你已经安装过 Android PoC。要让新电脑构建的 APK 继续覆盖它，需使用同一个调试签名：

- 旧电脑文件：`%USERPROFILE%\.android\debug.keystore`（文件存在，未打入源码包）。
- 旧签名证书公开 SHA-256：`07a3624e17f4e37a83b08a7fb36863bcd05911e197bdc72e01e7c4db5cb43bca`。
- 如果要迁移，请将这个文件**单独通过你信任的私有方式**传到新电脑；不要提交 Git 或放到公开网盘。
- 新电脑常见位置为 `%USERPROFILE%\.android\debug.keystore`。如果该路径已有文件，先保留原文件，不要让 Codex 直接覆盖；可由新 Codex 为本项目单独配置导入的签名。
- 迁移 ZIP 中已构建好的中文 APK 自带旧签名，直接使用它不需要重新签名。以后新构建能否覆盖，应由 apksigner 核对证书。
- 没迁移旧签名时，不要通过卸载旧应用来“解决”签名问题；卸载会丢失手机里的本地测试队列和安装 ID。

## 新机环境

| 工具 | 项目要求 / 旧机验证版本 |
| --- | --- |
| PowerShell | 推荐 7；旧机 7.6.5 |
| Git | 本机源码管理；不需要先注册或创建远程仓库 |
| Go | >=1.26；旧机 1.27.3 windows/amd64 |
| Node/npm | Node >=22.12；旧机 26.12.1 / npm 11.20.0 |
| Java | JDK 21 推荐；Gradle 8.13 使用 JDK 17–23，不使用 JDK 25 |
| Gradle | 包内 wrapper 8.13 自动下载；不必另装系统 Gradle |
| Android SDK | platform 36、build-tools 35.0.0、platform-tools、command-line tools |
| Android Studio/模拟器 | 本项目构建不强制依赖；内存紧张时优先真机，不安装模拟器也可完成宿主测试 |

首先从仓库根目录运行只读检查：

```powershell
./scripts/verify-migration.ps1
./scripts/inspect-dev-env.ps1
git init -b main
```

`verify-migration.ps1` 只校验迁移包解压文件；之后修改过源码再跑会显示差异，这是预期的。
`git init` 不创建提交、不推送。首次按需安装工具可交给新电脑的 Codex。
SDK/JDK 可以放在项目内 `bin/android-tooling/`，也可复用新机已有路径：

```powershell
. ./scripts/use-android-env.ps1 -JavaHome 'C:/path/to/jdk-21' -SdkRoot 'C:/path/to/android-sdk'
```

该脚本只设置当前 PowerShell 进程，不修改注册表/系统 PATH。隔离的 Gradle home 能避免用户全局 init 改写仓库；不要迁移旧电脑的全局 init。

## 按顺序验收，不并行跑大任务

先在 `server/` 执行：

```powershell
go test ./...
go vet ./...
go build ./cmd/relay
```

然后在仓库根目录执行：

```powershell
npm ci --prefix apps/web
npm --prefix apps/web run build
. ./scripts/use-android-env.ps1 -JavaHome 'C:/path/to/jdk-21' -SdkRoot 'C:/path/to/android-sdk'
./scripts/test-web-poc.ps1
```

完整宿主脚本会运行 Room/JVM 测试，再验证 100 条离线/ACK丢失/重启/网页读取；它会下载 Playwright Chromium，但不接触手机。
如果 SDK 还没安装，可先设置 JDK 21 的 `JAVA_HOME` 和项目级 `GRADLE_USER_HOME`，直接跑宿主脚本；它不要求 SDK。

最后在 `apps/android/` 构建：

```powershell
./gradlew.bat :app:assembleDebug :app:assembleDebugAndroidTest :app:lintDebug --max-workers=1 --no-watch-fs --console=plain
```

旧机 Go、14 项 JVM、8 项浏览器测试和 APK/lint 均通过；这些是历史结果，详见 [接手状态](CODEX-HANDOFF.md)。新机必须记录自己的结果。
Go race 只有具备 CGO/C 编译器时单独执行；不能把配置了 CI 当作运行通过。

## 不占 USB 的无线调试

Android 11 及以上官方支持通过无线调试配对，不要求先接 USB；电脑和手机需在同一网络。
HyperOS 的入口、网络限制和 `adb reverse` 的实际效果仍是 **Needs Verification**。
[官方步骤](https://developer.android.com/studio/run/device#wireless) / [adb 配对说明](https://developer.android.com/tools/adb#connect-to-a-device-over-wi-fi)。

1. 手机开启开发者选项里的“无线调试”，选择“使用配对码配对设备”。
2. 在新机用 platform-tools 的 adb 执行下列命令。端口从手机界面读取，不要照抄占位符；配对端口与连接端口可能不同。

```text
adb pair <手机IP>:<配对端口>
adb connect <手机IP>:<连接端口>
adb devices -l
adb -s <指定设备序列号> reverse tcp:8787 tcp:8787
```

配对码在 adb 提示时输入，不写入命令、Git 或聊天。新电脑应重新配对，不复制旧电脑的 adb 私钥。
需要分别选择两台小米设备，避免把安装/调试操作发到错误手机。
上述隧道保留应用到 `127.0.0.1:8787` 的本机传输约束；不要把 collector 改成 `0.0.0.0` 或暴露到局域网来替代它。
无线调试不能解决旧电脑内存不足，因此换到性能更好的电脑仍有帮助。

## 给新电脑 Codex 的提示词

在**解压后的项目文件夹**中新建本地对话，粘贴：

```text
请接手这个 OpenALC Relay 项目。我已从旧电脑迁移过来，先读取仓库根目录 AGENTS.md、docs/development/CODEX-HANDOFF.md 和 docs/development/MIGRATION.zh-CN.md。

先运行 scripts/verify-migration.ps1 校验包，运行 scripts/inspect-dev-env.ps1 检查新机环境。请自行准备缺失的免费开发工具与项目依赖，优先使用现有安装和项目级配置，不改全局仓库配置、不关闭 TLS 校验；涉及付费或破坏现有数据时先说明。

按文档依次完成 Go 检查、Web 构建、Kotlin/Room→Go→网页合成数据宿主验证以及 Android APK/lint。内存相关任务顺序执行。继续使用 JDK 21 和固定的 Gradle 8.13，保留中文操作文案。

我已经安装过 Android PoC，重新构建并覆盖安装前先处理旧调试签名，不能静默卸载应用或覆盖新机已有签名文件。手机调试优先使用无线 ADB，连接哪一台手机由我确认。只使用合成数据；真机操作/结果与宿主测试必须分开记录。

先恢复到与旧机同等验证状态，再说明剩余真机验证步骤。暂不开发正式 UI，不接入真实短信，不部署云服务器，不提交或推送 Git。不要把旧机测试记录当作新机或手机已通过。
```

长期频繁换机时，再考虑由你创建私有 Git 远程进行同步；这次无需先处理 Git 账号或发布仓库。源码与手工接手说明都能独立使用，不必搬运 Codex 全局配置和会话数据库。
