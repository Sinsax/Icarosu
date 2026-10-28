# 学习能力、情绪与分层记忆计划书

- 状态：方案稿（未实现）
- 适用范围：`cmd/icarosu` 新微内核 runtime；旧 `cmd/bot` 不改
- 目标：让“学习、情绪、短期/长期记忆”成为第一方插件能力，并保持微内核/插件架构一致

## 1. 目标

1. **学习分类**
   - 技能：触发条件 + 步骤 + 关联工具 + 成功/失败统计
   - 表达方式：口癖、语气、梗、常用回复
   - 黑话：群内术语/缩写/内部梗
   - 偏好/事实/关系：人物喜好、事件、关系
2. **短期记忆 = 当前会话工作区**
   - 最近原文窗口
   - 场景/话题/参与者
   - 热工具（近期成功使用的功能）
   - 热记忆（高频/近期/高相关/高情绪显著）
   - 激活技能与表达方式
   - 当前情绪
3. **长期记忆 = 默认保存全部、默认不注入**
   - 消息、事件、总结、学习条目全部落库
   - 只在显式 `memory_recall`、未知词/话题切换/追问/高情绪显著事件时召回
   - 使用后强化，达到阈值晋升到短期工作区
4. **情绪影响风格与功能**
   - 回复风格
   - 工具选择/排序
   - 决策主动性
   - 记忆显著度
   - 不影响 safety / filter / 权限
5. **先做可调试的大波动情绪**
   - `debug_volatile` 高增益、快衰减、宽范围
   - admin `emotion_set` + WebUI 调试入口
   - Trace 记录情绪变化与工具选择原因

## 2. 非目标

- 不替换旧 `cmd/bot` 流水线
- 不做第三方进程外插件、WASM、MCP
- 第一阶段不引入 embedding/向量库
- 不让情绪改变治理策略、权限或安全边界
- 不把“聊天窗口显示的全部消息”直接塞进模型上下文

## 3. 现状与问题

当前新标准包中：

- `memory.recent`：只保存最近 20 条，无摘要、无长期归档
- `reflection.summarizer`：独立维护 50 条阈值和 summary，与 recent 是两套窗口
- `stylelearn` / `jargonlearn`：各自内存 map，无统一学习模型、无使用次数/成功率
- `Turn` 没有情绪/工作区/学习字段
- `TurnEngine.generate` 总是把所有工具交给 LLM，没有情绪/技能驱动的工具选择

因此直接叠加会导致窗口重复、摘要重复、学习双写、工具不可控。

## 4. 总体设计

### 4.1 三层结构

1. **展示层（聊天窗口）**
   - 展示/分页/搜索全部消息
   - 不决定模型上下文
   - 可选把 viewport/选中消息作为 `page_hint` 提示，不作为记忆真相

2. **短期工作区 WorkingSet**
   - 进入 LLM 的上下文
   - 最近原文窗口 + 滚动摘要 + 场景 + 热工具 + 热记忆 + 技能/表达 + 情绪
   - 有 token 预算

3. **长期记忆 LongTerm**
   - SQLite 全量归档
   - 默认不注入
   - 按需召回

> 这里的“页面上下文”指当前会话页面（最近消息/话题/参与者），不是 WebUI 的 DOM。若 WebUI 有 viewport/选中消息，只作为 <code>page_hint</code> 提示，不作为记忆真相。

### 4.2 插件归属

推荐：

| 插件 | 类型 | 职责 |
|---|---|---|
| `memory.tiered` | memory | WorkingSet、最近窗口、滚动摘要、LongTerm 归档/召回、`memory_recall` 工具、工具使用统计 |
| `reflection.learning` | reflection | `learning_items`、表达/黑话/技能抽取、验证、强化、技能上下文 |
| `drive.emotion` | drive | 情绪状态、Observer、EmotionProvider、ContextProvider、ToolSelector |

旧的 `memory.recent`、`reflection.summarizer`、`stylelearn`、`jargonlearn`：

- 旧 bundle 保留
- 新标准包不再同时启用，避免两套窗口/双写
- 数据迁移到新表后逐步退役

### 4.3 聊天模式边界

“聊天模式”只负责：

- 选择 `WorkingSetProfile`
- 提供场景/模式提示
- 决定人格/表达规则

