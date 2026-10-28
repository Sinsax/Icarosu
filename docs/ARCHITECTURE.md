# Icarosu 架构与插件化设计

> 版本：v0.2（讨论稿，含红队评审修正）  
> 基线：master @ c43ac39，约 11.5k 行 Go 源码 + 3.5k 行测试  
> 范围：现状结构梳理、产品定位、微内核 + 全插件化目标架构、迁移路线  
> 结论先行：**泛化的是“能力装载方式”，不是“她是谁”。**

**当前决策：先做第一方插件。** 第三方运行时装载、MCP、WASM、插件市场暂缓。

**v0.2 修正**：按红队评审修正 <code>requires</code>/Host 能力来源/Event/Action/Decision 身份/命名，并补充 WebUI auth、Pool 竞态、SQLite 并发、notice 锁等 P0；明细见 [docs/KERNEL.md](KERNEL.md) 第 3.0 节。

配套文档：

- [docs/KERNEL.md](KERNEL.md)：内核必须留什么、还缺什么、第一方插件怎么装
- [docs/RULES.md](RULES.md)：40 条开发规矩与 CI 守则
- [docs/PLUGIN_DEV.md](PLUGIN_DEV.md)：插件开发推荐（Go / WASM / 脚本 / 协议）
- [docs/PLAN.md](PLAN.md)：Go SDK 插件化整体计划（M0–M5）

---

## 0. 一句话

Icarosu 是一个**住在群里的数字熟人**：有连续记忆、固定脾气和说话习惯，会看气氛，也会不想说话。她不是助手，也不是插件商店。

### 速读版

| 问题 | 回答 |
|---|---|
| 她是谁 | 有连续记忆和固定表达习惯的群聊居民，不是助手 |
| 现在是什么 | 模块化单体 + 事件插件流水线（最多 14 个，默认无 VLM 时 13 个）；能跑，但插件注册、配置、平台抽象都还是硬编码 |
| 最大的结构问题 | 三套扩展机制并存；平台类型泄漏；慢任务堵在回复热路径；<code>app.go</code> 手写装配；配置单体 |
| 目标形态 | 微内核（回合 / 总线 / 工具循环 / 动作分发 / Trace）+ 标准器官包 + 全插件化 |
| 类人模型 | 感官 → 注意力 → 记忆 → 人格 → 语言 → 工具 → 反思；插件是器官和手艺，身份不可插拔 |
| 插件契约 | manifest + config schema + manual + permissions + Host 能力；**本期只做第一方编译期插件** |
| 先做什么 | ① 平台中立 Event/Action + OneBot adapter；② 慢任务异步化；③ manifest / tool 契约 / Trace |
| 不要做什么 | 不推倒重写；不用 Go <code>plugin</code>；不做插件市场；不让插件改人格 |

本文分三部分：

1. **她是谁**：产品定位与设计红线；
2. **她现在长什么样**：当前代码结构与运行时链路；
3. **她该怎么长**：微内核 + 全插件化的目标架构与分阶段迁移。

---

## 1. 她的定位

### 1.1 一句话定位

> **一个有连续记忆和固定表达习惯的群聊居民；能力可以装载，身份不能拼装。**

如果只能记一句话：**她卖的不是“能做什么”，而是“像谁”。**

### 1.2 她是什么

- **群里的一个熟人**：记得你上次说过什么，有自己的口头禅，会接梗，也会冷场。
- **有身体的人**：QQ 只是当前身体；换平台不该换人格。
- **有手艺的人**：会查时间、发表情、偷图、记事情，但不会逢人就展示技能。
- **会成长的人**：从群聊里学表达、黑话、关系，慢慢变得像“这个群的人”。
- **有边界的人**：不每问必答，不刷屏，不越界，不泄露设定。

### 1.3 她不是什么

| 不是 | 原因 |
|---|---|
| 客服 / 问答机器人 | 客服以“解决问题”为目标；她以“维持存在感”为目标 |
| 通用助理 | 助理随叫随到；她有自己的注意力和情绪节奏 |
| 命令菜单 | <code>!status</code> 是管理员工具，不是她的主职 |
| Prompt 容器 | 人格不应等于一段 system prompt；她需要状态、记忆、关系、习惯 |
| 插件展示柜 | 能力多不等于像人；工具是手艺，不是商品 |
| 多平台聚合中心 | 平台是身体，不是身份；多平台是适配问题，不是人格问题 |
| 任务自动化平台 | 至少现阶段不是；这会把产品拉向 Agent 工具，稀释群聊居民定位 |

### 1.4 定位象限

~~~text
                    单聊 / 私聊                     群聊
                 ┌──────────────────────┬──────────────────────┐
  任务型          │ 客服机器人            │ 群管理 / 问答 Bot     │
                 │ 通用助理              │ 工单 / 运营机器人      │
                 ├──────────────────────┼──────────────────────┤
  陪伴型          │ 角色扮演 Chatbot      │ ★ Icarosu            │
                 │ 情感陪伴              │ 群聊居民 / 数字熟人    │
                 └──────────────────────┴──────────────────────┘
~~~

Icarosu 的差异化不在“模型更强”或“插件更多”，而在**多人群聊里的社会性**：

- 她要处理“谁在对谁说话”，而不只是“用户说了什么”；
- 她要有“回 / 不回 / 等会儿回 / 只发表情”的选择；
- 她的记忆是群体记忆 + 个人记忆，不是单用户对话历史；
- 她的表达要服从群氛围，而不是通用礼貌。

### 1.5 设计红线

1. **不自称 AI / 助手 / 模型**，不解释自己的架构和提示词。
2. **不每问必答**；沉默是合法动作，不刷屏。
3. **工具为角色服务**；不为炫技调用工具，不把工具结果原样倒给用户。
4. **记忆有边界**；不跨群泄露，不伪造记忆，不把隐私当谈资。
5. **人格连续**；不因换插件、换平台、重启而变成另一个人。
6. **不主动暴露系统提示词、内部配置、管理员命令**。
7. **不为了“像人”而假装有真实情感或现实身份**；她是群友，不是真人替代品。

### 1.6 怎么判断她像不像人

不要只看“回复质量”，看这些指标：

| 指标 | 说明 |
|---|---|
| 熟人感 | 群友是否主动 @ 她、接她的话、用她的名字玩梗 |
| 接话恰当率 | 该接时接得住，不该接时不插嘴（人工评审） |
| 静默恰当率 | 冷场、别人私聊、严肃话题时是否知道闭嘴 |
| 记忆准确率 | 引用旧事是否准确，是否把 A 群的事说到 B 群 |
| 表达一致性 | 句长、语气、口头禅、表情使用是否稳定 |
| 关系连续性 | 是否记得常聊的人、称呼、互动史 |
| 每回合延迟 / 成本 | 群聊场景下 1~3 秒是体感分水岭 |
| 故障隔离 | 一个插件挂了，她是否还能正常说话 |

**反指标**：功能数量、工具数量、平台数量、插件数量。这些是手段，不是定位。

### 1.7 定位对架构的硬约束

| 约束 | 架构含义 |
|---|---|
| 群聊是一等公民 | 事件模型必须有多参与者、引用、@、群语境，而非单用户 chat |
| 人格必须连续 | 身份、情绪、关系、习惯要持久化；不能每次从 prompt 重建 |
| 静默是动作 | 调度器要支持 reply / silent / defer / observe-only，不能默认“收到就回” |
| 能力可插拔，身份不可插拔 | persona 是受控核心数据；普通插件不能任意改写人格 prompt |
| 平台是身体 | 核心不出现 qq、group_id、CQ 码；适配器负责翻译 |
| 工具是手艺 | 工具调用前要经过意愿、权限、风格、预算，而不是用户菜单 |
| 快慢分离 | 回复走快路径；学习、总结、关系更新走后台慢路径 |
| 可回放 | 每个回合都能回答“她为什么回/为什么不回/为什么这样回” |

---

## 2. 她现在长什么样

### 2.1 代码地图

~~~text
Icarosu
├─ cmd/bot/main.go                 入口：flag → app.New → app.Run
├─ internal/
│  ├─ app/                         组装根：所有依赖在这里 new 并接线（414 行）
│  ├─ config/                      配置加载 / 迁移 / 校验 / 热更新（version 12）
│  ├─ qq/                          OneBot v11 正向 WebSocket 客户端
│  ├─ platform/                    平台适配器接口（当前是 qq 类型的别名）
│  ├─ handler/                     OneBot 事件 → plugin.Event；限流、会话锁
│  ├─ plugin/                      事件插件框架：Registry / Manager / Context / Stage
│  │  ├─ builtin/                  ping / sender / replylog（init 自注册）
│  │  ├─ filter/ safety/           准入与安全过滤
│  │  ├─ recorder/                 消息、图片、互动、活跃时间记录
│  │  ├─ steal/ membersync/        表情包收藏、群成员同步
│  │  ├─ summarizer/               群聊总结
│  │  ├─ stylelearn/ jargonlearn/  表达方式 / 黑话学习
│  │  ├─ trigger/                  是否接话
│  │  ├─ admin/                    管理员命令
│  │  └─ chat/                     LLM 对话 + 工具调用
│  ├─ core/                        领域逻辑：chat / trigger / summarizer / domain
│  ├─ llm/ vlm/                    模型客户端（OpenAI 兼容）
│  ├─ tools/                       LLM function-calling 工具注册表
│  ├─ memory/ semantic/ topic/     短期记忆、语义检索、话题稳定性
│  ├─ style/ attention/ group/     表达库、注意力、互通组
│  ├─ persona/ vision/ meme/       人设提示词、图片理解、表情库
│  ├─ store/                       SQLite 存储与仓储接口（schema v4）
│  ├─ queue/ session/ ratelimit/   工作池、会话串行锁、令牌桶
│  ├─ dryrun/ obs/ logger/         干跑、指标、日志缓冲
│  └─ webui/                       本地 WebUI（配置/状态/记忆/表情/插件/日志）
├─ configs/                        config.example.yaml + llm.yaml
├─ Dockerfile / docker-compose.yml / Makefile
└─ docs/                           本文
~~~

