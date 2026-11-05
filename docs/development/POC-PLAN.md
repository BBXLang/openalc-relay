# OpenALC Relay 技术验证计划

2026-11-03 范围更新：Web 客户端业务接入优先，HarmonyOS Push Kit 验证暂缓，不再阻塞当前 Android→Server→Web 路径。2026-11-05 已开始三端统一 UI，HarmonyOS Stage 工程在手机和平板模拟器运行；真机与业务能力仍待验证。见 [ADR-0003](../decisions/0003-web-first.md)和[HarmonyOS 界面记录](HARMONY-UI-2026-11-05.md)。原有原生测试项保留供将来恢复使用。

- 状态：Android 合成数据 PoC 已进入实现；一台小米已完成界面和合成 Room 记录实机检查，真实短信测试未执行。当前证据见 [ANDROID-POC.md](ANDROID-POC.md)和[设备记录](DEVICE-UI-2026-11-05.md)。
- 日期：2026-11-02
- 依据：[RFC-0001](../architecture/RFC-0001-system-architecture.md)、[官方证据与待验证项](PLATFORM-VERIFICATION.md)
- 目标：在正式业务开发前验证可行性与失败边界。三端收件箱界面首轮已启动，不用理想环境结果推断全平台可靠。

## 1. 前置条件与证据规范

继续登记两台小米、华为主力机的机型和完整 build、Android/HarmonyOS API、target SDK、安装来源、权限、后台设置、SIM/运营商及网络。厂商账号类型、地区、签名/profile、通知权益分别登记。已有一台小米的局部实机证据；第二台小米和 HarmonyOS 真机仍为 `Needs Verification`。

准备专用测试 SIM/自有收件号码、合成短信（无真实 OTP/个人信息）、测试服务器 TLS 和时钟同步。优先模拟输入与 fake SMS gateway 验证状态机；真实发送只做少量经用户明确授权的样本，记录分段和可能资费，不默认批量发短信。不能向运营商/第三方网关注册或付费来替代授权。

Android PoC 在 `apps/android/`，只含权限、同步与合成 fixture 所需的最小控件；HarmonyOS Stage 界面工程已在 `apps/harmony/`，未接入业务或 Push Kit。构建环境、宿主测试与设备证据分别登记。缺少账号/签名/真机应记 `Needs Verification`，不能用服务端 mock 声称厂商投递成功。

证据放本地忽略目录 `docs/development/poc-results/`；提交只含匿名汇总。日志不含正文、号码、token。每条记录至少含：case ID、软件版本、设备/build、权限/安装方式、UTC/单调时钟、输入 ID、队列/服务端提交时间、通知显示时间、结果、证据路径、偏差。raw adb/logcat/截图需要先检查敏感信息，不能直接提交。

## 2. 验证顺序与停止条件

| 阶段 | 验证 | Gate |
| --- | --- | --- |
| P0-A | Receiver → Room → HTTPS → ACK、重复投递、断网恢复 | 本地已提交事件不丢；服务端一份；ACK 丢失可恢复 |
| P0-B | 权限、HyperOS 后台、OTP 可见性、未接来电资格 | 得到真实支持矩阵；不具备的能力明确禁用 |
| P0-C | 小米/可用系统推送能否实际触发拉取 | 云侧接受、系统通知显示、代码执行分别举证；不可用则验证降级 |
| P0-D | 指定 SIM 远程短信与执行日志 | 所有崩溃窗口不自动重复调用系统；接受 unknown |
| P0-E | HarmonyOS 7 原生 Push Kit 通知 | 真实系统通知及点击后授权同步通过；mock 不算通过 |
| P1 | 功耗、延迟、部署/恢复 | 比较不同策略，形成有测量误差的参数建议 |

OTP 可见性或后台唤醒不通过时，停止相关即时性承诺，记录原因并向用户汇报产品取舍。缺少合法权限时禁用功能，不借无障碍/通知读取等额外权限绕开限制。厂商能力不可用仍可测试手动/前台同步。

## 3. 场景矩阵

除明确不适用外，在两台 Android 分别执行。首先做单变量测试，再对高风险组合做锁屏+断网、Doze+低电量、回收+换网；不要盲目全排列消耗真实短信。每种组合注明是否有真实证据。

