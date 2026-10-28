# Icarosu 内核规格（第一方插件版）

> 版本：v0.1（2026-10-25）  
> 前置：[docs/ARCHITECTURE.md](ARCHITECTURE.md)  
> 范围：只讨论**第一方插件**；第三方运行时装载、MCP、WASM、插件市场不在本期。  
> 一句话：**内核只保留“所有插件都依赖、且必须一致”的东西；其余全部下沉为插件。**

配套文档：

- [docs/ARCHITECTURE.md](ARCHITECTURE.md)：定位、现状、目标全景
- [docs/RULES.md](RULES.md)：开发规矩（Constitution）
- [docs/PLUGIN_DEV.md](PLUGIN_DEV.md)：插件开发推荐（Go / WASM / 脚本 / 协议）
- [docs/PLAN.md](PLAN.md)：Go SDK 插件化整体计划（M0–M5）
- [docs/research/plugin-arch.md](research/plugin-arch.md)：业界对标

**v0.2 修正（红队评审）**：

- <code>requires</code> 不再是“自身 provides 的子集”，改为在 capability registry 中解析（见 2.6.1 / 4.0）。
- Host 能力来源、单例仲裁、启动顺序定死；storage/memory 由插件提供，内核不假装自带。
- Event/Action/Decision 补身份字段；Decision 改为 <code>reply/silent/observe/defer</code> 枚举。
- 统一 plugin id / tool 名 / bundle / 配置路径。
- 3.0 补充 WebUI auth、Pool 竞态、SQLite 并发、notice 锁等真实 P0。
- 评审原始记录：[docs/research/kernel-review-v0.2.md](research/kernel-review-v0.2.md)。

---

## 0. 先定边界：三问法

任何东西要放进内核，先过三问：

1. **所有插件都依赖它吗？** 只有一两个插件用 → 插件。
2. **它涉及跨插件的一致性、安全或生命周期吗？** 是 → 内核。
3. **它与具体平台 / 具体业务无关吗？** 是 → 内核；否 → 插件。

三个都“是”才进内核。不确定时，**先放插件**；等第二个使用方出现，再上提为内核能力。这样内核只会因为真实复用而长大，不会因为设计想象力而膨胀。

### 0.1 删除测试

> 删掉所有插件，内核还能编译、启动、报告“没有 adapter / 没有 persona”、健康检查为 degraded，并干净退出。

如果做不到，说明某样业务/平台/人格的东西漏进了内核。

### 0.2 通用基础设施 vs 领域能力

“插件化”不等于“把 SQLite 驱动也做成插件”。建议：

| 可以留在内核 | 必须是插件 |
|---|---|
| 日志、指标、Trace 基础设施 | QQ / OneBot 协议与重连 |
| 配置加载、JSON Schema 校验 | 人格文案、说话风格 |
| 通用 HTTP 客户端 | 记忆表结构、召回算法 |
| 通用 SQLite 驱动 | 群话题、黑话、关系模型 |
| 时钟、ID、随机源 | 触发/注意力策略 |
| 密钥解析与环境变量注入 | 具体工具（时间、表情、搜索） |
| 文件路径/数据目录约定 | WebUI 页面与业务查询 |
| 队列/调度/超时原语 | 平台消息解析（CQ 段等） |

原则：**基础设施可以内核，领域行为必须插件。** 内核提供“插座”，不提供“电器”。

---

## 1. 必须留在内核的 7 个职责

### 1.1 总表

| # | 职责 | 一句话 | 不做什么 |
|---|---|---|---|
| K1 | Host / 生命周期 | 插件注册、manifest 校验、依赖排序、Init/Start/Stop/Health、配置挂载、存储命名空间 | 不实现插件业务 |
| K2 | 契约 / 适配器路由 | Event / Action / Part / Conversation / Actor / Tool 契约；把 Action 路由给正确 adapter | 不解析 QQ / CQ 码 |
| K3 | 调度 / 并发 / 背压 | 同会话串行、跨会话并行、队列上限、去重、超时、取消、优雅关闭 | 不决定“该不该回” |
| K4 | 回合引擎 | 收集 context block、预算裁剪、调用 Decider、组织 prompt、驱动 LLM 工具循环 | 不写人格、不实现记忆 |
| K5 | 工具执行 | 注册、命名空间、参数校验、权限、超时、结果截断、审计 | 不实现具体工具 |
| K6 | 治理 | 权限、预算、限流、错误分类、panic 恢复、审计 | 不写业务过滤词 |
| K7 | 可观测 | 每回合 Trace、结构化日志、指标、健康/就绪 | 不做 WebUI 页面 |

> K1 负责“聚合/决策”（哪些插件活着、能否启动），K7 负责“采集/暴露”（Trace/日志/指标/探针）；不重复实现。

### 1.2 K1：Host / 生命周期

**输入**：一组显式声明的插件（第一方 bundle）、内核配置、数据目录。  
**输出**：已初始化/已启动的插件集合、健康状态、可用的 Host 能力。

必须做：

- 校验 manifest：id、version、api_version、kind、provides / requires、config schema、manual。
- 拓扑排序依赖，检测环；关键插件失败则拒绝启动，非关键失败则 degraded。
- 按依赖顺序 Init/Start，逆序 Stop；每步有超时；panic 要 recover。
- 给插件注入 Host 能力、插件配置、存储命名空间、logger、metrics。
- 管理插件启用/禁用（第一方：停插件 + 摘工具，不卸载代码）。
- 提供 health/readiness 聚合。

不做：不读 QQ 消息、不写人格、不实现记忆。

### 1.3 K2：契约 / 适配器路由

**输入**：adapter 产出的 <code>Event</code>；插件产出的 <code>Action</code>。  
**输出**：路由到目标 adapter 的 Action；统一格式的 Event。

必须做：

- 定义平台中立的 <code>Event</code> / <code>Action</code> / <code>Part</code> / <code>Conversation</code> / <code>Actor</code>。
- 规定 ID 命名空间：<code>qq:group:123456</code>、<code>qq:user:987654321</code>。
- 校验 Event 基本字段；未知 kind 拒绝并记录。
- 按 <code>Conversation.AdapterID</code> 选择 adapter；检查 adapter capabilities。
- 提供 adapter 注册/注销；一个会话只属于一个 adapter。
- Action 出站前做基本校验（非空、part 类型、大小、目标会话）；平台转义/分段由 adapter 负责。

不做：不解析 platform payload；不拼 CQ 码；不处理重连（adapter 的事）。

### 1.4 K3：调度 / 并发 / 背压

**输入**：已验证的 Event。  
**输出**：按会话串行的 turn 执行；背景观察者任务。

必须做：