不负责：

- 自己保存窗口
- 自己摘要
- 自己维护长期库
- 自己维护学习/情绪状态

模式可以通过 `Conversation.Metadata["chat_mode"]` 或 `Event.Meta["chat_mode"]` 传给 `memory.tiered`，由后者选择 profile。

## 5. 窗口溢出算法

### 5.1 双水位

不要仅按“超过 N 条”压缩。使用：

- 最近原文窗口：`recent_max` + `recent_tokens`
- 未摘要缓冲：`pending`
- 压缩水位：`compress_after_count` / `compress_after_tokens`
- 摘要上限：`summary_tokens`
- 摘要水位：`summary_upto`

### 5.2 流程

```text
收到消息：
  append 到 recent
  异步写 LongTerm（消息/事件）
  若 recent 超过 recent_max/recent_tokens：
    把最旧消息移入 pending
  若 pending 超过压缩水位且无 inflight：
    异步 LLM 总结 pending
    成功后：
      summary = 滚动摘要
      summary_upto = 最后一条 pending 的 seq
      清空已总结 pending
    失败：
      pending 保留，下一轮重试
      LongTerm 已有原文，不丢数据
```

### 5.3 每轮模型上下文

按优先级和 token 预算组装：

1. persona
2. 情绪风格块（高优先级）
3. 最近原文窗口
4. 场景/话题/参与者
5. 滚动摘要
6. 热记忆
7. 热工具/技能提示
8. 按需召回的 LongTerm

约束：

- 当前用户消息和回复目标永不被裁掉
- 摘要只替代“更早的原文”，不替代最近窗口
- 展示窗口仍然显示全部
- 摘要失败不影响回复
- 同一段消息只摘要一次（summary_upto 水位）
- 摘要 token 上限固定，避免摘要越滚越大

### 5.4 重建

- 启动时从 LongTerm 读取最近消息重建 recent
- 读取已持久化的 summary/summary_upto
- pending 不持久化；重启后按 recent + summary 重建
- WorkingSet 可随时重建，不要求强一致

## 6. 数据模型

### 6.1 WorkingSet

```go
type WorkingSet struct {
    ConversationID ConversationID
    Recent         []MessageSummary
    Summary        string
    SummaryUpTo    uint64
    Scene          SceneState
    HotTools       []ToolHotness
    HotMemories    []MemoryItem
    Emotion        EmotionState
    UpdatedAt      time.Time
}
```

### 6.2 LongTerm

```go
type MemoryItem struct {
    ID          string
    Scope       string
    Kind        MemoryKind
    Content     string
    Tags        []string
    Source      string
    Confidence  float64
    Strength    float64
    UseCount    int
    SuccessCount int
    FailureCount int
    EmotionalSalience float64
    LastUsedAt  time.Time
    CreatedAt   time.Time
}
```

`MemoryKind`：fact / preference / relationship / event / summary / skill / expression / jargon。

### 6.3 Learning

```go
type LearningItem struct {
    ID           string
    Scope        string
    Kind         LearningKind
    Title        string
    Content      string
    Trigger      string
    ToolHints    []string
    Tags         []string
    Confidence   float64
    Status       string // candidate / active / pinned / archived
    UseCount     int
    SuccessCount int
    FailureCount int
    LastUsedAt   time.Time
    CreatedAt    time.Time
}
```

### 6.4 Emotion

```go
type EmotionState struct {
    Valence   float64 // -1..1
    Arousal   float64 // 0..1
    Dominance float64 // -1..1
    Intensity float64 // 0..1
    Label     string
    UpdatedAt time.Time
}
```

### 6.5 Tool usage

```go
type ToolHotness struct {
    Tool         string
    UseCount     int
    SuccessCount int
    FailureCount int
    LastUsedAt   time.Time
    Weight       float64
}
```

## 7. 冷热与强化

热分数：

```text
hotScore =
  0.40 * normalizedUse
+ 0.25 * recencyDecay
+ 0.20 * relevance
+ 0.15 * emotionalSalience
```

规则：

