# ADR-0003：Web 业务优先，暂缓 HarmonyOS 业务接入

日期：2026-11-03。状态：用户明确决定。

2026-11-05 更新：用户明确要求 Android、HarmonyOS、Web 共用一套 UI 风格，首屏选收件箱。HarmonyOS 原生 Stage 界面工程已在手机和平板模拟器运行；Web 优先的业务接入、正式 API、安全门槛和暂缓 Huawei Push Kit 的边界仍有效。见 [三端界面规范](../design/RELAY-UI.zh-CN.md)和[模拟器验证记录](../development/HARMONY-UI-2026-11-05.md)。

用户已有域名、云服务器和 SSL，先使用浏览器客户端，减少 HarmonyOS 原生工程、签名和 Push Kit 接入成本。优先客户端调整为 Vue 3 + TypeScript 的移动/桌面 Web；Android Kotlin Agent 与 Go Server 保持原有方向。

## 实施边界

- 三端界面首轮以用户提供的 ChatGPT 移动端截图为结构参考；目前仅完成收件箱壳层，正式业务流程仍须以可验证的 API 和安全能力为基础。
- 后续推荐同一个 HTTPS origin 提供 Web 静态资源和 `/v1` API，减少 CORS 与 cookie 配置复杂度。实际反向代理、域名、服务器访问方式在部署任务中核对，不擅自修改 DNS、上传证书或部署公网服务。
- SSL 解决传输证书问题；鉴权、资源授权、CSRF、加密存储、限速及审计仍须在真实通信数据接入前实现。
- 当前独立 synthetic collector 继续只允许 loopback，不作为云端接收接口暴露。
- 浏览器打开时通过授权 HTTPS 同步；HarmonyOS 目标浏览器的 Service Worker、Web Push、通知授权、后台投递与安装式 PWA 支持均为 `Needs Verification`。不承诺关掉页面也能像原生应用一样接收系统通知。
- Web 客户端选择不解决小米 Agent 的后台唤醒。命令排队、过期和不确定结果处理继续遵守 RFC。
- HarmonyOS ArkTS/ArkUI 与 Huawei Push Kit 的既有研究保留，退出当前实施关键路径。

后续顺序：Android APK/静态检查 → 合成数据设备 PoC → 正式认证与加密入站链路 → 按用户原型实现 Web → 用户明确授权后配置 HTTPS 云部署。用户提供部署信息时不应把密钥写进代码或聊天中的可提交配置。