- 每个 conversation 一条队列，同一时刻最多一个 running turn；不同 conversation 并行。
- 全局 worker pool 有上限；就绪队列公平调度，避免慢群饿死其他群。
- 队列有界；满时明确策略：拒绝新事件 / 合并同作者连续消息 / 丢弃低优先级 + 指标。
- 入站去重：adapter event ID + TTL；重复事件不重复回复、不重复写记忆。
- 每条插件调用都包 <code>ctx</code> + timeout + recover。
- 支持取消（关闭、超预算）与优雅关闭（停接入 → 有界排空 → 逆序停插件）。
- 支持可选的 burst coalescing（同一会话短时间内多条消息合并成一个 turn），让回复更像人而不是逐条回。

不做：不做“是否回复”的业务判断（Decider 的事）。

### 1.5 K4：回合引擎

**输入**：一个 Event + 当前人格 + 记忆/上下文提供者 + 工具清单。  
**输出**：<code>TurnResult</code>（Decision、Actions、Trace）。

必须做：

- 调用同步 <code>TurnHook.BeforeTurn</code>（记录当前消息、更新注意力），带小预算。
- 收集 <code>ContextProvider</code> 的 block，按优先级 + token 预算裁剪，记录来源与丢弃。
- 调用唯一 <code>Decider</code>，得到 <code>Decision{Kind, Reason, Confidence, Defer}</code>；<code>Kind ∈ reply/silent/observe/defer</code>。
- 不回复时写 Trace，结束；不产生 Action。
- 回复时组装 messages：固定 persona 前缀 + 动态 block + 最近消息 + 当前消息 + 工具定义。
- 内核持有 <code>LLM</code> port 和默认 OpenAI-compatible 客户端；其他 provider 后续可插件化。
- 确保一个实例只有一个 active <code>Persona</code>；没有 persona 拒绝启动，多于一个拒绝启动。
- 调 LLM，执行工具循环；工具调用受权限/超时/轮次/结果大小限制。
- 调用 <code>Renderer</code> 生成 0..N 个 Action；交给 K2 分发。
- 调 <code>TurnHook.AfterTurn</code>（记录回复、更新注意力），异步触发反思。

不做：不写 system prompt 文案（Persona 插件提供）、不实现记忆召回（Memory 插件提供）、不实现触发策略（Decider 插件提供）。

### 1.6 K5：工具执行

**输入**：LLM 的 tool call。  
**输出**：<code>ToolResult</code>（文本 + Parts + Meta）。

必须做：

- 工具名全局唯一；建议 <code>plugin_tool</code> 下划线命名，兼容 OpenAI function name 限制。
- 按 JSON Schema 校验参数；拒绝未知/多余/类型错误参数（或显式允许）。
- 权限检查：manifest 声明 + 当前会话/角色 scope。
- 超时、取消、结果大小上限、错误分类。
- 审计：谁、哪个 turn、什么参数摘要、耗时、结果摘要。
- 工具结果进入 prompt 前加 provenance（<code>tool:meme/send</code>），并标记 untrusted。
- 媒体结果用 <code>Parts</code> 返回，由内核统一发送；工具不直接调 adapter。
- 内置 <code>read_manual</code> 工具：按 manifest/工具注册表返回手册，只允许读已注册手册，不允许读任意文件。它是**内核原语**，不是业务工具，不违反“具体工具下沉插件”。

不做：不实现具体工具；不替工具决定业务语义。

### 1.7 K6：治理

**输入**：Event / Decision / ToolCall / Action。  
**输出**：允许/拒绝/降级；审计记录。

必须做：

- 权限模型：能力 + scope（会话、工具、插件），未声明即拒绝。
- 预算：每 turn 最大工具轮次、工具数、token、费用、时间；超预算降级（不再调工具/直接收尾）。
- 限流：按会话、按 adapter、按工具；出站也要限。
- panic 恢复：插件调用、hook、observer 都要 recover；不拖垮内核。
- 错误分类：配置/依赖/超时/取消/权限/预算/LLM/工具/适配器/限流。
- 审计：工具调用、危险动作、权限拒绝、插件启停。
- dry-run 统一在 <code>ActionSink</code> 层拦截；插件不各自判断“是否真的发”。

不做：不写具体安全关键词（Safety 插件的事）；不写管理员命令。

### 1.8 K7：可观测

**输入**：turn / plugin / tool / adapter 事件。  
**输出**：Trace、日志、指标、健康状态。

必须做：

- 每 turn 一条 Trace：event、decision(reason)、context blocks(source/tokens)、LLM rounds、tool calls、actions、latency、cost、errors。payload 默认 hash/截断，完整内容只在 debug + 采样时记录。
- 结构化日志统一字段：<code>turn_id</code>、<code>conversation_id</code>、<code>plugin_id</code>、<code>event_id</code>。
- 指标：turn 数/耗时/队列深度/插件健康/LLM token/工具调用/发送失败/限流。
- <code>/healthz</code>（进程活着）、<code>/readyz</code>（关键插件可用）、<code>/metrics</code>。
- 日志/trace 默认脱敏；消息内容只在 debug 或显式开启时记录。

不做：不做 WebUI 页面；不做业务统计报表。

---

## 2. 内核模块与接口

### 2.1 包结构

~~~text
Icarosu/
├─ cmd/icarosu/main.go             # 只 import bundles/standard
├─ internal/
│  ├─ sdk/                         # 契约，零项目内依赖
│  │  ├─ event.go action.go part.go ids.go
│  │  ├─ plugin.go manifest.go host.go
│  │  ├─ decision.go context.go tool.go
│  │  ├─ trace.go health.go errors.go
│  │  └─ testkit/                  # fake host / llm / adapter / clock
│  ├─ kernel/                      # 微内核，不 import 任何 plugin
│  │  ├─ kernel.go                 # New / Run / Stop / Health
│  │  ├─ registry.go               # 显式注册、manifest、依赖解析
│  │  ├─ capabilities.go           # Provider Registry / 单例仲裁 / facade
│  │  ├─ lifecycle.go              # Init/Start/Stop/Health
│  │  ├─ bus.go                    # 事件总线 + 观察者
│  │  ├─ scheduler.go              # 会话队列、worker pool、背压
│  │  ├─ turn.go                   # 回合引擎
│  │  ├─ context.go                # context block、预算、裁剪
│  │  ├─ tools.go                  # 工具注册/校验/执行/审计
│  │  ├─ actions.go                # Action 路由、能力检查、发送
│  │  ├─ permissions.go            # 权限/scope/审计
│  │  ├─ config.go                 # 内核配置 + 插件配置挂载
│  │  ├─ storage.go                # 存储命名空间 + 迁移 runner
│  │  ├─ trace.go                  # Trace 收集/导出
│  │  ├─ health.go                 # 健康/就绪
│  │  └─ clock.go ids.go
│  ├─ infra/                       # 通用基础设施（非领域，零项目依赖）
│  │  └─ log/ metrics/ sqlite/ httpx/ yamlx/ jsonx/
│  └─ plugins/                     # 第一方插件，id = <kind>.<name>
│     ├─ adapter/onebot/
│     ├─ storage/sqlite/
│     ├─ memory/recent/
│     ├─ memory/longterm/
│     ├─ persona/default/
│     ├─ renderer/default/
│     ├─ drive/attention/
│     ├─ tool/basic/
│     ├─ tool/meme/
│     ├─ reflection/summarizer/
│     ├─ governance/safety/
│     └─ interface/webui/
├─ bundles/standard/bundle.go      # 显式列出标准包
├─ configs/
│  ├─ kernel.yaml
│  └─ plugins/<id>.yaml
└─ docs/
~~~