| 维度 | 条件 |
| --- | --- |
| 屏幕/应用 | 前台、退后台、锁屏 5/30/120 分钟 |
| 网络 | Wi-Fi、移动网络、切换、无网、DNS/TLS 失败、弱网、服务端 429/5xx |
| 电源 | 充电、电池、低电量、省电模式、Doze/App Standby |
| 生命周期 | 普通进程回收、划掉最近任务、用户 force-stop、重启/首次解锁前后 |
| 权限 | 完整、拒绝/撤销 SMS、撤销 phone/call log、通知关闭、后台设置改变 |
| SIM | 单卡、双卡各自收发、默认 SIM 不同、禁用/拔卡/换槽、映射过期、多段短信 |
| 服务器 | 重启、事务失败、数据库忙/磁盘满、ACK 丢失、推送超时、恢复旧备份 |

强制停止与普通进程回收是不同状态。force-stop 后需要用户重新打开应用的行为单独列为限制，不将其计作普通后台恢复成功。

## 4. 核心测试用例

| ID | 操作与故障注入 | 判定与证据 |
| --- | --- | --- |
| SMS-01 | 普通短信到达，记录接收、Room commit、服务端 commit、ACK | 合成样本正确分段合并/归属；落盘早于网络；时间链可追溯 |
| SMS-02 | 同一事件重复上传、服务端提交后丢 ACK | 一个事件/变更/通知意图；再次 ACK；相同 ID 不同内容冲突 |
| SMS-03 | 断网接收，杀进程/重启，恢复网络 | Room 未确认事件保留，系统允许后补传；记录重试延迟 |
| SMS-04 | 在落盘与 WorkManager enqueue 间崩溃 | 下次启动/允许的修复任务找回待传事件，不靠再来一条短信触发 |
| SMS-05 | Doze、锁屏、低电量、HyperOS 默认与用户允许后台设置对比 | 分别记录广播有无、落盘有无、上传时刻，不合并成“没收到” |
| SMS-06 | 不同格式合成 OTP 与普通短信，记录系统/target SDK/合法角色 | 验证系统可见性延迟；不得绕过保护或用真实验证码测试 |
| CALL-01 | 响铃无人接听、拒接、主叫取消、占线、接听 | 合法权限下分类正确，不能仅靠 RINGING→IDLE 推断未接 |
| CALL-02 | 拒绝/无法授予 READ_CALL_LOG、双卡来电 | 明确降级和 SIM 不确定性；不误报已具备能力 |
| PUSH-01 | 官方允许的 provider 发提示，分别观察云回执/系统通知/Agent 拉取 | 只有实际回调与 HTTPS 拉取才证明执行窗口；透传停服不接旧接口 |
| PUSH-02 | 省电、force-stop、无推送、token 失效、云超时 | 命令仍持久化；客户端显示等待；手动同步可恢复，不能假成功 |
| CMD-01 | 用户选指定设备/SIM，调用真实 SmsManager 少量测试 | 收件机确认来源，分段 sent 回调匹配；sent 不等于 delivered |
| CMD-02 | 重复创建请求/claim/start/result，多 worker 并发 | 一次用户意图、一个有效 attempt；过期 lease 被 fencing 拒绝 |
| CMD-03 | start 前租约过期、命令过期、撤销、SIM mapping_version 改变 | 不调用系统；返回明确拒绝，不能静默切默认 SIM |
| CMD-04 | start ACK 丢失；invoking 前后、系统调用前后、结果落盘前后崩溃 | 可能已执行即 unknown；重启不重发；fake gateway 验证调用次数 |
| CMD-05 | 部分分段成功/失败、回调丢失/重复/迟到/冲突 | 记录每段证据，不自动重发整条；迟到结果仅更新同一 attempt |
| CMD-06 | start 后用户取消/凭证撤销、备份回滚 | 如实报告 too_late/unknown；恢复后暂停未完成命令，禁止自动重放 |
| HOS-01 | HarmonyOS 7 原生工程签名、获取/轮换 token、通知授权 | 真机系统通知可见，build/SDK/签名/账号地区完整记录 |
| HOS-02 | 前台、后台、锁屏、进程回收、断网恢复、通知关闭 | 通知投递/展示率、延迟；点击后 HTTPS 增量同步，推送无敏感正文 |
| HOS-03 | 分类权益、地区、失效 token、服务账号故障 | 记录平台拒绝与频控；不能把 HTTP 200 当展示成功 |
| SEC-01 | 用设备 A 凭证 claim B、设备 token 调用户接口、撤销后 start | 服务器拒绝；日志/推送不泄露测试正文、号码、密钥 |
| SEC-02 | 数据库/WAL/备份检索合成正文和号码、key 丢失/轮换 | 业务实现后证明认证加密和 AAD；当前骨架不以此项宣称通过 |
| OPS-01 | Rocky 9.5 单实例部署、SIGTERM、磁盘满、备份恢复 | HTTPS、SELinux/UID/权限、readiness、迁移原子性、恢复不重放 |

