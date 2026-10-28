# Icarosu 开发规矩（Constitution v0.1）

> 目的：让后续开发和扩展**不用每次重新讨论边界**；让违规在 CI 里失败，而不是在线上。  
> 范围：内核、第一方插件、未来的第三方插件。  
> 一句话：**规矩不是限制，是把“以后会踩的坑”提前变成“现在就能检查的约束”。**

配套文档：

- [docs/ARCHITECTURE.md](ARCHITECTURE.md)：定位与目标全景
- [docs/KERNEL.md](KERNEL.md)：内核边界、缺口、第一方插件装载

**v0.2 修正（红队评审）**：

- 40 条拆成“10 条现在强制 + 30 条 Phase 门控”；存量插件用 baseline 白名单，只减不增。
- 修 R1（infra 依赖）、R7（capability 解析）、R8（disable 语义）、R36（api_version 整数 major）、R39（可判定标准）。
- CI 立即补 <code>-race</code>；archtest/manifestlint 等按 Phase 落地，不再假装已存在。
- 评审原始记录：[docs/research/kernel-review-v0.2.md](research/kernel-review-v0.2.md)。

---

## 0. 为什么要有规矩

| 没有规矩 | 有规矩 |
|---|---|
| 新功能习惯性塞进 <code>app.go</code> | 新功能有默认落点：插件目录 + manifest |
| 加一个插件要改 5~7 个文件 | 加插件只改 bundle + 自己的目录 |
| 出问题只能翻日志猜 | Trace 能回答“为什么回/不回/为什么慢” |
| 换记忆/换平台要动核心 | Host 契约稳定，实现可替换 |
| 插件越多，人格越像工具 | persona 有单一来源，回归测试守住 |
| 配置版本互相绑架 | 插件配置独立 version/migration |
| 越权/提示词注入靠自觉 | 权限 + 审计 + 转义 + untrusted 标记 |
| 代码越写越不敢改 | 契约测试 + golden dialogue + 架构测试 |

**核心判断**：规矩要能被执行，否则只是口号。每条规矩后面都要有“怎么守”：CI、测试、lint、review 或显式 ADR。

---

## 1. 一页速查（最重要的 10 条）

1. **内核不 import 插件；插件不 import 插件。**
2. **无 manifest 不加载；无 schema 不配置；无 manual 不发布。**
3. **不用 <code>init()</code> 自注册；所有插件由 bundle 显式列出。**
4. **消费型插件不能直接改世界；所有副作用走 Host；能力提供型插件只通过 Host 暴露能力。**
5. **同一会话串行，跨会话并行；禁止跨会话共享可变状态。**
6. **所有插件调用都有 ctx + timeout + recover；不允许无界 goroutine。**
7. **人格只由 persona 插件提供；其他插件只能给带来源的 context block。**
8. **工具必须 schema 校验 + 权限 + 超时 + 结果上限 + manual。**
9. **每个 turn 必须有 Trace；每个决策必须有 reason。**
10. **新插件必须带契约测试；内核改动必须跑 golden dialogue + race。**

---

## 2. 规矩总表（40 条 = 10 条 Phase 0 强制 + 30 条 Phase 门控）

> **执行方式**：§1 的 10 条从现在起强制；下面 40 条中其余条目按 Phase 门控——存量插件在迁移到对应 Phase 前用 baseline 白名单豁免，**禁止新增违规**。不要把“Phase 门控”当成“永远不用做”。

### A. 依赖与边界

| # | 规矩 | 为什么 | 怎么守 |
|---|---|---|---|
| R1 | <code>sdk</code> 只依赖标准库；<code>kernel</code> 只依赖 <code>sdk</code> + <code>infra</code>；消费型 <code>plugins</code> 只依赖 <code>sdk</code> + <code>infra</code> + 标准库 + 自己声明的第三方 | 防止核心被插件反向污染 | <code>go list</code> 架构测试 |
| R2 | 禁止 <code>init()</code> 自注册；所有插件由 <code>bundles</code> 显式列出 | import 顺序隐式、测试串扰、无法裁剪 | 全局 registry lint + review |
| R3 | 插件之间禁止 import / 直接调用；共享契约放 <code>sdk</code>，协作走 Host/Bus | 避免插件网、循环依赖 | 架构测试 + import 白名单 |
| R4 | 内核不出现平台/业务概念（qq、group_id、CQ 码、人格文案、表情包） | 平台=/=身份；核心要平台中立 | 关键词 lint + review |
| R5 | 只有一个使用方的东西先放插件；第二个使用方出现才上提内核 | 防止内核因想象力膨胀 | 内核新增能力需 ADR + 两个使用方 |

