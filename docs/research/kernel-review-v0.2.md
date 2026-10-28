> 红队评审批次：v0.1 → v0.2。原始记录保留；部分问题已在 v0.2 文档修正（见 KERNEL 3.0 / RULES v0.2 注）。

# Icarosu 内核文档红队评审（第一方插件阶段）

> 总评 1：方向成立——第一方编译期插件、7 职责、manifest/schema/manual 三件套，与代码现状的取舍是清醒的。
> 总评 2：最贵的不是缺功能，而是接口层三处互斥（依赖解析、Host 能力来源、Event/Action 身份），按现状写代码必然在 Phase 1 返工。
> 总评 3：P0/P1 缺口清单漏掉 4 个真问题（WebUI 认证未接线、Pool 关闭竞态、SQLite 并发、notice 绕会话锁）；40 条规矩当前不可全量执行。

标记：**[F]** 事实错误/文档互斥，**[D]** 设计分歧/规格缺口，**[R]** 规矩问题。行号基于当前工作区；未修改任何源码或文档。

## 1. 七职责对照（K1–K7）

| 职责 | 判断 | 关键问题（证据） |
|---|---|---|
| K1 Host/生命周期 | 偏多、未落地 | 指标/健康在 K1 与 K7 重复（KERNEL:57,63）；disable 只改内存标记（manager.go:41-49），规范却要求停插件+摘工具（KERNEL:76,428）；缺配置写入/回滚与文件目录命名空间（KERNEL:467） |
| K2 契约/路由 | 少 | Conversation 无 AdapterID，K2 却按它路由（KERNEL:91 vs ARCH:741-746）；缺 adapter capability/part 协商、出站幂等键（ARCH:783, RULES:75） |
| K3 调度/并发 | 方向对、缺件 | notice 绕过会话锁（handler.go:136-157）；队列满策略、LLM/工具信号量、defer 重入队均无参数定义（KERNEL:105-110,397-399） |
| K4 回合引擎 | 边界偏多 | 内核持有默认 LLM 客户端（KERNEL:126）与“model 插件”（ARCH:720）冲突；persona 唯一性校验应在 K1 却写在 K4（KERNEL:127）；Decision 无 observe、Defer 无执行路径（KERNEL:248-254,333-353） |
| K5 工具执行 | 少+多 | 多：内核内置 read_manual 具体工具（KERNEL:148），违反“不做具体工具”（KERNEL:150, ARCH:549）；少：工具归属/注销/结果上限/副作用声明（ARCH:863-871 vs RULES:104-106） |
| K6 治理 | 重叠 | 限流/预算同时归内核（KERNEL:160-161）和 governance.safety（KERNEL:718）；缺 secrets 读取、审计落盘、Policy 合成顺序（ARCH:1078, KERNEL:164） |
| K7 可观测 | 依赖错位 | 健康/指标实际在 WebUI（webui/server.go:122-123），而 WebUI 默认关闭且非关键（defaults.go:101, KERNEL:719）→ 默认部署没有探针 |

## 2. 问题清单

**P0-1 [F] WebUI 认证是死代码，配置与密钥裸奔**
- 问题：`s.handler = s.auth(mux)` 只给测试用；真正监听的 `http.Server.Handler` 是裸 mux。默认 token 为空，即使走 auth 也不校验；/api/config 返回完整 Config（含 LLM/VLM/OneBot token），/api/config/raw 返回原始 YAML。KERNEL 只写成“有 token 的人能看到”，实际是“能访问端口的人都能”。
- 证据：`internal/webui/server.go:138,141,153-166,184-188`；`internal/webui/handlers_config.go:18,66-72`；`internal/webui/server_test.go:77,112`（只测 Handler()）；`internal/config/defaults.go:101-103`；`docs/KERNEL.md:550`。
- 建议：Start 改用 s.handler；token 为空时强制仅绑 127.0.0.1 并关闭 /api；config API 返回脱敏结构（只留 secret_set 布尔）；POST 校验 Validate+Origin；去掉 query token。
- 严重度：P0

**P0-2 [F] queue.Pool 关闭/提交竞态，且 job 的 ctx 被丢弃**
- 问题：Close 直接 close(jobs)，并发 Submit 的 select 发送会 panic（send on closed channel）；worker 用 `context.Background()` 执行 job，取消与关闭预算失效；app 关闭时先 Close pool、却不停 adapter、也不 Close 客户端，读循环仍可回调 Submit。直接违反 R12/R13/R15。
- 证据：`internal/queue/pool.go:40-50,53-58,63`；`internal/app/app.go:371-375,383,398-413`；`internal/qq/client.go:168-174`。
- 建议：Pool 加 closed 标志+锁/errgroup，Submit 双检查；job 携带原始 ctx；关闭顺序“停 adapter→有界 drain(deadline)→逆序 Stop→flush”；补关闭竞态与 panic 注入测试。
- 严重度：P0