- 长期条目默认保存，全部可查
- 被召回并实际使用：`UseCount++`、`LastUsedAt` 更新、`Strength` 上升
- 达到 `use_count >= 3 && strength >= 0.65`：进入 WorkingSet 热缓存
- WorkingSet 上限：热记忆 8、热工具 6、激活技能 4、表达 6
- 短期 TTL：30 分钟无活动衰减；话题切换清空旧话题热项
- 长期不删除，只归档/压缩；提供清理/导出

## 8. 学习流水线

### 8.1 分类

- **技能**：触发条件 + 步骤 + 关联工具 + 成功/失败统计
- **表达方式**：口癖、语气、万能回复、梗
- **黑话**：术语、缩写、内部梗
- **偏好/事实/关系**：人物喜好、事件、关系

### 8.2 产生

- 显式教学：用户说“记住……”/admin `learn` 工具
- 表达/黑话：学习插件采样消息 + LLM 抽取
- 技能：`AfterTurn` 检查成功工具调用链，LLM 抽取候选技能
- 事实/偏好/关系：总结、显式陈述、成员互动中抽取候选

### 8.3 验证

- 新条目 `candidate`，confidence 0.5
- 使用且任务成功：confidence 上升
- 失败：confidence 下降
- 技能达到 `success_count >= 2` 或人工确认才 `active`
- 同内容/标签去重；相似条目合并
- 低 confidence 自动 archived；支持手动清理

### 8.4 使用

- 表达：WorkingSet 中按标签/场景注入
- 技能：匹配触发条件时注入步骤，并给 ToolSelector 加权
- 黑话/事实/偏好：按需召回
- 所有注入受 token 预算与优先级限制

## 9. 情绪系统

### 9.1 状态

- 每会话一份 PAD；全局 baseline 可选
- 通过 Observer 更新；EmotionProvider 在 turn 开始提供快照
- 时间衰减

### 9.2 触发

- 赞美/辱骂关键词
- 被 @ / 被忽视
- 工具成功/失败、连续失败
- 用户情绪表达
- 后续可接轻量 LLM 情绪分类（非第一版必需）

### 9.3 影响

- **回复风格**：ContextProvider 注入高优先级风格块
- **工具选择**：ToolSelector 按情绪向量与工具标签算亲和度
- **决策**：回复概率/主动性/长度权重
- **记忆**：高情绪事件提高显著度

安全：

- safety / filter / governance 永远不参与情绪筛选
- 情绪不能改变工具权限

### 9.4 调试

- `emotion.debug_volatile: true`
  - 高增益、快衰减、宽范围
  - 单次最大变化可配置
- admin `emotion_set` 工具
- WebUI `/api/emotion`
- Trace 记录 emotion_before/after、工具选择原因

## 10. 工具选择

SDK：

```go
type ToolSelector interface {
    SelectTools(ctx context.Context, turn *Turn, all []ToolSpec) ([]ToolSpec, error)
}
```

`Tool` / `ToolSpec` 增加 `Tags []string`：

- `critical`：永不移除
- `social` / `playful` / `analytical` / `quiet` / `assertive`

规则：

- 保留 critical
- 按情绪/技能/热度排序
- 最多 12 个（可配置）
- 选择失败 fail-open 返回全部工具
- 记录 Trace
- 工具成功/失败写 tool_usage，驱动热工具晋升

## 11. SDK / 内核改动

### 11.1 SDK

新增：

- `internal/sdk/memory.go`：`MemoryKind`/`MemoryItem`/`WorkingSet`/`MemoryStore` 可选契约
- `internal/sdk/learning.go`：`LearningKind`/`LearningItem`/`LearningProvider`
- `internal/sdk/emotion.go`：`EmotionState`/`EmotionProvider`
- `internal/sdk/tool.go`：`Tags`
- `internal/sdk/context.go`：
  - `Turn.Emotion`
  - `Turn.Working`
  - `Turn.Learning`
  - `Turn.Tools`（最终工具）
  - `WorkingSetProvider`、`LearningProvider`、`ToolSelector`

### 11.2 Kernel / TurnEngine

`TurnOptions` 增加：

- `WorkingSetProvider`
- `EmotionProvider`
- `LearningProvider`
- `ToolSelector`

`Run` 顺序：

