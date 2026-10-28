# Icarosu

Go 编写的 AI QQ 群聊机器人：像群友一样聊天，插件化扩展。

## 特性

- OneBot v11 正向 WebSocket 接入（NapCat / Lagrange）
- 插件化事件分发，LLM 对话、记忆、表情包均可替换
- 触发决策：被 @ 必回，支持关键词、兴趣话题、随机接话与冷却
- SQLite 长期记忆：群成员、消息、话题、表达方式
- VLM 图片识别，自动收藏符合人设的表情包
- 本地 WebUI：状态 / 记忆 / 表情包 / 插件 / 配置

## 快速开始

准备一个 OneBot v11 协议端（NapCat / Lagrange），开启正向 WebSocket。

```bash
cp configs/config.example.yaml configs/config.yaml
cp configs/llm.yaml configs/llm.local.yaml
go run ./cmd/bot -config configs/config.yaml
```

需要改两处配置：

- `configs/config.yaml`：`bot.self_id`、`qq.onebot.ws_url`、`llm_config_file`（指向 `configs/llm.local.yaml`）
- `configs/llm.local.yaml`：`provider`、`api_key`、`model`

## SDK runtime（实验）

新微内核运行时的独立入口（旧 `cmd/bot` 保留）：

```bash
go run ./cmd/icarosu -config configs/config.yaml
go run ./cmd/icarosu -config configs/config.yaml -plugins-dir configs/plugins
go run ./cmd/icarosu -config configs/config.yaml -migrate-config
cp configs/kernel.example.yaml configs/kernel.yaml
go run ./cmd/icarosu -kernel-config configs/kernel.yaml
```

当前包含 OneBot adapter + persona / renderer / attention / memory.recent / tool.time / tool.system / filter / safety / replylog / summarizer / stylelearn / jargonlearn / membersync / steal / admin / WebUI 状态页。
插件配置可放到 `configs/plugins/<id>.yaml`（`config:` 段或直接字段），未提供时回退到主配置。
WebUI 状态页由 `webui.enabled/addr/token` 控制（默认关闭）。

## 双入口策略

- `cmd/bot`：旧稳定链路，默认入口，功能完整。
- `cmd/icarosu`：新 Go SDK runtime，用于验证微内核/标准包/新插件；尚未默认切换。支持 `-kernel-config`（内核/插件启停）与 `-plugins-dir`（插件业务配置）。
- `cmd/icarosu -migrate-config`：把旧配置拆成 `configs/plugins/<id>.yaml`，新 runtime 优先读取。
- 在 M4 收口（WebUI 完整管理、kernel.yaml、通用权限）之前，保持双入口，不回退旧链路。
- 2026-10-28 全仓 review 与修复记录见 [docs/REVIEW-2026-10-28.md](docs/REVIEW-2026-10-28.md)。
- 学习能力、情绪与分层记忆计划见 [docs/PLAN-LEARNING-EMOTION.md](docs/PLAN-LEARNING-EMOTION.md)。

## 命令

```bash
make build      # 编译
make test       # 测试
make vet        # 静态检查
make fmt        # 格式化
make docker-up  # docker compose 启动
```

## 目录

```
cmd/bot/     入口
internal/    核心逻辑：core / store / plugin / webui / adapters
configs/     配置与模板
data/        SQLite 与表情包（不入库）
```

## 架构

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)：分层、运行时链路、定位与目标
- [docs/KERNEL.md](docs/KERNEL.md)：内核边界、缺口清单、第一方插件装载
- [docs/RULES.md](docs/RULES.md)：40 条开发规矩与 CI 守则
- [docs/PLUGIN_DEV.md](docs/PLUGIN_DEV.md)：插件开发推荐（Go / WASM / 脚本 / 协议）
- [docs/PLAN.md](docs/PLAN.md)：Go SDK 插件化整体计划（M0–M5）
- [docs/research/plugin-arch.md](docs/research/plugin-arch.md)：业界对标研究

当前阶段只做第一方插件；第三方运行时装载 / MCP / WASM 暂缓。

## 插件

事件插件当前实现 `internal/plugin.Plugin` 接口（迁移目标见 [docs/PLAN.md](docs/PLAN.md) 的 Go SDK）。当前有两种注册方式：

- `internal/plugin/builtin` 通过 `init()` + `plugin.RegisterGlobal` 自注册；
- 其余插件在 `internal/app/app.go` 中显式构造并注册，便于注入依赖。

新增插件通常需要同时修改 `app.go`（接线）与 `internal/config`（配置）。
本仓库正在向“微内核 + 插件清单”演进，详见架构文档。

## 许可证

[MIT](LICENSE)