**P0-3 [D] SQLite 并发与迁移一致性没有方案**
- 问题：未设 WAL/busy_timeout/连接数上限，4 worker 并发写会 SQLITE_BUSY；迁移逐条 Exec 无事务，失败留半迁移且无版本记录；store 方法普遍无 ctx，DB 卡住无法取消 turn。文档 P0 #9 只谈命名空间、P1 #13 只谈 ctx，都没谈写并发。
- 证据：`internal/store/db.go:37-44,54-111`；`docs/KERNEL.md:545,568`。
- 建议：DSN 注入 `_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)`，写连接串行化；迁移包 tx+备份+失败回滚；仓储全面加 ctx；把“存储并发/一致性”列为 P0。
- 严重度：P0

**P0-4 [D] requires/provides 规则与示例互斥；Host 能力由插件提供却没有绑定机制**
- 问题：按字面 `requires ⊆ provides`，示例 manifest 的 `kernel.bus/kernel.actions/kernel.tools` 无人 provide，加载器会拒绝所有插件；Host.Memory/Storage 却要由 memory/storage 插件实现，规范把 Host 当内核自带能力，没有“插件提供内核端口”的注册、单例仲裁与就绪顺序；storage.sqlite 又是关键插件，形成 Host.Storage↔插件的环。
- 证据：`docs/ARCHITECTURE.md:1039,912-914`；`docs/KERNEL.md:675-677,609-624,290-301,475,709`。
- 建议：kernel.* 定义为内核预置 capability；新增运行时 service registry（Provider 注册、exactly-one、依赖就绪后再 Init）；storage 先于依赖插件启动；规则改为“可解析”而非“子集”。
- 严重度：P0

**P0-5 [F] 热更新链路无校验、非原子，且仍有多处数据竞争**
- 问题：Manager.Reload 只 Load 不 Validate；WebUI 保存也不校验；Load 在迁移/规范化时会主动重写配置文件，且与 WebUI 写文件无锁；listener 无错误返回、无回滚，任一 panic/失败即半更新。数据竞争至少还漏了 attention：Observe 读 `t.cfg` 未加锁；steal 在构造时复制 persona，热更只更新 chat，违反“人格单一来源”。
- 证据：`internal/config/manager.go:39-55`；`internal/config/load.go:31-57,331-345`；`internal/app/app.go:113,340-359,233`；`internal/attention/tracker.go:35-41,47`；`internal/plugin/filter/filter.go:41-43,62-65`；`docs/KERNEL.md:548`。
- 建议：Validate 后再原子 swap；staged config 全量 apply 成功才 commit，listener 返回 error；文件锁/atomic rename；热更字段统一 atomic.Pointer 或 RWMutex（含 Observe 读）；persona 只保留一个 provider，steal 经 Host 读取。
- 严重度：P0

**P1-6 [F] KERNEL P0#2 诊断错误：读循环没被阻塞，真正风险是无界 goroutine**
- 问题：文档称“队列满时阻塞 adapter 读循环”。实际 qq.Client 每个事件起 goroutine（带 recover），readLoop 继续读；Submit 阻塞发生在事件 goroutine，真正风险是无界 goroutine、worker 被会话锁占住（head-of-line）、Submit 的 ctx 永远是 Background。按错误诊断做“非阻塞 Submit”会掩盖背压缺失。
- 证据：`docs/KERNEL.md:538`；`internal/qq/client.go:168-174`；`internal/app/app.go:371-374`；`internal/handler/handler.go:106-109`。
- 建议：改为“每事件无界派生 goroutine + 会话锁占满 worker”；在 adapter→kernel 边界加有界 in-flight 信号量与丢弃计数；调度器用每会话 actor，别让 worker 等锁。
- 严重度：P1