### 2.2 运行时分层

~~~text
┌────────────────────────────── 接入层 ──────────────────────────────┐
│ OneBot v11 WS → qq.Client → handler.Handler → queue.Pool           │
│                    （事件解析、@ 判断、图片/引用提取、限流）         │
└──────────────────────────────────┬─────────────────────────────────┘
                                   │ plugin.Event
┌──────────────────────────────────▼─────────────────────────────────┐
│ 插件流水线 plugin.Manager                                          │
│  pre:  filter → safety → recorder → steal → membersync             │
│        → summarizer → stylelearn → jargonlearn → trigger           │
│  resp: admin → ping → chat                                         │
│  post: sender → replylog                                           │
└──────────────────────────────────┬─────────────────────────────────┘
                                   │ Context.Data{reply, force_split}
┌──────────────────────────────────▼─────────────────────────────────┐
│ 能力层（由 app 直接 new，不是插件）                                 │
│ llm / vlm / store / memory / semantic / topic / style              │
│ attention / group / persona / vision / meme / tools / obs          │
└──────────────────────────────────┬─────────────────────────────────┘
                                   │
┌──────────────────────────────────▼─────────────────────────────────┐
│ 基础设施：SQLite、HTTP、WebSocket、日志、配置、WebUI                │
└────────────────────────────────────────────────────────────────────┘
~~~

### 2.3 启动与依赖组装

<code>app.newApp()</code> 是事实上的“手写依赖注入容器”：

1. 加载配置（<code>config.Manager</code>），校验；
2. 创建 logger / metrics / group scope；
3. 创建 LLM 客户端；
4. 打开 SQLite，创建 memory / semantic / topic / attention；
5. 创建 VLM，探测可用性，创建 meme store / vision；
6. 构建 persona prompt；
7. 创建平台客户端（默认 <code>qq.Client</code>），包装 dry-run；
8. 创建 <code>plugin.Manager</code>，先注册 <code>init()</code> 自注册的 builtin 插件，再逐个 new 并注册其余插件；
9. 创建 tool registry、style library，注册 stylelearn / jargonlearn / chat；
10. 创建限流器、session manager、handler、worker pool；
11. 创建 WebUI；
12. 注册配置热更新回调。

**关键事实**：除了 <code>ping</code> / <code>sender</code> / <code>replylog</code> 三个 builtin 插件，其余插件全部在 <code>app.newApp()</code> 里手工 new、手工接线。两种注册方式目前并存，新增插件通常要同时改 <code>app.go</code> 与 config 类型。

### 2.4 一条群消息的完整旅程

~~~text
1. OneBot 推送事件
   └─ qq.Client.readLoop 解析 → goroutine 回调
2. handler.Handle
   ├─ 按 post_type / message_type 分流
   ├─ 解析 CQ 段：@、图片、引用
   ├─ 群消息走令牌桶限流
   └─ 按群号取 session 锁，保证同群串行
3. queue.Pool 提交任务（4 workers / buffer 128）
4. plugin.Manager.Dispatch
   ├─ Preprocessor（按 priority 从高到低）
   │   filter(1000) 白/黑名单，不通过则吞掉事件
   │   safety(990)  关键词过滤
   │   recorder(950) 写短期/长期记忆；同步 VLM 识图；记录活跃/互动/引用/撤回
   │   steal(940)   同步 VLM 判断并收藏表情包
   │   membersync(930) 同步调用 OneBot API 拉群成员（每小时一次/群）
   │   summarizer(920) 达到阈值时同步调用 LLM 总结群聊
   │   stylelearn(910) 2% 概率同步调用 LLM 学习表达
   │   jargonlearn(905) 2% 概率同步调用 LLM 学习黑话
   │   trigger(900)  @ / 关键词 / 兴趣 / 随机 / 冷却 → 决定是否继续
   ├─ Responder（第一个返回 true 的插件截断后续）
   │   admin(800) 管理员命令
   │   ping(100)  ping → pong
   │   chat(50)   构造上下文 → LLM（最多 5 轮工具调用）→ 后处理
   │              把 reply / force_split 写入 Context.Data
   └─ Postprocessor
       sender(10)   读取 reply，按群/私聊发送；force_split 时分多条
       replylog(0)  记录日志
5. qq.Client.SendGroupMessage / SendPrivateMessage
~~~

**chat 的上下文组装顺序**（<code>core/chat.buildMessages</code>）：

1. system：人设 prompt（<code>persona.BuildSystemPrompt</code>）；
2. system：当前群聊风格标签；
3. system：表达方式样本；
4. system：当前稳定话题提示；
5. system：长期记忆（语义检索优先，失败退回最近记忆）；
6. system：群数据（活跃时间、互动关系、相关事件）；
7. system：群内黑话；
8. system：群友画像；
9. user/assistant：最近 12 条短期上下文；
10. 当前消息不是独立 block：正常路径只用于语义检索/话题判断；只有 recent 为空时才作为 user 追加。图片识别结果另存 memory 上下文。

**回复后处理**：

- 去掉“作为 AI / 祝你 / 总的来说”等助手腔；
- 按自然分句拆分，50% 概率拆成多条发送；
- 不拆时调用 <code>SmartTruncate(reply, 7)</code>，超过 7 个字优先在句末/逗号截断，实在没有断点再调用一次 LLM 压缩。

这解释了她为什么会“惜字如金”：人设要求 1~7 个字，代码在执行这个要求。

### 2.5 当前插件清单

| 插件 | 阶段 | 优先级 | 职责 | 是否在消息热路径做慢操作 |
|---|---|---:|---|---|
| filter | pre | 1000 | 群/用户白名单、黑名单 | 否 |
| safety | pre | 990 | 按 forbidden 关键词拦截 | 否 |
| recorder | pre | 950 | 记忆、图片识图、活跃/互动/引用/撤回 | **是**（VLM） |
| steal | pre | 940 | 识别并收藏表情包 | **是**（VLM） |
| membersync | pre | 930 | 同步群成员昵称/群名片 | **是**（OneBot API） |
| summarizer | pre | 920 | 按阈值/间隔总结群聊 | **是**（LLM） |
| stylelearn | pre | 910 | 2% 采样学习表达方式 | **是**（LLM） |
| jargonlearn | pre | 905 | 2% 采样学习黑话 | **是**（LLM） |
| trigger | pre | 900 | 决定是否接话 | 否 |
| admin | responder | 800 | !status / !clear / !mem | 否 |
| ping | responder | 100 | ping → pong | 否 |
| chat | responder | 50 | LLM 对话 + 工具调用 | 是（LLM，预期内） |
| sender | post | 10 | 统一发送回复 | 否 |
| replylog | post | 0 | 记录回复日志 | 否 |

### 2.6 当前工具清单

| 工具 | 作用 | 备注 |
|---|---|---|
| <code>get_current_time</code> | 当前日期时间 | 纯本地 |
| <code>get_today_date</code> | 今天日期 | 纯本地 |
| <code>roll_dice</code> | 掷骰子 | 纯本地 |
| <code>get_random_emoji</code> | 随机表情/颜文字 | 纯本地 |
| <code>get_meme_text</code> | 随机梗/段子 | 纯本地 |
| <code>get_random_number</code> | 指定范围随机整数 | 纯本地 |
| <code>send_image</code> | 发送图片 | **chat 插件按名字特判**，registry 里的 Execute 只会报错 |
| <code>send_meme</code> | 发送表情包 | **chat 插件按名字特判**，需要 meme store |

### 2.7 数据与状态

SQLite（<code>internal/store</code>，schema v4）当前表：

- <code>members</code>：群成员、昵称、群名片、关系、备注、发言数；
- <code>memories</code>：长期记忆（总结、事件等）；
- <code>messages</code>：消息流水；
- <code>events</code> / <code>event_participants</code>：事件与参与者；
- <code>message_replies</code>：引用关系；
- <code>recalls</code>：撤回记录；
- <code>user_activity</code>：用户活跃时段；
- <code>interactions</code>：用户互动计数；
- <code>memory_embeddings</code>：本地哈希向量；
- <code>topic_states</code>：群话题稳定性；
- <code>style_expressions</code>：表达方式库；
- <code>jargons</code>：群黑话；
- <code>user_profiles</code>：群友画像。

内存态：短期记忆（每会话 20 条）、注意力分数、话题状态、表达库、会话锁、令牌桶、插件启停状态。

### 2.8 配置与热更新

- 单一 <code>config.Config</code>，当前 <code>version: 12</code>；
- 配置迁移、规范化、默认值、字段顺序全部手写（<code>config/migrate.go</code>、<code>canonical.go</code>、<code>defaults.go</code>、<code>default_nodes.go</code>、<code>load.go</code>、<code>types.go</code>）；
- LLM / VLM 单独拆到 <code>llm.yaml</code>，支持 provider 列表；
- <code>config.Manager.OnReload</code> 把新配置推给 LLM、filter、trigger、attention、limiter、chat persona；
- 新增一个配置项通常要改 5~7 个文件（类型、默认值、默认 YAML 节点、迁移、规范化、加载、测试）。

### 2.9 WebUI

本地 HTTP 服务，提供：

