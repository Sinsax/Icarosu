# Icarosu 插件开发推荐

> 结论先行：**现在用 Go 同进程插件；WASM 不是现在的答案。**  
> 第一方插件要的是“开发快、调试快、类型安全”，不是“沙箱最潮”。  
> 第三方/不可信插件以后再用进程外 JSON-RPC；WASM 留到最后。

前置：[docs/KERNEL.md](KERNEL.md)（内核边界）、[docs/RULES.md](RULES.md)（开发规矩）。

---

## 0. 一页速查

| 场景 | 推荐 | 不推荐 | 理由 |
|---|---|---|---|
| 现在：第一方插件 | **Go 同进程 + 显式 bundle** | WASM、JSON-RPC、Go <code>plugin</code> | 同语言、同调试器、同测试；无序列化/ABI |
| 未来：第三方工具 | **JSON-RPC over stdio** 或 **MCP（仅工具面）** | 一上来做 WASM | 进程隔离 + 多语言，复杂度远低于 WASM |
| 用户自定义规则 | **Starlark**（或 Lua）嵌入式脚本 | 让用户写 Go/编译 | 安全、纯函数、改完即生效 |
| 不可信纯函数 | **WASM（wazero）** 以后再说 | 现在把 WASM 当主插件模型 | ABI/工具链/调试成本高，收益要等生态 |
| 多语言完整插件 | 进程外 JSON-RPC | 同进程 FFI | 崩溃隔离、语言自由 |
| 只加一个工具 | MCP server | 写完整插件 | 生态现成、Python/TS 十分钟一个 |

---

## 1. 现在：Go 同进程插件

### 1.1 为什么

- **开发最快**：同一个 <code>go test</code>、同一个 debugger、同一个 profiler。
- **类型安全**：<code>sdk.Tool</code>、<code>sdk.ContextBlock</code> 编译期检查，重构不怕漏。
- **零序列化**：不用 JSON/protobuf 转换，不担心 ABI 版本。
- **不需要沙箱**：第一方插件是你自己写的，信任边界不在插件，而在“LLM 输出/外部内容”。
- **调试简单**：断点直接打在 <code>Provide</code> / <code>Execute</code> 里。

### 1.2 不是什么

- **不是 Go 标准库 <code>plugin</code> 包**：那是 <code>.so</code> 动态加载，平台/工具链限制多、不能卸载、崩溃拖垮主进程。
- 我们说的是**编译期 Go 包 + 显式 bundle**：<code>internal/plugins/...</code> 正常 <code>go build</code> 进去，由 <code>bundles/standard</code> 列出。

### 1.3 最小插件长什么样

~~~go
package time

type Plugin struct{}

func New() sdk.Plugin { return &Plugin{} }

func (p *Plugin) Manifest() sdk.Manifest { return sdk.MustManifest(manifestYAML) }

func (p *Plugin) Init(ctx context.Context, host sdk.Host) error { return nil }
func (p *Plugin) Start(ctx context.Context) error               { return nil }
func (p *Plugin) Stop(ctx context.Context) error                { return nil }
func (p *Plugin) Health(ctx context.Context) sdk.Health         { return sdk.Healthy() }

func (p *Plugin) Tools() []sdk.Tool {
    return []sdk.Tool{{
        Name:    "time_now",
        Summary: "获取当前日期和时间",
        Schema:  json.RawMessage(`{"type":"object","properties":{}}`),
        Manual:  timeNowManual,
        Execute: func(ctx context.Context, call sdk.ToolCall) (sdk.ToolResult, error) {
            return sdk.Text(time.Now().Format("2006-01-02 15:04:05")), nil
        },
    }}
}
~~~

### 1.4 推荐的接口形状

不要一个 20 方法的 <code>Plugin</code> 上帝接口。用“生命周期 + 可选能力”：

~~~go
type Plugin interface {
    Manifest() Manifest
    Init(ctx context.Context, host Host) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health(ctx context.Context) Health
}

type ToolProvider interface { Tools() []Tool }
type ContextProvider interface { Provide(ctx context.Context, t *Turn) ([]ContextBlock, error) }
type Decider interface { Decide(ctx context.Context, t *Turn) (Decision, error) }
type Observer interface { Observe(ctx context.Context, ev Event) }
type Adapter interface { Start(ctx context.Context, emit EmitFunc) error; Send(ctx context.Context, a Action) error; Close() error }
type Renderer interface { Render(ctx context.Context, t *Turn, reply string) ([]Action, error) }
type Policy interface { Check(ctx context.Context, t *Turn) error }
~~~

一个插件实现 <code>Plugin</code> + 需要的可选接口即可；不要强迫所有插件实现所有能力。

---

## 2. WASM 会不会太麻烦？

### 2.1 结论

**现在太麻烦，收益不成正比。**  
WASM 解决的是“不可信代码的沙箱”，不是“插件开发效率”。你现在只做第一方插件，这个问题不存在。

