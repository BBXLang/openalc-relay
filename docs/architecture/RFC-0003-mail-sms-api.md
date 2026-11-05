# RFC-0003：短信与邮件正式业务接口

- 日期：2026-11-06（Asia/Singapore）
- 状态：**Proposed**；本文件是提案，不是事实契约。只有实现切片落地并附契约测试后，才把对应 endpoint 写入 [OpenAPI](../../packages/openapi/openapi.json)。批次 2 交付的存储层、迁移与加密绑定是数据基础，本身不构成任何可用 API。
- 依据：[服务端 RFC-0002](RFC-0002-server-mvp.md)（§2 存储与加密、§4 接口流程、§6 阶段门槛、§7 远程发送）、[正式 UI 与短信、邮件实施计划](../development/FORMAL-UI-MAIL-PLAN.zh-CN.md) §3–§4、[系统架构 RFC-0001](RFC-0001-system-architecture.md)

## 1. 范围与前提

本 RFC 定义短信与邮件两个正式业务频道的资源边界、状态机与失败语义。实施顺序上
它服务于 FORMAL-UI-MAIL-PLAN 批次 2（正式数据基础）与批次 3（真实链路接入）：
批次 2 落地迁移、认证加密、owner/device/account 范围、分页、搜索、附件存储、
发送状态与审计，并全部以合成数据测试；批次 3 才接入真实 IMAP/SMTP 与 Android
正式 Agent。**批次 2 未通过验收前，任何真实数据入口保持关闭**，UI 上的发送控件
必须处于禁用并说明原因的状态，不得伪成功。

本 RFC 不覆盖：真实服务商凭证的申请、真实短信发送授权、厂商推送、HarmonyOS
正式数据客户端、多用户化。这些留在后续提案与验收中。

## 2. 资源组与操作

所有资源表带 `owner_id`；设备资源同时绑定 `device_id`，邮箱资源同时绑定
`account_id`。范围检查由存储层 SQL 与业务层共同强制，分页与搜索参数不得绕过
归属过滤。浏览器与 HarmonyOS 客户端使用 owner 会话；Android Agent 使用独立设备
凭证，从凭证推导 owner/device，绝不信任请求体中的归属声明。状态变更沿用
Origin/CSRF 校验（见 RFC-0002 §3）。

| 资源组 | 操作（提案） | 关键约束 |
| --- | --- | --- |
| 设备与 SIM | `GET /v1/devices`、`GET /v1/devices/{id}`、`GET /v1/devices/{id}/subscriptions`、`PUT /v1/devices/{id}/subscriptions/{sub}/label`、`POST /v1/devices/{id}/revoke` | owner scope；可用订阅与用户确认的号码标签来自设备上报，标签未确认时显示「号码未确认」，不推测；权限与撤销状态逐请求重查，撤销即时生效；设备凭证不能读取其他设备 |
| 短信 | `GET /v1/sms-messages`、`GET /v1/sms-messages/{id}`、`GET /v1/sms/search`、`POST /v1/sms-send-commands`、`GET /v1/sms-send-commands/{id}`；Agent 侧：领取、系统发送回调、结果上报 | 分页列表与详情限 owner scope，可按 device 过滤；搜索限当前频道与已选来源；发送命令必须显式单条确认、绑定设备＋SIM、携带幂等键、受限速与审计 |
| 邮箱账号 | `POST /v1/mail-accounts`、`GET /v1/mail-accounts`、`GET /v1/mail-accounts/{id}`、`POST /v1/mail-accounts/{id}/test`、`POST /v1/mail-accounts/{id}/revoke`、`DELETE /v1/mail-accounts/{id}` | 凭证只在创建/替换时提交，之后任何响应不回显；撤销停止同步；删除级联清理账号数据；连接测试结果只回状态与受控错误码，不回原始协议日志 |
| 邮件 | `GET /v1/mail-messages`（可按 account/folder 过滤）、`GET /v1/mail-messages/{id}`、`GET /v1/mail-messages/{id}/attachments`、`GET /v1/mail-attachments/{id}/content`、`GET /v1/mail/search`、`POST /v1/mail-send-intents`、`GET /v1/mail-send-intents/{id}` | account scope；附件按需下载、限制大小；发送先持久化意图与稳定 Message-ID；收件箱分页采用稳定游标 |