- 配置查看/编辑/热重载、原始 YAML 编辑；
- 状态面板、健康检查、Prometheus metrics；
- 记忆浏览/删除、表情包浏览/上传/删除；
- 插件列表与开关；
- 管理员操作（清空上下文、手动总结、发测试消息）；
- 群维度学习数据（表达、黑话、画像）；
- 日志查看与 SSE 实时流。

前端是 <code>static.go</code> 里的 692 行内嵌 HTML/CSS/JS，不是独立前端工程。

### 2.10 测试与质量

- 11.5k 行源码 + 3.5k 行测试，约 30% 测试/源码比；
- <code>go build ./...</code>、<code>go test ./...</code> 在当前基线通过；
- 测试覆盖较好的：config、webui、store、plugin manager、llm、chat；
- 测试空白：<code>handler</code>、<code>qq</code>、<code>memory</code>、<code>vision</code>、<code>recorder</code>、<code>steal</code>、<code>membersync</code>、<code>stylelearn</code>、<code>jargonlearn</code>、<code>session</code>、<code>ratelimit</code>；
- 没有架构约束测试（依赖方向、平台隔离、插件边界）；
- <code>internal/webui/server.go</code> 的中文注释存在乱码（GBK/UTF-8 混淆），属于历史遗留，建议顺手修掉。

---

## 3. 现状诊断：离“插件化底座”还差什么

> 结论：当前是**模块化单体 + 一条事件流水线**，不是插件系统。  
> 优点是能跑、测试不差、领域逻辑已经开始下沉到 <code>core</code>；  
> 问题是扩展点不统一、装配集中、平台泄漏、慢任务堵在热路径。

### 3.1 三种扩展机制并存

| 机制 | 注册方式 | 生命周期 | 能做什么 | 主要问题 |
|---|---|---|---|---|
| 事件插件 <code>plugin.Plugin</code> | <code>init()</code> 或 app 手工注册 | Init / Handle / Stop | 拦截、记录、回复、后处理 | 布尔语义、同步执行、绑定 QQ 事件 |
| LLM 工具 <code>tools.Tool</code> | <code>tools.Register</code> | 无 | 模型可调用 | 无权限、无手册、无上下文、特判 |
| 平台适配器 <code>platform.Adapter</code> | app 手工注入 | Connect / Close | 平台接入 | 类型别名泄漏 OneBot |
| 核心服务（memory/store/...） | app 直接 new | 无 | 领域能力 | 无法替换、无法插拔 |

同一个“发表情包”能力，分散在：meme store、chat 插件特判、tools 注册表、qq.Client 发图 API、sender 发送逻辑。**没有单一能力边界。**

### 3.2 平台抽象是假的

<code>internal/platform/platform.go</code>：

~~~go
type Event = qq.Event
type MessageHandler = qq.MessageHandler

type Adapter interface {
    plugin.Bot
    SetMessageHandler(handler MessageHandler)
    Connect(ctx context.Context) error
    Close() error
}
~~~

- <code>Event</code> 直接别名到 <code>qq.Event</code>；
- <code>plugin.Bot</code> 的方法签名是 <code>SendGroupMessage(groupID int64, ...)</code> / <code>SendPrivateMessage(userID int64, ...)</code>；
- <code>handler</code> 直接解析 OneBot 的 <code>post_type</code>、<code>message_type</code>、CQ 段；
- <code>plugin.Event</code> 里有 <code>GroupID</code>、<code>UserID</code>、<code>RawMessage</code>（CQ 码）、<code>Raw []byte</code>。

这意味着“接 Telegram / Discord”不是写一个 adapter 就完了，而是要改 <code>plugin.Event</code>、<code>plugin.Bot</code>、<code>handler</code>、<code>memory</code>、<code>tools</code> 里的所有 <code>int64 group_id</code> 假设。Git 历史里 <code>b9eee34 revert: remove Telegram adapter, keep OneBot as the only platform</code> 就是证据。

### 3.3 插件注册是编译期硬编码

- <code>plugin.RegisterGlobal</code> 只服务 3 个 builtin 插件；
- <code>app.newApp()</code> 手工 <code>new</code> 了 filter、safety、recorder、steal、membersync、summarizer、stylelearn、jargonlearn、trigger、admin、chat，并手工注入各自的依赖；
- 新增插件必须改 <code>app.go</code>，通常还要改 config 类型、默认值、WebUI 依赖、测试；
- 插件之间通过 <code>app</code> 的局部变量共享依赖，而不是通过 Host/Registry。

这不是“插件”，是“在一个大函数里 new 很多结构体”。

### 3.4 慢任务堵在消息热路径

Preprocessor 在 trigger/chat 之前同步执行，且整条链路持有该群的 session 锁：

- <code>recorder</code> 对每张图同步调 VLM；
- <code>steal</code> 再对同一张图调一次 VLM（有 10 分钟缓存，但仍可能阻塞）；
- <code>membersync</code> 每小时一次同步等 OneBot API 响应；
- <code>summarizer</code> 达到阈值时同步调 LLM；
- <code>stylelearn</code> / <code>jargonlearn</code> 2% 概率同步调 LLM。

结果：群友发一张图，可能先等几秒识图、再等总结/学习，才轮到“她要不要接话”。**这不像人，像串行批处理。**

### 3.5 布尔 Handle 语义过载

<code>Plugin.Handle</code> 返回 <code>true</code> 的含义随阶段变化：

- Preprocessor 返回 true → 整个 Dispatch 立即结束（吞掉事件）；
- Responder 返回 true → 停止后续 Responder，但仍执行 Postprocessor；
- Postprocessor 返回 true → Dispatch 提前返回；
- sender / replylog 返回 false，靠 <code>Context.Data</code> 传递副作用。

“是否消费事件”“是否已回复”“是否继续”“是否发送”混在一个 bool 里，导致：

- 插件顺序敏感、难以组合；
- 无法表达“我想观察但不处理”“我想延迟回复”“我想静默”；
- 测试只能测“true/false”，测不了意图。

### 3.6 配置单体

- 所有插件的配置都在 <code>config.Config</code> 一个结构里；
- 默认值、迁移、规范化、字段顺序、YAML 节点生成全是手写；
- 插件无法拥有自己的 schema、版本、默认值、校验；
- WebUI 配置表单与主配置强耦合；
- 热更新回调在 <code>app.registerReload</code> 里硬编码，每加一个需要热更新的插件就要改这里。

### 3.7 依赖倒置不彻底

当前实际依赖（节选）：

~~~text
core/chat      → memory, store          （核心依赖具体实现）
memory         → plugin                 （记忆反向依赖插件事件）
plugin         → core/domain            （插件框架依赖领域模型）
handler        → qq                     （接入层绑定 OneBot）
platform       → qq, plugin             （平台抽象依赖具体平台）
store          → config                 （存储依赖配置）
webui          → plugin/summarizer      （界面依赖具体插件）
app            → 几乎所有包             （组装根）
~~~

表现较好的地方：<code>store/repository.go</code> 已经按领域拆了窄接口，<code>core/chat</code> 也定义了 <code>Memory</code> / <code>StyleLibrary</code> / <code>TopicTracker</code> 等接口。方向是对的，但没有坚持到“核心不依赖具体实现”。

### 3.8 全局可变状态

- <code>plugin.RegisterGlobal</code> + 包级 <code>global</code> registry；
- <code>builtin.SetSenderMetrics</code> 包级变量；
- 插件在 <code>init()</code> 里注册，测试之间难以隔离；
- 多实例（多个 bot / 多平台）会共享全局状态。

### 3.9 没有清单、手册、权限、健康检查

<code>PluginState</code> 只有：Name / Description / Stage / Priority / Enabled。

缺少：

- 版本、作者、许可证、依赖、API 版本；
- 提供的 capacity / 需要的 capacity；
- 配置 schema、默认配置、配置版本；
- 权限声明（发消息、读记忆、网络、文件、管理员）；
- 工具清单与工具手册；
- 健康检查、启动耗时、错误率；
- 卸载/清理/数据迁移约定。

### 3.10 WebUI 是单体，不是插件

- 直接依赖 <code>*memory.Memory</code>、<code>*store.Store</code>、<code>*plugin.Manager</code>、<code>*summarizer.SummarizerPlugin</code>；
- 路由、页面、业务查询混在 <code>handlers_*.go</code>；
- 新增插件想加一个页面，必须改 <code>webui</code> 包；
- 前端是内嵌字符串，无法按插件扩展。

### 3.11 单实例假设

- 一个 <code>self_id</code>、一个 <code>qq.Client</code>、一个 <code>Manager</code>、一个 <code>memory</code>、一个 WebUI；
- 所有 key 都是 <code>int64 groupID</code>，没有 <code>platform + conversation</code> 命名空间；
- 没有多 persona、多租户、多实例的生命周期管理。

如果未来要“同一个人格接多个平台”或“多个角色”，当前结构会逼你做第二次重构。

### 3.12 小问题