### 2.2 核心类型（草案）

~~~go
// sdk/event.go
type Event struct {
    ID          string
    Kind        string      // message.created / notice.recalled / timer.fired ...
    OccurredAt  time.Time
    Source      SourceRef   // AdapterID / Platform
    Conversation Conversation // ID / AdapterID / Kind / ...
    Actor       Actor
    Self        ActorID     // 这条事件所属的 bot 身份
    Parts       []Part
    ReplyTo     string
    Mentions    []ActorID
    Raw         json.RawMessage // 仅 adapter/调试用
}

// sdk/action.go
type Action struct {
    ID             string
    TurnID         string
    Conversation   Conversation
    Actor          ActorID   // 发送者（通常是 Self）
    Parts          []Part
    ReplyTo        string
    IdempotencyKey string
    Silent         bool
}

// sdk/turn.go
type Turn struct {
    ID           string
    Event        Event
    Persona      Persona
    Blocks       []ContextBlock
    Tools        []ToolSpec
    Decision     Decision
    Actions      []Action
    Trace        *Trace
}

type DecisionKind string

const (
    DecisionReply   DecisionKind = "reply"
    DecisionSilent  DecisionKind = "silent"   // 看见了，选择不回
    DecisionObserve DecisionKind = "observe"  // 不是在说我，不进入回合
    DecisionDefer   DecisionKind = "defer"    // 稍后再看
)

type Decision struct {
    Kind       DecisionKind
    Reason     string
    Confidence float64
    Defer      *Deferral // 仅 Kind=defer 时有效
}

type ContextBlock struct {
    Source     string   // persona / memory.semantic / topic / profile / ...
    Role       string   // system / user / assistant
    Content    string
    Tokens     int
    Priority   int
    Cacheable  bool
    Provenance []string
}

type Persona struct {
    ID           string
    Name         string
    SystemPrompt string
    StyleRules   []string
    Boundaries   []string
    Examples     []string
}

type TurnResult struct {
    TurnID   string
    Decision Decision
    Actions  []Action
    Trace    *Trace
}

type Deferral struct {
    After       time.Duration
    Reason      string
    CancelKey   string // 同一事件/话题的 defer 去重键
    MaxAttempts int    // 防止无限延期
}
~~~

~~~go
// sdk/host.go（起步版 8 个能力，按需扩展）
type Host interface {
    ID() string
    Bus() Bus
    LLM() LLM
    Memory() Memory
    Tools() ToolRegistry
    Actions() ActionSink
    Storage(namespace string) (Storage, error)
    Config(section string, dst any) error
    Logger() *slog.Logger
    Clock() Clock
}
~~~

~~~go
// sdk/extension.go
type Decider interface {
    Decide(ctx context.Context, t *Turn) (Decision, error)
}

type ContextProvider interface {
    ID() string
    Priority() int
    Budget() int // 0 = 不单独限，走全局预算
    Provide(ctx context.Context, t *Turn) ([]ContextBlock, error)
}

type TurnHook interface {
    BeforeTurn(ctx context.Context, t *Turn) error
    AfterTurn(ctx context.Context, t *Turn, res TurnResult) error
}

type Renderer interface {
    Render(ctx context.Context, t *Turn, reply string) ([]Action, error)
}
~~~

**说明**：

- <code>Host</code> 是能力的聚合入口，但每个能力自己是窄接口；不要在一个接口里塞 30 个方法。
- 第一方阶段先实现 8 个能力：Bus / LLM / Memory / Tools / Actions / Storage / Config / Clock；Logger 通过 Host 注入。
- Scheduler / Metrics / Secrets 作为可选能力按需注入，避免“上帝 Host”。

### 2.3 回合状态机

~~~text
received
  → validated          # 字段/ID/kind 校验
  → deduped            # event ID 去重
  → queued             # 会话队列
  → running
      → hook.before    # 记录消息/注意力，50~200ms 预算
      → collect        # ContextProvider 并行/串行收集
      → budget         # 按优先级/token 裁剪
      → decide         # 唯一 Decider
      → silent/observe # 不回复，写 Trace，结束
      → compose        # persona + blocks + recent + current + tools
      → llm            # 最多 N 轮工具调用
      → render         # 0..N Actions
      → dispatch       # adapter 能力检查/转义/限流/重试
      → hook.after     # 记录回复/触发反思
  → traced
  → done
~~~

失败路径：

| 阶段失败 | 处理 |
|---|---|
| 字段校验失败 | 丢弃 + 指标，不回复 |
| Decider 失败 | 默认不回复 + Trace error（安全） |
| ContextProvider 失败 | 跳过该 block，记录 degraded |
| LLM 失败 | 有限重试；仍失败则静默或角色内兜底，不暴露错误 |
| 工具失败 | 把安全错误文本给 LLM，继续回合 |
| Renderer 失败 | 使用原文兜底 + 记录 |
| 发送失败 | 有限重试/降级；写 action 审计 |
| TurnHook 失败 | 记录，不影响已生成回复 |
| panic | recover + mark plugin degraded + Trace |

**Decision 语义**：

| <code>Kind</code> | <code>Defer</code> | 含义 |
|---|---|---|
| <code>reply</code> | nil | 正常回复；只有这个 kind 能产生 Actions |
| <code>silent</code> | nil | 看见了，但选择不回；必须写 reason，进 Trace，不产生 Action |
| <code>observe</code> | nil | 不是在说我 / 不进入回合；只记录，不产生 Action |
| <code>defer</code> | set | 稍后再看；必须有 reason、<code>CancelKey</code>、<code>MaxAttempts</code> |

约束：

