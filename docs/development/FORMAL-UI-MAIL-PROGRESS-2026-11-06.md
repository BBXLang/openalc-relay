# Relay 正式 UI 与短信/邮件 — 夜间执行台账（2026-11-06 00:00–08:30）

- 规格来源：[FORMAL-UI-MAIL-PLAN.zh-CN.md](./FORMAL-UI-MAIL-PLAN.zh-CN.md)（状态：待实施 → 本轮实施）
- 执行分支：`codex/server-mvp`（仅本地提交，不推送远程）
- 硬约束：最多 2 个 SubAgent 并行；阻塞项跳过并记 `Needs Verification`；07:30 冻结新功能；08:30 停止并总结。
- 本文件是执行过程的唯一事实台账；若会话中断，以本文件为准恢复/总结。

## 基线（Wave 0，00:06–00:12）

| 检查项 | 结果 |
| --- | --- |
| 本机时间 | 2026-11-06 00:06（周四午夜） |
| Go / Node / JDK | go 1.25.0 windows/amd64；node v24.15.0；JDK 25 于 `bin/android-tooling/jdk-25.0.5.0.2+1` |
| 分支状态 | `codex/server-mvp`，唯一未跟踪文件 = 本计划文档自身（随首个 docs 提交入库） |
| `server/ go test ./...` | 全部 ok（httpapi/identity/secrets/storage/synthetic 等 10 包） |
| `go vet ./...` / `go build ./cmd/relay` / gofmt | 通过 / 通过 / 干净 |
| `apps/web npm run build` | ✓ built in 122ms |

## 波次状态

| 波次 | 内容 | 状态 | 提交 |
| --- | --- | --- | --- |
| Wave 0 | 基线验证 + 本台账 | ✅ 完成 | — |
| Wave 1 | Web 正式 UI（SubAgent A）+ Android 正式 UI（SubAgent B） | ✅ 完成（01:50 收口） | 3 笔本地提交 |
| Wave 2 | HarmonyOS 正式 UI（SubAgent C）+ 服务端数据基础（SubAgent D） | ✅ 完成（03:15 收口） | 2 笔本地提交 |
| Wave 3 | 邮件适配包骨架（SubAgent E）+ 共享设计规范更新 + 截图证据 | ✅ 完成（04:05 收口） | 2 笔本地提交 |
| Wave 4 | 只读端点切片（提前完成）→ 冻结与交接证据终稿（07:30 起） | 🔄 进行中 | 1 笔提交 +终稿待提交 |

## 已完成明细

### Wave 4 前置增量（04:30 收口）— 批次 3：owner 会话只读端点切片

**HTTP 读面（SubAgent F）**：`server/internal/httpapi/mailbox.go` 6 个只读端点——`GET /v1/mail/accounts/{id}/messages`（游标分页、列表仅元数据+≤120 字符摘要）、`GET /v1/mail/messages/{id}`（text 默认；`sanitized_html` 默认只回说明字段，`?format=html` 显式 opt-in 带未净化警示）、`GET /v1/mail/search`（解密匹配主题/发件人/正文，owner+可选账号范围，q 缺失 400）、`GET /v1/sms/messages` / `{sms_id}` / `search`（同语义）。OpenAPI 0.3.0→0.4.0（+6 path/5 schema/8 parameter）与 contract_test 同变更同步。
**安全相关改动（已由主控审查 diff）**：`session.go` 的 `RawQuery` 全局拒绝改为精确路径/紧前缀白名单豁免（仅 6 个新 GET 端点），不波及 `/v1/mail-accounts` 写面；`Cache-Control: no-store` 在豁免判定前统一设置；新增回归测试锁定既有端点仍拒绝一切 query。
**mailstore 最小扩展**：`SearchMessages` 增游标支持与发件人匹配（旧测试调用点机械更新）。设备列表复用既有 synthetic surface（无重复实现）。
验证（含独立复核）：`go test -count=1` httpapi 13.3s + mailstore 3.6s ok、全量 15 包 ok、vet/build/gofmt 干净、OpenAPI JSON 可解析。7 组新测试：匿名 401/跨 owner 404/游标稳定/篡改密文 500 无部分数据/format=html 显式 opt-in/搜索 scope/query 豁免回归。日志红线：httpapi 包零日志调用（grep 证明），q/正文/凭证无进日志路径。

### Wave 1（01:50 收口）— 批次 1：Web + Android 正式 UI

