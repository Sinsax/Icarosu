# Icarosu Go SDK 插件化整体计划

> 状态：待评审（你判断后再开工）  
> 决策基线：**第一方 Go SDK、编译期 bundle、同进程**；WASM / JSON-RPC / MCP / 插件市场暂缓。  
> 目标：在不破坏现有行为的前提下，把 14 个事件插件迁到“微内核 + 标准包 + manifest/schema/manual”的 Go SDK 上。  
> 前置：[docs/ARCHITECTURE.md](ARCHITECTURE.md)、[docs/KERNEL.md](KERNEL.md)、[docs/RULES.md](RULES.md)、[docs/PLUGIN_DEV.md](PLUGIN_DEV.md)。

---

## 0. 一页结论

### 0.1 里程碑

| 里程碑 | 内容 | 产出 | 粗估 |
|---|---|---|---|
| **M0 稳定基线** | 先修评审发现的 P0：WebUI auth/config、Pool 竞态、SQLite 并发、notice 锁、热更新竞争 | 安全/稳定补丁 + 测试 | 1 周 |
| **M1 SDK v1** | 冻结 <code>internal/sdk</code>：Event/Action/Decision/Tool/Manifest/Host/Trace/Error；内核注册/生命周期/能力注册骨架；testkit | 可编译的 SDK + fake host + 示例插件 | 2 周 |
| **M2 OneBot adapter** | 平台中立 Event/Action；<code>qq.Client</code> 移进 adapter；旧流水线桥接 | 行为不变，<code>qq</code> 只被 adapter 引用 | 2~3 周 |
| **M3 回合内核 + 标准核心包** | Scheduler、TurnEngine、ContextProvider、Decider、ToolRegistry、ActionSink、Trace；迁移 filter/safety/recorder/recent/persona/renderer/trigger/chat/sender | 新内核可跑群聊；golden dialogue 对齐 | 4~6 周 |
| **M4 全量插件 + UX** | 剩余插件、manifest/schema/manual、配置拆分、权限/审计、WebUI 插件/手册/Trace 页 | 14 个插件全部迁移；新增插件不改内核 | 3~5 周 |
| **M5 多平台/多实例（可选）** | 第二个 adapter、profile、多 persona | 验证平台中立/实例隔离 | 4~6 周 |

> 粗估按“1 人全职、熟悉代码库”算；业余维护乘 1.5~2。**不是承诺，是排序依据。**

### 0.2 推荐顺序

~~~text
M0 稳定基线
  → M1 SDK v1（冻结契约）
    → M2 OneBot adapter（行为不变）
      → M3 回合内核 + 标准核心包（切默认）
        → M4 全量插件 + WebUI/权限
          → M5 多平台/多实例（可选）
~~~

**不要跳步**：M0 不做，后面是在沙上盖楼；M1 不冻结，M2/M3 一定返工；M3 不做，插件只是“换了目录”。

### 0.3 明确不做

- WASM / JSON-RPC / MCP / 插件市场；
- Go 标准库 <code>plugin</code> 包；
- 运行时装载/热重载代码；
- 在迁移期间加新功能（feature freeze）；
- 多平台/多实例提前到 M3 之前。

---

## 1. 目标与非目标

### 1.1 目标

1. **内核小**：只保留 7 个职责（Host/生命周期、契约/路由、调度/背压、回合引擎、工具执行、治理、可观测）。
2. **插件像插件**：每个插件有 manifest、config schema、manual、生命周期、契约测试；新增插件不改内核。
3. **平台中立**：核心不出现 QQ/CQ/group_id；OneBot 是第一个 adapter 插件。
4. **类人回合**：观察 → 评估 → 回忆 → 意图 → 表达 → 行动 → 反思；快慢分离。
5. **可回放**：每个 turn 有 Trace，能解释“为什么回/不回/为什么慢”。
6. **行为不回归**：迁移前后群聊行为一致；dry-run/golden 可验证。
7. **开发体验**：模板、testkit、lint、doctor、示例插件；写一个工具插件 < 30 分钟。

### 1.2 非目标

- 第三方插件生态/市场；
- WASM 沙箱；
- 进程外插件；
- 多平台（M5 前）；
- 重写记忆/人格算法（先搬家，不换发动机）；
- 新功能（迁移期冻结）。

### 1.3 成功标准（可验收）

