# HarmonyOS 模拟器双向合成交换 — 2026-11-05

## 范围

本轮将服务端 S1 配对、独立设备凭证、撤销与固定合成元数据双向交换连接到 HarmonyOS ArkUI 客户端。仅限本机 Go 服务、`hdc rport` 回环隧道和 HarmonyOS 7 模拟器。没有短信正文、号码、真实消息、远程短信发送、Push Kit 或公网部署。模拟器链路使用 HTTP 回环；正式客户端仍须使用 HTTPS。

## 环境与步骤

- DevEco Studio `26.0.0.865`、API 26 SDK `26.0.0.105 Release`；Stage bundle `org.openalc.relay.harmony`，target/compatible SDK `26.0.0`，unsigned HAP。
- `Relay_Tablet_API26`（tablet）与 `Relay_Phone_API26`（phone），均为 HarmonyOS `7.0.0(26.0.0)` 模拟器；每次在 `127.0.0.1:5555` 运行一个。没有 HarmonyOS 真机参与。
- 构建 HAP 并安装到当前模拟器，再从仓库根目录运行 `./scripts/test-harmony-emulator.ps1`。脚本为每次运行生成隔离的 SQLite、owner 和密钥文件，服务端只监听主机回环 `127.0.0.1:18080`，用 `hdc rport tcp:18080 tcp:18080` 让模拟器连接主机。脚本结束清理隧道、服务进程与临时密码/密钥；测试数据库、布局、截图和受控日志留在 Git 忽略的 `bin/harmony-exchange/run-*/`。
- Owner 创建 5 分钟邀请；ArkUI 申请配对并显示核对码；脚本从 owner 会话读取同一码，明确批准；客户端用一次性申请凭证兑换设备凭证。手机模拟器通过 Ability 启动参数传入无密钥的邀请 ID，绕过首次启动输入法的隐私授权弹窗。申请凭证、设备凭证不进入启动参数、URL 或日志。

## 实测结果

平板模拟器运行记录：`bin/harmony-exchange/run-8f291effe29c49a6acdfe43a588ae411/`。手机模拟器最终回归记录：`bin/harmony-exchange/run-0afeb65fc3ab42ff90d541869e1a431c/`。两端自动化均以退出码 0 完成，页面布局与截图已读取，手机最终页面显示「合成事件已提交」。每次各有一个配对请求、一台兑换成功的设备、一个服务端 ping、一个客户端事件。

| 验证点 | 平板模拟器 | 手机模拟器 |
| --- | --- | --- |
| Owner 核对码与客户端显示一致、显式批准、凭证单次兑换 | 通过 | 通过 |
| 服务端创建 ping，客户端拉取并显示；手动回执前数据库未标记 ACK | 通过 | 通过 |
| 客户端明确回执后，服务端数据库持久化 ACK | 通过 | 通过 |
| 客户端上传固定合成事件，服务端事务提交后返回 ACK，页面显示提交 | 通过 | 通过 |
| Owner 撤销设备后，下一次客户端同步被拒绝 | 通过 | 通过 |

Go 单元/HTTP 测试另覆盖跨 owner 查询、错误核对码、批准前兑换、重复兑换、凭证伪造、重复事件去重、SQLite 重开、无效回执回滚和并发重复。事实接口列于 `packages/openapi/openapi.json`。

## 边界与下一步

这验证了模拟器与本机服务之间的手动拉取、回执和上传。它没有验证系统推送、后台唤醒、长期断网、真机网络、真机权限或 Android 设备；这些均为 **Needs Verification**。HarmonyOS 客户端设备凭证目前仅在本次进程内保存，进程死亡后必须重新配对；本地偏好设置仅保留待提交合成事件 ID 和待回执 ping ID。加密通信负载、正式会话、Web 管理面板、Rocky Linux 部署仍未实现。Android 真机回归按用户要求安排在下一日。