**Android（SubAgent B）**：20 新文件 + MainActivity/strings 修改，`RelayInboxScreen.kt` 删除（功能并入 `SyntheticVerificationScreen`）。9 个导航目的地与规格 §2.1 导航表逐项对照通过；亮/暗令牌按规格落值（`theme/RelayTheme.kt`）；自管返回栈+BackHandler（零新依赖）；频道滑移 250ms 且 ANIMATOR_DURATION_SCALE=0 时禁用；PoC 入口移至 配置→开发→合成验证；app_name 改 `Relay`；文案集中 strings.xml（约 90 条）。
验证：`:poc-core:jvmTest` BUILD SUCCESSFUL 1m17s；`:app:compileDebugKotlin` BUILD SUCCESSFUL 2m16s（3 轮迭代修复 15 个编译错误）；`:app:lintDebug` BUILD SUCCESSFUL 11m18s。

**Web（SubAgent A）**：6 修改 + 24 新增 + 删 `src/api.ts`（并入 SyntheticVerification.vue）。`vue-router@4.6.4` 精确锁定；hash 路由（`/#/sms` 等，因 poc-collector 静态服务无 SPA fallback，见偏离记录）；`tokens.css` 亮/暗令牌按规格；侧栏文字 logo（0.14em / 45%）；StateBlock 六态；ChannelPager 250ms 双窗格滑移+reduced-motion；ComposeBar 发送恒禁用+原因可见；`sanitizeHtml.ts` 白名单净化（注释标注真实数据前须换经审计库）；PoC 整体迁 `/dev/synthetic`，根路径不再命中 collector；正式导航无 PoC 入口（e2e 断言）。
验证：`npm run build` ✓ 747ms（vue-tsc 0 错误）；`npx playwright test` **26/26 通过**（44.3s，desktop+mobile 双视口；chain.spec 8 条跑在真实 Go poc-collector + 100 条播种 fixture 链路上；formal-nav 9 条×2）。

**偏离记录**：1) Web 用 hash 而非 history 路由（collector 无 SPA fallback，保证既有 e2e 链路可跑）；2) Android 频道滑移用 `Animatable(tween(250ms))`（支持拖拽跟手+分段指示同步）；3) 演示卡片不可点入正式详情（防演示数据混淆）；4) 侧栏「连接测试/账号配置」子项留待批次 2/3 接口，未造假。

### Wave 2（03:15 收口）— 批次 1 收官（HarmonyOS）+ 批次 2（服务端）

**HarmonyOS（SubAgent C）**：16 新文件 + 5 修改；`Index.ets` 406 行单页 → `Navigation`+NavPathStack 宿主（9 屏：1 主内容双窗格 + 7 NavDestination + 合成验证），系统返回逐级回退；暗色令牌入 `base/dark` color.json；preferences 持久化主题覆盖；侧栏文字 logo 符合 0.14em/45% 规格；PoC 合成链路逻辑**逐行原样**搬入 `SyntheticPage.ets`（`LOCAL_SERVER`/队列/UUID 不动），入口仅 配置→开发；app label `Relay`。
验证：基线构建 BUILD SUCCESSFUL 4m53s；最终构建 BUILD SUCCESSFUL 1m52s，产物 `entry-default-unsigned.hap`（592,243 字节）。**模拟器目视验证通过**（API 26 / HarmonyOS 7.0.0）：分段滑移、搜索页频道参数、抽屉 logo/组、主题覆盖重启保持、合成验证页演示卡片；修复返回箭头反向缺陷。编译修复 3 轮（SemiBold→Medium、accessibilityText ResourceStr 收窄、pathInfo param 收窄）；剩 19 条 deprecation 警告。

**服务端（SubAgent D）**：`docs/architecture/RFC-0003-mail-sms-api.md`（Proposed，四资源组/短信发送状态机不变量 I1–I4/邮件意图与结果未知/连接安全/搜索语义/失败语义表/去重键）；迁移 000005–000010（mail_accounts、mail_messages、mail_attachments、sms_messages、sms_send_commands、audit_entries；全表 owner scope + device/account 绑定 + UNIQUE 去重键 + 游标索引 + AAD 域注释）；新包 `secrets/aad`（域分隔）、`randid`、`cursor`（keyset）、`audit`、`mailstore`、`smsstore`（状态机+每 owner 排队上限 10）；HTTP 最小切片 `POST/GET/DELETE /v1/mail-accounts`（凭证只写不回显），同一变更内 OpenAPI 0.2.0→0.3.0 + contract_test 同步。修复真实死锁（ListAccounts rows 遍历内嵌套查询）。`sqlite_test.go` 账本计数 4→10（跟随迁移集合的必要伴随改动）。go.mod/go.sum 零新依赖。
验证（含独立复核）：`go test ./...` 14 包 ok；`go vet` ✓；`go build ./cmd/relay ./cmd/relay-admin` ✓；gofmt 干净；OpenAPI 16 paths 校验通过。测试覆盖：AAD 域分离固定向量、密文篡改/错钥失败、去重同键同内容/不同内容冲突、游标稳定、重启存活、RAISE(ABORT) 回滚无部分写入、状态机非法跃迁全拒、unknown 终态不重发、范围外不可见、HTTP 匿名 401/CSRF 403/凭证不回显。

