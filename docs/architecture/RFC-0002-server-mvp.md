# RFC-0002：Relay 服务端与 Web 管理面板首版

- 日期：2026-11-05（Asia/Singapore）
- 状态：**Proposed**；实施进度另见[服务端阶段记录](../development/SERVER-IMPLEMENTATION-2026-11-05.md)，不能把提案当作已实现 API 或通过的测试
- 依据：[系统架构 RFC](RFC-0001-system-architecture.md)、[Web 优先决策](../decisions/0003-web-first.md)、[小米合成同步实测](../development/DEVICE-SYNC-2026-11-05.md)

下文“当前起点”和“当前事实契约”保留提案撰写时的基线。后续已实现配对、设备凭证与固定合成元数据双向接口，实际范围以[实施记录](../development/SERVER-IMPLEMENTATION-2026-11-05.md)及 [OpenAPI](../../packages/openapi/openapi.json) 为准；加密通信事件和管理面板仍待实施。

## 1. 当前起点与首版目标

`server/cmd/relay` 当前只提供 `/healthz`、`/readyz` 和 SQLite 迁移账本。
`server/cmd/poc-collector` 是独立的本机合成数据接收器，不能直接改成公网业务服务。
`apps/web` 当前按安装 ID 读取该接收器；安装 ID 不是认证。Android 真机只验证了
两条按钮生成的合成事件经 USB 回环隧道进入该接收器并出现在浏览器。当前没有
服务器部署、账号、配对、加密业务负载或正式会话数据。现有 Android/Web PoC
客户端也不能直接连接未来的认证业务 API，必须分别增加正式协议适配。

首版交付一个可在单台 Rocky Linux 主机自托管的**单人 Relay 服务**，使已认证的
Web 用户管理自己的设备，并查看已配对 Android Agent 上传的事件。先以合成事件
完成整个安全链路及持续运行验证。真实正文、号码、未接来电与远程短信发送的接入
分别设置后续门槛；首版不能因界面存在而自动开放它们。

### 首版使用路径

1. 管理员在服务器本地完成一次性初始化，注册可恢复的登录方式。
2. 用户通过同一 HTTPS origin 打开 Web，登录后发起短时设备配对，并明确批准。
3. Android Agent 使用自己的设备凭证上传本地已提交事件；服务器事务提交后逐条 ACK。
4. Web 仅按登录用户的 owner 范围列出设备、事件、同步状态和最近错误。
5. 用户撤销设备后，后续上传与读取立即失效；离线手机上已有数据不能承诺远程擦除。

### 首版不包含

远程发送短信、批量发送、真实短信/号码入站、未接来电、厂商推送、HarmonyOS 业务
客户端、在线状态保证、秒级同步 SLA、多用户 SaaS、Redis 或消息中间件。相关
设计留在 RFC-0001；没有实现时不写入事实 OpenAPI。

## 2. 模块、存储与边界

保持现有 Go `net/http` + `log/slog` 模块化单体与单进程 SQLite。只有正式业务功能
真正落地时才增设 `identity`、`devices`、`events`、`sync`、`audit` 的代码包；接口在
使用方定义。业务 handler 处理认证入口、输入和响应，事务与资源归属由业务层和
存储查询共同强制。PoC collector 的数据库、路由、无认证协议与正式服务分离。

首批 forward migration 只为实际实现的垂直切片建表：`owners`、登录凭证/会话、
`devices`、设备凭证、`pairings`、`events`、`changes`、`audit_entries`。每张资源表
带 `owner_id`，设备查询同时绑定 `device_id`。事件唯一键为
`(device_id, installation_id, client_event_id)`；服务端用带密钥摘要区分重复和
同 ID 不同负载冲突，不能对敏感正文使用普通散列。迁移一经发布只新增版本，不能
修改旧 SQL。

敏感正文、号码、设备命令负载和未来推送 token 在业务持久化前使用认证加密；
envelope 包含 `key_id`、版本、随机 nonce 与密文/tag，AAD 绑定 owner、device、
record ID 和类型。主密钥与 SQLite 及备份分开保存，支持读取旧 `key_id` 以便轮换。
本设计是服务器可解密的静态加密，**不是 E2EE**。首版先通过合成负载验证加密、
重启、轮换、错误密钥拒绝和备份恢复，再考虑真实通信数据。

## 3. 身份、会话与配对

- **Web 用户**：单 owner 首发，但用户身份与设备身份分离。第一次管理员创建只在
  服务器本地受控操作中进行，不开放公网“注册首个管理员”。先评估成熟的
  WebAuthn/passkey 实现与恢复方案；若选择密码备用，使用成熟 Argon2id 实现并
  记录依赖版本、许可证和参数。具体库与恢复流程在实施切片前确定。
- **Web 会话**：随机不透明会话，服务端存可撤销验证材料与过期时间；浏览器使用
  `Secure`、`HttpOnly`、`SameSite` cookie。所有状态变更校验 Origin 与 CSRF；同源
  静态资源和 API，不开放宽松 CORS。退出、过期、撤销均在服务端生效。
- **Agent 凭证**：每个安装独立的高熵凭证，存摘要并限制 audience/scope；从凭证
  推导 owner/device，绝不信任请求体中的设备归属。服务端逐请求检查撤销状态；
  Android 端凭证放 Keystore，不入日志、URL 或 Git。
- **配对**：登录用户创建短时、单次配对会话；新设备提交受限申请，Web 显示待配
  对设备与核对码，用户明确批准后才发可用设备凭证。短码不单独兑换长期凭证；
  失败次数受限。QR/链接不携带密钥或凭证。

## 4. 第一批业务接口：提案，不是 OpenAPI 事实契约

