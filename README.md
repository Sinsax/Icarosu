# Icarosu

Go 编写的 AI QQ 群聊机器人。目标是像群友一样聊天，并且从底层支持插件化扩展。

当前进度：**Phase 3/4 —— 上下文、触发决策、SQLite 记忆、工具调用**

## 功能

- OneBot v11 正向 WebSocket 接入
- 插件化事件分发：插件可独立处理群消息、私聊、群通知
- 示例插件：收到 `ping` 回复 `pong`
- LLM 对话插件：接入 OpenAI 兼容 API（DeepSeek / 通义 / 智谱 / Ollama）
- 群聊被 `@` 时回复，也会按关键词、兴趣、概率主动接话
- 私聊直接回复
- 短期群聊上下文：记住最近 N 条消息，能接话题
- SQLite 长期记忆：记录群成员、消息和长期记忆
- 群成员昵称/群名片记录
- YAML 配置 + 结构化日志
- 独立的 `bot` 个性配置：QQ号、名字、性格、说话风格、背景、兴趣、规则
- 消息过滤器：可按群号/QQ号白名单或黑名单，只处理指定群和指定私聊对象
- LLM 工具调用：当前时间、日期、骰子、随机数、随机表情、梗、发送图片等
- VLM 图片识别：识别群内图片，符合人设的表情包自动“偷”进本地表情库
- 本地表情包库：平时发表情包会从库里随机取
- info 日志会输出过滤后的接入消息、LLM 思考过程、工具调用和工具结果
- 配置文件带版本号，旧版本配置启动时自动迁移并保留 `.bak` 备份
- WebSocket 自动重连
- 引用回复 / 撤回消息收集
- 用户活跃时间段统计
- 互动关系图（谁经常回复谁）
- 管理员命令（!status / !clear / !mem）
- 每群串行处理，避免回复乱序
- 活跃时间/互动/事件作为上下文载入
- 表达方式自动学习与标签筛选
- 数据库 schema 版本迁移
- LLM 请求自动重试
- 每群限流器

## 快速开始

1. 准备一个 OneBot v11 协议端（如 NapCat、Lagrange），开启正向 WebSocket。
2. 复制配置：

```bash
cp configs/config.example.yaml configs/config.yaml
```

3. 复制并修改独立模型配置：

```bash
cp configs/llm.yaml configs/llm.local.yaml
```

然后在 `configs/config.yaml` 里把 `llm_config_file` 指向 `llm.local.yaml`（相对于 configs 目录），并修改：

   - `config.yaml`：`bot.self_id`、`qq.onebot` 连接信息
   - `llm.local.yaml`：`llm` 和 `vlm` 的 `provider`、`api_key`、`model`

4. 运行：

```bash
go run ./cmd/bot -config configs/config.yaml
```

5. 测试：
   - 群里发 `ping`，机器人回复 `pong`
   - 群里 `@机器人 你好`，机器人调用 LLM 回复
   - 私聊机器人任意内容，机器人调用 LLM 回复

## 消息过滤器

在 `filter` 配置段设置：

```yaml
filter:
  enabled: true
  mode: "allow" # allow=白名单，deny=黑名单
  group_ids:
    - 123456789
  user_ids:
    - 987654321
```

- `enabled: false`：不过滤，所有群和私聊都能触发
- `mode: "allow"`：白名单模式
  - 群消息只处理 `group_ids` 里列出的群
  - 私聊只处理 `user_ids` 里列出的 QQ
- `mode: "deny"`：黑名单模式
  - 不处理 `group_ids` 里列出的群
  - 不处理 `user_ids` 里列出的 QQ

默认 `mode` 为 `allow`，即默认都是白名单。

## 触发决策

触发决策已经并入 `bot` 配置，且始终开启，不需要开关：

```yaml
bot:
  mention_reply: true
  keywords:
    - "小智"
    - "机器人"
  random_probability: 0.05
  interest_reply: true
  cooldown_seconds: 60
```

- `mention_reply`：被 @ 时必回
- `keywords`：命中关键词时回复
- `interest_reply`：命中 `bot.interests` 里感兴趣的话题时主动回复
- `random_probability`：非 @ 消息随机接话概率
- `cooldown_seconds`：非 @ 触发后的冷却时间，避免刷屏

## 记忆与存储

短期记忆默认保留每个群/私聊最近 `20` 条消息：

```yaml
memory:
  short_term_size: 20
  long_term_enabled: true
  context_max_age_minutes: 30
  summary_threshold: 50
  summary_interval_minutes: 60
```