### Wave 3（04:05 收口）— 批次 3 脚手架 + 共享规范 + 截图证据

**邮件适配包（SubAgent E）**：`server/internal/mailadapter/`（policy/imap/smtp/mime + 4 测试文件）。依赖固定：go-imap/v2 v2.0.0-beta.8、go-message v0.18.2、go-smtp v0.21.3（均 MIT，govulncheck 零命中）；go.sum 仅图闭包新增。EndpointPolicy：端口白名单（25 需显式开启）、RFC1123 主机名校验、私网/环回默认拒绝+精确端点 allowlist 钩子、拨号 Control 复检防 DNS 漂移、TLS 1.2+ 永不跳过验证。`Submit` 结果未知语义：`ErrSubmitUnknown`（句点后应答丢失，上层禁止重交）/`ErrSubmitRejected`。隔离性：cmd/httpapi 对 mailadapter 与 emersion 零引用（grep 证明）。
验证：`go test -count=1 ./...` 25 新测试全过（进程内受控 IMAP/SMTP 端到端、双 TLS 模式、私网 12 组拒绝、真实断连注入、畸形 MIME 降级不 panic）；vet/build/gofmt 干净。环境注记：本沙箱 DNS 通配劫持，`TestPolicyUnresolvableHost` SKIP（localhost 环回拒绝路径已覆盖）。

**共享设计规范**：`docs/design/RELAY-UI.zh-CN.md` 升级 v2（设计令牌双主题表、侧栏文字 logo 规格、页面导航表、Web/Android/HarmonyOS 导航实现约定、卡片字段、撰写与发送禁用语义、频道切换 250ms、合成验证边界、验收要点）；v1 全文保留为历史章节。

**截图证据**（git 忽略目录 `bin/ui-evidence/2026-11-06/`）：Web 12 张（桌面 1280×900 + 移动 390×844；短信/邮件列表、搜索、设置、邮箱账号、详情不可达态、合成验证；亮+暗）——已目检：亮暗令牌正确、空态「正式同步未接通」与发送禁用原因「正式链路未接通；未配对设备」清晰。HarmonyOS 模拟器截图 10+ 张归档至同目录 `harmony/`（首页/抽屉/暗色/搜索/邮件页）。Android 无模拟器截图（Needs Verification，编译级验证）。

### Needs Verification（新增）
- 邮件适配层：真实服务商 IMAP/SMTP 互通完全未验证（计划明令禁止连接真实邮箱，属用户授权实测）；附件 FETCH 与大小上限、断线重连/退避/有界轮询、多 AUTH 机制、更多字符集注册为适配层已知缺口。
- govulncheck 23 条 stdlib 发现（go1.26.0 vs 1.26.1）与 x/crypto GO-2026-5932 为批次前已存在，与本轮无关。
- HarmonyOS：系统返回手势逐级回退（CLI 未模拟）、平板视口、暗色对比度正式测量、TalkBack、ANIMATOR_DURATION_SCALE 真机生效、合成链路端到端（需本机 collector 在线）、真机安装/签名。
- 服务端：端口/私网允许列表与证书主机名校验属批次 3 连接层（RFC 已定义未实现）；Android 设备侧领取/上报协议需真机；OpenAPI 4096 字节请求体上限对长正文场景需批次 3 复核。
- Android：滑动手势/动画观感、键盘遮挡、TalkBack、暗色对比度——仅编译级验证，无真机/模拟器。
- Web：真机视口外手势与键盘遮挡；暗色对比度未正式测量；低端浏览器 popstate 行为。
- 两端正式列表均为诚实空态（「正式同步未接通」），卡片 onClick/数据源等待批次 2/3。

## 阻塞项登记

（暂无；登记格式：事项 / 原因 / 处置）

## Needs Verification 清单

（随执行追加）

## 待办（交接种子）

（随执行追加）
