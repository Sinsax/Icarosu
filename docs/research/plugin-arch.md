# Icarosu 插件化 / 微内核化 · 外部对标研究

> 范围：Koishi、NoneBot2、AstrBot、LangBot、MaiBot、Rasa，以及 MCP、Go 插件装载机制。
> 标注约定：**✅ = 有来源支撑**（末尾 [S#] 对应原始 URL）；**🔶 = 本文推断/工程判断**（未在来源中直接陈述）。
> 代码观察均基于本仓库（`internal/plugin/*`、`internal/app/app.go`、`internal/tools/registry.go`），不修改任何源码。

## 1. 结论速览

1. 成熟框架的共同点不是"插件接口"，而是**元数据驱动**：清单（manifest/metadata）+ 配置 schema + 使用手册三件套，同时喂给加载器、WebUI、帮助系统和市场。✅ [S3][S6][S8][S13][S17]
2. 依赖表达有两种成熟范式：Koishi 的**服务注入（inject 声明式依赖 + 变化即回滚重载）**，和 MaiBot/LangBot 的**进程外能力代理（capability proxy）**。前者适合同进程第一方，后者适合第三方。✅ [S2][S15][S16]
3. Go 没有可用的"运行时卸载本进程代码"能力，`plugin` 标准库被官方文档明确列为多限制、不推荐用于可移植产品。→ **热插拔只能靠"进程外"或"WASM"**。✅ [S22]
4. 对 Icarosu：**第一方编译期内嵌（带 manifest 数据 + 能力注入）+ 第三方进程外（MCP 工具 / 窄 JSON-RPC 事件插件）**，是三到六个月内唯一低风险路线。🔶

## 2. 主对比表（框架维度）

| 框架 / 语言 | 扩展单元 | 注册方式 | 进程模型 | 配置归属 | 平台抽象 | 工具暴露 | 热插拔 |
|---|---|---|---|---|---|---|---|
| **Koishi** (TS/JS) | plugin（函数/类/对象，可嵌套）+ service | `ctx.plugin()`；`koishi.yml` 的 `plugins:` 键即插件路径；`inject` 声明服务依赖 | 单进程 Node，插件同进程 | 每插件在 `koishi.yml` 独立键；Schema(schemastery) 校验并自动生成控制台表单 | adapter 插件 → 统一 session 与跨平台消息元素 | 无 LLM tool 抽象（命令/中间件为中心） | ✅ `fork.dispose()` 撤销副作用；服务变更自动回滚重载 [S1][S2][S4] |
| **NoneBot2** (Python) | plugin + matcher（事件响应器） | 插件模块被加载；`on_command()` 等辅助函数创建 matcher，携带 priority/block/rule | 单进程 asyncio；driver 决定传输（forward/reverse） | `PluginMetadata.config` 配置类；全局配置合并 | 两层：driver（传输）+ adapter（协议），插件声明 `supported_adapters` | 无内建 LLM tool 抽象（Matcher 派发） | 🔶 以加载期为主；`--reload`/uvicorn reload 属开发期进程重载 [S5][S6] |
| **AstrBot** (Python) | Star 插件类（`main.py`）+ 装饰器 Handler | 目录扫描 `data/plugins`；`@filter.command` 等装饰器注册；运行时注入 | 单进程 asyncio | 每插件 `_conf_schema.json`（含 secret/options/slider）+ WebUI 表单 | 多平台适配器（aiocqhttp/telegram/…），`support_platforms` 声明 | ✅ 插件可"为 AI 提供新的工具"；具体注册 API 本次未抓取 | ✅ WebUI 单插件"重载"，失败插件可单独重试 [S8][S9][S10][S11] |
| **LangBot** (Python) | plugin（`manifest.yaml`）+ 组件（Command/EventListener/Tool/KnowledgeEngine/Parser/Page） | `data/plugins` 目录；组件由 `lbp comp` 生成独立 YAML | ✅ **独立 Plugin Runtime 进程**（WS 控制口 5400，生产可 stdio）+ SDK | `spec.config` 表单 schema（含 `show_if` 条件、`llm-model-selector` 等控件） | 平台适配器 + 对话流水线（Pipeline） | ✅ Tool 组件：`parameters` + `llm_prompt`，供内置 Agent 调用 [S12][S13][S14] |
| **MaiBot** (Python) | plugin（`_manifest.json` v2）+ 组件（Tool/Command/Hook/EventHandler/MessageGateway/LLMProvider/ReplyExtension/WebUI 页） | `_manifest.json` 声明 + Host/Runner IPC；`capabilities` 逐项授权 | ✅ **Runner 子进程**，msgpack RPC，UDS/TCP/named pipe，一 Supervisor 一 Runner | `config_model` + `config.toml`（每安装实例，Runner 维护） | `plugin_type: adapter`；消息网关组件 | ✅ Tool 组件 + LLMProvider；能力未声明即拒调用 | ✅ Supervisor 负责健康检查与插件热重载 [S15][S16][S17] |
| **Rasa** (Python) | channel connector + custom action | `credentials.yml`/`endpoints.yml` 配置；action 由独立 Action Server 承载 | agent 进程 + 独立 action server | 配置文件（domain/endpoints/credentials） | ✅ 自定义 connector（input/output channel 分离开放） | 无 LLM tool 抽象（custom action 调外部） | 🔶 非热插拔，改配置/模型需重启或重训 [S18] |

> 读表要点：**"平台抽象"与"工具暴露"是两条独立轴**。Koishi/NoneBot 强在平台抽象但无 tool 概念；LangBot/MaiBot 强在 tool 但平台抽象仍是内置适配器。Icarosu 需要同时具备两者。🔶

## 3. 补充表：Go 侧装载机制

| 机制 | 隔离性 | 卸载/热更 | 跨语言 | 版本约束 | 适用判断 |
|---|---|---|---|---|---|
| 编译期内嵌（interface + 注册表） | 无（同进程同地址空间） | 仅配置/开关级 | 否 | 编译期绑定 | ✅ 第一方主力 |
| 标准库 `plugin` | 同进程 | 只能加载一次，**无法关闭** | 否 | 必须完全相同的 toolchain/build tags/依赖源码 | ❌ 官方文档列举多重缺陷，含 race detector 支持差、崩溃风险、"很多用户因此选择 IPC" [S22] |
| hashicorp/go-plugin | 子进程 + RPC | 可重启插件进程 | ✅ gRPC（任意语言） | 有 protocol version，可失效旧插件 | 🔶 适合可信第三方；但只保证本地可靠网络，依赖较重 [S23] |
| WASM（wazero / Extism） | 强沙箱（独立内存，不可破坏宿主） | ✅ 可实例化/销毁 | ✅（PDK 多语言） | 自定义导入导出 ABI | 🔶 适合纯函数型钩子（过滤/打分/改写），不适合要 DB/长连接的插件 [S24][S25] |
| MCP（stdio / Streamable HTTP） | 子进程或远程 | 由宿主管理生命周期 | ✅ JSON-RPC + schema | 协议版本 + `capabilities` 协商 | 🔶 适合"LLM 工具/资源"，不覆盖聊天事件 [S19] |

## 4. 各家机制要点（可直接借用的部分）

### 4.1 Koishi：Context 即容器，服务即依赖
- 插件是"接收 ctx 与 config 的函数/类/对象"，**可嵌套**，嵌套带来独立热重载粒度。✅ [S1]
- 服务是 IoC 容器：`ctx.database`/`ctx.console` 等由插件提供；`inject` 支持 `required`/`optional`，**服务为真值前插件体不加载；服务变化则插件回滚后重载**。✅ [S2]
- 局部功能依赖用 `ctx.inject([...], cb)` 注册为子插件，避免整插件被拖死；卸载插件时其提供的服务一并移除。✅ [S2]
- 元属性：`name`、`Config`(Schema)、`usage`、`reusable`、`using`、`filter`；Schema 同时负责校验、默认值、控制台表单。✅ [S1][S3]
- 生命周期：`ready`（已在运行态加载则立即触发）、`dispose`（副作用清理）、Service 的 `start/stop/fork`。✅ [S4]

### 4.2 NoneBot2：三层拆分（driver / adapter / matcher）
- **driver 只管收发**（forward 客户端型 / reverse 服务端型），adapter 管协议语义，matcher 管"这条事件要不要响应"。✅ [S5]
- matcher 的 `priority` + `block` 构成"责任链短路"，与 Icarosu 现有 Stage/Priority 语义同源。✅ [S5][S7]
- `PluginMetadata`（name/description/usage/type/homepage/config/supported_adapters/extra）是**帮助系统与插件市场的唯一数据源**；`extra` 是社区约定的扩展位。✅ [S6]
- 插件可声明 `supported_adapters`，加载器据此做兼容性提示——这是"能力声明"的最轻量形态。✅ [S6]

### 4.3 AstrBot：装饰器注册 + 每插件配置 schema
- `Star` 基类 + `Context` 门面；Handler 必须在插件类内、前两参为 `(self, event)`；文件名必须 `main.py`。✅ [S9]
- `metadata.yaml` 覆盖展示名、`support_platforms`、`astrbot_version`（PEP 440 区间，不满足则阻止加载）；`requirements.txt` 管依赖；数据必须写 `data/` 而非插件目录。✅ [S8]
- `_conf_schema.json` 支持 `type/description/hint/default/options/labels/slider/secret/invisible`，直接渲染成管理面板；文档明确 **secret 只遮 UI、不加密存储**。✅ [S10]
- 插件目录可带 `skills/`（SKILL.md），把"能力包"和"代码插件"解耦。✅ [S8]

### 4.4 LangBot：清单 + 组件 + 独立运行时进程
- `manifest.yaml`：`apiVersion/kind/metadata(author,name,version,description,label,icon)/spec(config,components)/execution.python(path,attr)`；组件列表**自动回填**。✅ [S13]
- 配置项类型丰富（`select`、`prompt-editor`、`llm-model-selector`、`bot-selector`、`tools-selector`）+ `show_if` 条件渲染——证明"配置 schema 即 UI DSL"。✅ [S13]
- 每个 Tool 一个 YAML：`metadata.name/label/description` + `spec.parameters` + `spec.llm_prompt` + `execution`。✅ [S14]
- **插件跑在独立 Runtime 进程**，主程序经 `plugin.runtime_ws_url`（`ws://localhost:5400/control/ws`）连接，生产可用 stdio。✅ [S15]

### 4.5 MaiBot：Host/Runner IPC + 逐项能力授权
- 插件在**独立子进程**，msgpack 编码 RPC；传输支持 UDS/TCP/Named Pipe；Host→Runner 用 `invoke_plugin()`，Runner→Host 用 `ctx.send.text()`、`ctx.db.query()` 这类**能力代理**。✅ [S16]
- `_manifest.json` v2：`id/version/author/license/urls/host_application{min,max}/sdk{min,max}/dependencies/plugin_type/capabilities/i18n`；semver 用**闭区间**，并要求上限写"实测过的次版本 + .99"，禁止 `1.99.99` 这种伪上限。✅ [S17]
- `capabilities` 是**逐项白名单**："声明了哪项才能调用哪项，没声明的调用会被拒绝并报未获授权能力"。✅ [S17]
- `plugin_type` 枚举（adapter/tool/provider/management/data/media/game/integration/extension）用于 WebUI 分类与默认图标——低成本高收益。✅ [S17]

### 4.6 Rasa / MCP：分层与协议边界
- Rasa 把"通道连接器"与"对话/动作"分开，自定义 connector 只需实现输入/输出两侧；行动逻辑可外置为独立 HTTP 服务。✅ [S18]
- MCP 是 **host → client → server** 的 JSON-RPC 协议，只标准化"上下文交换"，**不规定 AI 应用如何用模型或管理上下文**。✅ [S19]
- 三类原语的控制主体不同：**tools 模型控制**、**resources 应用控制**、**prompts 用户控制**；server 必须声明 `capabilities`（如 `tools.listChanged`）。✅ [S20][S21]
- 安全模型是"SHOULD 有人在环"（human in the loop），**协议本身不提供权限/同意机制**——权限必须由宿主实现。✅ [S20]

## 5. 对本项目可执行的结论（15 条）

1. **用显式注册表替换 `init()` + `RegisterGlobal`**：因为 import 顺序隐式、测试间共享全局单例，且无法按配置裁剪编译单元；改成 `cmd/bot` 里一份 `[]plugin.Spec` 显式清单，注册顺序与依赖可见。✅（现状见 `internal/plugin/builtin/ping.go`、`registry.go`）
2. **内核只保留 7 个窄能力**（Send / FetchHistory / Memory / LLM / ToolHost / Config / Clock），**废掉 `plugin.Bot` 这个 4 方法的"上帝接口"**：因为每加一种平台能力（撤回、合并转发、群管理）都要改所有插件与 dryrun 包装。🔶
3. **用"事件主题"取代 pre/responder/post 三段式**：`message.received`、`memory.recalled`、`reply.pre_send`、`tool.invoked`、`schedule.tick`；因为三段式无法表达"记忆写入""打断/取消""异步后处理"，新增阶段必须改内核。🔶（三段枚举见 `internal/plugin/plugin.go`）
4. **Manifest 单一事实源**：第一方用 Go struct 常量，第三方用 YAML，同一套字段派生 WebUI 表单、`/help` 文案、权限校验、市场卡片；因为 Koishi/LangBot/AstrBot/MaiBot 全都证明"元数据即 UI"。✅ [S1][S6][S8][S13][S17]
5. **配置按插件拆分到 `configs/plugins/<id>.yaml` 并各自 `config_version`**：因为单体 `Config`（`CurrentVersion = 12`，`internal/config/migrate.go`）会把所有插件的迁移绑在一个发布节奏上，任何插件加字段都要动核心版本号。🔶（现状见 `internal/config/types.go`）
6. **工具注册必须命名空间化 + JSON Schema 校验 + 超时/并发/结果截断**：因为工具表当前是共享可写（app 创建后注入）、重名直接 `panic`，而 LLM 可以按名字调用任意已注册工具；改成 `pluginid.toolname` 并在内核侧统一限流。✅（`internal/tools/registry.go`）
7. **权限用"能力 + 作用域"白名单，未声明即拒绝**（MaiBot 式 `capabilities`）：因为插件生态里最贵的故障是越权发消息、越权读记忆、越权发起 HTTP；布尔开关无法表达"只能发到本群"。✅ [S17]
8. **第一方插件不做代码热重载，只做"配置/启停"热更新；要热更就走进程外**：因为 Go 无法安全卸载已加载代码，标准库 `plugin` 也不能 `Close`。✅ [S22]
9. **第三方入口先做 MCP 客户端（工具），再做自有 stdio JSON-RPC（事件/触发器）**：因为 MCP 只覆盖"LLM 工具/资源/提示词"，不覆盖聊天事件流、插件配置、UI 扩展。✅ [S19][S20]
10. **不要把平台适配器做成 MCP server**：MCP 的 server 生命周期由 host 拉起、面向"给模型提供上下文"，而 QQ 适配器是长连接事件源，语义错配会导致重连/多实例/背压都无处安放。🔶
11. **`hashicorp/go-plugin` 只在"需要 Go 接口级 RPC + 多语言"时选用，不作为唯一协议**：因为它只保证本地可靠网络、协议版本只有单调整数，且引入 gRPC/protobuf 依赖；自建窄 JSON-RPC 更贴合"每事件一次调用"的形态。✅ [S23] + 🔶
12. **管道内每次插件调用加 `recover` + `ctx` 超时 + 并发上限**：因为 `Manager.Dispatch` 目前串行调用且无 `recover`（生产代码中仅 `internal/qq/client.go` 有 recover），一个插件 panic 会带走整个进程。✅（代码观察）
13. **可观测性以 `plugin_id` 为一等标签**（日志字段、调用计数、错误率、P95、工具审计含参数哈希）：因为没有它，"回复变慢/变怪"无法归因到具体插件。🔶
14. **记忆/人格这类热路径，"内核持数据 + 插件供策略"**：内核管存储与生命周期，插件只实现 `Retriever/Ranker/PersonaRenderer`；因为让插件各自持 DB 会造成数据割裂、备份与迁移失控。🔶
15. **插件化同时交付三件测试基建**：manifest lint（semver/权限子集/必需字段）、契约测试（fake kernel + 每能力一组用例）、配置迁移 golden test；因为回归面从 1 个入口变成 N 个独立单元。🔶

## 6. Plugin Manifest 草案（YAML）

```yaml
apiVersion: icarosu/v1          # 协议版本；破坏性变更升 v2，宿主并行支持 N/N-1
id: icarosu.memory.vector       # 全局唯一，点分命名；工具名派生为 memory_vector.recall
version: 1.2.0                  # 严格 semver
kind: memory                    # adapter|memory|persona|trigger|learning|tool|webui|integration|core
display:
  name: {zh-CN: 向量记忆, en: Vector Memory}
  description: {zh-CN: 语义召回长期记忆, en: Semantic long-term recall}
  icon: assets/icon.svg
  tags: [memory, retrieval]

runtime: inproc                 # inproc | process | wasm
entry:
  inproc:  {package: icarosu/internal/plugins/vectormem, constructor: New}
  process: {command: ["./bin/vectormem"], protocol: jsonrpc-stdio, protocol_version: 1}
  wasm:    {module: vectormem.wasm, abi: icarosu-abi/1}

compat:
  host: {min: 0.9.0, max: 0.9.99}   # 闭区间，上限写"实测次版本+.99"
  sdk:  {min: 1.0.0, max: 1.99.99}
  go:   ">=1.24"                    # 仅 inproc 有意义

provides:                       # 我向容器注册什么（服务名 + 契约版本）
  - {service: memory.recall, contract: memory.recall/v1, version: 1.2.0}
  - {service: tool.recall_memory, contract: tool/v1}
requires:                       # 我依赖什么；optional 缺失时须降级而非崩溃
  plugins:
    - {id: icarosu.store.sqlite, version: ">=1.0.0 <2.0.0"}
  services:
    - {name: llm.embed, contract: llm.embed/v1, optional: false}
    - {name: memory.write, contract: memory.write/v1, optional: true}

config_version: 3
config_schema:                  # JSON Schema 子集 + UI 提示；内核据此校验并渲染表单
  type: object
  properties:
    top_k:       {type: integer, default: 8, minimum: 1, maximum: 50, title: 召回条数, ui: slider}
    embed_model: {type: string, default: local-256, enum: [local-256, bge-m3]}
    api_key:     {type: string, secret: true}          # secret 仅遮 UI，落盘须加密
  required: [top_k]
config_migrations:
  - {from: 1, to: 2, fn: migrate_v1_v2}
  - {from: 2, to: 3, fn: migrate_v2_v3}

permissions:                    # 能力白名单；未声明即拒绝（运行时返回 permission_denied）
  - message.read
  - memory.read
  - memory.write
  - llm.embed
  - {name: http.request, scopes: ["https://api.example.com/*"], methods: [POST]}
  - {name: storage.kv, scopes: ["vectormem/*"]}
  - {name: message.send, scopes: ["group:self"]}       # 只允许回到触发它的会话
limits: {timeout_ms: 800, concurrency: 4, max_result_bytes: 16384}

manual:                         # 使用手册：/help、WebUI、市场详情共用
  summary: 语义召回群聊长期记忆
  usage: |
    /memory recall <关键词>
    /memory forget <id>
  examples: ["/memory recall 上次说的灰度方案"]
  config_notes: top_k 越大越慢；embed_model 切换需重建索引
  failure_modes: [embedding 服务不可用时降级为关键词检索]
  min_role: member              # 谁能触发

events:                         # 订阅/发布；带版本与阻塞语义
  subscribe:
    - {topic: message.received, version: 1, priority: 50, blocking: false, timeout_ms: 300}
    - {topic: reply.pre_send,   version: 1, priority: 10, blocking: true}
  publish:
    - {topic: memory.recalled, version: 1}

tools:                          # 暴露给 LLM 的工具；与 events 解耦
  - name: recall_memory
    description: 按语义检索本群长期记忆
    parameters_schema: schemas/recall_memory.json
    permission: memory.read
    side_effect: read             # read | write | external
    timeout_ms: 500
    expose: [llm]                 # llm | mcp | both
    audit: {log_args: hash, log_result: false}
```

## 7. 推荐落地方案

**结论：第一方编译期内嵌（首选）+ 第三方进程外（MCP 工具 / 自有 stdio JSON-RPC 事件插件），不采用 Go `plugin` 包，也不把 WASM 作为一期目标。**

理由：
- Go 的运行时不可卸载代码，且 `plugin` 包要求"完全相同的工具链/构建标签/依赖源码"，官方文档直接建议"很多用户改用 IPC"。✅ [S22]
- 进程外方案在 MaiBot（Runner + msgpack RPC + 能力代理）与 LangBot（Plugin Runtime + WS/stdio）已被验证可承载完整插件生态。✅ [S15][S16]
- MCP 现成生态大，但只覆盖"给模型的工具/资源"，不能承载 QQ 事件与插件配置，所以只能做**工具面**的第一阶段。✅ [S19][S20]

分期（每期都有可验收产物）：

| 阶段 | 内容 | 验收标准 |
|---|---|---|
| P0（1–2 周） | 冻结内核接口：`kernel.Caps`（7 能力）、`plugin.Spec`（manifest 结构体）、事件主题常量；`Dispatch` 加 recover/超时/并发闸 | 现有 14 个插件全部迁移到 `Spec` 注册；`go test ./...` 通过；panic 插件不再拖垮进程 |
| P1（2–3 周） | 配置拆分：`configs/core.yaml` + `configs/plugins/<id>.yaml`；每插件 `config_version` + 迁移函数；WebUI 由 `config_schema` 渲染 | 新增插件零改动核心 `Config.Version`；旧配置自动迁移且 golden test 覆盖 |
| P2（2–4 周） | 工具面：命名空间、JSON Schema 校验、权限闸、审计；接入 MCP client（stdio server 列表来自配置） | 至少 2 个外部 MCP server 可用；越权调用返回 `permission_denied` |
| P3（按需） | 事件面进程外：`icarusud` 宿主 + stdio JSON-RPC（握手 → 订阅 → 事件 → 能力回调），支持重启/健康检查 | 第三方插件崩溃仅影响自身；热重启 < 2s；事件不丢（有背压） |
| P4（可选） | WASM（wazero）承载纯函数钩子：过滤、打分、文本改写 | 单次调用 < 5ms；无网络/文件能力；沙箱逃逸测试通过 |

迁移顺序建议：先 P0 的"事件主题 + recover/超时"（改动小、收益立竿见影），再 P1 配置拆分，最后才是进程外。**不要先做运行时装载**——在没有 manifest/权限/配置归属之前，运行时装载只会把现有耦合放大。🔶

## 8. 要避免的坑（按代价排序）

1. **`init()` 自注册 + 硬编码组装**：顺序隐式、测试串扰、无法裁剪；且重名注册当前直接 panic。✅（代码）
2. **上帝接口**：`plugin.Bot` 与平台类型（`qq.Event` 别名）泄漏到插件层；平台中立性一旦破坏，第二个平台接入就是重写。✅（`internal/platform/platform.go` 仍 `type Event = qq.Event`）
3. **同步阻塞的流水线**：chat 插件在 Dispatch 内直接调 LLM，慢请求会卡住同群后续消息；需要"决策与生成分离 + 每会话串行、跨会话并行"。🔶
4. **无 panic 隔离/无超时**：一个插件 bug 等于整机下线。✅（代码）
5. **配置 schema 的 `secret` 只遮 UI**：AstrBot 文档已明确这一点；Icarosu 必须真加密或改为环境变量注入。✅ [S10]
6. **版本上限造假**：MaiBot 明确禁止 `1.99.99` 这类"没有真实版本"的上限；否则等于不设上限。✅ [S17]
7. **把 MCP 当万能插件协议**：MCP 不提供配置 UI、权限同意、聊天事件、UI 扩展；且 prompts 是"用户控制"、resources 是"应用控制"，都不能替代事件钩子。✅ [S20][S21]
8. **热重载丢状态**：stylelib/attention/session 都在内存，重载即失忆；必须定义 `on_load/on_unload` 与状态外置约定。🔶（`internal/style`、`internal/attention`）

## 9. 来源

- [S1] Koishi — About Plugin（插件形态/嵌套/配置路径解析）: https://koishi.chat/en-US/guide/plugin/
- [S2] Koishi — Service & Dependency（inject/服务生命周期）: https://koishi.chat/en-US/guide/plugin/service.html
- [S3] Koishi — Schema（schemastery 校验与表单）: https://koishi.chat/en-US/guide/plugin/schema.html
- [S4] Koishi — Lifecycle（ready/dispose）: https://koishi.chat/en-US/guide/plugin/lifecycle.html
- [S5] NoneBot2 — 选择驱动器（driver 分层/forward-reverse）: https://nonebot.dev/docs/advanced/driver
- [S6] NoneBot2 — 插件信息（PluginMetadata/supported_adapters）: https://nonebot.dev/docs/advanced/plugin-info
- [S7] NoneBot2 — 事件响应器（matcher/priority/block）: https://nonebot.dev/docs/tutorial/matcher
- [S8] AstrBot — 插件开发指南（metadata.yaml/版本区间/依赖/热重载/skills）: https://docs.astrbot.app/dev/star/plugin-new.html
- [S9] AstrBot — 最小实例（Star/Context/装饰器）: https://docs.astrbot.app/dev/star/guides/simple.html
- [S10] AstrBot — 插件配置（_conf_schema.json/secret 只遮 UI）: https://docs.astrbot.app/dev/star/guides/plugin-config.html
- [S11] AstrBot — 插件使用页（"为 AI 提供新的工具"）: https://docs.astrbot.app/use/plugin.html
- [S12] LangBot — 插件目录结构: https://docs.langbot.app/en/plugin/dev/directory-structure.md
- [S13] LangBot — manifest 与 spec.config: https://docs.langbot.app/en/plugin/dev/basic-info.md
- [S14] LangBot — Tool 组件（parameters/llm_prompt）: https://docs.langbot.app/en/plugin/dev/components/tool.md
- [S15] LangBot — Plugin Runtime（独立进程/WS 5400/stdio）: https://docs.langbot.app/en/develop/plugin-runtime.md
- [S16] MaiBot — 插件开发指南（Host/Runner IPC/能力代理）: https://docs.mai-mai.org/plugin/
- [S17] MaiBot — Manifest 系统（capabilities/版本闭区间/plugin_type）: https://docs.mai-mai.org/plugin/manifest
- [S18] Rasa — Custom Connectors: https://rasa.com/docs/rasa/connectors/custom-connectors/
- [S19] MCP — Architecture overview（host/client/server，只管上下文交换）: https://modelcontextprotocol.io/docs/learn/architecture
- [S20] MCP — Tools（模型控制/inputSchema/outputSchema）: https://modelcontextprotocol.io/specification/2026-07-28/server/tools
- [S21] MCP — Resources（应用控制）与 Prompts（用户控制）: https://modelcontextprotocol.io/specification/2026-07-28/server/resources · https://modelcontextprotocol.io/specification/2026-07-28/server/prompts
- [S22] Go — package plugin 官方警告: https://pkg.go.dev/plugin
- [S23] hashicorp/go-plugin（RPC 子进程/跨语言/协议版本）: https://github.com/hashicorp/go-plugin
- [S24] wazero（纯 Go、零依赖、沙箱）: https://wazero.io/ · https://wazero.io/docs/
- [S25] Extism（WASM 插件 + 宿主函数/PDK）: https://extism.org/docs/concepts/plug-in

> 注：本文件中标注 🔶 的条目为基于上述来源与 Icarosu 代码现状的工程推断，未在任何来源中被直接陈述；实施前应结合压测与兼容性验证。