### B. 注册与生命周期

| # | 规矩 | 为什么 | 怎么守 |
|---|---|---|---|
| R6 | 无 manifest 不加载；无 schema 不配置；无 manual 不发布；manifest 必须通过 lint（id/version/api_version/kind/provides/requires/permissions/manual） | 加载器、WebUI、帮助系统需要唯一事实源 | <code>manifestlint</code> + <code>manuallint</code> CI |
| R7 | <code>provides</code> 必须注册到 capability registry；<code>requires</code> 必须能解析（内核预置 + 已加载 provides）；<code>optional_requires</code> 缺失降级；无环；关键插件失败拒绝启动 | 避免“启动到一半”的僵尸状态；避免 requires ⊆ provides 的错误 | 依赖排序单测 + 启动 smoke test + capability 冲突测试 |
| R8 | 插件必须实现 Init/Start/Stop/Health；Stop 幂等、逆序、带超时；Disable = 摘流量→Stop→注销 owned capabilities，不能只改内存 map | 生命周期可预测，避免泄漏/残留 | 生命周期契约测试 + disable 测试 |
| R9 | 第一方只做配置/启停热更新；代码不热重载；未声明 Reloader 的插件配置变更需重启 | Go 不能安全卸载代码；假装热插拔会丢状态 | Reloader 接口 + WebUI 提示“需重启” |
| R10 | 插件失败必须降级/隔离，不得拖垮内核 | 一个插件 bug ≠ 整机下线 | panic 注入测试 + health 单测 |

### C. 回合与并发

| # | 规矩 | 为什么 | 怎么守 |
|---|---|---|---|
| R11 | 同一会话同一时刻只有一个 turn；不同会话可并行；禁止跨会话共享可变状态 | 上下文错乱、重复回复、串群 | 调度器单测 + race |
| R12 | 所有插件调用带 ctx + timeout + recover；禁止无界 goroutine / 阻塞调用；时间/随机/ID 走 Host 注入 | 慢插件/panic 会拖垮整机；可测试性 | 调用包装器 + 超时注入测试 + lint |
| R13 | 所有队列有界 + 明确背压策略；不得阻塞 adapter 读循环 | 队列满时不能把接入层卡死 | 队列满/慢消费者测试 |
| R14 | 入站事件去重；出站动作幂等；平台重投不得重复回复/记忆 | 平台会重投；重复回复很“机器人” | event ID TTL 单测 + 发送幂等键 |
| R15 | 优雅关闭：停接入 → 有界排空 → 逆序停插件 → flush trace | 避免丢在途消息/DB 写入 | 关闭时序测试 + 超时测试 |

### D. 能力与副作用

| # | 规矩 | 为什么 | 怎么守 |
|---|---|---|---|
| R16 | 消费型插件的所有副作用走 Host（发消息/读记忆/调模型/定时/存储/网络）；能力提供型插件（adapter/storage）可以在自己内部封装协议/驱动，但只能通过 Host 暴露能力 | 权限、审计、预算、沙箱的唯一入口；同时允许 adapter/storage 自洽 | fake Host 契约 + import 检查 + 能力提供者白名单 |
| R17 | 权限最小化、未声明即拒绝、scope 到会话/工具；危险动作需 admin/确认 | 防 LLM 被诱导越权；防插件越界 | 权限单测 + 审计日志 |
| R18 | 外部内容（消息/网页/图片/工具结果）标记 untrusted；不得覆盖系统规则 | 提示词注入是真实威胁 | provenance 字段 + prompt 组装测试 |
| R19 | 所有出站（含 WebUI/admin/定时任务）走 ActionSink；能力检查 + 转义/结构化；禁止 CQ/mention 注入；@全体默认禁止 | LLM 输出可能包含恶意 CQ 码；出站路径不能绕过审计 | adapter 转义单测 + 注入用例 + 路径检查 |
| R20 | 密钥只从环境/secret store 注入；日志/trace/WebUI 脱敏；WebUI auth 必须接到真实 server，config API 默认脱敏 | 密钥泄露是最贵的事故；auth 死代码等于裸奔 | secret scan + API 脱敏测试 + 启动集成测试 |