- <code>internal/webui/server.go</code> 中文注释乱码；
- <code>plugin.Event.Raw</code> 存整段原始 JSON，事件模型与 OneBot 强绑定；
- <code>tools.Registry</code> 没有参数 schema 校验、超时、结果大小限制、权限；
- <code>llm.ChatWithTools</code> 最多 5 轮、串行执行工具、无预算/成本上限；
- <code>plugin.Manager.Dispatch</code> 串行调用且没有 <code>recover</code> / 超时：一个插件 panic 会带走整个进程；
- <code>tools.Registry</code> 是共享可写注册表（由 app 创建后注入），重名直接 <code>panic</code>，没有命名空间，第三方插件无法安全共存；
- <code>style</code> / <code>attention</code> / <code>session</code> 的状态在内存里，重启才失忆；当前配置热更只改部分字段，且 <code>attention.Observe</code> 存在数据竞争；
- 配置里的密钥目前只做到“不写进示例文件”，没有真正的加密/环境变量注入约定；
- WebUI 的 <code>http.Server.Handler</code> 用的是裸 <code>mux</code>，<code>s.auth(mux)</code> 只给测试用；<code>/api/config</code> 还返回完整配置（含 API Key）；
- <code>queue.Pool.Close</code> 直接 <code>close(jobs)</code>，并发 <code>Submit</code> 可能 send on closed channel；worker 用 <code>context.Background()</code>，取消/关闭预算失效；
- SQLite 未设 WAL/busy_timeout，迁移逐条 Exec 无事务；4 worker 并发写可能 <code>SQLITE_BUSY</code>，迁移失败会留半状态；
- <code>notice</code>/<code>recall</code> 直接 Dispatch，不走会话锁；session key 是裸 int64，群 123 与私聊用户 123 会共锁；
- <code>config.Manager.Reload</code> 只 Load 不 Validate，listener 无 error/回滚，热更新可能半更新；
- 部分插件包没有测试（见 2.10）。

### 3.13 诊断结论

| 维度 | 现状 | 目标 |
|---|---|---|
| 扩展单元 | 事件插件 / 工具 / adapter 三套 | 统一为“能力插件” |
| 装配 | <code>app.go</code> 手工 new | Host + manifest + bundle |
| 平台 | QQ 类型泄漏到核心 | 平台中立 Event/Action |
| 配置 | 单体结构 + 手写迁移 | 插件自带 schema + 内核校验 |
| 热路径 | 同步 VLM/LLM/API | 快慢分离，后台观察者 |
| 插件边界 | 可 import 任意内部包 | 只依赖 SDK + Host 能力 |
| 可观测 | 日志 + 计数器 | 每回合 Trace + 插件健康 |
| 安全 | 全局 token + 关键词 | 能力权限 + 审计 + 预算 |
| 多实例 | 不支持 | 内核实例化，按 profile 隔离 |

---

## 4. 目标：类人微内核

### 4.1 人的类比

| 人的部分 | 系统对应 | 归属 |
|---|---|---|
| 生命循环 | 内核：收事件、开回合、调工具、发动作 | **内核** |
| 感官 | 平台适配器、图片/语音/链接理解 | 插件 |
| 注意力 / 意愿 | 是否接话、何时沉默、主动开口 | 插件（默认包） |
| 记忆 | 工作记忆、情景记忆、语义记忆、程序性记忆 | 插件 |
| 人格 / 价值观 | 身份、性格、边界、关系、情绪 | 插件（受控） |
| 语言 | LLM + 表达风格 + 渲染 | 内核 LLM 能力 + 插件 |
| 手艺 | 工具：时间、搜索、表情、提醒、生图 | 插件 |
| 反思 | 总结、学表达、学黑话、关系更新 | 插件（异步） |
| 身体 | 发送消息、发表情、撤回、加群 | 适配器插件 |
| 免疫 / 治理 | 安全、权限、限流、预算 | 插件（默认包） |
| 仪表盘 | WebUI、日志、Trace、手册 | 插件 / 内核接口 |

**一句话**：内核是“活着的那条回路”，插件是“器官和手艺”。  
不要让插件决定“怎么当人”，只让它决定“用什么器官、会什么手艺”。

### 4.2 最小内核的 7 个职责

1. **Host / 生命周期**：插件注册、manifest 校验、依赖排序、Init/Start/Stop/Health、配置挂载、存储命名空间。
2. **契约 / 适配器路由**：平台中立 Event/Action/Part；按会话路由到 adapter。
3. **调度 / 并发 / 背压**：同会话串行、跨会话并行、队列上限、去重、超时、取消、优雅关闭。
4. **回合引擎**：context block 预算/裁剪、Decider、prompt 组装、LLM 工具循环。
5. **工具执行**：注册、命名空间、schema、权限、超时、结果截断、审计。
6. **治理**：权限、预算、限流、错误分类、panic 恢复、审计。
7. **可观测**：Trace、结构化日志、指标、健康/就绪。

更细的边界、缺口和第一方插件装载见 [docs/KERNEL.md](KERNEL.md)。

内核**不**知道：QQ、群号、CQ 码、SQLite、表情包目录、人格文案、具体工具。

### 4.3 默认器官包：空内核不是产品

“底座只有工具调用和聊天回复”不能理解成“裸 LLM + 空插件”。那会得到一个通用 Agent，不是 Icarosu。

推荐形态：**最小内核 + 标准器官包（standard bundle）**。

标准包至少包含：

- <code>adapter-onebot</code>：QQ 接入；
- <code>storage-sqlite</code>：通用存储与迁移；
- <code>memory-recent</code>：短期上下文；
- <code>memory-longterm</code>：长期记忆与检索；
- <code>persona-default</code>：默认人格、边界、说话风格；
- <code>renderer-default</code>：句长、拆句、去助手腔；
- <code>drive-attention</code>：注意力 / 接话决策；
- <code>tool-basic</code>：时间、随机、表情；
- <code>reflection-summarizer</code>：总结 / 学习（异步）；
- <code>governance-basic</code>：限流、安全、预算；
- <code>interface-webui</code>：本地管理界面。

**标准包就是产品**；插件化是让标准包可替换、可裁剪、可扩展，而不是把产品拆成零件卖给用户。

### 4.4 一个类人回合

~~~text
Observation（一条群消息 / 一个定时器 / 一次 @）
   │
   ▼
Appraise（评估显著性：在说我吗？重要吗？现在方便吗？）
   │        ← drive 插件：注意力、关系、情绪、冷却
   ▼
Recall（回忆：最近说了什么？关于这个人我知道什么？这个话题之前聊过吗？）
   │        ← memory 插件：工作/情景/语义/程序性记忆
   ▼
Intend（意图：回 / 不回 / 等会儿回 / 只发表情 / 主动起话题）
   │        ← drive + policy 插件
   ▼
Compose（组织：人格 + 风格 + 上下文块 + 工具清单）
   │        ← persona 插件 + 内核预算器
   ▼
Speak / Act（说，或用工具做事）
   │        ← LLM + tool 插件
   ▼
Dispatch（通过适配器发出；失败重试/降级）
   │        ← 内核 + adapter 插件
   ▼
Reflect（事后：记住、学习、更新关系、总结）
            ← reflection 插件（异步，不阻塞回复）
~~~

关键点：

- **回不回** 是独立阶段，不是 chat 插件的副作用；
- **说什么** 与 **做什么** 共享同一个 LLM 回合，但工具结果要经过“表达”再出口；
- **反思** 永远不阻塞回复；
- 每个阶段都可被插件替换/扩展，但阶段顺序由内核固定。

### 4.5 快慢分离

| 路径 | 内容 | 目标延迟 | 实现 |
|---|---|---|---|
| 快路径 | 感知 → 评估 → 回忆 → 决定 → 说/做 → 发 | 1~3s | 内核同步回合 |
| 慢路径 | 识图、总结、学表达、学黑话、关系更新、表情收藏 | 秒~分钟 | 观察者总线 + 后台 worker |
| 离线路径 | 重建索引、迁移、批量总结 | 分钟~小时 | 管理命令 / 定时任务 |

具体做法：

- 适配器只做“收事件 / 发动作”，不在收事件时做 VLM/LLM；
- 图片先入队，先给一个占位描述（或直接不描述），识图完成后写入记忆并可选地触发一次“补一句”；
- 总结/学习订阅 <code>message.observed</code> 事件，异步消费；
- 每个后台任务有队列、并发上限、超时、重试、丢弃策略；
- 所有慢任务的结果写回状态存储，下一回合可见。

### 4.6 能力中介原则：普通人只能使用工具

这是“类人模式”最重要的架构约束：

> **插件不能直接改世界，只能通过 Host 提供的能力去行动。**

- 想发消息？调用 <code>Host.Actions().Send(action)</code>，由内核检查权限、限流、dry-run、审计；
- 想读记忆？调用 <code>Host.Memory().Recall(query)</code>，而不是直接开 SQLite；
- 想调模型？调用 <code>Host.LLM().Chat(...)</code>，带预算和 trace；
- 想定时？调用 <code>Host.Scheduler().After(...)</code>，而不是自己起 goroutine；
- 想发网络请求？声明 <code>network.outbound</code> 权限，通过 Host 的 HTTP 能力（可选）；
- 想加页面？实现 <code>interface</code> 插件，由内核挂路由。

这样做的收益：

- 权限、审计、预算、限流有唯一入口；
- 插件可被测试（fake Host）；
- 第三方插件可以被沙箱/进程隔离；
- 内核保持小，插件不能“绕过物理定律”。

### 4.7 身份与能力分层

~~~text
┌──────────────────────────────────────────────┐
│ 身份层（Identity）—— 不可随意插拔             │
│ 名字、自我认知、价值观、边界、说话习惯、关系   │
│ 由 persona 插件 + 长期记忆共同维护             │
└──────────────────────────────────────────────┘
┌──────────────────────────────────────────────┐
│ 能力层（Capabilities）—— 可插拔               │
│ 感官、记忆后端、工具、反思、平台、界面         │
└──────────────────────────────────────────────┘
┌──────────────────────────────────────────────┐
│ 内核层（Kernel）—— 稳定                       │
│ 回合、总线、调度、工具循环、动作分发、Trace    │
└──────────────────────────────────────────────┘
~~~

- **身份层**是“她是谁”，变更要走显式配置/迁移，不能让任意插件在 prompt 里加一段话就改掉；
- **能力层**是“她会什么”，可以自由装载；
- **内核层**是“她怎么活”，必须稳定、可测试、可观测。

