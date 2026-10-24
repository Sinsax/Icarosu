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

## 插件

实现 `internal/plugin.Plugin` 接口，在 `init()` 中调用 `plugin.RegisterGlobal(New())`，
再在 `cmd/bot/main.go` 匿名导入该包，即可自动注册。

## 许可证

[MIT](LICENSE)