**P1-7 [D] Event/Action/Decision 缺“身份”，R14/R17 无法落地**
- 问题：目标 Event 有 ID/Time，K2 去重靠 adapter event ID，但现有事件无 ID/Time/来源；Conversation 无 AdapterID 却被 K2 用来路由；Decision 的 Speak/Silent 可同时为真、无 observe、Defer 只有结构没有重入队/上限；Action 无 ID/Actor/TurnID，出站幂等与 scope 校验没有挂点。
- 证据：`docs/KERNEL.md:91,248-254,333-353`；`docs/ARCHITECTURE.md:741-746,123,769-776`；`docs/RULES.md:75,83`。
- 建议：Phase 0 冻结 sdk：Event{ID,OccurredAt,Source.AdapterID,Mentions}；Decision 改枚举（reply/silent/observe/defer）+ Defer{CancelKey,MaxAttempts}；Action{ID,TurnID,Actor,IdempotencyKey}；写明 defer 谁定时、何时取消、重复如何幂等。
- 严重度：P1

**P1-8 [D] 权限模型与 Host 能力集不闭合**
- 问题：权限表声明 network.outbound/filesystem.workspace/secrets.read/schedule/admin，但 Host 没有 Secrets/HTTP/FS/Sandbox/Scheduler；Host 在 Init 时整体交给插件，“未声明即拒绝”只能是注释；interface 插件要挂路由，Host 也没有 Router/UI 能力，只能自起服务器，绕过“内核不做页面”和 R16。
- 证据：`docs/ARCHITECTURE.md:1069-1078,627-632`；`docs/KERNEL.md:290-301,331`；`docs/RULES.md:82-83`。
- 建议：按 manifest 生成 per-plugin Host facade（deny-by-default），补 Secrets/HTTP(域名白名单)/Scheduler/Route 端口；第一方阶段明确“权限=声明+审计，非沙箱”，并用 archtest 兜底。
- 严重度：P1

**P1-9 [F] 三份文档给出三套 id/工具名/bundle/config 形态，无法同时实现**
- 问题：id 有 `onebot` / `adapter.onebot` / `tool.meme`；工具名有 `send_meme` / `meme_send` / `plugin_tool`；bundle 有 `[]sdk.Plugin` 与 `[]sdk.PluginSpec{ID,New,Critical}`；配置有单文件 `plugins:` 与 `configs/plugins/*.yaml`；目录有 `internal/plugins`、`internal/bundles`、根 `bundles/`。另外“14 个插件”是条件注册（默认无 VLM 时 steal 不存在）。
- 证据：`docs/ARCHITECTURE.md:904,932,948-957,1211-1225,1007,26`；`docs/KERNEL.md:667,683-692,609-624,455-463,229,639,750`；`internal/app/app.go:229-243`。
- 建议：Phase 0 只保留一份 schema：id=`<kind>.<name>`、tool=`<plugin>_<tool>`（send_image/send_meme 保留一版别名）、bundle 只收 New+Manifest、配置二选一；可选依赖在 manifest 声明 optional_requires 并暴露 degraded，禁止运行时静默不注册。
- 严重度：P1

**P1-10 [F] “禁用插件=停插件+摘工具”在代码中不存在**
- 问题：SetEnabled 只改 map，Dispatch 跳过；已 Init 的插件不 Stop，工具不注销、页面不摘除（未来 provider 也无注销 API），重启即恢复。关键插件清单三处不一致，且运行期禁用 persona/adapter 的语义未定义。
- 证据：`internal/plugin/manager.go:41-49,116-146`；`internal/webui/handlers_plugin.go:40`；`internal/tools/registry.go:35-44`（无 Unregister）；`docs/ARCHITECTURE.md:1042`；`docs/KERNEL.md:422,428,709-714`。
- 建议：Disable = 摘流量→有界 drain→Stop→注销 owned capabilities，并持久化 `plugins.<id>.enabled`；关键性只由 manifest 一处声明；critical 禁用要求重启+显式确认。
- 严重度：P1

**P1-11 [F] 会话串行只覆盖 message；notice 绕锁 + group/private key 冲突**
- 问题：notice/recall 直接 Dispatch 不加锁；session key 是裸 int64，群 123 与私聊用户 123 共用锁；Cleanup 会删除正在 Lock 的 session，下一次 Get 新建 mutex → 同会话并发 turn。
- 证据：`internal/handler/handler.go:106-125,136-157,65-67`；`internal/session/manager.go:57-66`；`docs/ARCHITECTURE.md:486-487`（只承认命名空间缺失，未进 P0/P1）。
- 建议：key 改 `{adapter,kind,id}`；notice 与 message 进同一 per-conversation actor；Cleanup 跳过 in-use（引用计数）；补 race 与并发消息测试。
- 严重度：P1

