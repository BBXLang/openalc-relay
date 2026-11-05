# 初始化交付与验证记录

日期：2026-11-02（Asia/Hong_Kong）。本记录仅说明本轮实际执行，不代表平台 PoC 或正式功能已完成。

这是初始化轮的历史记录。后续 Android 合成数据 PoC 的实现与测试结果见 [ANDROID-POC.md](ANDROID-POC.md)。

## 初始状态与交付

- 初始目录为空，不存在 `.git` 或已有代码；已在本地初始化 `main` 分支。
- 上级 `C:/devlopment/AGENTS.md` 针对另一个 ThinkPHP 项目，本仓库按用户明确指定的 Go/Kotlin/ArkTS/Vue 方向建立独立规范。
- 已建立 RFC、PoC 计划、平台依据、AGENTS、英文/中文 README、贡献/安全说明、依赖/决策记录。
- 已实现 Go 健康服务、结构化日志、配置校验、退出处理、SQLite 连接与事务迁移账本、实际 API 的 OpenAPI 3.1 契约。
- 已提供 Docker Compose、Dockerfile、systemd 与 GitHub Actions 示例；移动端/Web 只有边界说明。
- 未创建提交、添加远端、推送、部署公网服务、发短信或调用厂商推送。测试服务已停止。
- 项目许可证待所有者选择，未擅自添加 `LICENSE` 或宣称许可证已确定。

## 本地已执行

环境：Windows/amd64，Go 1.27.3，`CGO_ENABLED=0`。

| 检查 | 实际结果 |
| --- | --- |
| `gofmt` | Go 文件已格式化 |
| `go mod tidy` / `go mod verify` | 完成，模块校验通过，固定版本与 go.sum 已写入 |
| `go test ./...` | 通过：配置、HTTP/HEAD、隐私错误响应、OpenAPI 一致性、SQLite/迁移 |
| 迁移专项 | 重复启动、WAL/foreign_keys 等配置、DDL/账本事务回滚、历史校验和变更、未知未来版本、插入历史版本、路径中的空格与 # |
| `go vet ./...` | 通过 |
| Windows 构建 | `bin/relay.exe` 构建成功，产物被 Git 忽略 |
| Linux 交叉构建 | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` 构建 `bin/relay-linux-amd64` 成功；不是 Linux 运行验证 |
| 实际二进制 smoke | loopback 启动，`GET /healthz` 200、`GET /readyz` 200 且 status=ok、`HEAD /readyz` 200 无正文、`POST /v1/commands` 404；数据库已创建 |
| 文档与忽略项 | 本地 Markdown 文件链接检查通过；`.env`、数据库、构建产物、原始 PoC 记录被 Git 忽略 |

OpenAPI 的本地测试检查已声明操作与 handler 的状态、正文和 HEAD 行为，并非完整的 OpenAPI/JSON Schema 标准验证器。初次测试发现测试夹具使用空字节占位 checksum 不满足 SQLite length 约束，已改为正常 64 字符夹具，随后全套检查通过。

## 未执行与后续门槛

- Linux 原生测试、SIGTERM/systemd 退出、Rocky Linux 9.5、SELinux、容器镜像/卷权限：尚未执行。本机没有 Docker。
- race 测试：本机 CGO 关闭且未发现 C 编译器，未运行；已配置 Linux CI 的 `go test -race`。
- Go 1.26 基线运行：本地实际使用 1.27.3；CI 配置 1.26.x，尚未触发。
- GitHub Actions：只写入 workflow，没有远程运行结果。
- Android/HarmonyOS 原生工程、权限、短信、双卡、Doze/HyperOS、厂商账号/推送、功耗：全部待 PoC 真机验证。
- 用户/设备认证、业务加密、远程发送、限速审计、同步游标、备份恢复：只有设计，未实现。
- 发布前完整依赖许可证清单、漏洞扫描、容器 digest 固定和项目许可证：待后续发布门槛。

下一项建议为 Android 入站事件的最小垂直 PoC：先确认机型/系统/权限与 OTP 可见性，再做本地持久化和断网 ACK 恢复。真实数据上报必须先具备认证与加密服务端能力；未具备之前使用合成数据和本地测试 harness，不能把健康服务当业务接收器。