- <code>silent</code> 与 <code>observe</code> 必须区分：前者是“她选择闭嘴”，后者是“这条消息不属于她”。
- 只有 <code>reply</code> 能产生 Action；<code>silent/observe</code> 必须没有 Action，否则按 error 记录。
- <code>defer</code> 由调度器重新入队；同一 <code>CancelKey</code> 去重，超过 <code>MaxAttempts</code> 转 <code>silent</code> 并写 Trace。
- 静默不是失败；不要把“不回”记成 error。

### 2.4 扩展点

| 扩展点 | 数量 | 谁实现 | 失败影响 |
|---|---|---|---|
| Adapter | 1..N | 平台插件 | 该平台不可用 |
| Persona | 1 | persona 插件 | 无法启动 |
| Decider | 1 | drive 插件 | 默认不回复 |
| ContextProvider | 0..N | memory/knowledge | 跳过该 block |
| TurnHook | 0..N | recorder/attention | 记录，不阻断 |
| Tool | 0..N | tool 插件 | 工具错误返回给 LLM |
| Renderer | 1 | persona 插件 | 原文兜底 |
| Policy | 0..N | governance 插件 | 拒绝/降级 |
| Observer | 0..N | reflection 插件 | 异步丢弃 + 指标 |
| UI/Command | 0..N | interface 插件 | 该页面/命令不可用 |
| LLMProvider | 1（默认） | kernel 默认 + 未来插件 | 聊天不可用 |

**单一 active**：Decider、Renderer、Persona 各一个；其他可多个。多个同类插件要显式做“组合/路由”插件，不能靠隐式顺序。

### 2.5 并发与调度

~~~text
Adapter goroutine
   → Kernel.Receive(event)
       → dedup
       → ConversationScheduler.enqueue(conversationID, event)
           → per-conversation queue (max depth N)
           → ready queue (fair)
           → worker pool (global max W)
               → one active turn per conversation
                   → LLM semaphore (max L)
                   → Tool semaphore (max T)
       → Observer pool (bounded, separate)
~~~

规则：

- **同会话串行**：避免上下文错乱、重复回复。
- **跨会话并行**：一个慢群不阻塞其他群。
- **公平**：ready queue 轮转，不让某个群连续占满 worker。
- **有界**：所有队列都有上限；满时按策略处理，绝不无限阻塞 adapter 读循环。
- **去重**：event ID TTL；重复事件只计数，不重复执行。
- **超时**：turn 总预算 + 每插件/工具预算；超时取消。
- **后台隔离**：Observer 用独立 pool，慢反思不占回复 worker。
- **关闭**：停止接收 → 有界排空 → 取消在途 → 逆序停插件 → flush trace。

### 2.6 生命周期

~~~text
Register(spec) → ValidateManifest → ResolveDeps → TopoSort
→ Init (timeout) → Start (timeout) → HealthLoop
→ Reload? → Disable? → Stop (reverse, timeout) → Cleanup
~~~

- **关键插件**（adapter / storage / memory.recent / persona / renderer / decider）：失败则拒绝启动。
- **非关键插件**（reflection / tools / webui）：失败则 degraded，继续跑。
- **Init 失败**：清理已 Init 的插件，报错退出。
- **Start 失败**：Stop 已 Start 的插件，报错/降级。
- **Stop 幂等**：重复调用安全；每步有超时。
- **Reload**：插件实现 <code>Reloader</code> 才热更；否则标记“需重启”；失败保留旧配置。
- **Disable**：停止插件、摘掉它注册的 tool/provider；不卸载 Go 代码。

### 2.6.1 能力注册（Provider Registry）

Host 上的 <code>Memory / Storage / Tools / Actions / LLM</code> 不是内核凭空实现的；有些由插件提供。必须有明确的注册与就绪规则，否则会出现“Host 能力为空”或“storage 插件依赖 Host.Storage”的死循环。

**内核预置 capability**（任何插件都可 require，永远可解析）：

- <code>kernel.bus</code>、<code>kernel.actions</code>、<code>kernel.tools</code>
- <code>kernel.llm</code>（默认 OpenAI-compatible）
- <code>kernel.config</code>、<code>kernel.clock</code>、<code>kernel.log</code>、<code>kernel.trace</code>

**插件提供的 capability**：

- <code>storage.sqlite</code> → <code>storage</code>（单例）
- <code>memory.recent</code> → <code>memory.recent</code>（单例）
- <code>memory.longterm</code> → <code>memory.longterm</code>（可选）
- <code>persona.default</code> → <code>persona</code>（单例）
- <code>renderer.default</code> → <code>renderer</code>（单例）
- <code>drive.attention</code> → <code>decider</code>（单例）
- <code>adapter.onebot</code> → <code>adapter.onebot</code>（1..N）

**规则**：

1. **先注册、后解析**：插件在 <code>Init</code> 阶段注册自己提供的 capability；内核在全部关键插件 Init 完成后才允许 <code>Start</code> 消费方。
2. **单例仲裁**：标记为单例的 capability 若被注册两次，拒绝启动并报冲突；多例（adapter/tool）允许，但必须唯一命名。
3. **按需 Host facade**：内核按 manifest 的 <code>requires</code> 生成 per-plugin Host facade；未声明的 capability 调用直接返回 <code>permission_denied</code>，不是靠注释。
4. **optional 降级**：<code>optional_requires</code> 缺失时插件仍可 Init，但 health 标记 degraded；插件用 <code>Host.Has(cap)</code> 判断。
5. **禁止环**：storage provider 不得 require memory/persona；memory 可 require storage；adapter 只 require kernel.actions；persona/renderer/decider 不得互相 require。
6. **启动顺序**：kernel core → storage → memory/persona/renderer/decider → adapters → tools/reflection/UI。adapter 最后启动，避免消息在依赖未就绪时进来。
7. **禁用**：摘掉该插件提供的 capability；依赖它的插件进入 degraded，而不是静默拿到 nil；关键 capability 缺失时内核拒绝继续 Start。

这解决了两个问题：

- <code>requires</code> 不是“自己 provides 的子集”，而是“能在 registry 里解析”；
- Host 能力不会出现“由插件提供、但内核不知道谁提供”的空窗。

### 2.7 错误与降级

统一错误结构：

~~~go
type Error struct {
    Code      string // config / manifest / dependency / timeout / canceled / permission / budget / llm / tool / adapter / rate_limited / invalid_event
    Message   string // 给日志/开发者
    UserText  string // 给人看的人话（可选）
    Retryable bool
    PluginID  string
    Cause     error
}
~~~

规则：