具体的路径、方法、请求/响应 schema 在实现切片完成时一次性确定并同步事实
OpenAPI，不能由三端各自发明协议。列表统一使用稳定游标分页（见 §7）。

## 3. 短信发送状态机

发送命令状态：`queued → claimed → system_sent → delivery_receipt / failed /
unknown`。命令入队、推送接受、设备领取、系统发送回调与运营商送达回执是不同
事实，页面逐状态区分，不能把「已入队」显示成「已发送」。

| 起始状态 | 允许的迁移 | 触发者 | 记录时间戳 |
| --- | --- | --- | --- |
| `queued` | `claimed` | 设备领取 | `claimed_at_ms` |
| `claimed` | `system_sent` | 设备上报已调用系统发送 | `system_sent_at_ms` |
| `claimed` | `failed` | 设备明确报告调用前失败 | `resolved_at_ms` |
| `claimed` | `unknown` | 设备无法确认是否已调用发送 | `resolved_at_ms` |
| `system_sent` | `delivery_receipt` | 设备转发运营商回执 | `resolved_at_ms` |
| `system_sent` | `failed` | 系统/运营商明确失败 | `resolved_at_ms` |
| `system_sent` | `unknown` | 结果长时间无法确认（人工/系统标记，绝不自动重发） | `resolved_at_ms` |

不变量：

- **I1（绝不自动重发）**：命令一旦可能已调用系统发送（状态到达 `claimed` 之后
  的任何状态，含 `unknown`），系统不得自动重新入队或新建发送；幂等键重试只返回
  既有命令与 `duplicate` 语义。`unknown` 是终态，不接受任何出边。
- **I2（幂等）**：`(owner_id, idempotency_key)` 唯一。同键同负载重试返回既有命令
  与 `duplicate`；同键不同负载返回冲突，不覆盖。
- **I3（设备范围）**：领取、上报只能由命令所属 `device_id` 的凭证执行；其他设备
  一律 `not found` 语义。
- **I4（限速与审计）**：创建命令受每 owner 排队上限与请求限速约束；创建与每次
  迁移在同一事务写审计，审计只含受控元数据。

## 4. 邮件发送与结果未知

- 提交发送前，服务端先在同一事务持久化发送意图（完整收件方、主题、正文、附件
  引用）与**稳定 Message-ID**；持久化成功才允许连接 SMTP Submission。
- SMTP 会话中断时区分两种结果：服务端确认未被服务商接受 → `failed`；可能已被
  接受但无法确认 → `unknown`。**结果未知时绝不自动重新提交**；用户重试须显式
  操作且产生新的用户确认，系统仍按幂等键返回既有意图状态。
- 页面区分「上传中、服务器已接收、邮件服务商已接受、失败、结果未知」；只有
  服务商确认接受才可显示已发送。
- 发出的邮件写入本系统已发送记录；不导入邮箱原有已发送文件夹（计划 §1）。

## 5. 邮箱账号与连接安全

- 用户手填 IMAP/SMTP 主机、端口、TLS 模式（`starttls` / `implicit_tls`）、用户名
  与应用密码/授权码；凭证以密钥环认证加密保存（AAD 绑定 owner＋account），只在
  设置时提交，可替换、可撤销，永不回显。
- 连接前限制端口与目标地址：仅允许明确的 submission/IMAPS 端口集合；**私网与
  环回目标地址仅在服务器管理员显式允许列表内放行**，默认拒绝。
- 强制校验服务器证书与主机名，不允许跳过验证的选项；协议行为依据 IMAP
 （RFC 9051）、邮件提交（RFC 6409）、邮件 TLS（RFC 8314）。
- 撤销账号立即停止同步与发送；同步采用有界轮询、失败退避，最近成功同步时间
  持久化可见，不承诺即时到达。

## 6. 搜索语义

- 搜索限定当前频道（短信/邮件）、当前 owner 范围与已选来源（设备或账号）。
- 授权后对限定范围的密文记录逐条解密匹配：短信匹配发件方、收件方、正文；邮件
  另匹配主题。**不建立明文内容索引**，任何全文索引（含 SQLite FTS）不得保存
  解密结果。
- 搜索词不进入日志、URL 查询串的敏感诊断记录或推送正文；扫描行数有上界，
  结果按与列表一致的稳定顺序返回。
- owner 范围之外的记录对搜索不可见（SQL 过滤先于解密）。

