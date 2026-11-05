# ADR-0002: 合成短信 PoC 的验证边界

日期：2026-11-02。状态：按用户明确授权实现的 PoC 选择，不改变正式产品安全模型。

## 背景

用户要求最小 Android 短信接收、本地持久化、离线补传 PoC，使用合成数据，不开发正式 UI。现有服务没有认证和业务加密，本机尚未发现 Android SDK。

## 选择

1. Android receiver 只接受完整的 `OPENALC_RELAY_POC:v1:<uuid>`，不保留号码、原始正文或 PDU。元数据含 fixture/event UUID、安装 UUID、来源、接收时间；SIM 未确认时为 null。
2. 使用真实 Room 数据库。一个很小的 Kotlin Multiplatform 模块只共享 Android/JVM 队列、DAO 与同步代码，从而在宿主机验证实际 SQLite 文件与重开行为；不复用到 HarmonyOS，也不引入其他移动平台目标。
3. 使用独立 Go collector 与独立迁移/数据库，只绑定 loopback，不给正式 Relay 添加未鉴权的业务入口。接收 JSON 不允许正文/号码等未知字段。
4. 调试传输仅 `127.0.0.1` HTTP + adb reverse，Android debug policy 限定该地址且禁用 release 变体。无公网明文传输。生产 HTTPS、配对、设备权限、负载加密未实现。
5. Room 提交后安排 WorkManager；ACK 后改状态，失败持有原 ID。保留 rejected 记录；后台修复处理提交与 enqueue 之间的崩溃间隙。APPEND_OR_REPLACE 防止新事件遇到正在结束的 worker 而丢失调度。

## 取舍与证据

不选择内存队列代替持久化，不把 JVM 测试或手动注入算作系统短信交付成功。保留 Room 文件重开、真实 HTTP、ACK 丢失、独立进程/Go 接收器的可重复检查。Android OS 权限、锁屏/Doze/HyperOS、重启/强制停止、SIM 和功耗仍必须真机验证。

因为只记录严格限制的合成元数据，这轮不扩展正式认证/加密实现。任何取消 allowlist、加入正文/号码或将 collector 改为公网地址的请求，都必须先落实正式安全模型，不属于当前授权范围。

官方依据：[Room KMP](https://developer.android.com/kotlin/multiplatform/room)、[Room 测试](https://developer.android.com/training/data-storage/room/testing-db)、[广播 API](https://developer.android.com/reference/android/provider/Telephony.Sms.Intents)、[网络安全配置](https://developer.android.com/privacy-and-security/security-config)。实测状态见 [Android PoC 记录](../development/ANDROID-POC.md)。