- 插件错误必须包 <code>PluginID</code>，便于归因。
- 用户可见错误必须是人话；不暴露堆栈、密钥、内部路径。
- 可重试错误有限重试 + 退避；不可重试直接降级。
- 任何 panic 都转成 <code>Error{Code:"plugin_panic"}</code> + Trace，不让进程退出。
- 降级优先级：静默 > 角色内兜底 > 通用错误提示（尽量不用）。

### 2.8 配置

~~~text
configs/
├─ kernel.yaml                 # 内核：实例、数据目录、日志、模型、插件启停、预算
└─ plugins/
   ├─ adapter.onebot.yaml      # 插件配置（version + enabled + config）
   ├─ persona.default.yaml
   ├─ memory.recent.yaml
   └─ ...
~~~

规则：

- 内核配置只管内核；插件配置归插件，独立 <code>version</code> 与迁移函数。
- 插件 manifest 引用 <code>config.schema.json</code>；内核加载时校验 + 填默认值。
- 密钥只从环境变量/secret store 注入，例如 <code>{{ONEBOT_TOKEN}}</code>；不写进 WebUI 可回显的配置。
- 变更热更新：插件实现 <code>Reloader</code> 才热更；否则提示重启。
- 所有“死配置”要么实现，要么删除（例如当前 <code>cache_prefix</code> 配置未在客户端生效）。

### 2.9 存储

- 内核定义 <code>Host.Storage(namespace)</code> **端口**；实现由 <code>storage.sqlite</code> 等 storage 插件提供；内核不假装自带存储。
- 插件只能访问自己的 namespace。
- 插件自带迁移列表；storage 插件按 plugin ID + version 顺序执行；迁移表记录已执行版本。
- 迁移只加不删；破坏性变更走新表/双写/回填。
- 事务边界由插件声明；内核负责超时与错误包装。
- 备份/导出由 storage 插件或内核管理命令负责；插件不直接碰数据目录。

### 2.10 可观测

Trace 最小字段：

~~~json
{
  "turn_id": "t_...",
  "event_id": "e_...",
  "conversation_id": "qq:group:123",
  "actor_id": "qq:user:456",
  "decision": { "kind": "reply", "reason": "at_bot", "confidence": 0.9 },
  "blocks": [{ "source": "memory.recent", "tokens": 180, "priority": 50 }],
  "llm": [{ "round": 1, "tools": ["time_now"], "latency_ms": 800, "tokens": 600 }],
  "tools": [{ "name": "meme_send", "latency_ms": 30, "ok": true }],
  "actions": [{ "adapter": "onebot", "parts": 1, "ok": true }],
  "latency_ms": 1200,
  "cost_usd": 0.0004,
  "errors": []
}
~~~

指标最小集：

- <code>kernel_turns_total{result}</code>
- <code>kernel_turn_duration_seconds</code>
- <code>kernel_queue_depth{adapter,kind}</code>（conversation 只进日志，避免无界标签）
- <code>plugin_health{plugin}</code>
- <code>llm_requests_total</code> / <code>llm_tokens_total</code> / <code>llm_errors_total</code>
- <code>tool_calls_total{tool,result}</code> / <code>tool_duration_seconds</code>
- <code>actions_total{adapter,result}</code>
- <code>rate_limited_total{scope}</code>

### 2.11 测试

- <code>sdk/testkit</code>：FakeHost、FakeLLM、FakeAdapter、FakeClock、FakeStorage、FakeDecider。
- 每个插件：manifest lint + lifecycle + config schema + provider/tool 契约测试。
- 内核：调度顺序、去重、超时、panic、背压、关闭、预算裁剪。
- 行为：golden dialogue（输入事件 → 期望 decision/action 属性），不比对不确定的 LLM 原文。
- 架构：<code>go list</code> 依赖规则测试。
- 并发：<code>go test -race ./...</code>。
- 失败注入：插件 panic、LLM 超时、adapter 断线、storage 满、队列满。
- 迁移：旧 config/DB → 新结构 golden test。

---

## 3. 现在还缺什么（P0 / P1 / P2）

> 判断标准：  
> **P0** = 不补就会在迁移中“崩、卡、查不到、配不动”；  
> **P1** = 第一个插件真正迁移到新内核前必须有；  
> **P2** = 第一方多平台/多角色/生态阶段再做。

### 3.0 评审补充（v0.2 新增，P0/P1 遗漏）

> 来自红队评审的四类真问题：安全、进程稳定性、存储一致性、契约闭环。它们比“功能缺口”更优先。

| # | 缺口 | 证据 | 影响 | 级别 |
|---|---|---|---|---|
| S1 | WebUI 认证未接线，配置/密钥明文 | <code>server.go</code> 的 <code>http.Server.Handler</code> 用的是裸 <code>mux</code>，不是 <code>s.auth(mux)</code>；<code>/api/config</code> 返回完整 Config | 能访问端口的人可直接读 API Key；权限/审计/密钥三条线都失效 | P0 |
| S2 | queue.Pool 关闭/提交竞态，job ctx 被丢弃 | <code>Close</code> 直接 <code>close(jobs)</code>；并发 <code>Submit</code> 可能 send on closed channel；worker 用 <code>context.Background()</code> | 关闭时 panic；取消/超时失效；关闭预算失效 | P0 |
| S3 | SQLite 并发与迁移一致性 | 未设 WAL/busy_timeout/连接上限；迁移逐条 Exec 无事务 | 4 worker 并发写会 SQLITE_BUSY；迁移失败留半状态 | P0 |
| S4 | notice/recall 绕过会话锁；session key 冲突 | <code>handleNotice</code> 直接 Dispatch；key 是裸 int64，群 123 与私聊用户 123 共锁 | 同会话并发 turn；上下文错乱 | P0 |
| S5 | requires 解析与 Host 能力来源未闭环 | 文档曾写 <code>requires ⊆ provides</code>；Host.Memory/Storage 由插件提供却无注册/仲裁 | 加载器会拒绝所有插件，或 Host 能力为空 | P0 |
| S6 | 热更新不校验、非原子，仍有竞争 | <code>Manager.Reload</code> 不 Validate；listener 无 error/回滚；attention/filter/chat 裸读写 | 半更新配置；数据竞争 | P0 |
| S7 | 日志/Trace 未脱敏，schema 不一致 | Info 级打 reasoning/回复/工具参数与结果；两份 Trace 字段不同 | 泄露消息/密钥；Trace 无法 lint | P1 |
| S8 | 禁用插件未真正停插件/摘能力 | <code>SetEnabled</code> 只改 map；插件不 Stop，工具不注销 | 禁用后仍占资源；provider 残留 | P1 |
| S9 | Event/Action 缺身份字段 | 无 event ID/time/source/self；Action 无 ID/TurnID/Actor/IdempotencyKey | 去重、幂等、scope 校验无挂点 | P1（契约已在 2.2 修正，待实现） |
| S10 | id / 工具名 / bundle / config 三套形态 | 文档混用 <code>onebot</code>、<code>adapter.onebot</code>、<code>send_meme</code>、<code>meme_send</code>、单文件与分文件配置 | Phase 1 必然返工 | P1（命名已在 4.0 定死，待迁移） |