### 4.8 业界对标：借什么、不借什么

> 完整对比（Koishi / NoneBot2 / AstrBot / LangBot / MaiBot / Rasa / MCP / Go 装载机制，25 条来源）见 [docs/research/plugin-arch.md](research/plugin-arch.md)。

| 框架 | 最值得借的 | 不照搬的 |
|---|---|---|
| Koishi | 插件 = 函数 + ctx + config；service inject 声明依赖；Schema 同时做校验/默认值/控制台表单；dispose 清理 | 同进程热重载依赖 JS 模块系统，Go 做不到 |
| NoneBot2 | driver / adapter / matcher 三层；PluginMetadata 作为帮助/市场唯一数据源；supported_adapters 能力声明 | Python asyncio 的 matcher 责任链；我们的回合语义更窄 |
| AstrBot | 每插件 <code>_conf_schema.json</code> 直接渲染 WebUI；metadata 声明版本区间/平台/依赖 | 装饰器隐式注册；插件目录当数据目录；secret 只遮 UI |
| LangBot | manifest + 组件化 Tool（parameters + llm_prompt）；独立 Plugin Runtime 进程；config schema 即 UI DSL | 组件 YAML 数量爆炸；一期就上运行时进程 |
| MaiBot | Host/Runner IPC；capabilities 逐项白名单；plugin_type 分类；semver 闭区间且禁止伪上限 | msgpack/RPC 复杂度；一期就做进程外 |
| Rasa | connector 输入/输出分离；action 外置独立服务 | 训练式对话管理，不适用 |
| MCP | tools / resources / prompts 的控制主体分离；JSON-RPC + capabilities 协商 | 当万能插件协议：它不管聊天事件、配置 UI、权限同意 |

**五条共识**：

1. **元数据驱动**：manifest + config schema + manual 三件套，同一份数据喂加载器、WebUI、帮助系统、市场。
2. **配置归插件**：每插件独立 schema/version/迁移；密钥真加密或走环境变量，不能只靠 UI 遮罩。
3. **能力白名单**：未声明即拒绝；权限带 scope（例如只能发回触发它的会话）。
4. **第一方同进程 + 第三方进程外（本期只做第一方）**：Go 标准库 <code>plugin</code> 不作为方案。
5. **热插拔只承诺进程外**；同进程只承诺配置/启停热更新，不承诺卸载代码。

**三条不要照搬**：

- 不要为了“生态”提前做市场/安装器；
- 不要把 MCP 当事件/适配器协议，它只覆盖“给模型的工具/资源”；
- 不要用“伪版本上限”（如 <code>1.99.99</code>）冒充兼容性，上限要写真实测试过的版本。

**一个关键取舍：事件主题 vs 回合阶段。**

研究建议用事件主题（<code>message.received</code> / <code>reply.pre_send</code> / <code>tool.invoked</code>）取代 pre/responder/post 三段式，因为三段式无法表达“记忆写入”“打断/取消”“异步后处理”。这个判断是对的，但**不能把整个回合也拆成自由主题**，否则“她怎么想”会变成一堆插件的隐式顺序。

推荐组合：

- **同步社交回路**用固定阶段（Observe → Appraise → Recall → Intend → Compose → Speak/Act → Dispatch）；
- **异步观察/反思**用事件主题（<code>message.observed</code>、<code>memory.written</code>、<code>reply.sent</code>、<code>schedule.tick</code>）；
- 插件可以订阅主题，但不能改变同步阶段的顺序。

这样既保留“像人”的固定回合，又给后台能力足够的扩展空间。

---

## 5. 插件模型

### 5.1 插件种类

| kind | 作用 | 扩展点 | 例子 |
|---|---|---|---|
| <code>adapter</code> | 平台接入 | Adapter | onebot、telegram、discord、cli、web |
| <code>sense</code> | 多模态理解 | Observer + ContextProvider | vlm、asr、ocr、link-preview |
| <code>storage</code> | 通用存储 | Storage | sqlite、postgres、kv |
| <code>memory</code> | 记忆读写 | Memory + ContextProvider | recent、longterm、vector、summary |
| <code>persona</code> | 人格与关系 | PersonaProvider | default-persona、mood、relationship |
| <code>renderer</code> | 表达渲染 | Renderer | default、stylized |
| <code>drive</code> | 意愿与注意力 | Decider | attention、trigger、silence、initiative |
| <code>tool</code> | 手艺 | Tool | time、dice、meme、search、reminder、image-gen |
| <code>reflection</code> | 事后反思 | Observer | summarizer、stylelearn、jargonlearn |
| <code>governance</code> | 治理 | Policy | safety、rate-limit、budget、permission |
| <code>interface</code> | 界面/管理 | UI + Command | webui、admin-cli、manual-browser |
| <code>model</code> | 模型供应商 | LLMProvider | 默认 OpenAI-compatible 在内核；ollama/local 以后插件化 |

一个插件可以同时提供多个 kind（例如 <code>meme</code> 同时提供 tool 和 sense），但 manifest 要写清楚。

### 5.2 契约：Event / Action（平台中立）

~~~go
// sdk/event.go
type Event struct {
    ID           string
    Kind         string            // message.created / notice.recalled / timer.fired / ...
    OccurredAt   time.Time
    Source       SourceRef         // adapter id, platform
    Conversation Conversation
    Actor        Actor
    Self         ActorID           // 这条事件所属的 bot 身份
    Parts        []Part            // 文本、图片、语音、文件、引用
    ReplyTo      string
    Mentions     []ActorID
    Raw          json.RawMessage   // 仅适配器/调试用，核心不解析
}

type Conversation struct {
    ID        string            // 全局唯一，如 "qq:group:123456"
    AdapterID string            // 负责这个会话的 adapter，如 "onebot"
    Kind      string            // group / private / channel / thread
    Title     string
    Metadata  map[string]string
}

type Actor struct {
    ID       string            // "qq:user:987654321"
    Name     string
    Card     string
    IsSelf   bool
    IsAdmin  bool
    Metadata map[string]string
}

type Part struct {
    Type string // text / image / audio / video / file / reply / mention
    Text string
    URL  string
    MIME string
    Data []byte
    Meta map[string]string
}
~~~

~~~go
// sdk/action.go
type Action struct {
    ID             string
    TurnID         string
    Conversation   Conversation
    Actor          ActorID           // 发送者（通常是 Self）
    Parts          []Part
    ReplyTo        string
    IdempotencyKey string
    Silent         bool              // 明确不发送（用于 dry-run / 审计）
    Meta           map[string]string
}
~~~

**规则**：

- 核心不出现 <code>group_id</code> / <code>user_id</code> / CQ 码；
- 适配器负责把平台事件翻译成 <code>Event</code>，把 <code>Action</code> 翻译成平台 API；
- 需要原生 ID 时，放 <code>Metadata["native_id"]</code>，只有适配器读；
- <code>Part</code> 是可扩展的，新增类型不破坏旧适配器（不支持的 part 应显式报错/降级，而不是静默丢弃）。

### 5.3 契约：Plugin / Host

~~~go
// sdk/plugin.go
type Plugin interface {
    Manifest() Manifest
    Init(ctx context.Context, host Host) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health(ctx context.Context) Health
}

type Manifest struct {
    APIVersion   int          // 整数 major；version 才是 semver
    ID           string
    Version      string
    Kind         Kind
    Name         string
    Description  string
    Provides     []string
    Requires     []string
    OptionalRequires []string
    ConfigSchema string      // 文件路径或内嵌 JSON Schema
    Permissions  []string
    Tools        []ToolSpec
    Events       EventSpec
    Manual       string      // 文件路径或内嵌 Markdown
}
~~~

~~~go
// sdk/host.go
type Host interface {
    ID() string
    Bus() Bus
    LLM() LLM
    Tools() ToolRegistry
    Memory() Memory
    Actions() ActionSink
    Storage(namespace string) (Storage, error)
    Config(section string, dst any) error
    Logger() *slog.Logger
    Metrics() Metrics       // 可选
    Scheduler() Scheduler   // 可选
    Clock() Clock
}
~~~

起步版可以只保留 8 个能力：Bus / LLM / Memory / Tools / Actions / Storage / Config / Clock；Scheduler、Metrics、Secrets 作为可选能力按需挂载。

**拆分原则**：Host 不要做成 30 个方法的上帝接口。按能力拆成小接口，插件只依赖自己需要的部分：

~~~go
type Bus interface {
    Publish(ctx context.Context, ev Event) error
    Subscribe(kind string, fn Observer) (unsubscribe func())
}
type LLM interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}
type ToolRegistry interface {
    Register(t Tool) error
    List() []ToolSpec
}
type Memory interface {
    Recall(ctx context.Context, q RecallQuery) ([]MemoryItem, error)
    Remember(ctx context.Context, item MemoryItem) error
}
type ActionSink interface {
    Send(ctx context.Context, a Action) error
}
~~~

插件通过 <code>Host</code> 获取这些能力，而不是互相 import。

### 5.4 契约：Tool

~~~go
// sdk/tool.go
type Tool struct {
    Name        string
    Summary     string          // 一行描述，常驻 prompt
    Schema      json.RawMessage // JSON Schema，参数校验
    Manual      string          // 详细手册，按需读取
    Permissions []string
    Timeout     time.Duration
    Execute     func(ctx context.Context, call ToolCall) (ToolResult, error)
}

type ToolCall struct {
    ID           string
    Name         string
    Arguments    json.RawMessage
    Conversation Conversation
    Actor        Actor
    TurnID       string
}