- 启动时会自动从 SQLite 读取最近消息补充短期上下文
- 上下文只使用最近 `context_max_age_minutes` 分钟内的消息，避免冷场很久后突然被唤醒时接旧话很尴尬
- 累计消息达到 `summary_threshold` 或距离上次总结超过 `summary_interval_minutes` 时，会自动总结并写入长期记忆

长期记忆使用 SQLite：

```yaml
store:
  driver: "sqlite"
  dsn: "data/bot.db"
```

会自动记录：

- 群成员昵称/群名片（支持通过 OneBot API 同步完整成员列表）
- 群聊消息记录
- 长期记忆内容
- 事件系统：事件与参与人关联，形成关系图

## 个性配置

机器人的“人设”统一放在 `bot` 配置段：

- `self_id`：机器人 QQ 号
- `name` / `nicknames`：名字和昵称
- `personality`：性格
- `speaking_style`：说话风格
- `background`：背景设定
- `interests`：兴趣爱好
- `group_rules`：群聊行为规则
- `forbidden`：禁止事项
- `system_prompt`：可选，填写后完全使用自定义提示词，忽略上面的结构化字段

如果 `system_prompt` 为空，程序会自动把上面的结构化字段组装成一份完整的系统提示词，并附加“群聊强化要求”和“群聊语气示例”，减少机器感。

LLM 返回后还会经过轻量后处理：去掉列表符号、压缩多余空行，让输出更像群友随手发的消息。

## VLM 通用图片识别

VLM 是**通用图片理解模块**，不是只用来偷表情包。

- 群里发图片时，VLM 会识别图片内容
- 识别结果会作为“图片上下文”写入短期记忆
- 这样 LLM 看到图片消息时，能知道图里是什么，能接话、吐槽、玩梗

```yaml
vlm:
  base_url: "https://api.deepseek.com/v1"
  api_key: ""
  model: "deepseek-vl2"
  timeout_seconds: 30
  prompt: "你是一个通用图片理解助手..."
```

只要配置了 `vlm` 并且启动时能连通 API，这条图片理解链路会自动开启，不需要 `enabled` 开关。

## 偷表情包（基于 VLM 的工具）

偷图是建立在 VLM 通用识别之上的一个工具：

- VLM 识别图片后，如果判断“是表情包”且“符合当前人设”
- 就按概率保存到本地表情库

```yaml
meme:
  directory: "data/memes"
  steal_probability: 0.8
```

`meme` 也不需要开关，VLM 链路可用时会自动启用。

平时模型可以通过工具 `send_meme` 从表情库随机发表情包：

- 表情包以 `base64://` 方式发送，跨机器也能用
- 表情库为空时会提示模型“表情库还是空的”

## 日志

`info` 级别会输出：

- 过滤后进入 LLM 的消息
- LLM 的思考过程（如果模型返回 reasoning）
- 工具调用名称和参数
- 工具返回结果
- LLM 最终回复
- LLM 缓存命中情况（如果 API 返回 usage 缓存字段）

例如：

```text
msg="incoming message" type=group_message group_id=123 user_id=456 message="今天几号"
msg="tool call" name=get_today_date arguments={}
msg="tool result" name=get_today_date result="2025-01-01"
msg="llm reply" reply="今天是 2025-01-01 呀"
```

## 配置文件结构（推荐顺序）

```text
version
llm_config_file  # 指向独立的 llm.yaml
bot              # 人设 + 触发决策
filter           # 群/私聊白名单或黑名单
memory           # 短期/长期记忆
store            # SQLite 存储
meme             # 表情包库/偷图（自动启用）
attention        # 注意力系统（影响回复频率）
qq.onebot        # QQ 连接
log              # 日志
```

`llm` 和 `vlm` 配置放在独立的 `configs/llm.yaml` 中，主配置只保留引用路径。

配置文件示例已经按这个顺序组织，程序迁移/规范化时也会按这个顺序重排。

## Provider 配置

`providers` 统一管理每个提供方的 `base_url`、`api_key` 和可用模型列表：

```yaml
providers:
  qwen:
    base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
    api_key: ""
    models:
      - name: "qwen-plus"
        kind: "llm"
      - name: "qwen-vl-plus"
        kind: "vlm"
```

`llm` / `vlm` 只引用 `provider + model`：

```yaml
llm:
  provider: "qwen"
  model: "qwen-plus"

vlm:
  provider: "qwen"
  model: "qwen-vl-plus"
```

- `base_url` / `api_key` 从 `providers` 中自动获取
- 如果 `models` 列表里没有对应模型，会报错提示
- 内置了 `deepseek` / `openai` / `qwen` / `zhipu` / `ollama` 的默认 provider，即使不写 `providers` 也能用

## 学习表达方式（规划）

当前已经实现：