### 3.1 P0：迁移前必须补

| # | 缺口 | 现状证据 | 影响 |
|---|---|---|---|
| 1 | 插件调用无 recover / timeout | <code>plugin.Manager.handlePlugin</code> 直接调用；<code>queue.Pool.worker</code> 无 recover | 一个插件 panic 带走整个进程；慢插件卡住同群 |
| 2 | 无会话调度器 / 背压 | <code>handler</code> 用 session mutex + <code>queue.Pool.Submit</code> 阻塞 | 4 个慢群可占满 worker；队列满时阻塞事件 goroutine 并堆积，adapter 没有背压信号 |
| 3 | 平台中立 Event/Action 缺失 | <code>platform.Event = qq.Event</code>；<code>plugin.Bot</code> 是 QQ 方法 | 第二个平台要改核心、记忆、工具，等于重写 |
| 4 | 插件无 manifest/生命周期/依赖 | <code>PluginState</code> 只有 name/desc/stage/priority/enabled | 无法排序、健康检查、配置挂载、权限声明 |
| 5 | 工具无 schema/权限/超时/结果上限 | <code>tools.Registry.Execute</code> 只做 JSON unmarshal | 参数错误、越权、慢工具、超长结果进 prompt |
| 6 | 无上下文预算/来源 | <code>core/chat.buildMessages</code> 直接 append 多段 system | prompt 膨胀、不可解释、缓存不友好、成本不可控 |
| 7 | 无 turn Trace | <code>obs.Span</code> 只记 duration，不导出 | 无法回答“她为什么回/不回/为什么慢” |
| 8 | 配置单体 + 无 schema | <code>config.Config</code> version 12；默认值/迁移/规范化手写在 6 个文件 | 插件配置无法独立演进；加字段要动核心版本 |
| 9 | 无存储命名空间 / 插件迁移 | <code>store</code> 全局 schema v4，一个 <code>schema_version</code> | 插件数据割裂；迁移冲突；无法单独备份/清理 |
| 10 | 出站消息未转义 / 未结构化 | <code>qq.Client.SendGroupMessage</code> 直接发字符串 | LLM 输出 CQ 码可 @全体/注入；平台差异难处理 |
| 11 | 无事件去重 / 幂等 | <code>handler</code> 不使用 event ID | 平台重投会重复回复、重复写记忆 |
| 12 | 热更新非线程安全 | <code>llm.Client.UpdateConfig</code>、<code>filter.Update</code>、<code>chat.UpdatePersona</code>、<code>attention.Observe</code> 裸读写共享字段 | 数据竞争；热更新时可能崩溃/读到半更新配置 |
| 13 | 无优雅关闭预算 | <code>app.Close</code> / <code>Run</code> 的 defer 顺序关闭，无每步超时 | 在途 LLM/发送可能被硬断；goroutine 泄漏 |
| 14 | 配置接口泄露密钥 | WebUI <code>GET /api/config</code> 返回完整 <code>Config</code>；<code>/api/config/raw</code> 返回 YAML；且 auth 未接线 | 默认 token 为空时，能访问端口的人都能看到 API Key；日志/截图易泄露 |
| 15 | 无插件级热更新契约 | <code>app.registerReload</code> 硬编码 LLM/filter/trigger/attention/limiter/chat | 新增插件要改 app；部分配置热更、部分不热更，行为不一致 |

### 3.2 P1：第一个插件迁移前要有

| # | 缺口 | 现状证据 | 影响 |
|---|---|---|---|
| 1 | 无健康/就绪 | <code>/healthz</code> 固定返回 <code>{"status":"ok"}</code> | 不知道 adapter/LLM/DB 是否可用 |
| 2 | 无 Decision 接口 | trigger 是 preprocessor，用 bool 表示“继续/吞掉” | 无法表达 silent / defer / observe-only |
| 3 | 无后台观察者总线 | 识图/总结/学习都在 preprocessor 同步跑 | 慢任务堵回复；关掉就丢学习 |
| 4 | 无测试基建 | 没有 fake host/LLM/adapter | 插件只能靠集成测试，迁移风险高 |
| 5 | 无架构约束测试 | 没有 import 规则检查 | 耦合会慢慢反弹回 app.go |
| 6 | 无权限/审计 | 只有 admin_ids 与命令 | 工具越权不可控；出问题无法追溯 |
| 7 | 无密钥管理 | YAML 明文；没有环境变量注入约定 | 密钥泄露；WebUI 回显 |
| 8 | LLM cache_prefix 是死配置 | 配置存在，但 <code>llm.Client</code> 未使用 | 用户以为开了缓存，实际没有；成本偏高 |
| 9 | 无 token/费用预算 | 只有固定 <code>MaxTokens</code>；工具轮次固定 5 | 单 turn 费用不可控；被刷时无刹车 |
| 10 | 无手册 / read_manual | 只有 PluginState.Description | 模型不知道工具何时用/何时不用；工具多时 prompt 膨胀 |
| 11 | 出站路径不统一 | WebUI <code>handleAdminSend</code> 直接调 <code>bot.Send*</code> | 绕过队列/限流/审计；dry-run 仍生效，但一致性仍缺失 |
| 12 | 指标缺少关键维度 | <code>obs.Metrics</code> 只有基础计数；<code>/metrics</code> 有 uptime/LLM/DB 计数 | 没有延迟分布/队列深度/工具失败率/插件健康 |
| 13 | 存储调用无 ctx/超时 | <code>store</code> 方法多为同步 DB 调用，无 context | DB 慢/锁等待会卡住 turn，无法取消 |
| 14 | Event/Action 无 schema 版本 | <code>plugin.Event</code> / <code>qq.Event</code> 直接结构体 | 平台/插件升级时无法兼容旧事件 |

### 3.3 P2：第一方多平台/多角色再说

| # | 能力 | 说明 |
|---|---|---|
| 1 | 多实例 / profile | 一个进程跑多个 bot / 多个人格；内核要实例化，ID 要命名空间化 |
| 2 | 多平台 adapter | Telegram / Discord / CLI / Web；第一方也可以后做 |
| 3 | LLM provider 插件 | OpenAI-compatible 之外的 provider；默认实现留在内核 |
| 4 | 多 persona | 每群/每实例不同人格；需要 persona 路由 |
| 5 | 进程外插件 / MCP / WASM | 第三方/不可信插件；本期明确不做 |
| 6 | 插件市场 / 安装器 | 没有稳定 API 与第一批插件前不做 |