type ToolResult struct {
    Text  string
    Parts []Part   // 例如图片、文件
    Meta  map[string]string
}
~~~

**关键改动**：工具结果可以直接带 <code>Parts</code>，由内核统一渲染/发送。这样 <code>send_image</code> / <code>send_meme</code> 不再需要在 chat 插件里按名字特判：

~~~go
// tool: send_meme
return ToolResult{
    Text:  "表情包已发送",
    Parts: []Part{{Type: "image", URL: memeURL}},
}, nil
~~~

### 5.5 Manifest 草案

~~~yaml
# internal/plugins/adapter/onebot/manifest.yaml
api_version: 1
id: adapter.onebot
version: 0.1.0
kind: adapter
name: OneBot v11
description: QQ 群/私聊接入（NapCat / Lagrange）
provides:
  - adapter.onebot
  - conversation.qq
requires:
  - kernel.bus
  - kernel.actions
config_schema: ./config.schema.json
permissions:
  - network.outbound
  - messaging.send
events:
  emits:
    - message.created
    - notice.recalled
  consumes:
    - action.send
tools: []
manual: ./MANUAL.md
~~~

~~~yaml
# internal/plugins/tool/meme/manifest.yaml
api_version: 1
id: tool.meme
version: 0.1.0
kind: tool
name: 表情包
description: 收藏、检索、发送群聊表情包
provides:
  - tool.meme
  - sense.meme
requires:
  - kernel.tools
  - kernel.actions
config_schema: ./config.schema.json
permissions:
  - filesystem.workspace
  - messaging.send
tools:
  - name: meme_send
    summary: 从表情库随机发一张表情包
    schema: ./schemas/meme_send.json
    manual: ./manuals/meme_send.md
    permissions: [messaging.send]
  - name: meme_save
    summary: 把一张图收藏进表情库
    schema: ./schemas/meme_save.json
    manual: ./manuals/meme_save.md
    permissions: [filesystem.workspace]
manual: ./MANUAL.md
~~~

### 5.6 使用手册：给人看，也给模型看

**两套读者，一份源文件。**

| 读者 | 形式 | 何时可见 |
|---|---|---|
| 人（开发者/管理员） | <code>MANUAL.md</code>，WebUI 插件页渲染，CLI <code>icarosu plugin manual &lt;id&gt;</code> | 随时 |
| 模型 | <code>summary</code>（常驻）+ <code>manual</code>（按需） | summary 在工具清单；manual 通过 <code>read_manual</code> 工具读取 |
| 人格 | persona 手册里的“她会怎么用这个能力” | 组装 prompt 时由 persona 插件决定 |

推荐手册结构（CI 可 lint）：

~~~markdown
# <插件名>

## 是什么
## 什么时候用
## 什么时候不要用
## 参数
## 示例
## 失败模式
## 权限与隐私
## 版本与迁移
~~~

**渐进披露**：工具很多时，不要把全部手册塞进 system prompt。常驻只放 <code>summary</code>；模型需要时调用 <code>read_manual(tool_name)</code>。这既省 token，也让“她”像人一样“先知道有这个手艺，真要用时再翻说明书”。

### 5.7 配置 schema

- 每个插件自带 <code>config.schema.json</code>（JSON Schema）；
- 内核加载时校验 <code>configs/plugins/&lt;id&gt;.yaml</code> 的 <code>config</code> 段；
- WebUI 按 schema 自动渲染表单；
- 插件配置有独立 <code>version</code> 和迁移函数；
- 密钥走环境变量 / secret store，不写进 WebUI 可回显的 YAML；
- 热更新：插件实现 <code>Reloader</code> 才热更，否则标记“需重启”。

目标配置形态：

~~~yaml
# configs/kernel.yaml
kernel:
  instance_id: main
  data_dir: ./data
  log: { level: info }
  models:
    chat: { provider: openai, base_url: "...", model: "..." }
    vision: { provider: openai, base_url: "...", model: "..." }
  plugins:
    enabled:
      - adapter.onebot
      - storage.sqlite
      - memory.recent
      - persona.default
      - renderer.default
      - drive.attention
~~~

~~~yaml
# configs/plugins/adapter.onebot.yaml
version: 1
enabled: true
config:
  ws_url: ws://127.0.0.1:6700
  access_token: "{{ONEBOT_TOKEN}}"
~~~

~~~yaml
# configs/plugins/persona.default.yaml
version: 1
enabled: true
config:
  name: 小智
  nicknames: [小智, 智子]
  personality: "..."
~~~

### 5.8 生命周期与依赖

~~~text
discover → validate manifest → resolve requires → topological sort
→ init (并行/串行按依赖) → start → health loop → stop (逆序) → cleanup
~~~

规则：

- <code>requires</code> 必须能在「内核预置 capability + 已加载插件的 <code>provides</code>」中解析；解析不到则拒绝加载，<code>optional_requires</code> 可降级为 degraded；
- 不存在“<code>requires</code> 是自身 <code>provides</code> 子集”的规则——自己的 provides 不是自己的依赖；
- 检测循环依赖；
- 插件失败不应拖垮内核：记录健康状态，标记 degraded，继续跑其他插件；
- 关键插件（adapter、persona、memory）失败时，内核决定“降级运行”还是“拒绝启动”；
- 每个插件有启动超时、停止超时、健康检查间隔；
- 卸载时调用 <code>Stop</code>，并保留数据（数据清理是显式操作）。

### 5.9 装载方式：编译期 / 进程外 / WASM

| 方式 | 适用 | 优点 | 缺点 | 建议 |
|---|---|---|---|---|
| 编译期内嵌（Go 包） | 第一方插件、标准包 | 简单、快、类型安全、易调试 | 需重新编译；无法真正卸载 | **本期唯一方式** |
| 进程外（stdio JSON-RPC / HTTP） | 第三方插件、多语言 | 隔离、可重启、语言无关、可热插拔 | 协议/性能/部署复杂 | **暂缓（第一方阶段不做）** |
| MCP 桥接 | 工具类第三方能力 | 生态现成、协议简单 | 只管工具，不管 adapter/事件 | **暂缓** |
| WASM（wazero/extism） | 不可信小工具 | 沙箱、可移植 | 生态/网络/DB 能力弱 | **暂缓** |
| Go <code>plugin</code> 包 | —— | —— | 平台/工具链限制、不能卸载、崩溃拖垮进程 | **不推荐** |

**本期只做第一方编译期内嵌。** 不引入进程外协议/沙箱，但 manifest、权限、事件面按“未来可进程外”设计，避免以后推翻接口。

推荐路线：

1. **本期只执行这条**：第一方编译期内嵌 + 显式 bundle，**不要用 init() 全局注册**；
2. 以后：第三方工具走进程外 JSON-RPC，或 MCP server 桥接；
3. 以后：第三方 adapter 走进程外，内核监督；
4. 以后：热插拔只承诺“进程外插件可启停”；编译期插件只承诺“启用/禁用”，不承诺卸载。

### 5.10 权限与安全

**权限模型**（manifest 声明，Host 强制执行）：

| 权限 | 含义 | 例子 |
|---|---|---|
| <code>messaging.send</code> | 发消息/图片 | adapter、tool |
| <code>memory.read</code> / <code>memory.write</code> | 读写记忆 | memory、reflection |
| <code>network.outbound</code> | 出站网络 | search、vlm、adapter |
| <code>filesystem.workspace</code> | 读写工作区文件 | meme、log export |
| <code>llm.invoke</code> | 调模型 | chat、summarizer |
| <code>schedule</code> | 定时任务 | reminder、initiative |
| <code>admin</code> | 管理操作 | admin、webui |
| <code>secrets.read:&lt;name&gt;</code> | 读指定密钥 | adapter、provider |

第一阶段 manifest 的 <code>permissions</code> 可以先用字符串白名单；需要 scope 时升级为 <code>{name, scopes}</code>（例如 <code>message.send</code> 只允许发回触发会话），由 Host 在执行点校验。

**安全红线**：

- 外部内容（网页、图片 OCR、他人消息）是**不可信数据**，不能当指令；
- 工具结果要标注来源，进 prompt 时用“数据块”而不是“系统指令”；
- 危险工具（发消息、加群、踢人、文件写）默认需要管理员/配置允许；
- 每回合有工具调用次数、token、费用、时间预算；
- 进程外插件无环境变量继承，显式传配置/密钥；
- 审计日志记录：谁（插件）、在哪个会话、调了什么工具、参数摘要、结果摘要、耗时。

### 5.11 可观测性

每个回合一条 <code>Trace</code>：

~~~json
{
  "turn_id": "t_01J...",
  "event_id": "e_...",
  "conversation_id": "qq:group:123456",
  "actor_id": "qq:user:987654321",
  "decision": { "kind": "reply", "reason": "at_bot", "confidence": 0.92 },
  "blocks": [
    { "source": "persona", "tokens": 320 },
    { "source": "memory.semantic", "tokens": 180, "items": 5 },
    { "source": "topic", "tokens": 60 }
  ],
  "llm": [
    { "round": 1, "tools": ["time_now"], "latency_ms": 820, "tokens": 600 }
  ],
  "tools": [
    { "name": "time_now", "latency_ms": 30, "ok": true }
  ],
  "actions": [
    { "adapter": "onebot", "parts": 1, "ok": true }
  ],
  "latency_ms": 1240,
  "cost_usd": 0.0004,
  "errors": []
}
~~~

用途：

- WebUI “回合查看器”：她为什么回/不回、看了哪些记忆、调了什么工具；
- 调试“她为什么突然变助手腔”；
- 评估上下文预算、工具爆炸、慢插件；
- 回放/回归测试（golden trace）。