| 接口组 | 调用者 | 首版行为 |
| --- | --- | --- |
| 登录、会话查询与退出 | Web 用户 | 建立和撤销 owner 会话；具体认证路由随所选登录方式确定 |
| `POST /v1/pairings`、配对申请/批准/交换 | Web 用户 / 新设备 | 短时单次、显式批准、有限尝试；批准前无 Agent 权限 |
| `GET /v1/devices`、`POST /v1/devices/{id}/revoke` | Web 用户 | owner 范围设备列表与即时撤销；`last_seen` 标明观测时间 |
| `PUT /v1/agent/status` | Agent | 仅报告自身能力、权限、软件版本、最近同步；不声称设备实时在线 |
| `POST /v1/agent/events` | Agent | 有界批次、设备归属、加密入库、原子去重与逐条提交后 ACK |
| `GET /v1/events` | Web 用户 | owner 范围、稳定游标分页、受控元数据与授权解密 |
| `GET /v1/sync` | Web 用户客户端 | owner 绑定游标、变更与墓碑；仅在多端增量确有使用时实现 |

第一批只实现用到的路由，完成一个切片就同步
[`packages/openapi/openapi.json`](../../packages/openapi/openapi.json) 与契约测试。
当前事实契约仍只有健康检查。请求体和批次大小有界；错误使用受控 problem JSON，
日志不含请求体、查询串、凭证、号码或正文。

## 5. 事务、同步与 Web 管理面板

Agent 本地先持久化，再上传同一 `client_event_id`。服务器在**同一事务**内校验
设备有效性、写加密事件、去重、分配 owner 范围变更序列与写审计；提交成功后
才 ACK。ACK 丢失时同 ID 重试返回 duplicate；同 ID 异内容返回明确冲突并保留
可诊断元数据。拒绝、429 和临时 5xx 各有不同重试语义，不能丢弃本地未确认事件。

Android 在此阶段增加独立于 PoC loopback transport 的 HTTPS 业务适配：设备凭证
放 Keystore，校验服务器证书，复用本地事件 ID，逐条应用 ACK，并在 401/撤销时
停止无意义重试。先只上传合成事件；不能把现有无加密的原始短信接入业务服务。

Web 管理面板沿用现有中文视觉规范，首版只做登录、收件箱、设备列表/状态、配对、
撤销与同步错误提示。收件箱逐行标注事件来源、设备、时间和真实同步状态；合成
数据显式标记，不能伪装成短信会话。安装 ID 不再充当访问凭证。分页和筛选由
服务端按 owner scope 执行。浏览器不直接持有 Agent 凭证。

## 6. 实施顺序与验收门槛

| 阶段 | 交付 | 必须通过的验收 |
| --- | --- | --- |
| S0 安全起点 | 威胁模型、管理员初始化与登录方式选型、密钥配置约定 | 无初始化凭证/密钥时业务路由不启动；敏感值不进 Git/日志 |
| S1 身份与设备 | 真实会话、配对、设备凭证、撤销和 owner scope | 未登录、越权、错 audience、过期/重复配对、撤销后请求全部被拒 |
| S2 加密事件入站 | 业务表、envelope、Agent HTTPS 适配、上传与 ACK、审计 | 加密前不落敏感负载；重复/冲突/ACK 丢失/事务失败/并发重试均正确 |
| S3 Web 管理面板 | 登录、设备与事件页面、配对/撤销、分页与错误态 | 真正使用 S1/S2 API；越权与 CSRF 测试；桌面/手机浏览器复验 |
| S4 运维与部署 | Rocky Linux 单实例、HTTPS 反向代理、备份/恢复、指标 | 只暴露 HTTPS；一致性备份与密钥分离；恢复后游标失效；重启及迁移失败安全 |
| S5 持续实测 | 两台 Android 的合成数据长时间运行 | 记录样本数、入站延迟、待同步峰值、重复、失败、锁屏/断网/重启矩阵；不以宿主测试替代实机 |

每阶段应有 Go 单元/集成与契约测试。服务端执行 `go test ./...`、`go vet ./...`、
`go build ./cmd/relay`；Linux CI 跑 race。S2 必测磁盘失败不 ACK、重复回调和
事务边界；S3 必测越权与 CSRF；S4 必测恢复旧备份后不静默继续旧游标。

S5 可以先跑**合成事件**的连续观察，再讨论真实短信试验。要从合成数据升级到
真实数据，另需 Android 端安全存储、权限/平台验证、最小数据保留策略，以及
用户对测试样本和设备的明确授权；不能因为服务器完成而自动开启。

## 7. 后续阶段

远程发送命令必须单独实施：显式单条用户操作、设备/SIM 选择、幂等键、原子限速、
审计、领取/start 租约与不确定结果状态机；在可能已调用系统发送的窗口绝不自动
重发。未接来电、厂商唤醒、HarmonyOS Push Kit 和正式 HarmonyOS 数据客户端
分别作为独立能力验证，不阻塞首版只读管理面板。

## 8. 部署输入与参考

真正部署时再核对域名、反向代理、证书、Rocky 主机访问、存储位置、备份目标、
密钥保存和恢复责任；这些信息不能由现有“已有服务器和 SSL”推断。部署前所有
真实数据入口保持关闭；不把独立 PoC collector 暴露到公网。

设计依据：本仓库 RFC-0001。外部参考（核查于 2026-11-05）：
[OWASP 会话管理](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet)、
[OWASP CSRF 防护](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)、
[SQLite 在线备份](https://sqlite.org/backup.html)、
[Go AEAD 文档](https://pkg.go.dev/crypto/cipher)、
[W3C WebAuthn](https://www.w3.org/TR/webauthn-3/)。
