# 平台依据与 Needs Verification

核查日期：2026-11-02。以下仅为官方文档依据与设计推论；当前没有 Android/HarmonyOS 实机、厂商账号或 SDK 验证记录。华为部分动态指南正文无法由浏览工具直接加载；只引用可检索到的官方页面信息，不推断 HarmonyOS 7 已通过验证。

| 主题 | 官方依据 | 对 Relay 的含义 / 尚需验证 |
| --- | --- | --- |
| BroadcastReceiver | [API reference](https://developer.android.com/reference/android/content/BroadcastReceiver#goAsync())：异步处理仍受广播时间窗口限制 | Receiver 先有界持久化，网络不占用整个窗口；目标 HyperOS 实际交付时间 Needs Verification |
| 持久后台任务 | [WorkManager](https://developer.android.com/develop/background-work/background-tasks/persistent)：面向持久、可延期工作 | 任务可延后，不能承诺后台即时联网；重启与回收恢复需实测 |
| 快速任务配额 | [Define work](https://developer.android.com/develop/background-work/background-tasks/persistent/getting-started/define-work)：expedited 有配额和降级行为 | 使用可退化的短时任务；不要把 expedited 等同无限唤醒 |
| Doze / standby | [Android power guidance](https://developer.android.com/training/monitoring-device-state/doze-standby)：空闲状态限制 CPU/网络/任务 | Doze 与普通锁屏、force-stop 分开测试；省电白名单不等于所有限制解除 |
| 前台服务 | [Background start restrictions](https://developer.android.com/develop/background-work/services/fgs/restrictions-bg-start) | 不能随意从后台启动服务保活；type、权限、启动条件跟目标 API 核对 |
| OTP 读取变化 | [Android 17 all apps](https://developer.android.com/about/versions/17/behavior-changes-all)、[target 17+](https://developer.android.com/about/versions/17/behavior-changes-17) | 官方描述了 WebOTP 与普通 OTP 的延迟及适用条件；目标 ROM、target SDK、角色资格和真实广播结果 Needs Verification，不能承诺任意 OTP 实时可见 |
| 明确 SIM 发送 | [SmsManager](https://developer.android.com/reference/android/telephony/SmsManager) | 使用明确 subscription 的 API；验证 SEND_SMS 授予、换卡映射、sent/delivery 回调及多段失败 |
| 通话记录权限 | [READ_CALL_LOG](https://developer.android.com/reference/android/Manifest.permission#READ_CALL_LOG) | 属 hard restricted 权限，安装器许可影响授予；普通侧载/运行时弹窗不保证可用 |
| 短信接收权限 | [RECEIVE_SMS](https://developer.android.com/reference/android/Manifest.permission#RECEIVE_SMS) | 同样属于 hard restricted 权限；必须验证安装器 allowlist 与用户授权，PoC 不能假定侧载必定授予 |
| 分发政策 | [Google Play SMS/Call Log policy](https://support.google.com/googleplay/android-developer/answer/10208820) | 商店政策与设备运行时权限不同；侧载不代表免于系统限制，是否上架另行决策 |
| 小米透传 | [停止透传服务公告](https://dev.mi.com/xiaomihyperos/documentation/detail?pId=1530) | 官方已公告停服；旧版透传教程不能作为可行方案，适配器当前 unavailable |
| 小米消息规范 | [2026 分类变更](https://dev.mi.com/xiaomihyperos/documentation/detail?pId=2322)、[官方目录](https://dev.mi.com/xiaomihyperos/documentation/detail?pId=1529) | 当前类别/模板/账号资格需核对；能展示通知不代表能后台拉取并执行命令 |
| HarmonyOS 原生推送 | [Push Kit 官方介绍](https://developer.huawei.com/consumer/cn/sdk/push-kit/)、[使用入门](https://developer.huawei.com/consumer/cn/doc/doccenter-capabilities/push-gettingstart) | 采用原生通知能力；后台消息不能当作保证唤醒的通道，Token 轮换、授权、服务账号均需验证 |
| 华为签名与地区 | [开通推送服务](https://developer.huawei.com/consumer/cn/doc/harmonyos-guides/push-config-setting) | 核查开发/发布签名、profile、通知分类权益、处理地与服务地区；目标账号及分发模式 Needs Verification |
| SQLite 驱动 | [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) | 纯 Go、BSD-3-Clause；v1.61.1 已由 Go module 元数据核对。其余传递依赖许可证仍需发布前清单审查 |

## 实机信息登记（尚未填写）

| 节点 | 必须记录的内容 | 当前状态 |
| --- | --- | --- |
| Android A / B | 机型、Android/HyperOS build、安全补丁、SIM/运营商、安装方式、权限/默认角色、应用 target SDK | Needs Verification |
| HarmonyOS 主力机 | 机型、HarmonyOS 7 完整 build、API level、DevEco/SDK、签名/profile、账号地区、通知权限、分发方式 | Needs Verification |
| Linux | Rocky 9.5 内核/架构、SELinux、部署方式、TLS/域名、磁盘、备份介质 | Needs Verification |

默认短信应用、伴随设备角色、FCM 和厂商特权通道都不是现成前提。PoC 必须记录“系统允许什么”，不能通过降低 target SDK 或规避平台保护来声称通用兼容。