## 7. 分页与游标

- 列表使用稳定 keyset 游标：`(received_at_ms, id)` 降序，游标为不透明编码值；
  服务端返回 `next_cursor`，为空表示结束。
- 分页期间新记录到达不得改变旧游标的后续结果；旧记录清理可能导致游标失效，
  客户端收到失效错误后应重置到首页（恢复旧备份后同理，见 RFC-0002 S4）。
- 每页大小有界（建议默认 20、上限 100）；分页与筛选参数在服务端按 scope 执行。

## 8. 失败语义、去重与审计

所有入口统一失败语义：

| 情形 | 行为 |
| --- | --- |
| 密钥缺失 / 密钥环不可用 | 拒绝写入与读取，返回受控错误；**绝不返回成功或 ACK** |
| 解密失败（密文篡改、AAD 不匹配、错误密钥） | 该记录读取失败并按错误上报；绝不把零值/空内容当成功返回 |
| 数据库写失败（含迁移失败、事务提交失败） | 操作失败，事务回滚，不留部分写入；入站不 ACK |
| 重复拉取 / ACK 丢失重试 | 去重键命中且负载一致 → 返回 `duplicate`，不重复入库 |
| 去重键同键不同负载 | 返回明确冲突，保留既有记录与可诊断元数据 |

去重键：

- 正式 Agent 事件：`(device_id, client_event_id)`（多安装扩展为
  `(device_id, installation_id, client_event_id)`，随正式协议确定，见 RFC-0002 §2）。
- 短信记录：`(device_id, remote_event_id)`。
- 邮件记录：`(account_id, folder, remote_message_id)`。

重复/冲突判定不得使用无密钥散列；以 AAD 解密后比较负载（本批存储层实现）。
审计条目记录 actor（owner 会话/设备/系统）、动作、主体与受控元数据，与业务写
入同一事务提交；禁止任何凭证、消息正文、号码进入审计、日志或错误响应。

## 9. 实施顺序建议与 OpenAPI 事实边界

建议切片顺序（每片须过 `go test ./...`、`go vet ./...`、契约测试）：

1. 存储层与迁移（批次 2，已交付部分）：AAD 域工具、mailstore/smsstore/audit、
   篡改/错误密钥/回滚/重启/去重/游标/状态机测试。
2. 邮箱账号管理 API：创建（凭证只写不回显）、列表、详情、撤销、删除＋连接测试
   占位（明确「未接通」而非成功）→ 实现后写入 OpenAPI。
3. 邮件入库与收件箱：受控 IMAP 适配、增量同步、附件按需、分页与详情。
4. 短信入站与发送状态机：正式 Agent 入站、设备领取、单条发送、结果上报。
5. 搜索与三端接入；每片先 RFC 修订再实现，OpenAPI 只随已实现 endpoint 更新。

OpenAPI 事实边界：`packages/openapi/openapi.json` 只描述已实现并通过契约测试的
endpoint；本 RFC 的其余部分（含状态机与失败语义的文本描述）不属于事实契约。
批次 2 的存储层包（mailstore/smsstore/audit）是内部实现，不出现在 API 表面。
截至本文撰写，切片 2 的最小子集已随批次 2 落地并通过契约测试：`POST/GET
/v1/mail-accounts` 与 `DELETE /v1/mail-accounts/{account_id}`（凭证只写不回显、
连接测试与任何邮件/短信数据交换仍不可用，真实数据入口保持关闭）。

## 10. 验收对照（批次 2）

| 验收项 | 证据位置 |
| --- | --- |
| 迁移（重复启动、失败原子性、历史保护） | `internal/storage/sqlite` 测试＋集成级重启测试（mailstore/smsstore） |
| 认证加密与 AAD 绑定 | `internal/secrets/aad_test.go`、两个 store 的篡改/错误密钥测试 |
| owner/device/account 范围 | 两个 store 的跨 owner/跨设备/跨账号拒绝测试 |
| 分页稳定性 | 两 store 的游标稳定性测试 |
| 搜索 | 两 store 的范围外不可见与命中/未中测试 |
| 附件存储 | mailstore 附件按需落库与状态一致性测试 |
| 发送状态 | smsstore 状态机非法跃迁与 unknown 不变量测试 |
| 审计 | audit 包测试＋同事务写入与回滚测试 |