### E. 上下文与人格

| # | 规矩 | 为什么 | 怎么守 |
|---|---|---|---|
| R21 | 一个实例一个 active persona；人格只由 persona 插件提供；其他插件不得改 system prompt | 防止人格被功能稀释/撕碎 | persona 唯一性启动检查 |
| R22 | 其他插件只能提供带 source/priority/token budget 的 context block；超预算丢弃要记录 | prompt 膨胀、成本失控、不可解释 | 预算器单测 + Trace 检查 |
| R23 | 不得注入隐藏 system prompt；所有 block 可回放；稳定前缀可缓存 | 可解释、可调试、省成本 | Trace 必含 block 来源 |
| R24 | 人格/模型/提示词变更必须过 persona regression（不自称 AI、不刷屏、长度/语气） | 架构改动最容易伤到“她是谁” | golden dialogue + 人工评审 |

> 第一方阶段的权限系统主要防“LLM 被诱导 / 插件误用”，不是防恶意第三方插件；声明、scope、审计先做，复杂沙箱后置。

### F. 工具与动作

| # | 规矩 | 为什么 | 怎么守 |
|---|---|---|---|
| R25 | 工具名唯一、命名空间化；必须 JSON Schema 校验 | 工具爆炸、重名、参数错误 | 注册表单测 + schema 校验 |
| R26 | 工具声明权限/超时/结果上限/副作用类型；内核统一执行，不信任插件自报 | 安全边界不能由工具自己说了算 | ToolSpec 必填字段 lint |
| R27 | 工具输出是数据不是指令；加 provenance；超长截断；错误安全 | 工具结果可能被注入/刷屏 | 结果包装器 + 截断测试 |
| R28 | 媒体用 <code>Parts</code> 交内核统一发送；工具不得自己发消息（除非显式 immediate） | 发送顺序/权限/审计要统一 | ToolResult 契约测试 |

### G. 配置与状态

| # | 规矩 | 为什么 | 怎么守 |
|---|---|---|---|
| R29 | 配置归插件；独立 schema/version/migration；内核只校验和挂载；配置项必须有实现或删除（禁止死配置） | 避免所有插件绑在一个 Config.Version 上；死配置会误导使用者 | schema lint + 迁移 golden test + dead-config lint |
| R30 | 持久状态走 Host.Storage 命名空间；迁移只加不删；内存态必须可重建或声明 ephemeral | 数据安全、可迁移、可清理 | 迁移测试 + namespace 检查 |
| R31 | 热更新必须线程安全（atomic snapshot/lock）；不得裸写共享字段 | 热更新数据竞争会崩 | <code>go test -race</code> + 配置热更测试 |

### H. 观测与错误

| # | 规矩 | 为什么 | 怎么守 |
|---|---|---|---|
| R32 | 每 turn 必须用**单一 <code>sdk.Trace</code> schema**（decision/context/tools/action/latency/cost）；决策有 reason；payload 默认 hash/截断 | 没有 Trace 就无法调“像人”；两份 schema 无法 lint | Trace 字段 lint + golden trace |
| R33 | 日志统一字段 turn_id/conversation_id/plugin_id/event_id；错误分类；用户可见错误是人话 | 可归因、可检索、不吓到用户 | 日志规范 review + 错误码单测 |
| R34 | 每个插件有 health/metrics；启动/停止/调用失败可归因 | 出问题先看健康，不靠猜 | 插件契约测试 + /readyz |

### I. 测试与兼容

| # | 规矩 | 为什么 | 怎么守 |
|---|---|---|---|
| R35 | 每插件契约测试 + fake host；内核改动跑 golden dialogue + race | 插件化后回归面变大 | CI 必跑 |
| R36 | 配置/DB 迁移有 golden test；<code>version</code> 用 semver；<code>api_version</code> 是整数 major，破坏性变更升 major | 升级不能靠运气 | 迁移测试 + manifest 校验 |
| R37 | 标准包独立跑通；新增插件不改变默认行为 | 标准包就是产品 | 标准包 smoke test + 默认对话回归 |