- [ ] 新增一个 tool 插件：只改 <code>bundles/standard</code> + 插件目录，**0 处内核改动**；
- [ ] 14 个已定义插件（默认运行 13）全部有 manifest/schema/manual/契约测试；
- [ ] 单插件 panic / 超时不会拖垮进程或卡住其他群；
- [ ] 每个 turn 有 Trace（decision/context/tools/action/latency/cost）；
- [ ] p95 回复延迟不高于基线 + 10%；
- [ ] 标准包独立启动，golden dialogue 通过；
- [ ] CI：gofmt / vet / test / <code>-race</code> / archtest / manifestlint 全绿；
- [ ] <code>internal/qq</code>、<code>internal/store</code> 只被 adapter/storage 引用；
- [ ] 旧 <code>config.yaml</code> 自动迁移到 <code>kernel.yaml + plugins/&lt;id&gt;.yaml</code>；
- [ ] WebUI auth 接线、config API 脱敏、<code>Reload</code> 先 Validate。

> 口径：当前 **14 个事件插件**（默认运行 13）≠ 目标 **标准包 12 个插件**。标准包会重新拆分（storage/persona/renderer/memory 独立），所以“14 个迁移完”指旧事件插件全部有归宿，不一定还是 14 个包。

---

## 2. 决策基线

| 决策 | 当前选择 | 状态 | 备注 |
|---|---|---|---|
| 插件模型 | Go 同进程 + 编译期 bundle | **已定** | 你确认 |
| 第三方插件 | 暂缓 | **已定** | M5 之后 |
| WASM | 暂缓 | **已定** | 未来只做纯函数 |
| JSON-RPC / MCP | 暂缓 | **已定** | 未来工具面 |
| 配置形态 | <code>kernel.yaml</code> + <code>plugins/&lt;id&gt;.yaml</code> | 建议 | 需你确认 |
| SDK 契约 | Event/Action/Decision/Tool/Manifest v1 | 建议 | M1 冻结 |
| 权限/审计 | M1 声明，M4 强制执行 | 建议 | 第一方先防 LLM 诱导 |
| 多平台 | M5 | 待定 | 先做 QQ |
| 多实例 | M5 | 待定 | 内核按实例作用域设计 |
| WebUI | 先开发工具，M4 产品化 | 建议 | 涉及权限/审计 |
| Admin 命令 | 保留为 <code>admin</code> 插件 | 建议 | 默认仅管理员 |
| Starlark 规则层 | 暂不做 | 建议 | 有真实需求再加 |

---

## 3. 工作流

| 工作流 | 内容 | 主要产出 | 依赖 |
|---|---|---|---|
| W0 稳定 | P0 安全/并发/存储修复 | 补丁 + 测试 | 无 |
| W1 SDK | 契约、manifest、Host、testkit | <code>internal/sdk</code> + fake | W0 |
| W2 内核 | registry/lifecycle/capabilities/scheduler/turn/trace | <code>internal/kernel</code> | W1 |
| W3 接入 | OneBot adapter、Event/Action 翻译、旧桥接 | <code>plugins/adapter/onebot</code> | W1 |
| W4 标准包 | persona/memory/renderer/trigger/chat/sender 迁移 | 可跑的新内核 | W2/W3 |
| W5 全量 | 剩余插件、WebUI、权限、配置拆分 | 14 插件全迁移 | W4 |
| W6 DX | 模板、lint、doctor、示例、文档 | 开发者体验 | W1 起并行 |
| W7 多平台 | 第二个 adapter、profile、多 persona | 平台中立验证 | W5 |

---

## 4. 里程碑详情

### M0：稳定基线（1 周）

**目标**：先消除“会崩、会漏、会卡”的 P0，不改行为。

**交付**：

- [ ] WebUI <code>Start</code> 使用 <code>s.handler</code>（auth 接线）；token 为空时限制仅 loopback；<code>/api/config</code> 脱敏；<code>Reload</code> 先 <code>Validate</code>。
- [ ] <code>queue.Pool</code>：closed 标志 + 锁，<code>Submit</code> 不 send on closed；job 保留原始 ctx；补关闭竞态测试。
- [ ] SQLite：WAL + busy_timeout + 连接上限；迁移包事务；补迁移失败回滚测试。
- [ ] notice/recall 进同一会话锁；session key 改为 <code>{adapter,kind,id}</code>；Cleanup 跳过 in-use。
- [ ] attention/filter/chat 热更新加锁或 atomic snapshot；补 <code>-race</code>。
- [ ] CI 加 <code>go test -race ./...</code>。
- [ ] 关闭顺序：停 adapter → 有界 drain → 逆序 Stop 插件 → flush trace；补一次关闭测试。