## 5. 样本、指标与初始验收目标

以下为实验目标，不是已达到的 SLA：

- 本地/服务端故障注入每个关键 crash window 至少 100 次合成执行；任何一次未允许的重复系统调用都不通过。
- 无外部费用的入站模拟，每台设备每个关键正常/离线场景至少 100 个事件；真实运营商样本经授权后另列，不用模拟样本替代。
- 初始正常联网前台上传目标 P95 ≤ 5 秒；后台/推送目标先测分布再定，不以不满足秒级视为需要保活。
- 入站持久化后丢失数为 0，去重后多余记录数为 0；队列容量溢出必须可见。
- 真实远程发送先每个 SIM 1 条，权限和归属确认后再决定额外样本；需要用户授权及预算，当前未执行。
- 记录 P50/P95/最大值、样本数、失败/丢失/unknown 数。小样本不声称稳定 P95 或百分之百可靠。
- 通知指标分为 provider 接受率、设备可观察到达/展示率、点击同步完成率，分母和超时窗口明确；30 秒仅为初始观察窗口，不是投递保证。

## 6. 功耗与自适应策略实验

对比 A（事件上传+系统低频修复）、B（A+合法可用推送）、C（B+仅前台短时实时连接）。如需评估长连，只作为有界、明确记录的实验对照，不进入默认配置。

每台设备固定信号条件、SIM、应用权限、系统设置、短信负载与屏幕计划，交叉顺序测试每策略至少三轮 24 小时，并提供未运行 Agent 的基线。用 Battery Historian/系统 batterystats 等实际可用工具，登记仪器/系统计量局限，不用单次电池百分比推导精确耗电。

记录增量电量、CPU/wakelock、后台网络次数/流量、有效上传/命令延迟、Doze 中恢复行为。可先以相对基线额外不超过 2 个电量百分点/24h 为探索目标，最终阈值取决于实机误差与用户取舍；无法可靠计量时标记 inconclusive。禁止以高频心跳“修复”延迟结果。

## 7. 可重复的手工诊断步骤

仅针对专用测试设备执行，PoC 尚未执行。以下来自 [Android 官方 Doze 测试流程](https://developer.android.com/training/monitoring-device-state/doze-standby)：

```sh
adb shell dumpsys battery unplug
adb shell dumpsys deviceidle force-idle
# 在另一个已授权测试通道产生合成事件，记录结果。
adb shell dumpsys deviceidle unforce
adb shell dumpsys battery reset
```

每轮结束恢复设置并登记；命令输出可能含设备标识，不能提交原始输出。测试 force-stop、重启、断网与权限变化各自单独执行并记录恢复步骤。不能将 adb 可触发的调试能力当普通应用运行时权限。

## 8. 结果模板与正式开发门槛

```text
Case / date / tester:
Device + OS build + API / target SDK / app commit:
Install source / permissions / roles / background settings:
SIM alias + mapping version / network / power / screen:
Input IDs + sample count (synthetic vs carrier):
Expected observations / observed stage timestamps:
Outcome: Pass | Fail | Blocked | Inconclusive
Loss / duplicate / unknown / latency / power:
Sanitized evidence location / reproduction / limitations:
```

最终产出支持矩阵、失败复现、功耗/延迟数据及 RFC 修订。每一项必须区分官方文档推论、mock、模拟器与真机结果。只有权限与入站持久化通过才启用真实数据能力；只有安全状态机通过才启用真实远程发送；只有 HarmonyOS 7 真机通知通过才宣称原生通知支持。用户已授权 Android 合成数据 PoC，正式业务开发仍按上述门槛推进。
