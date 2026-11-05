# Web 合成数据链路 PoC

初始范围（2026-11-03，Asia/Hong_Kong）：本机合成数据，最小功能验证页。当时的“等待 HTML 原型”决定已被 2026-11-05 用户批准的三端首屏 UI 工作取代。

2026-11-05 新机复验：在 JDK 25 / Gradle 9.1.0 下运行 `scripts/test-web-poc.ps1` 通过。100 条合成 fixture 离线 pending、恢复后 100 条 synced，collector 记录 100 条、rejected 0；collector 重启后桌面与移动视口 Chromium 共 8 项通过。证据位于忽略的 `bin/poc-integration/fb5d2b0b4d544feba87a5dba2dfa9ea6/`。下文 `ea7ccc6d7a924988985d8ae1c1e30823`、preview PID 和安装 ID 是旧机历史记录，进程没有迁移。新机没有连接真机或运行长期预览进程。

同日 UI 更新后复验：改为移动优先的收件箱卡片布局，仍只显示 collector 的合成元数据。`scripts/test-web-poc.ps1` 再次通过：100 条离线保留并恢复同步、collector 重启、8 项 Chromium 测试（桌面与手机视口），无浏览器写请求或横向溢出。证据在忽略的 `bin/poc-integration/1b68b7b0d3d54999a4c420e9c443ad17/`；截图在 `apps/web/build/playwright-results/`。手机视口测试不等于真机 Web 验证。

随后用户提供实际移动端截图并否定卡片式首屏。现版 Web 改为空白收件箱、顶部双标签和底部安装 ID 读取栏；100 条合成数据仍按行展示。再次完整执行 `scripts/test-web-poc.ps1`：100 条 pending 恢复为 synced，collector 记录 100、rejected 0，重启恢复与桌面／移动视口 8 项测试通过。证据在忽略的 `bin/poc-integration/5343448531b54971b8d6161bdf8d3d2e/`。

## 实现范围

```text
Kotlin fixture → Room 文件 → 离线 pending
             → 恢复上传 → Go collector SQLite 提交 → ACK → Room synced
                                                 ↓
                          同源 Vue 3 / TypeScript 网页手动读取
```

Android app 的最小控件现在显示可选择复制的 Installation ID，便于在网页筛选。
宿主自动化使用同一个 Room DAO、队列和同步核心；它不会调用 Android 系统广播或 WorkManager。

- `GET /poc/v1/events` 必须指定 `installation_id`，默认 25 条、上限 50 条。
- `after` 是上一页的最后一个事件 UUID；按 UUID 升序，使用现有主键，无迁移或历史 SQL 修改。
- 计数和当前页在一个只读事务中读取；未知安装返回空数组，不混入其他安装。
- 翻页不是整个数据集的固定快照：新插入到已翻过范围的 UUID 需要从首页刷新才能看到。
- 显示 ID、来源、设备记录时间和可空订阅元数据；不包含正文、号码、凭证或内部 checksum。
- 网页只读、无自动轮询/后台通知/生成数据按钮；刷新失败或更换安装清除旧结果。
- 独立 collector 的 `-web-dir` 可选提供打包页面；正式 `cmd/relay` 仍只有健康接口。

## 运行与复验

完整命令见 [Web README](../../apps/web/README.md)。仓库根目录的 PowerShell：

```powershell
. ./scripts/use-android-env.ps1
./scripts/test-web-poc.ps1
```

脚本使用独立临时目录和随机本机端口，结束时关闭自己启动的 collector。不会访问手机、域名或云服务器。
依赖由 npm lockfile 和 Gradle 固定；原始数据、截图、日志不入 Git。

## 证据与待验证项

实际执行结果：

| 检查 | 结果 |
| --- | --- |
| Go `test ./...` / `vet ./...` / `build ./cmd/relay` | 全部通过；新增范围含 57 条分页/安装隔离/SQLite 重开、非法查询、Host/Origin/跨站边界和静态资源限制 |
| Room/JVM | 14 项通过，0 failures / 0 errors |
| 100 条独立进程离线补传 | offline_pending=100、recovered_synced=100、collector_event_count=100、rejected=0；首次 ACK 丢失后正确去重 |
| collector 进程重启 | 同一个 SQLite 文件重新启动后，网页仍读到全部 100 条 |
| Vue/TypeScript / Vite | 类型检查及打包通过；构建资源全部来自本地 |
| Chromium 实际页面 | 桌面 1280×900 / 移动视口 390×844，共 8 项通过，0 skipped / 0 flaky；成功链路无控制台错误/警告，未产生网页写请求 |
| ID 核对 | 遍历 4 页，网页 ID 与 Kotlin/Room seed manifest 的 100 个 UUID 逐条一致，无重复 |
| Android 增量构建 | 主 APK / 测试 APK / lint 通过；lint 0 errors、8 warnings；未执行设备测试 |

最终集成 run ID：`ea7ccc6d7a924988985d8ae1c1e30823`。原始输出在忽略的
`bin/poc-integration/<run-id>/result.json` 和 `browser-report.json`；截图在 `apps/web/build/playwright-results/`。
首次浏览器运行因空 favicon 被 CSP 拦截产生控制台告警；修正为仅允许 data 图片后完整复跑通过，未忽略控制台检查。
测试工具进程有 NO_COLOR/FORCE_COLOR 环境提示，与网页控制台检查分开记录。
测试结束后另启动了同一数据文件的本机查看进程，入口为 `http://127.0.0.1:8787/`；
本轮安装 ID 是 `82ab0121-59d4-410b-a013-9497743a9f38`。
进程 ID、路径和入口记录在同目录的 `preview.json`，可按记录的 PID 停止该本机进程。

以下仍为 **Needs Verification**：

- 小米实机上的短信广播、权限、Room Android 运行、WorkManager 调度、Doze/锁屏/重启/强停后的行为。
- Android 真机通过 USB `adb reverse` → collector → 网页的完整路径；本轮没有连接设备。
- 实际 HarmonyOS 手机浏览器兼容性与后台行为；移动尺寸 Chromium 仅验证视口布局。
- 公网 HTTPS、真实域名、反向代理、生产认证授权与负载加密；本轮未部署。
- Windows 本机没有 C 编译器/CGO，未运行 Go race；Linux CI 配置的 race 不等于已执行。

## PoC 安全边界

保留仅合成 fixture 输入，不接受任意正文或号码。查询的安装 ID 是过滤条件，不能代替用户/设备鉴权。
同时校验 loopback peer 和 literal-loopback Host，拒绝跨来源/跨站浏览器请求，防止域名重绑定入口。
同源 GET 放行，浏览器 POST 仍拒绝；不启用 CORS。页面不使用 CDN，CSP 限制为本机静态资源和同源读取。
响应 `no-store`，静态服务只提供首页及明确的 JS/CSS 资源，不提供目录列表或任意文件下载。
不得把这个没有认证和业务加密的实验 collector 反代到公网。