### 2.2 麻烦在哪

| 成本 | 说明 |
|---|---|
| ABI/宿主函数 | 要设计内存布局、字符串传递、错误码、host call 协议 |
| 序列化 | 每次调用都要 JSON/msgpack 编解码，调试麻烦 |
| 异步/网络 | WASM 里做 HTTP/WebSocket/DB 很别扭，通常要宿主代理 |
| 语言工具链 | Go/Python/JS/Rust 各自的 WASM 目标、WASI 版本、依赖打包 |
| 调试 | 断点/堆栈/日志都比同进程难一个量级 |
| 版本 | ABI、WASI、host function 都要版本化 |
| 性能 | 小调用有实例化/边界开销；大调用才划算 |
| 生态 | 很多库在 WASM 下不可用（CGO、线程、文件系统） |

### 2.3 各语言 WASM 现状

- **Go**：<code>GOOS=wasip1 GOARCH=wasm</code> 可编译，<code>wazero</code> 可跑；但二进制大（MB 级）、标准库/网络受限、调试一般。
- **Rust**：WASM 最顺，配 <code>extism</code>/<code>wasmtime</code> 生态好；但要引入 Rust。
- **Python/JS**：运行时重、打包复杂，不适合“小而快”的插件。
- **TinyGo**：二进制小，但语言子集/生态受限。

### 2.4 什么时候再考虑 WASM

满足任意两条再考虑：

- 要运行**不可信第三方**代码；
- 插件是**纯函数**（过滤、打分、改写、策略），不需要 DB/长连接；
- 插件数量多、需要密集隔离/快速启停；
- 要边缘部署/浏览器复用；
- 已有 Rust/Extism 技术栈。

**如果只是第三方工具**：进程外 JSON-RPC/MCP 已经给你 80% 的隔离收益，成本 20%。

### 2.5 如果以后做 WASM

建议：

- 运行时：<code>wazero</code>（纯 Go、零依赖）或 Extism；
- ABI：JSON in/out + 少量 host functions，不要自定义复杂内存协议；
- 能力：默认无网络/无文件；只给 <code>log</code>、<code>kv</code>、<code>http(domain allowlist)</code>；
- 限制：单次超时、内存上限、调用次数；
- 范围：只做纯函数钩子（filter/score/rewrite），不做 adapter/memory/storage；
- 版本：<code>abi_version</code> + 每函数 schema。

---

## 3. 语言/方案对比

| 方案 | 开发速度 | 隔离 | 语言 | 适合 | 结论 |
|---|---|---|---|---|---|
| Go 同进程 | ★★★★★ | 无 | Go | 第一方、adapter、memory、tool | **现在就用** |
| Go <code>plugin</code> 包 | ★★ | 同进程 | Go | 不推荐 | ❌ |
| 嵌入式 Starlark | ★★★★ | 逻辑沙箱 | Starlark | 用户规则/过滤/改写 | **可选，规则层** |
| 嵌入式 Lua | ★★★★ | 需自建限制 | Lua | 用户脚本/游戏式扩展 | 可选 |
| 嵌入式 JS（goja） | ★★★ | 需自建限制 | JS | Web 背景用户 | 不优先 |
| JSON-RPC 进程外 | ★★★ | 进程隔离 | 任意 | 第三方工具/adapter | **未来主力** |
| MCP server | ★★★★★ | 进程/远程 | Python/TS | 只加工具 | **未来工具面** |
| gRPC/hashicorp go-plugin | ★★ | 进程隔离 | 多语言 | Go 生态 RPC | 不优先 |
| WASM | ★★ | 强沙箱 | Rust/Go/TinyGo | 不可信纯函数 | **以后再说** |
| HTTP/WebSocket 服务 | ★★★ | 网络隔离 | 任意 | 已有服务集成 | 视情况 |

---

## 4. 外部插件协议（未来）

如果以后要让别人用 Python/TS 写插件，建议**窄 JSON-RPC over stdio**，不要一上来 gRPC：

### 4.1 方法集（控制在 10 个以内）

| 方法 | 方向 | 作用 |
|---|---|---|
| <code>plugin.manifest</code> | host → plugin | 握手：id/version/api_version/capabilities |
| <code>plugin.init</code> | host → plugin | 注入配置、host 能力 |
| <code>plugin.start</code> / <code>stop</code> | host → plugin | 生命周期 |
| <code>plugin.health</code> | host → plugin | 健康检查 |
| <code>tools.list</code> | host → plugin | 工具清单 + schema + manual |
| <code>tools.call</code> | host → plugin | 执行工具，返回 text/parts |
| <code>context.provide</code> | host → plugin | 请求上下文块（可选） |
| <code>events.observe</code> | host → plugin | 异步观察事件 |
| <code>actions.send</code> | plugin → host | 请求发消息/发图 |
| <code>config.get</code> | plugin → host | 读取自己的配置 |

### 4.2 传输与工程细节