**验收**：

- 现有测试 + race 全绿；
- 行为与基线一致（golden 群聊片段不回归）；
- 配置 API 不返回 secret；
- 队列/关闭竞态测试稳定。

**回滚**：每个补丁独立 PR，可单独 revert。

**风险**：SQLite WAL 对现有测试/部署有影响；先在小范围验证。

---

### M1：SDK v1 冻结（2 周）

**目标**：把接口定死，后面只实现不返工。

**交付**：

- [ ] <code>internal/sdk</code>：
  - Event / Action / Part / Conversation / Actor / IDs；
  - Plugin / Manifest / Host / Tool / Decision / ContextBlock / Trace / Error / Health；
  - 可选接口：ToolProvider / ContextProvider / Decider / Observer / Adapter / Renderer / Policy / UI。
- [ ] <code>internal/kernel</code> 骨架：
  - Registry（显式 PluginSpec）、Manifest 校验、依赖解析、拓扑排序；
  - Capability Registry（kernel.* 预置 + 插件 provides + 单例仲裁 + per-plugin facade）；
  - Lifecycle（Init/Start/Stop/Health）+ panic/timeout 包装。
- [ ] <code>sdk/testkit</code>：FakeHost / FakeLLM / FakeAdapter / FakeClock / FakeStorage。
- [ ] 示例插件：<code>tool.time</code>（纯函数）+ 契约测试。
- [ ] 架构测试：sdk 零依赖、kernel 不 import plugins、plugins 不 import 内部具体包。
- [ ] manifest lint 骨架：id/version/api_version/kind/provides/requires/optional_requires/permissions/manual。

**验收**：

- 一个 fake 插件能被注册、Init/Start/Stop、调用工具；
- 故意 panic/超时不会退出进程；
- 依赖缺失/循环/单例冲突有明确错误；
- 架构测试能拦住违规 import；
- 现有程序行为不变（新包未接线）。

**冻结规则**：M1 结束后，<code>sdk</code> 只允许“加字段/加可选接口”，不允许改已有字段语义；破坏性变更需 ADR + M2 前完成。

---

### M2：OneBot adapter（2~3 周）

**目标**：平台抽象做真，但先不换聊天逻辑。

**交付**：

- [ ] 新建 <code>internal/plugins/adapter/onebot</code>；把 <code>qq.Client</code> 移进该包（或子包），其他包不再 import <code>internal/qq</code>。
- [ ] 平台事件 → <code>sdk.Event</code>：ID/Time/Source/Conversation.AdapterID/Actor/Self/Parts/Mentions/ReplyTo。
- [ ] <code>sdk.Action</code> → OneBot：文本/图片/引用；转义 CQ；能力检查；发送幂等键。
- [ ] 旧桥接：<code>sdk.Event</code> → 旧 <code>plugin.Event</code>，继续喂旧 Manager；行为不变。
- [ ] adapter 契约测试：连接/重连/发送/图片/@/引用/去重。
- [ ] 出站统一入口：WebUI/admin 也走 ActionSink（dry-run 保持生效）。

**验收**：

- 现有群聊行为不变；dry-run 回放一致；
- <code>grep -r "internal/qq"</code> 只出现在 adapter；
- 断线重连/发送失败有测试；
- 旧流水线仍是默认，可回退。

---

### M3：回合内核 + 标准核心包（4~6 周）

**目标**：新内核真正接管群聊回合。

**交付**：