### 5.12 插件作者清单

发布一个插件前，至少要有：

- [ ] <code>manifest.yaml</code> manifest（id/version/api_version/kind/provides/requires/optional_requires/permissions）
- [ ] <code>config.schema.json</code>（含默认值、校验、敏感字段标记）
- [ ] <code>MANUAL.md</code>（人）+ 每个 tool 的 <code>summary</code> / <code>manual</code>（模型）
- [ ] <code>Init/Start/Stop/Health</code> 生命周期
- [ ] 所有副作用走 Host 能力，不直接 import 内核/其他插件
- [ ] 超时、取消、重试、错误降级
- [ ] 结构化日志（带 plugin id / turn id）
- [ ] 指标（调用次数、失败率、延迟）
- [ ] 契约测试（fake Host / fake LLM / fake adapter）
- [ ] 权限最小化说明
- [ ] 数据迁移/清理说明
- [ ] 版本兼容声明（<code>api_version</code>）

---

## 6. 目标目录结构

~~~text
Icarosu/
├─ cmd/icarosu/main.go              # flag → kernel.New → bundle.Load → Run
├─ internal/
│  ├─ kernel/                       # 微内核，不 import 任何 plugin
│  │  ├─ kernel.go                  # 生命周期、Host 实现
│  │  ├─ registry.go                # 显式注册、依赖解析
│  │  ├─ capabilities.go            # Provider Registry / 单例仲裁 / facade
│  │  ├─ lifecycle.go               # Init/Start/Stop/Health
│  │  ├─ bus.go                     # 事件总线、观察者、背压
│  │  ├─ scheduler.go               # 会话队列 / worker pool
│  │  ├─ turn.go                    # 回合引擎
│  │  ├─ actions.go                 # Action → adapter 路由
│  │  ├─ trace.go                   # Trace / 审计
│  │  └─ config.go                  # 内核配置 + 插件配置挂载
│  ├─ sdk/                          # 插件公共契约，零项目内依赖
│  │  ├─ event.go action.go plugin.go host.go
│  │  ├─ tool.go decision.go trace.go errors.go
│  │  └─ testkit/                   # fake host / fake llm / contract test
│  ├─ infra/                        # 通用基础设施（非领域）
│  │  └─ log/ metrics/ sqlite/ httpx/ yamlx/
│  └─ plugins/                      # id = <kind>.<name>
│     ├─ adapter/onebot/
│     ├─ storage/sqlite/
│     ├─ sense/vision/
│     ├─ memory/recent/
│     ├─ memory/longterm/
│     ├─ persona/default/
│     ├─ renderer/default/
│     ├─ drive/attention/
│     ├─ tool/time/
│     ├─ tool/meme/
│     ├─ reflection/summarizer/
│     ├─ governance/safety/
│     └─ interface/webui/
├─ bundles/
│  └─ standard/bundle.go            # 标准包：显式列出默认插件
├─ docs/
│  ├─ ARCHITECTURE.md               # 本文
│  └─ plugins/&lt;id&gt;/MANUAL.md        # 插件手册（或随插件包）
├─ configs/
└─ data/                            # 运行期数据（不入库）
~~~

### 6.1 依赖规则

必须在 CI 里用测试守住：