```text
BeforeTurn hooks
  -> WorkingSet snapshot
  -> Emotion snapshot
  -> Learning snapshot
  -> ContextProviders（persona/工作区/情绪/表达/技能/可选长期召回）
  -> Policies
  -> Decider
  -> ToolSelector
  -> LLM + Tools
  -> OutputPolicy
  -> Renderer
  -> Send
  -> AfterTurn hooks（强化/情绪/学习）
```

`buildEngine`：

- 收集单例 `WorkingSetProvider`
- 收集单例 `EmotionProvider`
- 收集单例 `LearningProvider`
- 收集单例 `ToolSelector`
- 重复注册直接报错

`generate`：

- 使用 `turn.Tools` 的 Definitions
- 不再直接 `e.tools.Definitions()`
- ToolSelector 失败时回退全量

### 11.3 AdminView / Trace

- `AdminView` 增加：
  - `WorkingSet(convID)`
  - `Emotion(convID)`
  - `Learning(scope)`
- Trace 增加：
  - `Tier` / `WorkingSummaryTokens`
  - `EmotionBefore/After`
  - `ToolSelection{Kept, Dropped, Reasons}`
  - `RecalledMemoryIDs`

## 12. 存储

新增 v5 migration（v4 -> v5）：

- `memory_items`
- `learning_items`
- `tool_usage`
- `emotion_states`
- `conversation_summaries`（可选，或并入 memory_items）

建议字段：

```sql
memory_items(
  id TEXT PRIMARY KEY, scope TEXT, kind TEXT, content TEXT,
  tags TEXT, source TEXT, confidence REAL, strength REAL,
  use_count INTEGER, success_count INTEGER, failure_count INTEGER,
  emotional_salience REAL, last_used_at DATETIME, created_at DATETIME
)
learning_items(
  id TEXT PRIMARY KEY, scope TEXT, kind TEXT, title TEXT, content TEXT,
  trigger TEXT, tool_hints TEXT, tags TEXT, confidence REAL, status TEXT,
  use_count INTEGER, success_count INTEGER, failure_count INTEGER,
  last_used_at DATETIME, created_at DATETIME
)
tool_usage(
  conversation_id TEXT, tool TEXT, use_count INTEGER,
  success_count INTEGER, failure_count INTEGER, last_used_at DATETIME,
  PRIMARY KEY (conversation_id, tool)
)
emotion_states(
  conversation_id TEXT PRIMARY KEY, valence REAL, arousal REAL,
  dominance REAL, intensity REAL, label TEXT, updated_at DATETIME
)
```

旧表：

- `memories` / `style_expressions` / `jargons` 保留
- 首次启动可选迁移为 legacy 条目
- 不破坏旧 runtime

## 13. 插件与标准包

新标准包建议：

- `memory.tiered`（替代 memory.recent + summarizer）
- `reflection.learning`（替代 stylelearn + jargonlearn）
- `drive.emotion`（新增）
- 现有 persona/renderer/attention/filter/safety/tool.* 保留

旧插件：

- 旧 bundle 保留
- 新 bundle 不同时启用
- 文档标注 deprecated

## 14. 配置草案

```yaml
memory:
  tiered:
    profiles:
      group:
        recent_max: 20
        recent_tokens: 1200
        compress_after_messages: 40
        compress_after_tokens: 1600
        summary_tokens: 400
        pending_max: 80
      private:
        recent_max: 30
        recent_tokens: 1600
        compress_after_messages: 60
        summary_tokens: 400
    longterm:
      save_all: true
      recall_limit: 5
      recall_budget_tokens: 600
    hot:
      max_memories: 8
      max_tools: 6
      max_skills: 4
      max_expressions: 6

learning:
  enabled: true
  min_confidence: 0.6
  promote_after_success: 2
  sample_every: 20

emotion:
  enabled: true
  debug_volatile: true
  valence_gain: 0.4
  decay_minutes: 5
  max_delta: 0.8
  style_budget_tokens: 120
```

## 15. 实施阶段

### Phase 0：契约与存储

- SDK 类型/接口
- Store v5 migration + repo 接口
- 兼容旧表
- 测试：migration、旧数据、空配置

### Phase 1：memory.tiered（WorkingSet + LongTerm）

- 最近窗口/pending/滚动摘要
- LongTerm 全量归档
- `memory_recall` 工具
- 热工具/热记忆与强化
- 测试：窗口溢出、摘要失败、默认不召回、重启恢复