- [ ] <code>ConversationScheduler</code>：每会话 actor/队列、跨会话并行、去重、背压、关闭排空。
- [ ] <code>TurnEngine</code>：TurnHook → ContextProvider → Decider → Compose → LLM loop → Renderer → ActionSink → Trace。
- [ ] <code>ContextBudget</code>：source/priority/token/缓存前缀/丢弃记录。
- [ ] <code>Decider</code>：reply/silent/observe/defer；迁移 trigger + attention。
- [ ] <code>ToolRegistry</code>：命名空间、JSON Schema、权限、超时、结果上限、<code>read_manual</code>。
- [ ] <code>ActionSink</code>：能力检查、转义、限流、重试/幂等、dry-run、审计。
- [ ] 迁移核心 9 个（6 个旧插件 + 3 个新核心）：filter / safety / recorder / trigger / chat / sender + memory.recent / persona / renderer（trigger + attention 在 Decider 里一起迁）。
- [ ] 慢任务异步：识图/总结/学习/成员同步走 Observer，不堵回复。
- [ ] 非核心插件（steal/membersync/summarizer/stylelearn/jargonlearn/admin/ping/replylog）在 M3 先禁用或保留旧链路，M4 迁；不要阻塞核心回合。
- [ ] 每 turn Trace；WebUI 最小 Trace 查看器。
- [ ] feature flag <code>kernel.turn_engine</code>，可切回旧链路。

**验收**：

- golden dialogue（30~50 组）行为对齐；
- p95 延迟不高于基线 + 10%；
- 单插件 panic/超时不影响回复；
- Trace 可回答 decision/context/tools/action；
- 关闭时在预算内排空。

**回滚**：flag 关掉回旧流水线；核心插件保留一版旧实现。

---

### M4：全量插件 + UX（3~5 周）

**目标**：所有插件都有 manifest/schema/manual，WebUI 可用。

**交付**：

- [ ] 迁移剩余 8 个旧插件：steal / membersync / summarizer / stylelearn / jargonlearn / admin / ping / replylog。
- [ ] 14 个插件全部 manifest + config schema + MANUAL + tool manual + 契约测试。
- [ ] 配置拆分：<code>kernel.yaml</code> + <code>plugins/&lt;id&gt;.yaml</code>；旧配置自动迁移 + golden test。
- [ ] 权限/审计：声明 + scope + 关键动作强制执行；审计日志。
- [ ] WebUI：插件页、手册页、Trace 页、按 schema 渲染配置。
- [ ] 健康/就绪/指标：per-plugin health、queue depth、tool latency、send failures。
- [ ] 禁用语义：摘流量 → Stop → 注销 capability；持久化 enabled。
- [ ] 文档：插件开发 10 分钟教程、模板、lint、doctor。

**验收**：

- 新增 tool 插件只改 bundle + 插件目录；
- 14 个插件全部可启停/健康可见；
- 旧配置迁移无损；
- WebUI 无 secret 泄露；
- 标准包独立跑通。

---

### M5：多平台/多实例（可选，4~6 周）

**目标**：验证平台中立与实例隔离。

**交付**：

- [ ] 第二个第一方 adapter（CLI 或 Telegram）。
- [ ] 内核实例化：profile / instance_id / 多 persona。
- [ ] 多实例数据命名空间隔离。
- [ ] 文档：如何写第二个 adapter。

**验收**：

- 不改内核接新平台；
- 同进程多实例互不污染；
- 单实例故障不影响其他实例。

---

## 5. 第一批 PR（M0 + M1 起步）

> 每个 PR 独立可回滚；先补测试再改行为。

| # | PR | 内容 | 验收 |
|---|---|---|---|
| 1 | CI 基线 | 加 <code>-race</code>、archtest 骨架、manifestlint 骨架 | CI 绿 |
| 2 | WebUI 安全 | auth 接线、config 脱敏、Reload Validate | 安全测试 |
| 3 | Pool 稳定性 | ctx/close 竞态、关闭顺序 | 竞态测试 |
| 4 | SQLite 稳定性 | WAL/busy_timeout/事务迁移 | 并发/迁移测试 |
| 5 | 会话锁 | notice 同锁、复合 key、Cleanup 安全 | 并发测试 |
| 6 | SDK 契约 | Event/Action/Decision/Tool/Manifest/Host/Trace/Error | 编译 + 单测 |
| 7 | 内核骨架 | Registry/Lifecycle/Capability/panic/timeout | fake 插件测试 |
| 8 | testkit + 示例 | FakeHost/FakeLLM + <code>tool.time</code> | 示例跑通 |
| 9 | 架构测试 | import 规则 + manifest lint | 违规即 fail |
| 10 | 文档 | 更新 ARCH/KERNEL/RULES/PLUGIN_DEV，加迁移说明 | 链接/格式检查 |

