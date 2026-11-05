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
| Wave 2 | HarmonyOS 正式 UI（SubAgent C）+ 服务端数据基础（SubAgent D） | ⏳ 待开始 | — |
| Wave 3 | 邮件适配包骨架（SubAgent E）+ 共享设计规范更新 + 全量验证 | ⏳ 待开始 | — |
| Wave 4 | 冻结新功能，交接证据终稿 | ⏳ 待开始（07:30 起） | — |

## 已完成明细

### Wave 1（01:50 收口）— 批次 1：Web + Android 正式 UI

**Android（SubAgent B）**：20 新文件 + MainActivity/strings 修改，`RelayInboxScreen.kt` 删除（功能并入 `SyntheticVerificationScreen`）。9 个导航目的地与规格 §2.1 导航表逐项对照通过；亮/暗令牌按规格落值（`theme/RelayTheme.kt`）；自管返回栈+BackHandler（零新依赖）；频道滑移 250ms 且 ANIMATOR_DURATION_SCALE=0 时禁用；PoC 入口移至 配置→开发→合成验证；app_name 改 `Relay`；文案集中 strings.xml（约 90 条）。
验证：`:poc-core:jvmTest` BUILD SUCCESSFUL 1m17s；`:app:compileDebugKotlin` BUILD SUCCESSFUL 2m16s（3 轮迭代修复 15 个编译错误）；`:app:lintDebug` BUILD SUCCESSFUL 11m18s。

**Web（SubAgent A）**：6 修改 + 24 新增 + 删 `src/api.ts`（并入 SyntheticVerification.vue）。`vue-router@4.6.4` 精确锁定；hash 路由（`/#/sms` 等，因 poc-collector 静态服务无 SPA fallback，见偏离记录）；`tokens.css` 亮/暗令牌按规格；侧栏文字 logo（0.14em / 45%）；StateBlock 六态；ChannelPager 250ms 双窗格滑移+reduced-motion；ComposeBar 发送恒禁用+原因可见；`sanitizeHtml.ts` 白名单净化（注释标注真实数据前须换经审计库）；PoC 整体迁 `/dev/synthetic`，根路径不再命中 collector；正式导航无 PoC 入口（e2e 断言）。
验证：`npm run build` ✓ 747ms（vue-tsc 0 错误）；`npx playwright test` **26/26 通过**（44.3s，desktop+mobile 双视口；chain.spec 8 条跑在真实 Go poc-collector + 100 条播种 fixture 链路上；formal-nav 9 条×2）。

**偏离记录**：1) Web 用 hash 而非 history 路由（collector 无 SPA fallback，保证既有 e2e 链路可跑）；2) Android 频道滑移用 `Animatable(tween(250ms))`（支持拖拽跟手+分段指示同步）；3) 演示卡片不可点入正式详情（防演示数据混淆）；4) 侧栏「连接测试/账号配置」子项留待批次 2/3 接口，未造假。

### Needs Verification（新增）
- Android：滑动手势/动画观感、键盘遮挡、TalkBack、暗色对比度——仅编译级验证，无真机/模拟器。
- Web：真机视口外手势与键盘遮挡；暗色对比度未正式测量；低端浏览器 popstate 行为。
- 两端正式列表均为诚实空态（「正式同步未接通」），卡片 onClick/数据源等待批次 2/3。

## 阻塞项登记

（暂无；登记格式：事项 / 原因 / 处置）

## Needs Verification 清单

（随执行追加）

## 待办（交接种子）

（随执行追加）