### 3.4 先修三个（红队评审版）

1. **安全与配置可信度**：WebUI auth 接线、config API 脱敏、Reload 先 Validate、原子 swap/回滚。  
   不做这个，权限/审计/密钥三条线都建在沙上。
2. **冻结契约**：manifest/tool 命名、<code>requires</code> 解析、capability-scoped Host、Event/Action/Decision 身份字段。  
   不做这个，Phase 1 必然返工。
3. **进程稳定性**：Pool ctx/关闭顺序、SQLite WAL/busy_timeout/事务迁移、per-conversation actor（notice 同锁、复合 key）。  
   不做这个，P0 清单的其余改造只是“把一个大函数拆成更多能崩的点”。

平台中立（Event/Action + OneBot adapter）与可解释性（Trace + Decision）仍然要做，但排在以上三个之后。

### 3.5 P0 做完后的最小闭环

- 内核能启动、注册第一方插件、按依赖排序；
- 一条 OneBot 消息能变成 <code>Event</code>，进入会话队列；
- 插件 panic/超时不会拖垮进程；
- 每个 turn 有 Trace，能看到 decision/context/tools/action；
- 配置能按插件挂载、校验、热更新（线程安全）；
- 出站 Action 统一走 adapter，带能力检查/转义/审计；
- 关闭时能在预算内排空并逆序停插件。

---

## 4. 第一方插件怎么装

### 4.0 先定死命名（v0.2）

四套命名必须只留一套，否则 Phase 1 会返工：

| 对象 | 规则 | 示例 |
|---|---|---|
| Plugin ID | <code>&lt;kind&gt;.&lt;name&gt;</code>，小写点分；kind 来自枚举 | <code>adapter.onebot</code>、<code>tool.meme</code>、<code>memory.recent</code> |
| Tool 名 | <code>&lt;plugin_short&gt;_&lt;tool&gt;</code>，小写下划线，满足 <code>^[a-z][a-z0-9_]*$</code> | <code>meme_send</code>、<code>meme_save</code>、<code>time_now</code> |
| Go 包/目录 | <code>internal/plugins/&lt;kind&gt;/&lt;name&gt;/</code> | <code>internal/plugins/tool/meme/</code> |
| Bundle | 仓库根 <code>bundles/standard/bundle.go</code>，返回 <code>[]sdk.PluginSpec{New, Critical}</code>；manifest 由 <code>New()</code> 返回，唯一事实源 | <code>internal/plugins/...</code> 不被 cmd 直接 import |
| 配置 | <code>configs/kernel.yaml</code> + <code>configs/plugins/&lt;id&gt;.yaml</code> | <code>configs/plugins/tool.meme.yaml</code> |
| 版本 | <code>version</code> 是 semver；<code>api_version</code> 是整数 major | <code>version: 0.1.0</code>、<code>api_version: 1</code> |

兼容处理：

- 现有 <code>send_image</code> / <code>send_meme</code> 保留一个版本的别名；新工具按 <code>meme_send</code> 这类命名。
- 现有单文件 <code>config.yaml</code> 的 <code>plugins:</code> 段由迁移器拆到 <code>configs/plugins/&lt;id&gt;.yaml</code>；迁移后只保留一种形态。
- “14 个插件”是上限：默认无 VLM 时 <code>steal</code> 不注册，实际 13 个；文档不要再把它写成固定值。

### 4.1 只用显式 Bundle，不用 init()

~~~go
// bundles/standard/bundle.go
// manifest 由 New() 返回；这里只决定“装哪些、哪些关键”。
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

为什么不用 <code>init()</code>：

- import 顺序隐式，测试之间共享全局状态；
- 无法按配置/构建标签裁剪；
- 重名注册当前直接 panic；
- 读代码时看不到“到底装了什么”。

<code>cmd/icarosu/main.go</code> 只 import <code>bundles/standard</code>，不 import 具体插件包。

### 4.2 插件目录规范

~~~text
internal/plugins/tools/meme/
├─ plugin.go              # New()，实现 sdk.Plugin
├─ manifest.yaml          # go:embed，唯一事实源
├─ config.schema.json     # go:embed，配置校验 + WebUI 表单
├─ MANUAL.md              # go:embed，人看的手册
├─ manuals/
│  ├─ meme_send.md        # 模型按需读的工具手册
│  └─ meme_save.md
├─ schemas/
│  ├─ meme_send.json
│  └─ meme_save.json
├─ config.go              # 配置结构 + 默认值 + 迁移
├─ plugin_test.go         # 契约测试
└─ tool_test.go           # 工具测试
~~~

约定：

- manifest / schema / manual 用 <code>go:embed</code> 打包，避免运行时路径问题。
- **消费型插件**（memory/persona/tool/reflection…）只 import <code>internal/sdk</code> + <code>internal/infra</code>（通用原语）+ 标准库；不得 import <code>internal/store</code>/<code>internal/qq</code>/<code>internal/config</code>/其他插件。
- **能力提供型插件**（adapter/storage）可以在自己的包内封装协议/驱动实现，但不得把实现类型暴露给其他插件；例如 OneBot 客户端最终应移进 <code>adapter/onebot</code> 自己的包，而不是让全仓库依赖 <code>internal/qq</code>。
- 插件不直接开 DB / 发消息 / 起 goroutine；走 Host。能力提供型插件实现 Host 的那一层能力时例外。
- 插件数据写 <code>data/</code> 下的命名空间，不写插件目录。

### 4.3 Manifest / Schema / Manual 三件套

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
optional_requires:
  - memory.longterm
config_schema: config.schema.json
permissions:
  - filesystem.workspace
  - messaging.send
tools:
  - name: meme_send
    summary: 从表情库随机发一张表情包
    schema: schemas/meme_send.json
    manual: manuals/meme_send.md
    permissions: [messaging.send]
  - name: meme_save
    summary: 把一张图收藏进表情库
    schema: schemas/meme_save.json
    manual: manuals/meme_save.md
    permissions: [filesystem.workspace]
manual: MANUAL.md
~~~

**三件套各自的作用**：

| 文件 | 读者 | 作用 |
|---|---|---|
| <code>manifest.yaml</code> | 内核 / WebUI / 文档 | 加载、依赖、权限、工具清单 |
| <code>config.schema.json</code> | 内核 / WebUI | 校验、默认值、表单渲染 |
| <code>MANUAL.md</code> + tool manual | 人 / 模型 | 何时用、怎么用、失败模式；模型通过 <code>read_manual</code> 按需读 |