---

## 6. 工作分解（WBS，粗粒度）

| ID | 任务 | 工作流 | 估算 | 依赖 |
|---|---|---|---|---|
| A1 | WebUI auth/config | W0 | M | — |
| A2 | Pool ctx/close | W0 | S | — |
| A3 | SQLite WAL/事务 | W0 | M | — |
| A4 | notice/session key | W0 | M | — |
| A5 | 热更新竞争 | W0 | M | — |
| B1 | sdk Event/Action/Part | W1 | M | A* |
| B2 | sdk Plugin/Manifest/Host | W1 | M | B1 |
| B3 | sdk Tool/Decision/Trace/Error | W1 | M | B1 |
| B4 | testkit | W1 | M | B2/B3 |
| B5 | 架构测试/lint | W1 | S/M | B1 |
| C1 | Registry/Manifest/依赖 | W2 | M | B2 |
| C2 | Capability Registry | W2 | M | C1 |
| C3 | Lifecycle/panic/timeout | W2 | M | C1 |
| C4 | Scheduler | W2 | L | C3 |
| C5 | TurnEngine/context | W2 | L | C4 |
| C6 | ToolRegistry/ActionSink | W2 | L | C5 |
| C7 | Trace/health | W2 | M | C5 |
| D1 | OneBot adapter | W3 | L | B1 |
| D2 | 旧桥接 | W3 | M | D1 |
| D3 | 出站统一/转义 | W3 | M | D1 |
| E1 | 迁移 filter/safety | W4 | M | C* |
| E2 | 迁移 recorder/recent | W4 | L | C* |
| E3 | 迁移 persona/renderer | W4 | L | C* |
| E4 | 迁移 trigger/chat/sender | W4 | L | C* |
| E5 | 慢任务异步 | W4 | M | E2 |
| F1 | 剩余 8 插件 | W5 | L | E* |
| F2 | manifest/schema/manual | W5 | L | F1 |
| F3 | 配置拆分/迁移 | W5 | L | F1 |
| F4 | 权限/审计 | W5 | M | F1 |
| F5 | WebUI 插件/手册/Trace | W5 | L | F1 |
| G1 | 模板/生成器 | W6 | M | B2 |
| G2 | doctor/lint | W6 | M | B5 |
| G3 | 示例/教程 | W6 | M | B4 |
| H1 | 第二 adapter | W7 | L | F* |
| H2 | 多实例/profile | W7 | L | F* |

估算：S ≈ 0.5~1 天，M ≈ 1~3 天，L ≈ 3~7 天（1 人）。

---

## 7. 质量门

| 门 | 何时 | 必须通过 |
|---|---|---|
| G0 | M0 后 | test + race + 安全测试 + 行为不回归 |
| G1 | M1 后 | SDK 编译、fake 插件、架构测试、manifest lint |
| G2 | M2 后 | adapter 契约、dry-run parity、<code>qq</code> 隔离 |
| G3 | M3 后 | golden dialogue、p95、Trace、panic/timeout |
| G4 | M4 后 | 14 插件 manifest/schema/manual、配置迁移、WebUI 无 secret |
| G5 | M5 后 | 第二 adapter、多实例隔离 |

**每个 PR 必跑**：gofmt / vet / test；涉及并发的加 <code>-race</code>；涉及行为的加 golden/dry-run。

---

## 8. 风险与应对

| 风险 | 概率 | 影响 | 应对 | 触发信号 |
|---|---|---|---|---|
| 行为回归 | 高 | 高 | golden dialogue + feature flag + 灰度 | 回复风格/触发率变化 |
| 接口返工 | 高 | 高 | M1 冻结；破坏性变更 ADR | M2/M3 频繁改 sdk |
| 性能下降 | 中 | 中 | 快路径 benchmark；慢任务异步 | p95 +20% |
| 配置迁移失败 | 中 | 高 | 自动迁移 + 备份 + golden | 旧 config 启动失败 |
| 并发 bug | 高 | 高 | per-conversation actor + race + 压测 | 重复回复/串群 |
| 范围膨胀 | 高 | 高 | feature freeze；M5 延后 | 迁移期加新功能 |
| 单人带宽 | 高 | 中 | 里程碑可停；可选插件后置 | 连续两周无进展 |
| 人格漂移 | 中 | 中 | persona regression release gate | 助手腔/长度异常 |
| 安全漏洞 | 中 | 高 | M0 先修；权限/审计/转义 | secret 泄露/越权 |
| 数据丢失 | 低 | 高 | 迁移只加不删；备份 | 迁移失败/回滚 |