~~~text
sdk        → 只依赖标准库（可选：通用第三方）
kernel     → sdk + infra + 标准库
plugins/*  → sdk + infra + 标准库 + 自己声明的第三方依赖（消费型）
bundles/*  → kernel + plugins/*
cmd/*      → kernel + bundles/*
~~~

**禁止**：

- <code>kernel</code> import <code>plugins/*</code>；
- <code>plugins/a</code> import <code>plugins/b</code>；
- <code>sdk</code> import <code>kernel</code> / <code>plugins</code>；
- <code>plugins/*</code> 直接 import <code>internal/store</code> / <code>internal/qq</code> / <code>internal/config</code>。

**能力提供型插件例外**：adapter/storage 可以在自己的包内封装协议/驱动（例如把 <code>qq.Client</code> 移进 <code>adapter/onebot</code>），但不能把实现类型暴露给其他插件。消费型插件仍只能走 Host。

用 <code>go list</code> 写一个架构测试，比口头约定可靠。

### 6.2 标准包

<code>bundles/standard/bundle.go</code> 显式列出默认插件：

~~~go
// manifest 由 New() 返回；bundle 只决定“装哪些、哪些关键”。
func Plugins() []sdk.PluginSpec {
    return []sdk.PluginSpec{
        {New: onebot.New,     Critical: true},  // adapter.onebot
        {New: sqlite.New,     Critical: true},  // storage.sqlite
        {New: recent.New,     Critical: true},  // memory.recent
        {New: longterm.New,   Critical: false}, // memory.longterm
        {New: persona.New,    Critical: true},  // persona.default
        {New: renderer.New,   Critical: true},  // renderer.default
        {New: attention.New,  Critical: true},  // drive.attention
        {New: basic.New,      Critical: false}, // tool.basic
        {New: meme.New,       Critical: false}, // tool.meme
        {New: summarizer.New, Critical: false}, // reflection.summarizer
        {New: safety.New,     Critical: false}, // governance.safety
        {New: webui.New,      Critical: false}, // interface.webui
    }
}
~~~

显式列表的好处：

- 测试可控（不会因为某个包 init 就偷偷注册）；
- 依赖顺序可读；
- 分支/构建标签可以裁剪标准包；
- 没有全局可变状态。

---

## 7. 迁移路线

> 原则：**绞杀者模式**。不推倒重来，每一步都能跑、能测、能回滚。  
> 每个阶段结束都应该保持 <code>go build ./... &amp;&amp; go test ./...</code> 通过。

### Phase 0：立规矩（小）

**目标**：不改变行为，先把边界和文档立起来。

- [ ] 新增 <code>docs/ARCHITECTURE.md</code>（本文）；
- [ ] 修正 README 的插件注册说明，或改成“先看 app.go”；
- [ ] 修复 <code>internal/webui/server.go</code> 的乱码注释；
- [ ] 新增 <code>internal/sdk</code> 空壳（Event/Action/Plugin/Host 接口草案）；
- [ ] 新增架构测试：扫描 <code>go list</code> 依赖，禁止 kernel/sdk 反向依赖；
- [ ] 明确“标准包 = 产品”的概念，避免为插件化而插件化。

**验收**：行为不变，测试全绿，架构测试能拦住违规 import。

### Phase 1：中立模型 + OneBot 适配器插件（中）

**目标**：把“平台”从核心剥出去，但先不换聊天逻辑。

- [ ] 在 <code>sdk</code> 定义 <code>Event</code> / <code>Action</code> / <code>Adapter</code>；
- [ ] 新建 <code>plugins/adapter/onebot</code>，包装现有 <code>qq.Client</code>；最终把 qq 实现移进 adapter 包，其他包不再 import <code>internal/qq</code>；
- [ ] <code>handler</code> 变成 OneBot 适配器内部的 normalizer：平台事件 → <code>sdk.Event</code>；
- [ ] 内核先把 <code>sdk.Event</code> 转成旧 <code>plugin.Event</code>，继续喂现有流水线（绞杀者）；
- [ ] <code>qq</code> 包只被 adapter 引用；
- [ ] 增加 adapter 契约测试：连接、重连、发送、图片、@、引用。

**验收**：现有群聊行为不变；<code>grep -r "internal/qq"</code> 只出现在 adapter；dry-run 回放一致。

### Phase 2：回合内核 + 默认器官（大）

**目标**：让聊天、人格、记忆、决策、工具都跑在新内核上。

- [ ] <code>kernel.Turn</code>：观察 → 评估 → 回忆 → 决定 → 生成 → 动作；
- [ ] <code>ContextProvider</code> 接口 + 预算器；
- [ ] <code>Decider</code> 接口（回复/静默/延迟/观察）；
- [ ] <code>ToolRegistry</code> + 新 <code>Tool</code> 契约（含 Parts/权限/超时/手册）；
- [ ] 迁移 chat / persona / memory / trigger / sender 到新插件；
- [ ] <code>send_image</code> / <code>send_meme</code> 改为 ToolResult.Parts，删掉特判；
- [ ] 后台观察者总线：recorder/steal/summarizer/stylelearn/jargonlearn 异步化；
- [ ] 每回合 Trace；
- [ ] 旧 <code>plugin.Manager</code> 只保留给尚未迁移的插件。

**验收**：

- 回复行为与旧版对齐（golden dialogue 测试）；
- 图片消息不再阻塞“是否接话”；
- p95 回复延迟不高于基线（慢任务移出后应有下降）；
- 内核零 <code>plugins/*</code> import；
- 任意单个 reflection 插件失败不影响回复。

### Phase 3：全量插件化 + 手册/权限（中）

**目标**：所有能力都有 manifest、schema、manual、权限。

- [ ] filter / safety / membersync / admin / ping / replylog 迁移；
- [ ] WebUI 变成 interface 插件，支持插件页/手册页/Trace 页；
- [ ] 每个插件补 <code>manifest.yaml</code> + <code>config.schema.json</code> + <code>MANUAL.md</code>；
- [ ] 内核权限执行器 + 审计日志；
- [ ] 插件健康检查、启停、降级；
- [ ] WebUI 按 schema 渲染插件配置；
- [ ] 文档生成：<code>docs/plugins/&lt;id&gt;/MANUAL.md</code>。

**验收**：新增一个工具插件不需要改内核/配置结构；插件页能看到 manifest/manual/health；权限测试覆盖发消息/读记忆/网络。

### Phase 4：多平台 / 多实例（第一方，延后）

**目标**：在“只做第一方”的前提下，验证平台中立与实例隔离。

- [ ] 第二个第一方 adapter（Telegram / CLI）验证 <code>sdk.Event/Action</code>；
- [ ] 内核实例化：一个进程多 profile / 多 persona；
- [ ] 可选：把某个 adapter 拆成独立二进制（仍是第一方），验证进程边界；
- [ ] **暂缓**：第三方插件、MCP、WASM、插件市场（等第一方 API 稳定后再评估）。

**验收**：不改内核即可接一个新平台；同进程多实例互不污染；不承诺第三方插件生态。

### 7.1 迁移中的行为对齐

- **Golden dialogue**：沉淀 30~50 组群聊片段 + 期望行为（回/不回/长度/语气），每次迁移跑一遍；
- **Shadow mode**：新旧两条链路同时跑，只发旧的，比较决策与回复；
- **Dry-run**：现有 <code>dryrun.Bot</code> 保留，作为安全网；
- **Feature flag**：<code>kernel.enabled</code> / 按插件切换，出问题可回退；
- **数据库兼容**：新插件复用现有表，或加命名空间表；迁移只加不删。

### 7.2 风险

| 风险 | 说明 | 缓解 |
|---|---|---|
| 过度抽象 | 接口太多、插件太小，开发变慢 | 先迁移真实插件，再抽象；一个插件至少带来 2~3 个能力才独立 |
| 行为回归 | 新回合引擎改变回复风格/时机 | golden dialogue + shadow mode + feature flag |
| 性能下降 | 总线/接口/序列化开销 | 快路径零拷贝/少分配；基准测试；只对慢任务异步 |
| 配置不兼容 | 插件 schema 与旧 config 冲突 | 配置迁移器 + 旧字段兼容层 |
| 安全 | 插件越权（第一方也要防 LLM 诱导） | 权限 manifest + Host 中介 + 审计；进程隔离留到第三方阶段 |
| 生态早产 | 没有稳定 API 就做市场 | 先第一方插件，后第三方；API 版本化 |
| 人格稀释 | 插件越多越像工具平台 | 标准包 + persona 回归测试 + 定位红线 |
| 单人维护 | 架构复杂度超过人力 | 分阶段，每阶段可停；不追求一步到位 |

### 7.3 最该先做的三件事

如果只能做三件：

1. **把平台抽象做真**：<code>sdk.Event/Action/Adapter</code> + OneBot adapter 插件；
2. **把慢任务移出热路径**：recorder/steal/summarizer/stylelearn/jargonlearn 异步化；
3. **把插件契约立起来**：manifest + tool 契约 + Host 能力 + Trace。

这三件做完，后面的插件化是“搬家”；不做这三件，后面的插件化是“重建”。

---

## 8. 不要做的事

1. **不要用 Go 标准库 <code>plugin</code> 做第三方插件**：平台/工具链限制、不能卸载、崩溃拖垮主进程。
2. **不要把所有东西都做成插件**：内核的回合、总线、调度、动作分发必须稳定；插件是能力，不是微模块。
3. **不要让插件互相 import**：共享契约放 <code>sdk</code>，协作走 Bus/Host。
4. **不要让插件随意改 system prompt**：上下文块要有来源、预算、优先级；人格只由 persona 插件维护。
5. **不要把所有任务都异步**：同会话必须串行；异步只用于反思/学习/识图等非回复关键路径。
6. **不要先做插件市场/安装器**：没有稳定 API 和第一批真实插件，市场是空壳。
7. **不要为了多平台而多平台**：先把 QQ 群聊体验做好；平台抽象做对，接第二个平台才便宜。
8. **不要让“工具多”成为产品目标**：她的价值是像人，不是像工具箱。
9. **不要一次性重写**：绞杀者模式，每一步可回滚。
10. **不要丢掉 dry-run、测试、日志**：这些是迁移期的安全带。

---

## 9. 需要拍板的决策

这几个问题会显著影响架构，建议先定：

> 已定：**先做第一方插件**。下面是剩余的收敛问题。

1. **第一方阶段要不要为第三方留接口？**
   - 留（推荐）：manifest、permissions、Event/Action 按可进程外设计，但不实现宿主；
   - 不留：接口可以更简单，但未来接第三方要改协议。

2. **要不要多平台/多实例？**
   - 只做 QQ → 可以先保留 int64 ID，但仍建议平台中立事件；
   - 要多平台/多角色 → 内核必须实例化，ID 必须命名空间化。

3. **WebUI 是产品还是开发工具？**
   - 产品 → 做成 interface 插件，认真设计权限/审计；
   - 开发工具 → 可以保持轻量，不急于插件化。

4. **“她”的状态要做到多深？**
   - 轻：persona + 记忆 + 注意力（当前方向）；
   - 重：情绪、关系图、目标、主动计划、自我叙事（需要 scheduler/state store）。

5. **运行时装载什么时候做？**
   - 暂不排期。第一方 API 稳定、至少有两个真实 adapter / 一批工具之后，再评估进程外；
   - 先做会拖慢核心迁移，且没有真实需求验证协议。

**建议**：第一方插件 + 编译期 bundle + manifest/manual + 平台中立，先把标准包跑稳；第三方/运行时装载不设时间表。

---

## 附录 A：当前插件与阶段顺序

~~~text
Preprocessor（从先到后）
  filter(1000) → safety(990) → recorder(950) → steal(940)
  → membersync(930) → summarizer(920) → stylelearn(910)
  → jargonlearn(905) → trigger(900)

Responder（从先到后，首个 true 截断）
  admin(800) → ping(100) → chat(50)

Postprocessor（从先到后）
  sender(10) → replylog(0)
~~~

## 附录 B：当前依赖违规清单（建议逐步清零）

| 违规 | 位置 | 目标 |
|---|---|---|
| 核心依赖具体记忆/存储 | <code>core/chat</code> → <code>memory</code>, <code>store</code> | 只依赖 <code>sdk.Memory</code> / 窄接口 |
| 记忆反向依赖插件 | <code>memory</code> → <code>plugin</code> | 用 <code>sdk.Event</code> / 领域模型 |
| 平台抽象依赖具体平台 | <code>platform</code> → <code>qq</code> | <code>sdk.Event/Action</code> |
| 接入层绑定 OneBot | <code>handler</code> → <code>qq</code> | OneBot adapter 内部 |
| 存储依赖配置 | <code>store</code> → <code>config</code> | 存储只接收 <code>StoreConfig</code> 值对象 |
| 界面依赖具体插件 | <code>webui</code> → <code>plugin/summarizer</code> | interface 插件 + 窄接口 |
| 组装根依赖一切 | <code>app</code> → 几乎所有包 | bundle + kernel |

## 附录 C：术语

| 术语 | 含义 |
|---|---|
| 内核 / Kernel | 回合、总线、调度、工具循环、动作分发、Trace |
| 插件 / Plugin | 带 manifest 的能力包，通过 Host 扩展内核 |
| 器官 / Organ | 感官、记忆、人格、意愿、反思等“像人的部分” |
| 手艺 / Tool | LLM 可调用的动作，是插件暴露给模型的能力 |
| Manifest | 插件清单：id、版本、kind、依赖、权限、配置、手册 |
| Host | 内核给插件的能力入口，所有副作用的中介 |
| Adapter | 平台适配器插件，负责 Event/Action ↔ 平台 API |
| Turn | 一次“观察 → 决定 → 说/做 → 发”的完整回合 |
| Trace | 一回合的可回放记录 |
| Standard Bundle | 标准器官包，等于“产品默认形态” |

## 附录 D：对标研究与来源

完整研究见 [docs/research/plugin-arch.md](research/plugin-arch.md)（238 行，含 25 条来源 URL，正文用 ✅ 有来源 / 🔶 推断 标注）。

最相关的来源：

- Koishi — Plugin / Service / Schema / Lifecycle  
  https://koishi.chat/en-US/guide/plugin/  
  https://koishi.chat/en-US/guide/plugin/service.html  
  https://koishi.chat/en-US/guide/plugin/schema.html  
  https://koishi.chat/en-US/guide/plugin/lifecycle.html
- NoneBot2 — Driver / Plugin Metadata / Matcher  
  https://nonebot.dev/docs/advanced/driver  
  https://nonebot.dev/docs/advanced/plugin-info  
  https://nonebot.dev/docs/tutorial/matcher
- AstrBot — 插件开发 / 配置 schema  
  https://docs.astrbot.app/dev/star/plugin-new.html  
  https://docs.astrbot.app/dev/star/guides/plugin-config.html
- LangBot — Manifest / Tool / Plugin Runtime  
  https://docs.langbot.app/en/plugin/dev/basic-info.md  
  https://docs.langbot.app/en/plugin/dev/components/tool.md  
  https://docs.langbot.app/en/develop/plugin-runtime.md
- MaiBot — 插件开发 / Manifest（capabilities）  
  https://docs.mai-mai.org/plugin/  
  https://docs.mai-mai.org/plugin/manifest
- MCP — Architecture / Tools / Resources / Prompts  
  https://modelcontextprotocol.io/docs/learn/architecture  
  https://modelcontextprotocol.io/specification/2026-07-28/server/tools
- Go 标准库 plugin 官方警告  
  https://pkg.go.dev/plugin
- hashicorp/go-plugin / wazero / Extism  
  https://github.com/hashicorp/go-plugin  
  https://wazero.io/  
  https://extism.org/docs/concepts/plug-in

> 对标结论有时间窗口；落地前应重新核对版本与文档。

---

> **最后一句**：  
> 插件化不是为了让她“什么都能装”，而是为了让她“换一种活法也不用重新投胎”。  
> 底座要小，器官要真，身份要稳，手册要写给人看，也要写给她看。