### 4.4 标准包清单（第一方）

| 插件 | 关键性 | 职责 |
|---|---|---|
| <code>adapter.onebot</code> | 关键 | OneBot v11 接入、重连、消息解析、发送 |
| <code>storage.sqlite</code> | 关键 | 通用 KV/SQL 存储与迁移 runner |
| <code>memory.recent</code> | 关键 | 每会话短期上下文 |
| <code>memory.longterm</code> | 非关键 | 长期记忆、语义检索、总结落库 |
| <code>persona.default</code> | 关键 | 身份、性格、边界、示例、渲染规则 |
| <code>drive.attention</code> | 关键 | 注意力、接话决策、冷却、静默 |
| <code>renderer.default</code> | 关键 | 句长、拆句、去助手腔、表情策略 |
| <code>tool.basic</code> | 非关键 | 时间、随机、骰子、梗文本 |
| <code>tool.meme</code> | 非关键 | 表情包收藏/发送 |
| <code>reflection.summarizer</code> | 非关键 | 群总结、表达/黑话学习（异步） |
| <code>governance.safety</code> | 非关键 | 安全过滤、限流、预算 |
| <code>interface.webui</code> | 非关键 | 状态/配置/记忆/插件/手册/Trace 页面 |

**标准包就是产品**：它可以裁剪可选插件，但关键插件必须能让机器人“开口说话”。

### 4.5 新增第一方插件的 Checklist

- [ ] 目录在 <code>internal/plugins/&lt;kind&gt;/&lt;name&gt;/</code>
- [ ] 实现 <code>sdk.Plugin</code>（Init/Start/Stop/Health）
- [ ] <code>manifest.yaml</code>：id/version/api_version/kind/provides/requires/permissions
- [ ] <code>config.schema.json</code> + 默认值 + 迁移
- [ ] <code>MANUAL.md</code> + 每个 tool 的 summary/manual/schema
- [ ] 所有副作用走 Host；不 import 其他插件/内核/具体 store/qq
- [ ] 有 ctx/timeout/cancel；不 panic 到内核；错误分类
- [ ] 有 logger/metrics/health；日志字段带 plugin_id
- [ ] 契约测试 + fake host；必要时 golden dialogue
- [ ] 加入 <code>bundles/standard</code> 或显式说明为什么可选
- [ ] 更新 README/架构文档中的插件清单

---

## 5. 落地顺序（第一方版）

> 原则：每一步都保持 <code>go build ./... &amp;&amp; go test ./...</code> 通过；旧链路可运行；新链路可回退。

### Step 0：立契约与规矩（不改变行为）

- <code>internal/sdk</code>：Event/Action/Part/Plugin/Host/Tool/Decision/Trace/Error。
- <code>internal/kernel</code>：Registry/Manifest/Lifecycle 骨架。
- <code>sdk/testkit</code>：FakeHost/FakeLLM/FakeAdapter/FakeClock。
- 架构测试：<code>sdk</code> 零依赖、<code>kernel</code> 不 import plugins、plugins 不 import 内部具体包。
- 补 P0 的“进程稳定性”：recover/timeout/队列上限/关闭预算。
- **验收**：现有插件（最多 14 个）仍能注册；行为不变；架构测试生效。

### Step 1：平台中立 + OneBot adapter

- 新 <code>sdk.Event/Action</code>；<code>plugins/adapter/onebot</code> 包装现有 <code>qq.Client</code>。
- <code>handler</code> 变成 adapter 内部 normalizer；旧 <code>plugin.Event</code> 通过兼容层继续跑。
- **验收**：QQ 群聊行为不变；<code>grep -r "internal/qq"</code> 只出现在 adapter；dry-run 回放一致。

### Step 2：回合内核 + 标准包

- 实现 <code>ConversationScheduler</code>、<code>TurnEngine</code>、<code>ContextProvider</code>、<code>Decider</code>、<code>ToolRegistry</code>、<code>ActionSink</code>、<code>Trace</code>。
- 迁移 <code>persona / memory.recent / memory.longterm / drive.attention / renderer / chat / sender</code>。
- 慢任务（识图/总结/学习/成员同步）改异步 Observer。
- 删掉 <code>send_image</code> / <code>send_meme</code> 的特判，改 ToolResult.Parts。
- **验收**：golden dialogue 对齐；p95 延迟不高于基线；单插件 panic 不影响回复；Trace 可看。

### Step 3：全量第一方插件 + 手册/权限

- 迁移 filter/safety/membersync/admin/ping/replylog；WebUI 变 interface 插件。
- 每个插件补 manifest/schema/manual/health；工具权限/审计/结果上限。
- WebUI：插件页、手册页、Trace 页、配置表单。
- **验收**：新增一个工具插件不改内核/配置结构；权限测试覆盖发消息/读记忆/网络；手册可读。

### Step 4：多平台/多实例（延后）

- 第二个第一方 adapter（Telegram/CLI）验证平台中立。
- 内核实例化、profile、多 persona。
- **验收**：不改内核接新平台；同进程多实例互不污染。

### 回滚点

- Step 0：纯新增，零风险。
- Step 1：旧流水线仍是默认；adapter 可切回旧 <code>qq.Client</code> 直连。
- Step 2：feature flag <code>kernel.turn_engine</code>，旧 chat 插件保留一个版本。
- Step 3：插件粒度开关；WebUI 可回退旧页面。

---

## 6. 还需要你拍板的剩余问题

1. **单实例还是多实例？** 建议第一方先单实例，但内核按实例作用域设计，避免以后重构。
2. **WebUI 是产品还是开发工具？** 产品化需要权限/审计/手册/Trace；开发工具可以更轻。
3. **管理员命令保留吗？** 建议保留为 <code>admin</code> tool/command 插件，默认仅私聊/管理员可用。
4. **要不要语音/合并转发/撤回？** 这决定 Adapter capability 的最小集。
5. **记忆保留策略？** 永久/按群/按人/可删除；涉及隐私与备份。
6. **默认沉默策略？** 群聊随机接话的概率、冷却、主动开口频率；这是“人味”的核心参数。
7. **模型供应商范围？** 第一方先 OpenAI-compatible 单 provider，还是同时支持 Ollama/本地？
8. **是否保留 WebUI 的原始 YAML 编辑？** 建议保留但默认关闭，且必须脱敏。

回答这 8 个问题后，Step 0 的接口就能冻结，不需要反复改。

---

> **内核判断口诀**：  
> 所有插件都要用的，进内核；  
> 涉及一致性/安全/生命周期的，进内核；  
> 和平台、人格、业务有关的，做插件；  
> 只有一个插件在用的，先别上提。