---

## 9. 需要你拍板

1. **先做 M0 还是直接 M1？**  
   建议 M0：WebUI auth / Pool / SQLite / notice 是真实 P0，先修成本低。
2. **迁移期是否 feature freeze？**  
   建议是：只修 bug + 迁移，不加新功能；否则范围必炸。
3. **M1 是否包含权限/审计？**  
   建议 M1 只声明 + 关键动作检查；完整审计放 M4，避免拖慢契约冻结。
4. **配置拆分现在做还是 M3/M4？**  
   建议 M1 定 schema，M4 做文件拆分 + 迁移；不要一开始就动所有配置。
5. **WebUI 是产品还是开发工具？**  
   建议 M4 前按开发工具；产品化需要权限/审计/Trace，放后面。
6. **M5 多平台/多实例是否在范围内？**  
   建议先做到 M4，M5 按真实需求再开；不要提前。
7. **Admin 命令保留吗？**  
   建议保留为 <code>admin</code> 插件，默认仅管理员/私聊。
8. **是否接受 M1 前内部 API 破坏性变更？**  
   建议接受：项目未 1.0，内部包不承诺兼容；M1 后冻结。
9. **目标延迟/成本？**  
   建议 p95 ≤ 基线 + 10%；每 turn token/费用预算可配置。
10. **你的投入节奏？**  
    全职/业余会显著影响里程碑；计划里的粗估按全职算。

---

## 10. 我的建议

- **先批 M0 + M1**，不要一次批到 M4。M0 验证安全/稳定性，M1 验证 SDK 是否好用；两关过了再投 M2/M3。
- **关键路径是 M3**（4~6 周）。如果只能做一半，先做 M0~M2：稳定性和平台抽象已经能拿到大部分架构收益。
- **M1 结束必须有一个“10 分钟写一个 tool 插件”的 demo**；如果做不到，说明 SDK 太复杂，先改 SDK 而不是继续迁移。
- **M3 结束必须能回答**：她为什么回/不回、为什么慢、哪个插件参与了；否则 Trace 没做对。
- **M4 结束才谈多平台**；否则会把平台抽象当借口，推迟真正的标准包迁移。
- **迁移期不做 WASM/RPC/市场**；这些是“有了稳定 SDK 之后”的奖励，不是前提。

---

> **一句话**：  
> 先修地基（M0），再定接口（M1），再搬身体（M2），再换大脑（M3），最后全量装修（M4）。  
> 你判断哪一步先批，我就从哪一步开始。

## 11. 执行记录

### 2026-10-25 / M0（基本完成，剩余项见下）

已完成：

- WebUI auth 接线（真实 <code>http.Server.Handler</code> 使用 <code>s.handler</code>）；<code>/metrics</code> 也需 token；移除 query token；token 为空启动告警。
- <code>/api/config</code> 与 <code>/api/config/raw</code> 脱敏；POST 留空时保留旧 secret；<code>Reload</code> 前 <code>Validate</code>；listener panic 隔离。
- <code>queue.Pool</code>：closed 标志 + RLock、Submit 保留提交 ctx、关闭/提交竞态修复、job panic recover、测试。
- SQLite：WAL + busy_timeout + foreign_keys、连接数上限、迁移事务化 + 失败回滚。
- notice/recall 同群锁；session key 改为 <code>group:id</code> / <code>user:id</code>；Cleanup 跳过 in-use。
- attention/filter/chat/llm 热更新加锁或配置快照；全量 <code>-race</code>。
- app 关闭顺序：adapter → 队列排空 → 逆序停插件；<code>Run</code>/<code>Close</code> 都会关闭 adapter。
- plugin Dispatch / Init / Stop 的 panic 隔离。
- CI 改为 <code>go test -race ./...</code>。
- 新增测试：WebUI auth/脱敏、queue close/ctx、session key/cleanup、config 非法 reload、plugin panic、adapter close。

验证：

- <code>go test -race ./...</code> 全绿
- <code>go vet ./...</code> 通过
- <code>go build ./...</code> 通过