**P1-12 [F] 日志脱敏与 Trace 口径自相矛盾**
- 问题：Info 级日志直接打 LLM reasoning、回复、tool 参数与结果、dry-run 全文；K7 说“消息内容只在 debug”，R20 要求脱敏，Trace 示例又含消息正文。Trace 两份 schema 字段不同（context_blocks/llm_rounds vs blocks/llm/tools/actions），R32 无法 lint。健康/指标只在 WebUI，默认关闭即无探针；`kernel_queue_depth{conversation}` 无界标签且 /metrics 不鉴权。
- 证据：`internal/llm/client.go:195,202,218,226`；`internal/plugin/chat/chat.go:295-302`；`internal/dryrun/bot.go:25-28`；`docs/KERNEL.md:180,492-499,505`；`docs/ARCHITECTURE.md:1102-1110`；`internal/webui/server.go:122-123,180`；`internal/config/defaults.go:101`。
- 建议：单一 sdk.Trace schema；payload 默认 hash/截断，仅显式 debug+采样；分离最小 HTTP 探针；标签限 adapter/kind，conversation 只进日志；补延迟直方图、队列深度、发送失败。
- 严重度：P1

**P1-13 [R] 规矩硬冲突、执行空转，且对存量过重**
- 问题：R1 允许 kernel→infra，ARCH 6.1 只允许 sdk+stdlib；R36 要求 api_version semver，manifest 写 `api_version: 1`（int）；R24/R40 说“必须”过 persona regression，CI 表却是 warn；R39 的判据（2~3 能力“或”独立生命周期）不可判定：按能力口径 persona/decider/renderer 是单能力，按生命周期口径又全合规。CI 无 `-race`，archtest/manifestlint/schemalint/manuallint/golden 仍是“建议新增”，但规则表已把它们当“怎么守”；R6/R8/R35 又要求每个插件 manifest/生命周期/契约测试，14 个存量插件在 Phase 0 就会集体违规，且只能逐条 ADR 豁免。
- 证据：`docs/RULES.md:52,129,95,138,137,163,150,156-165,186`；`.github/workflows/ci.yml:18-31`；`docs/ARCHITECTURE.md:1190-1194`；`docs/KERNEL.md:709-714`。
- 建议：规矩分两级：现在可执行的 10~12 条（import/panic/队列/密钥/race）+ Phase 门控目标；给存量插件 baseline 白名单+到期日；先解决 4 处冲突再宣称“40 条”；CI 立刻加 -race。
- 严重度：P1

**P2-14 [F] 其余事实误差（不伤方向，但会误导实现）**
- 问题：KERNEL 把 admin send 的 dry-run 一致性列为被绕过，实际 `s.bot` 是 dryrun 包装（app.go:205-208,323），dry-run 仍生效，只绕过队列/限流/审计；ARCH 说 style/attention/session“热重载即失忆”，reload 不重建它们，attention.Update 只换 cfg（tracker.go:35-41），失忆只发生在重启；ARCH 把“当前消息”列为独立第 10 块，buildMessages 正常路径只回放 recent（service.go:519-542），当前文本只用于语义/话题（485-493），图片描述另存 memory（memory.go:104-111）；“14 个插件”是上限，默认无 VLM 时 13 个；“指标只有 4 个计数”不准确，/metrics 还有 uptime/LLM/DB 计数（handlers_status.go:24-45），但仍无延迟/队列。
- 证据：`docs/KERNEL.md:566,567,750`；`docs/ARCHITECTURE.md:500,260,26`；`internal/app/app.go:205-208,229-243,323`；`internal/core/chat/service.go:485-493,519-542`；`internal/memory/memory.go:104-111`。
- 建议：把上述 5 处改成实测口径；P0/P1 表里凡是“证据”列，尽量写可复现命令/文件行，而不是印象。
- 严重度：P2

## 3. 如果只修三个

1. **修 WebUI 认证与配置可信度（P0-1 + P0-5）**：Start 走 auth、默认拒绝、config 脱敏、Validate+原子 reload。否则“权限/审计/密钥”三条线都建在沙上。
2. **冻结装载与能力契约（P0-4 + P1-7/8/9）**：id/工具命名、requires 解析、service provider、capability-scoped Host、Event/Action/Decision 身份字段，一次定死；这五个决定 Phase 1 是否返工。
3. **补进程稳定性（P0-2 + P0-3 + P1-11）**：Pool ctx/close、关闭顺序、SQLite WAL/busy_timeout/事务迁移、per-conversation actor。不做这三项，P0 清单的其余改造只是“把一个大函数拆成更多能崩的点”。

> 不建议现在全量执行 40 条；先用“10 条可执行 + 8 条 Phase 门控”的版本，等 P0 清单落地后再逐条转正。
