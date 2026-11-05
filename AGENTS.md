# OpenALC Relay — repository instructions

本文件适用于整个仓库。上级目录中针对 ThinkPHP/TP6-Vite 的项目说明不适用于 Relay；以用户明确要求和本仓库规范为准。

## 目标与阶段

- 构建开源、自托管、跨设备的个人短信与未接来电管理系统。
- 首要环境：两台小米 Android Agent、手机/桌面浏览器 Web 客户端、Rocky Linux 9.5 服务器。
- 2026-11-03 用户决定 Web 优先；2026-11-05 又明确启动 Android、HarmonyOS、Web 三端统一风格的 UI 工作，首屏为收件箱。HarmonyOS 业务接入和 Huawei Push Kit 仍暂缓；已有域名、云服务器和 SSL，具体部署信息待实际部署任务确认。
- 当前为架构与可行性验证阶段。未验证的平台行为必须标记 `Needs Verification`。
- 当前包含 Go 基础骨架、Android 合成短信 PoC、本机 Web 验证页和三端 UI 首轮；正式业务 API、真实会话数据与厂商推送尚未实现。
- Android PoC 只允许固定格式的合成 fixture，不保存原始正文/号码；独立 loopback collector 不是正式服务。
- 用户已指定参考 ChatGPT 移动端风格，首轮 UI 规范见 `docs/design/RELAY-UI.zh-CN.md`；保持 Relay 独立标识，不将合成元数据冒充真实会话。

## 技术与架构

- Server: Go，标准库 `net/http`、`log/slog`；SQLite 首发，PostgreSQL 以后通过存储适配层支持。
- Android: Kotlin、Compose、Room、WorkManager。HarmonyOS: ArkTS、ArkUI、原生 Push Kit。
- Web: Vue 3、TypeScript。API: REST、OpenAPI。部署: Linux binary、Docker Compose。
- 保持模块化单体，不引入 Redis、消息中间件、微服务或 Kubernetes 作为默认依赖。
- 通信遵循 Offline-first + Event-driven + Push-assisted + Adaptive Transport。
- HTTPS 是公网唯一传输方式；WebSocket 只能是可选短时优化，不作为正确性基础。
- 事件先本地落盘，服务端事务提交后 ACK；推送接受、设备收到、命令执行是不同事实。
- 命令结果不确定时禁止自动再次发送短信；不能承诺跨系统的 exactly-once。

## 目录与边界

- `server/`: 独立 Go module，入口 `cmd/relay`，实现放 `internal/`。
- `server/migrations/`: 嵌入二进制的有序 SQLite forward migrations。
- `apps/android/`: debug PoC，Room 队列/同步在 JVM 实盘验证；`apps/web/` 是 Vue/TypeScript 本机合成数据验证页；`apps/harmony/` 是 API 26 ArkTS/ArkUI Stage 界面工程，手机和平板模拟器已验证，HarmonyOS 真机仍待验证。
- `packages/openapi/`: 已实现 API 的事实契约；未来接口先在 RFC 中设计。
- `packages/protocol/`: 与语言无关的同步和命令状态约定。
- `deploy/{docker,systemd}/`: 最小部署示例，生产 TLS 接入见部署文档。
- `docs/architecture/`, `development/`, `deployment/`, `decisions/`: 设计、验证、运维与决策记录。

## 代码与依赖

- 代码、类型、变量、API、提交信息使用英文；与用户交流使用中文。
- Go 使用 gofmt、明确错误处理、context 超时、参数化 SQL；handler 不放领域业务。
- 优先标准库；新增依赖解释目的、版本与许可证。固定版本，提交 go.sum。
- 不预建无实现的抽象层；边界接口在实际使用位置定义。
- 不用空函数、随机成功、内存假队列冒充完成的持久化或厂商集成。
- SQL 迁移发布后不可修改；新增版本向前迁移，破坏性变更先备份并征求确认。

## 安全不变量

- 设备与用户身份分离；所有资源查询强制 owner/device scope，撤销即时生效于服务器。
- 配对短时、单次、用户明确批准；密钥、凭证不得写入 Git、日志、URL 或推送正文。
- 短信正文、手机号、命令负载、推送 token 在业务持久化前必须实现认证加密。
- 当前骨架没有认证和负载加密，不得接受真实通信数据或声明生产可用/E2EE。
- 远程发送必须同时具有鉴权、显式用户操作、幂等键、限速与审计；不得提供无人确认的批量发送。
- 日志只记受控元数据；不要记录请求体、查询串、完整密钥、验证码或号码。
- `.env`、数据库、备份、设备抓取和 PoC 原始证据不入库；使用合成数据。

## 验证与文档

- 在 `server/` 执行 `go test ./...`、`go vet ./...`、`go build ./cmd/relay`。
- CI 在 Linux 额外运行 race 测试；本机 CGO 不可用时说明限制，不冒称已通过。
- Android 宿主测试：在 `apps/android/` 用 JDK 25 和 Gradle 9.1.0 wrapper 运行 `./gradlew -PpocJvmOnly=true :poc-core:jvmTest`；有 SDK 再 assemble/lint，不能用 JVM 结果代替真机结论。
- PowerShell 集成验证：`./scripts/test-android-poc.ps1`；只用合成数据和本机端口，不访问手机。
- 三端宿主集成：`./scripts/test-web-poc.ps1`，包含浏览器读取与重启恢复；JVM 与浏览器测试不能代替 Android 真机结论。
- 迁移测试覆盖重复启动、失败原子性、历史修改检测和未知版本拒绝。
- 业务开发时必须覆盖越权、去重、重复回调、租约竞态、崩溃窗口和不确定结果。
- OpenAPI 只列真实实现的 endpoint；更改行为时同步契约和对应测试。
- 实机验证记录机型、系统 build、target SDK、权限、安装方式、网络、样本数、原始证据位置。
- README 以英文为主，中文入口在 `README.zh-CN.md`；平台证据注明来源和核查日期。
- RFC 提案、代码已实现、官方文档陈述、实测通过必须分开描述。

## Codex 工作方式

1. 先查看仓库与相关文档，简述修改范围，再实现并运行相关检查。
2. 低风险、范围明确的工作直接推进，不反复确认。
3. 重大架构偏离、数据破坏、外部付费、Git 远程推送、安全敏感操作先说明并征求确认。
4. 用户明确授权优先；不要将既有授权重复变成确认流程。
5. 不擅自提交、推送、发布、发真实短信或注册付费服务。
6. 汇报实际变更、验证结果、未解决事项；不能以文档或测试替代真实设备验证。