未完成（M0 收尾 / M1 前置）：

- <code>Manager.Start/Stop</code> 的超时（panic 已隔离）。
- Reload listener 失败回滚（目前 panic 隔离，不整体回滚）。
- token 为空时强制 loopback（目前只告警）。
- disable 真正停插件/摘能力（P1）。
- store 方法全面 ctx（P1）。

> 下一步：先做 M1 的 <code>internal/sdk</code> 契约 + 内核骨架；M0 剩余项作为 M1 期间顺手收尾。

### 2026-10-25 / M1（完成）

- <code>internal/sdk</code>：Event/Action/Part/IDs、Plugin/Manifest、Host/LLM/Memory/Tools/Actions/Storage、Tool/Decision/ContextBlock、Trace/Error/Health。
- <code>internal/manifest</code>：YAML 解析 + manifest 校验（api_version/kind/id/version/tools）。
- <code>internal/kernel</code>：显式注册、manifest 校验、capability 注册/单例冲突、依赖解析/拓扑排序、Init/Start/Stop panic→error、超时、host facade、health 聚合。
- <code>internal/sdk/testkit</code>：FakeHost/FakeLLM/FakeBus/FakeMemory/FakeToolRegistry/FakeActionSink/FakeStorage/FakeClock。
- 示例插件 <code>internal/plugins/tool/time</code>：manifest/schema/manual/tool 全套。
- <code>internal/archtest</code>：sdk 零项目依赖、kernel 不 import plugins、插件不 import kernel/其他插件。
- 测试：kernel 生命周期顺序、缺失依赖、init panic、critical start 回滚、重复 capability；manifest 正反例；time 插件工具执行。

验证：<code>go test -race ./...</code>、<code>go vet ./...</code>、<code>go build ./...</code> 全绿。

### 2026-10-25 / M2（核心库完成，app 接线留到 M3）

- <code>internal/plugins/adapter/onebot</code>：OneBot 事件 → <code>sdk.Event</code>（消息/撤回/通知）、<code>sdk.Action</code> → 消息段（text/image/reply/mention）、adapter 生命周期/能力声明。
- <code>internal/qq</code>：新增 <code>SendGroupRaw</code>/<code>SendPrivateRaw</code>，旧 Send* 改为委托，避免 CQ 拼接。
- <code>internal/bridge</code>：<code>sdk.Event</code> → 旧 <code>plugin.Event</code> 转换 + Dispatch，供迁移期旧流水线使用。
- 测试：normalize 群消息/撤回、action 消息段顺序、CQ 文本保持字面量、Start emit、legacy 转换。

未接线：app 仍走旧 <code>handler</code>/<code>plugin.Manager</code>；M3 接入新内核时启用 adapter + bridge。

### 2026-10-25 / M3（核心完成，M4 继续）

- <code>internal/kernel</code>：
  - <code>toolRegistry</code>：工具注册/JSON schema 校验/名称校验/超时/结果截断；
  - <code>actionRouter</code>：按 <code>Conversation.AdapterID</code> 路由 Action；
  - <code>TurnEngine</code>：TurnHook → ContextProvider → Policy → Decider → LLM（含工具循环）→ Renderer → ActionSink → Trace；
  - <code>Scheduler</code>：每会话串行、跨会话并行、有界 queue、关闭排空；
  - Kernel 自动收集 PersonaProvider/Decider/Renderer/ContextProvider/TurnHook/Policy/Observer/AdapterProvider，并启动 adapter + scheduler。
- 标准包 <code>bundles/standard</code>：persona.default、renderer.default、drive.attention、memory.recent、tool.time、governance.filter、governance.safety。
- OneBot adapter 包装成 <code>adapter.onebot</code> 插件；kernel 启动时连接并路由 action。
- 新入口 <code>cmd/icarosu</code>：kernel + standard bundle + OneBot adapter + <code>llm.SDKClient</code>，新运行时可直接跑（旧 <code>cmd/bot</code> 保留）。
- 测试：TurnEngine 回复/工具/策略拦截、Scheduler 同会话保序/关闭拒绝、standard bundle 启动并回复、kernel+adapter 端到端私聊发送。
- 验证：<code>go test -race ./...</code>、<code>go vet ./...</code>、<code>go build ./...</code> 全绿。

