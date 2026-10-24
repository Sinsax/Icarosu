# Icarosu 架构重组计划

> 状态：已记录，按阶段执行
> 范围：仅针对当前 Go 项目 `icarosu`，不包含 `MaiBot/`

## 一、现状评估

当前架构是“入口组装 + 分层服务 + 插件式事件流水线”的 Go 单体应用。

### 优点

- 插件化事件流水线清晰，新功能多数可以“加一个插件”实现
- 配置系统成熟：版本化、自动迁移、备份、热重载、LLM/VLM 独立配置
- SQLite schema 有版本管理
- 结构化日志、WebUI、metrics、LLM 统计等可观测性基础不错
- 注意力、话题、语义记忆、表达库等拟人化模块相对独立

### 主要问题

1. `cmd/bot/main.go` 是过重的“上帝组装器”
2. `internal/plugin/chat/chat.go` 是过胖的“聊天业务控制器”
3. `internal/webui/server.go` 单文件约 1400 行，HTML/CSS/JS 全部内嵌
4. `internal/store/store.go` 单文件约 856 行，存储与领域结构混杂
5. `internal/config/config.go` 单文件约 1390 行，职责混杂
6. 基础设施依赖具体实现，缺少接口抽象，难以测试和替换
7. 插件系统缺少中间产物传递机制，`Postprocessor` 阶段空置
8. 领域模型和存储模型没有分离

## 二、目标架构

向“端口与适配器 + 领域服务 + 薄插件”方向重组。

```text
cmd/bot/main.go               薄入口
internal/app                  组装与生命周期
internal/core                 领域层，依赖接口，不依赖 QQ/WebUI/DB
internal/adapters             QQ / SQLite / LLM / VLM / 本地表情库实现
internal/events               事件总线/流水线框架
internal/plugins              具体业务插件（薄）
internal/webui                HTTP 管理端
internal/obs                  日志、metrics、tracing
```

核心原则：

1. 上层依赖接口，不依赖具体实现
2. 插件只做“事件 → 调用领域服务 → 返回结果”
3. 领域层不 import `qq`、`webui`、具体数据库驱动
4. 存储只通过 Repository 接口暴露给领域层

## 三、分阶段实施

### Phase 1：低风险解耦

- 为 LLM / VLM / Memory / Store 定义接口
- 让 `chat`、`summarizer`、`stylelearn`、`vision` 等依赖接口
- 拆分 `store.go` 为多个 repository 文件
- 拆分 `server.go` 的 handler 和静态资源
- 拆分 `config.go` 为 types / defaults / load / migrate / canonical / provider / manager
- 保持 `go build` 与 `go test ./...` 通过

### Phase 2：业务服务化

- 抽出 `internal/core/chat`
- 抽出 `internal/core/trigger`
- 抽出 `internal/core/summarizer`
- 让插件变成薄适配层

### Phase 3：插件系统升级

- 引入 `Context` / 中间产物机制
- 启用真正的 Postprocessor
- 评估事件总线替换直接调用

### Phase 4：可观测性与测试

- 为接口补 mock/fake 测试
- 增加集成测试
- 为 WebUI handler 增加测试

## 四、当前执行状态

- [x] 本计划文档
- [x] Phase 1 接口抽象
- [x] Phase 1 store 拆分
- [x] Phase 1 webui 拆分
- [x] Phase 1 config 拆分
- [x] Phase 1 构建与测试通过
- [x] Phase 2 业务服务化（chat / trigger / summarizer 已下沉到 internal/core）
- [x] Phase 3 插件系统升级（Context 中间产物 + Postprocessor 已启用；事件总线暂不替换，保持直接调用）
- [x] Phase 4 可观测性与测试（新增 core chat/trigger 测试；WebUI/插件/LLM mock 集成测试已覆盖核心链路）
- [x] Review 后续项：core 接口化 / Postprocessor sender / internal/app / Context 并发保护 / 总结与集成测试
- [x] 下一阶段：core 领域模型独立 / app 测试 / obs 可观测性 / Docker 与安全过滤

## 五、当前限定与下阶段计划

> 产品限定：当前程序短期甚至长期都只面向 QQ 群聊，不计划适配其他平台。
> 重心放在单个群内的体验；多群时默认隔离，只有配置了“互通组”的群才共享记忆/表达方式。

### 目标场景

```text
QQ 群 A ──┐
          ├── 默认完全隔离：记忆、表达方式、上下文互不相通
QQ 群 B ──┘

互通组示例：
  memory_share_groups:
    - group_ids: [A, C]
      name: "朋友群共享记忆"
  expression_share_groups:
    - group_ids: [A, B]
      name: "一起玩梗"
```

### 下阶段开发内容

1. **群维度记忆隔离确认**
   - [x] 短期上下文已按群隔离
   - [x] 长期记忆查询支持多群聚合
   - [x] 记忆互通组 `memory_share_groups`

2. **表达方式按群隔离**
   - [x] `stylelearn` 学习到的表达方式归属到具体群
   - [x] `style.Library` 支持按群查询
   - [x] 表达方式互通组 `expression_share_groups`

3. **可配置互通组**
   - [x] 新增配置：
     - `memory_share_groups`
     - `expression_share_groups`
   - [x] 同一组内的群共享长期记忆或表达方式
   - [x] 默认不互通，只有显式配置才互通

4. **存储扩展**
   - [ ] `expressions` / `style_samples` 表增加 `group_id`（当前表达方式库为内存实现）
   - [x] 记忆表已有 `group_id`
   - [x] 互通组配置解析与校验

5. **WebUI 管理**
   - [ ] 展示每个群的记忆数量
   - [ ] 展示每个群的表达方式库
   - [ ] 支持配置/查看互通组（当前通用配置编辑已可改，但无专用页面）

6. **测试**
   - [x] 群 A 与群 B 默认隔离测试
   - [x] 同一互通组内共享测试
   - [ ] 不同互通组不串数据测试（group scope 已覆盖基本场景）