- stdio：newline-delimited JSON-RPC 2.0（最简单，日志走 stderr）。
- 远程：HTTP POST JSON-RPC（同一套方法）。
- 超时：每次调用带 deadline；host 侧 kill/restart。
- 幂等：<code>call_id</code> / <code>idempotency_key</code>。
- 日志：stderr 输出 JSON lines，host 收集并打 <code>plugin_id</code>。
- 配置：host 在 <code>init</code> 时下发；插件自己校验 schema。
- 密钥：显式传，不继承环境变量。
- 崩溃：指数退避重启；连续失败 N 次标记 degraded。
- 版本：<code>api_version</code> 握手；不兼容拒绝加载。

### 4.3 语言 SDK 优先级

1. **Python**：AI 工具生态最好，写 MCP/JSON-RPC 最快。
2. **TypeScript/Node**：Web/API 集成方便。
3. **Go**：第一方/性能/单二进制。
4. **Rust**：WASM/高性能，后期。

---

## 5. 用户自定义规则：Starlark 还是 Lua？

如果目标是“让普通用户改一点行为”，不要让他们写完整插件；给一个**规则脚本层**：

### 5.1 Starlark（推荐先试）

- Go 原生库 <code>go.starlark.net</code>；Python 风格；确定性；默认无 I/O。
- 适合：触发条件、过滤、回复改写、打分。
- 不适合：网络、DB、长任务、复杂依赖。

~~~python
def should_reply(event):
    if event.at_bot:
        return True
    return "摸鱼" in event.text or "下班" in event.text
~~~

### 5.2 Lua（更灵活）

- <code>gopher-lua</code> 嵌入简单；用户熟悉；可暴露 host 函数。
- 需要自己做指令数/内存/超时限制；能力越强，越要小心。
- 适合：更复杂的用户脚本、游戏式扩展。

### 5.3 JS（goja）

- Web 背景用户熟悉；但运行时重、异步语义复杂；不优先。

**建议**：先 Starlark 做“规则层”，不要做“插件层”。规则层只返回数据/布尔，不直接发消息；副作用仍由内核 ActionSink 执行。

---

## 6. MCP：最省事的外部工具

- **适合**：给 LLM 加一个工具/资源；Python/TS 写 server 很快；host 管理进程。
- **不适合**：QQ adapter、事件总线、配置 UI、权限/审计。
- **用法**：写一个 MCP server → 在 Icarosu 配一个 <code>tool.mcp</code> 桥接插件 → 工具出现在 LLM 工具表。
- **注意**：MCP 协议本身不提供权限/同意机制，权限仍由 Host 做。

---

## 7. 怎么让插件开发“方便快捷”

技术选型只是一半，另一半是 DX：

| 工具 | 作用 |
|---|---|
| <code>make plugin-new kind=tool name=foo</code> | 生成目录 + manifest + schema + manual + test |
| <code>sdk/testkit</code> | FakeHost / FakeLLM / FakeAdapter / FakeClock |
| <code>plugin lint</code> | manifest/schema/manual/semver/permissions 校验 |
| <code>plugin doctor</code> | 检查 capability、配置、health、冲突 |
| <code>--dry-run</code> | 完整走流程，不真发 |
| Trace inspector | 看 decision/context/tools/action/latency |
| 示例插件 | time（纯函数）→ meme（文件/媒体）→ memory.recent（状态）→ onebot（生命周期） |
| 10 分钟教程 | 从 <code>plugin-new</code> 到跑通一个工具 |

**插件作者最需要的不是“强大”，是“改一行就能看到结果”。**

---

## 8. 推荐路线

| 阶段 | 做什么 | 不做什么 |
|---|---|---|
| 现在 | Go SDK + 模板 + testkit + 2~3 个示例 | WASM、RPC、市场 |
| 下一步 | 规则层（Starlark）如果真有用户需求 | 把 Starlark 做成完整插件 API |
| 以后 | JSON-RPC stdio + Python SDK | 一上来 gRPC/protobuf |
| 再以后 | MCP tool bridge | 把 MCP 当事件/ adapter 协议 |
| 最后 | WASM 纯函数钩子（wazero） | 把 WASM 当主插件模型 |

---

## 9. 决策清单

- 需要运行**不可信代码**吗？否 → 同进程 Go。
- 需要**非 Go 作者**吗？是 → 进程外 JSON-RPC/MCP。
- 需要**用户改规则**吗？是 → Starlark/Lua。
- 需要**强沙箱**吗？是 → WASM，但只做纯函数。
- 需要**运行时安装**吗？否 → 编译期 bundle。
- 需要**多语言完整插件**吗？是 → 进程外。

---

> **一句话**：  
> 方便快捷 ≠ 技术最潮；方便快捷 = 同语言、热调试、fake host、模板、lint、trace。  
> 先用 Go 把插件接口和标准包做稳，WASM 等有“不可信插件”的真实需求再说。