> 说明：M3 用“新核心插件 + TurnEngine”替代旧 chat/trigger/recorder/sender 语义，不是逐行搬旧代码；旧 app 仍默认走旧链路，新 <code>cmd/icarosu</code> 是独立 runtime。

### 2026-10-25 / M4（完成）

- <code>internal/kernel</code>：新增有界 Trace 缓冲、<code>sdk.AdminView</code>（PluginStatuses / Health / RecentTraces）；<code>HandleEvent</code> 自动记录 Turn Trace。
- <code>internal/plugins/interface/webui</code>：只读 HTTP 插件，提供 <code>/healthz</code>、<code>/api/status</code>、<code>/api/plugins</code>、<code>/api/traces</code>，支持 Bearer token；<code>cmd/icarosu</code> 已注册。
- <code>interface.webui</code> 首页升级为可交互状态页（token 输入、状态/插件/Trace 自动刷新）。
- <code>interface.admin</code>：<code>admin_status</code> / <code>admin_plugins</code> / <code>admin_traces</code> 三个只读工具，<code>cmd/icarosu</code> 已注册。
- <code>reflection.replylog</code>、<code>tool.system</code>（system_ping）加入标准包。
- <code>reflection.summarizer</code>：每 N 条消息异步 LLM 总结，作为 <code>memory.summary</code> 上下文。
- <code>reflection.stylelearn</code> / <code>reflection.jargonlearn</code>：采样消息异步学习表达/黑话，作为上下文块。
- 新增可选 capability：<code>sdk.MemberDirectory</code> / <code>sdk.Vision</code> / <code>sdk.MemeStore</code> 及对应 Host 接口；OneBot adapter 实现成员列表，VLM 客户端适配成 <code>sdk.Vision</code>。
- <code>reflection.membersync</code>：按间隔同步群成员名单，作为上下文。
- <code>reflection.steal</code>：图片经 VLM 判断后按概率收藏表情包。
- <code>cmd/icarosu</code>：按配置接入 VLM / 表情库；未配置时两个插件自动 no-op。
- kernel 工具/动作增加审计日志；<code>PluginStatus</code> 增加 name/description/has_manual。
- 权限第一版：<code>admin</code> 权限的工具要求 <code>Actor.IsAdmin</code>；OneBot adapter 支持管理员列表并在 normalize 时标记。
- <code>sdk.ManualProvider</code> + <code>AdminView.PluginManual</code>；WebUI 增加插件手册查询与页面入口。
- 测试：summarizer 阈值触发与上下文、stylelearn/jargonlearn 学习与 Provide。
- 配置拆分（第一版）：<code>cmd/icarosu -plugins-dir configs/plugins</code>，优先读取 <code>configs/plugins/&lt;id&gt;.yaml</code> 的 <code>config:</code> 段，缺失时回退主配置；已有测试。
- <code>cmd/icarosu -migrate-config</code>：把旧主配置里的 persona/attention/filter/safety/webui 段写成拆分后的插件 YAML；已有测试。
- <code>cmd/icarosu -kernel-config</code> + <code>configs/kernel.example.yaml</code>：内核配置/插件启停/调度参数与插件业务配置分离；已有测试。
- kernel metrics（turns/tool calls/actions/errors）+ WebUI <code>/metrics</code> 与 <code>/api/status.metrics</code>。
- 测试：WebUI endpoint、kernel trace、plugin status。
- 验证：<code>go test -race ./...</code>、<code>go vet ./...</code>、<code>go build ./...</code> 全绿。

M4 收口结果：

- 迁移剩余旧插件：steal/membersync（新插件 + capability）。
- WebUI：状态 / 插件 / Trace / 手册 / metrics / 运行时启停 / 插件配置 raw 编辑。
- 配置拆分：<code>kernel.yaml</code> + <code>configs/plugins/&lt;id&gt;.yaml</code> + <code>-migrate-config</code>。
- 权限第一版：manifest 声明 + <code>admin</code> 权限强制 + 工具/动作审计 + metrics；完整 scope/沙箱留到 post-M4。
- 双入口：<code>cmd/bot</code> 默认，<code>cmd/icarosu</code> 新 runtime，README 已明确。

> M4 完成（第一方插件范围）。后续增强：完整权限 scope、WebUI 结构化配置表单、第三方进程外插件。



