# 服务端 MVP 阶段记录 — 2026-11-05

依据 [RFC-0002](../architecture/RFC-0002-server-mvp.md)。本记录区分已实现的代码、本机验证和仍需实施的阶段；它不表示可接入真实短信。

## 已实现：S0、S1 Web 会话和合成设备交换

- `relay-admin keygen` 在指定路径新建 256 位密钥文件，不覆盖已有文件。密钥文件不进入 Git 或数据库备份；加载时检查格式、长度、版本 ID 和 Unix 文件权限。加密 envelope 使用 AES-256-GCM、随机 nonce、`key_id` 和调用方指定的 AAD。尚无业务负载写入。
- `relay-admin bootstrap` 只在服务器本地通过密码文件创建单个 owner，不提供 HTTP 注册。密码使用 Argon2id（3 次、64 MiB、2 线程）验证；密码文件不得进入 Git。CLI 不在参数和日志中传递密码。暂未实现凭证恢复或 WebAuthn，丢失管理员密码的恢复流程仍需设计。
- SQLite forward migrations 新增 `owners` 与 `web_sessions`。会话 token 使用 32 字节随机值，数据库只存 SHA-256 摘要；每次请求查询过期与撤销状态。浏览器 cookie 为 `Secure`、`HttpOnly`、`SameSite=Strict`、`__Host-` 前缀；退出校验 Origin 与 CSRF token。
- 初始化后，业务进程必须配置 `RELAY_MASTER_KEY_FILE` 和 HTTPS `RELAY_PUBLIC_ORIGIN`，并只监听回环地址，否则启动失败。登录限流为单进程每分钟 10 次尝试、最多 2 次并行密码计算；尚未设计多实例或代理侧限流。
- S1 增加 5 分钟单次设备邀请、申请时核对码、owner 显式批准、一次性申请凭证兑换、独立设备凭证和即时撤销。申请/设备凭证均为高熵随机值，数据库只存 SHA-256 摘要。每次 Agent 同步在事务中按凭证检查设备与 owner，不信任客户端自报归属。
- 仅新增固定合成元数据：owner 可为自己的设备创建 ping，Agent 手动拉取并明确回执；Agent 可提交合成事件 ID，服务端落盘后 ACK，重复 ID 返回 `duplicate`。无效回执使同事务事件回滚。没有短信、号码、自由文本命令负载或业务加密事件。
- [OpenAPI](../../packages/openapi/openapi.json) 只列实际路由。独立 `poc-collector` 仍只处理固定合成 fixture，和此服务不共享凭证或数据库。HarmonyOS 模拟器双向结果见[单独记录](HARMONY-EXCHANGE-2026-11-05.md)。

## 本地初始化示例

以下命令仅说明文件与进程关系；应在受控主机上使用实际私有路径，不要把密码文件、密钥或数据库加入仓库。在 Unix 上将密码文件和密钥文件限制为所有者可读，并将密钥与数据库备份分开保存。Windows 的 ACL 需由管理员核查。

```sh
cd server
go build -o relay-admin ./cmd/relay-admin
go build -o relay ./cmd/relay
./relay-admin keygen -out /private/relay/keys.json
./relay-admin bootstrap -database /private/relay/relay.db -key-file /private/relay/keys.json -password-file /private/relay/owner-password.txt
RELAY_DATABASE_PATH=/private/relay/relay.db RELAY_MASTER_KEY_FILE=/private/relay/keys.json RELAY_PUBLIC_ORIGIN=https://relay.example.com RELAY_HTTP_ADDR=127.0.0.1:8080 ./relay
```

`RELAY_PUBLIC_ORIGIN` 必须与实际 HTTPS 代理的浏览器 origin 一致。现有 Docker/systemd 示例没有配置完整 TLS 代理，也没有针对 Rocky Linux 实测，不能直接视为公网部署方案。

## 验证与下一步

本机 Go 包测试覆盖单次初始化、错误密码、会话过期/撤销、跨站 Origin/CSRF、密钥轮换与密文/AAD 篡改、SQLite 迁移历史。执行 `go test ./...`、`go vet ./...`、`go build ./cmd/...`，并通过进程级测试检查启动门禁与登录。GitHub Actions 已由推送触发；其结果以[实际运行记录](https://github.com/BBXLang/openalc-relay/actions)为准，本机通过不能代替云端结果。Linux race 测试由 CI 定义，本机未运行。

接下来实施 S2 owner/device 范围内的加密通信事件入站、内容冲突处理和审计，再让 Android HTTPS 适配与 Web 管理页真正使用这些接口。本轮固定合成元数据和 HarmonyOS 模拟器联调不能替代该阶段。正式短信、号码、远程发送、Push 和 HarmonyOS 真机业务接入均仍未实现。两台 Android 长期合成实测、锁屏/断网/重启、Rocky Linux HTTPS/备份恢复与 HarmonyOS 真机结果均为 **Needs Verification**。