### J. 产品与阶段

| # | 规矩 | 为什么 | 怎么守 |
|---|---|---|---|
| R38 | 先第一方，后第三方；当前不做运行时装载/市场/Go plugin | 没有稳定 API 的生态是空壳 | 阶段评审 + ADR |
| R39 | 不为插件而插件；一个插件至少有一个独立能力域 **或** 独立生命周期（persona/decider/renderer 这类单能力但生命周期独立的也算） | 防止碎片化、接口爆炸 | 插件评审 checklist |
| R40 | 像人优先：工具数量不是成功指标；改动必须过 persona regression/静默评审 | 防止变成通用 Agent/工具商店 | release gate（不是 CI warn）+ 指标看板 |

---

## 3. 怎么守

### 3.1 CI 必跑

~~~bash
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
go list ./...          # 供架构测试解析依赖
~~~

**当前 CI 还没有跑 <code>-race</code>；Phase 0 第一件事就是加上。** 其余检查按 Phase 落地，不要一次全上。

按 Phase 新增：

| 检查 | 内容 | 失败后果 |
|---|---|---|
| <code>archtest</code> | 解析 <code>go list</code>，禁止内核/插件反向 import | Phase 0 起 CI fail |
| <code>manifestlint</code> | manifest 字段、semver、permissions、provides/requires | Phase 1 起 CI fail |
| <code>schemalint</code> | config schema 可解析、默认值合法、secret 标记 | Phase 1 起 CI fail |
| <code>manuallint</code> | 每个插件有 MANUAL.md；每个 tool 有 summary/manual | Phase 3 起 CI fail |
| <code>secret-scan</code> | 仓库不含真实 key；配置 API 不返回 secret | Phase 0 起 CI fail |
| <code>persona-regression</code> | 不自称 AI、长度/语气/静默率 | release gate（不阻塞普通 PR） |
| <code>golden-dialogue</code> | 关键群聊片段 → 期望 decision/action 属性 | Phase 2 起 CI fail |
| <code>migration-test</code> | 旧 config/DB → 新结构 | Phase 1 起 CI fail |

### 3.2 PR Checklist

每个 PR 至少回答：

- [ ] 这是内核能力还是插件？
- [ ] 为什么不能放插件？如果放内核，有第二个使用方吗？
- [ ] 新插件：manifest / schema / manual / contract test 齐了吗？
- [ ] 新配置：schema / 默认值 / migration 齐了吗？
- [ ] 新工具：permission / timeout / result limit / manual 齐了吗？
- [ ] 新 Event/Action：版本/兼容/未知字段处理了吗？
- [ ] 有 Trace / 日志字段 / 指标吗？
- [ ] 有超时、取消、panic 恢复吗？
- [ ] 跑过 race / golden / 标准包 smoke 吗？
- [ ] 文档/README/插件清单更新了吗？

### 3.3 例外流程

规矩不是绝对的，但例外必须显式：

1. 写 ADR：<code>docs/adr/NNNN-&lt;title&gt;.md</code>，说明为什么破例、影响、回滚、到期时间。
2. PR 描述链接 ADR；review 至少一人同意。
3. 代码里留 <code>// ADR-NNNN: ...</code>，到期必须处理。
4. 不允许“静默例外”；没有 ADR 的违规按 bug 处理。

### 3.4 破窗处理

- 发现违规，**先修边界，再加功能**；不要在上一个违规上叠第二个。
- 架构测试失败时，优先修 import，而不是加白名单。
- 紧急修复可以临时破例，但必须在同一周补 ADR/后续任务。
- 每月看一次：架构测试白名单、未处理 ADR、插件健康告警。

### 3.5 存量 baseline（只减不增）

R6/R8/R35 这类“每插件必须有 manifest/生命周期/契约测试”的要求，不能假装 14 个已定义插件（默认运行 13 个）在 Phase 0 就全部满足。用**只减不增**的 baseline：