- `internal/style` 表达方式库
- 根据标签（`lazy` / `night` / `funny` / `food` 等）随机选择后缀/短句
- 后处理引擎接入：回复清理后按检测到的群聊风格标签随机追加表达

后续“自动学习”规划：

1. 从白名单群里收集真人发言
2. 用规则或 LLM 给每条发言打标签
3. 提取高频短句/语气词/表情作为表达模板
4. 存入 SQLite 或 `style.Library`
5. 回复时根据当前群聊风格标签，从库里随机选择表达方式

## 注意力系统

注意力系统会跟踪每个群的“关注度”，并影响回复频率：

- 被 @ → 注意力提升
- 提到 bot 名字 → 注意力提升
- 当前话题命中兴趣 → 注意力提升
- bot 自己发言后 → 注意力提升
- 注意力越高 → 随机回复概率越高、冷却时间越短
- 注意力会随时间衰减
- 持续满注意力一段时间后自动熔断归零

```yaml
attention:
  enabled: true
  max_score: 100
  decay_per_minute: 5
  boost_at_bot: 20
  boost_mention: 10
  boost_interest: 15
  boost_bot_replied: 10
  trip_hold_seconds: 300
```

## 配置文件版本

- 当前版本：`12`
- 旧版本配置会被自动迁移到当前版本
- 迁移时会自动补上新增字段，并把旧位置的 `qq.onebot.self_id`、`llm.system_prompt` 迁移到 `bot` 配置段
- 从 v3 迁移到 v4 时会自动补上 `filter` 配置段
- 从 v4 迁移到 v5 时会自动补上 `filter.user_ids`
- 从 v5 迁移到 v6 时会自动补上 `memory`、`store` 配置段
- 从 v6 迁移到 v7 时会把 `trigger` 配置段合并进 `bot`，并新增 `llm` 缓存配置
- 从 v7 迁移到 v8 时会自动补上 `vlm`、`meme` 配置段
- 从 v8 迁移到 v9 时会移除 `vlm.enabled`、`meme.enabled`，改为配置且连通后自动开启
- 从 v9 迁移到 v10 时会为 `llm`、`vlm` 增加 `provider` 字段
- 从 v10 迁移到 v11 时会自动补上 `attention` 配置段
- 从 v11 迁移到 v12 时会自动补上 `attention.trip_hold_seconds`
- 原文件会先备份为 `config.yaml.bak`，再写入迁移后的配置
- 如果配置文件版本高于当前程序支持的版本，会报错拒绝启动
- 迁移或检测到顺序混乱时，会自动按预设顺序重排配置项

## 目录结构

```text
cmd/bot/                  程序入口
internal/config           配置加载
internal/logger           日志
internal/plugin           插件抽象与管理器
internal/plugin/builtin   内置插件
internal/plugin/chat      LLM 对话插件
internal/plugin/filter    消息过滤器插件（群聊+私聊）
internal/plugin/trigger   触发决策插件
internal/plugin/recorder  消息记录插件
internal/plugin/steal     偷图/表情包收藏插件
internal/plugin/summarizer 群聊自动总结插件
internal/plugin/membersync 群成员昵称同步插件
internal/memory           短期/长期记忆服务
internal/store            SQLite 存储（含事件关系图）
internal/meme             本地表情包库
internal/vlm              VLM 图片识别客户端
internal/vision           通用图片理解 + 偷图服务
internal/llm              OpenAI 兼容 API 客户端（支持工具调用）
internal/persona          人设提示词组装
internal/tools            内置工具注册表
internal/style            表达方式库（标签随机选择）
internal/qq               OneBot v11 WebSocket 客户端
internal/handler          QQ 事件 → 插件事件转换与分发
configs/                  配置文件示例
```

## 如何写一个新插件

1. 在 `internal/plugin/` 下新建包，实现 `plugin.Plugin` 接口。
2. 在包的 `init()` 中调用 `plugin.RegisterGlobal(NewYourPlugin())` 自注册。
3. 在 `cmd/bot/main.go` 中匿名导入该包：

```go
_ "icarosu/internal/plugin/yourplugin"
```

主程序会自动收集全局注册表里的插件并注册到 Manager。

插件接口：

```go
type Plugin interface {
    Name() string
    Version() string
    Description() string
    Priority() int
    Init(ctx context.Context, bot Bot) error
    Handle(ctx context.Context, bot Bot, e *Event) bool
    Stop(ctx context.Context) error
}
```

- `Handle` 返回 `true` 表示事件已被消费，后续插件不再处理。
- `Priority` 越大越先执行。

## 后续规划

- 按群不同人设
- 图片/表情包发送
- 群管理命令
- 自动摘要旧聊天记录
- 稳定性、重连、Docker 部署

## 许可证

本项目基于 [MIT License](LICENSE) 开源。