### Phase 2：reflection.learning

- learning_items repo
- 表达/黑话迁移
- 技能抽取/验证
- ContextProvider + LearningProvider
- 测试：分类、去重、confidence、成功/失败强化

### Phase 3：drive.emotion + ToolSelector

- PAD 状态、Observer、EmotionProvider
- 情绪风格块
- ToolSelector + tool tags
- 大波动调试 + emotion_set
- 测试：高低情绪风格/工具差异、critical 工具保留、safety 不受影响

### Phase 4：WebUI/Admin 与调试

- /api/emotion
- /api/memory/working
- /api/learning
- admin emotion_set/learn/memory_recall
- Trace 页面展示

### Phase 5：收口

- 全量 `go test -race ./...`
- `go vet` / `go build`
- 架构文档、MANUAL、配置示例
- 旧 runtime 回归

## 16. 验收标准

1. 100 条消息后：
   - 展示层仍能看到全部
   - 模型上下文 = 最近窗口 + 滚动摘要
   - 摘要不重复、不丢原文
2. LongTerm 默认不注入；显式 `memory_recall` 能召回。
3. 被召回并使用的记忆 `UseCount/strength` 增加，达到阈值进入 WorkingSet。
4. expression/jargon/skill 分类存入 learning_items。
5. 技能成功/失败改变 confidence 与工具提示。
6. 高/低情绪：
   - 风格块不同
   - 工具集不同
   - critical 工具不丢
7. safety/filter/governance 不受情绪影响。
8. 重启后 LongTerm/learning/tool_usage/emotion 保留。
9. `go test -race ./...`、vet、build 全绿。

## 17. 测试计划

- 单元：
  - 窗口水位/摘要水位
  - 摘要失败保留 pending
  - 热分数/晋升/衰减
  - 学习去重/confidence
  - 情绪衰减/触发/上下限
  - ToolSelector critical/fail-open
  - 存储迁移/重启
- 集成：
  - Fake LLM + fake tool：技能学习影响工具选择
  - 高低情绪风格/工具集
  - memory_recall 工具调用
  - 标准包启动
- Race：全量 `go test -race`
- 手工：
  - `emotion_set` 大波动
  - WebUI WorkingSet/Trace
  - 真实 OneBot/LLM smoke

## 18. 风险与决策

- **长期保存全部**会增长：提供 retention/compaction、按 scope 清理、导出。
- **摘要成本/延迟**：异步，失败保留 pending，不阻塞回复。
- **工具过滤风险**：critical + fail-open，不因情绪移除安全/治理。
- **学习污染**：candidate/active、confidence、去重、人工清理。
- **多聊天模式**：只提供 profile，不各自建窗口/长期库。
- **旧 runtime**：不切默认，新能力只进新 runtime。
- **情绪稳定性**：大波动只在 debug，生产用稳定参数。

## 19. 文件地图

新增：

- `internal/sdk/memory.go`
- `internal/sdk/learning.go`
- `internal/sdk/emotion.go`
- `internal/plugins/memory/tiered`
- `internal/plugins/reflection/learning`
- `internal/plugins/drive/emotion`
- `internal/store/memory_repo.go`
- `internal/store/learning_repo.go`
- `internal/store/emotion_repo.go`

修改：

- `internal/sdk/context.go`
- `internal/sdk/tool.go`
- `internal/sdk/admin.go`
- `internal/kernel/turn.go`
- `internal/kernel/kernel.go`
- `internal/kernel/host.go`
- `internal/store/schema.go`
- `bundles/standard/bundle.go`
- `cmd/icarosu/main.go`
- `internal/plugins/interface/admin`
- `internal/plugins/interface/webui`
- `docs/ARCHITECTURE.md` / `docs/KERNEL.md` / `docs/PLUGIN_DEV.md`

## 20. 建议的第一批任务

1. SDK 契约 + v5 migration
2. `memory.tiered` 的最近窗口/摘要/长期归档
3. `memory_recall` 工具
4. `reflection.learning` 的 expression/jargon
5. `drive.emotion` 的风格块 + debug_volatile
6. ToolSelector + tool tags
7. WebUI/Admin 调试页
8. 全量测试与真实 smoke