| 插件 | 缺什么 | 补齐 Phase | 到期 |
|---|---|---|---|
| filter | manifest/schema/manual/契约测试 | Phase 3 | 迁移时 |
| safety | manifest/manual/契约测试 | Phase 3 | 迁移时 |
| recorder | manifest/manual/契约测试 | Phase 3 | 迁移时 |
| steal | manifest/manual/契约测试 | Phase 3 | 迁移时 |
| membersync | manifest/manual/契约测试 | Phase 3 | 迁移时 |
| summarizer | manifest/manual/契约测试 | Phase 3 | 迁移时 |
| stylelearn | manifest/manual/契约测试 | Phase 3 | 迁移时 |
| jargonlearn | manifest/manual/契约测试 | Phase 3 | 迁移时 |
| trigger | manifest/manual/契约测试 | Phase 3 | 迁移时 |
| admin | manifest/manual/契约测试 | Phase 3 | 迁移时 |
| chat | manifest/schema/manual/契约测试 | Phase 2 | 迁移时 |
| ping | manifest/manual/契约测试 | Phase 3 | 迁移时 |
| sender | manifest/manual/契约测试 | Phase 2 | 迁移时 |
| replylog | manifest/manual/契约测试 | Phase 3 | 迁移时 |

规则：

- 白名单只允许**存量**；新插件不享受豁免。
- 每迁移一个插件，就从表里删一行；表只能变短。
- 白名单条目必须写“到期 Phase”，不允许“待定”。
- 关键路径（adapter/memory/persona/decider/renderer）在 Phase 2 前必须补齐。

---

## 4. 规矩与收益对照

| 规矩组 | 防止的问题 | 带来的好处 |
|---|---|---|
| A 依赖与边界 | 循环依赖、核心被插件污染 | 可替换、可测试、可裁剪 |
| B 注册与生命周期 | 启动顺序错、资源泄漏、僵尸插件 | 可插拔、可诊断、可降级 |
| C 回合与并发 | 卡死、重复回复、饥饿、串群 | 稳定、公平、像人 |
| D 能力与副作用 | 越权、密钥泄露、CQ 注入 | 安全、可审计、可沙箱 |
| E 上下文与人格 | prompt 膨胀、人格漂移、不可解释 | 成本可控、身份连续 |
| F 工具与动作 | 工具注入、慢工具、越权发送 | 工具可信、发送可控 |
| G 配置与状态 | 版本地狱、迁移冲突、丢数据 | 独立演进、可回滚 |
| H 观测与错误 | 无法归因、用户看到堆栈 | 快速定位、体验稳定 |
| I 测试与兼容 | 回归、升级翻车 | 敢改、敢发 |
| J 产品与阶段 | 功能膨胀、定位漂移 | 始终是“她”，不是工具商店 |

---

## 5. 违规的代价（来自当前代码的真实教训）

| 现状 | 后果 |
|---|---|
| <code>platform.Event = qq.Event</code> | Telegram adapter 加了又 revert；第二个平台要重写核心 |
| <code>app.go</code> 手工 new 所有插件 | 加插件要改 app/config/webui/test，插件化名不副实 |
| 慢任务在 preprocessor 同步跑 | 识图/总结/学习会拖慢“要不要接话” |
| <code>Manager.Dispatch</code> 无 recover | 一个插件 panic 带走整机 |
| <code>config.Config</code> 单体 version 12 | 加一个字段要改 6 个文件，插件迁移被核心绑架 |
| WebUI <code>/api/config</code> 返回完整配置 | API Key 可能被有 token 的人看到 |
| 出站直接发字符串 | LLM 输出 CQ 码可能 @全体/注入 |

这些不是“代码风格问题”，是**架构债**。规矩的作用就是防止下一笔债。

---

## 6. 版本与修订

- 本文件版本：<code>v0.1</code>；与内核 <code>api_version: 1</code> 同步演进。
- 修改规矩需要：ADR + 说明影响 + 更新 CI/测试。
- 删除规矩比新增更难：必须证明“这条约束已经被更强的机制替代”。
- 每季度回看一次：哪些规矩从未被违反（可能已内化）、哪些经常破例（可能设计有问题）。

---

> **一句话收尾**：  
> 规矩不是为了让你写得慢，是为了让你**敢改、敢换、敢加插件，而不用每次重新害怕一遍**。
