# OpenALC Relay

面向用户自有多部 Android 手机与 SIM 卡的自托管通信管理系统。2026-11-03 用户决定业务接入先使用 Web 客户端；2026-11-05 又启动 Android、HarmonyOS、Web 三端统一界面工作。HarmonyOS 业务接入与 Push Kit 仍暂缓。已有域名、云服务器与 SSL。详见 [Web 优先决策](docs/decisions/0003-web-first.md)和[三端界面规范](docs/design/RELAY-UI.zh-CN.md)。

[English README](README.md) · [架构 RFC](docs/architecture/RFC-0001-system-architecture.md) · [PoC 计划](docs/development/POC-PLAN.md)

[服务端与 Web 管理面板首版计划](docs/architecture/RFC-0002-server-mvp.md)明确身份、配对、加密事件同步、管理页面、部署与持续实测的实施顺序。当前已实现本地管理员初始化和 Web 会话接口，其余业务接口仍是提案。实施细节见[服务端阶段记录](docs/development/SERVER-IMPLEMENTATION-2026-11-05.md)。

换电脑请先看[迁移清单与新 Codex 提示词](docs/development/MIGRATION.zh-CN.md)和[接手状态](docs/development/CODEX-HANDOFF.md)。迁移包包含尚未提交的源码及文件校验清单，不包含凭证和缓存。

当前包含 Go 基础骨架、一次性本地管理员初始化与 Web 会话接口、Android 合成短信接收/Room 持久化/WorkManager 补传 PoC、本机 Web 验证页与三端收件箱界面首轮。PoC 只处理固定 fixture 的元数据，不保存或上传原始正文和号码，使用独立本机测试接收器。HarmonyOS ArkUI Stage 工程已在手机和平板模拟器构建运行，尚无业务数据接入。设备配对、加密业务事件存储、正式短信收发、未接来电与厂商推送尚未实现，不能接入真实通信数据或作为正式产品部署。

使用 Go 1.26+，在 `server/` 下运行：

```sh
go run ./cmd/relay
```

另一个终端访问 `http://127.0.0.1:8080/healthz` 或 `/readyz`。PowerShell 可用 `Invoke-RestMethod`。默认只监听本机，数据写入当前工作目录的 `data/relay.db`；Go 程序不会自动加载 `.env`，需自行设置环境变量。

架构遵循 Offline-first + Event-driven + Push-assisted + Adaptive Transport。推送不等于命令执行成功，发送不确定时禁止自动重发，不默认维持 WebSocket 或高频心跳。

小米透传停服、Android OTP 可见性、权限、HyperOS 后台行为、HarmonyOS 7 原生通知接入与地区/账号条件均需按[平台验证清单](docs/development/PLATFORM-VERIFICATION.md)核对。已有一台小米的界面与合成 Room 数据实机检查，见[设备记录](docs/development/DEVICE-UI-2026-11-05.md)；不能据此推断真实短信、后台同步或第二台小米。HarmonyOS 仅有模拟器结果，见[验证记录](docs/development/HARMONY-UI-2026-11-05.md)。

构建、测试、目录说明见英文 README；本地部署见[基础部署说明](docs/deployment/BOOTSTRAP.md)。业务功能仍需按 PoC 与安全门槛逐步验证。

本轮实际检查与尚未验证的环境见[初始化验证记录](docs/development/VERIFICATION.md)。

后续 Android PoC 的运行方式、自动测试与待真机项目见 [Android PoC 记录](docs/development/ANDROID-POC.md)和[工程说明](apps/android/README.md)。

已增加 Vue 3/TypeScript 本机只读验证页，按安装 ID 读取 collector 实际落盘的合成事件，支持分页和手动刷新。运行与自动化复验见 [Web 工程说明](apps/web/README.md)及 [Web PoC 记录](docs/development/WEB-POC.md)。宿主链路测试不代替 Android 真机测试，未接入域名或云服务器。

已在一台小米上验证两条按钮生成的合成事件经 USB 回环隧道上传到本机 collector，并在手机/桌面浏览器收件箱读取；首次 ACK 丢失后手动同步恢复且服务端没有重复记录。详见[真机合成同步记录](docs/development/DEVICE-SYNC-2026-11-05.md)。后台调度、真实短信和第二台小米仍待验证。

本项目采用 [MIT 许可证](LICENSE)，版权归 2026 BBXLang；依赖仍遵循各自许可证，版本与许可说明见[依赖记录](docs/development/DEPENDENCIES.md)。
