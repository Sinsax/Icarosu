package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"icarosu/internal/sdk"
)

// StorageFactory 为插件创建命名空间存储。
type StorageFactory func(namespace string) (sdk.Storage, error)

// ConfigLoader 读取某个插件的配置段。
type ConfigLoader func(section string, dst any) error

// Options 是内核的可选依赖。第一方阶段可以只注入用到的能力。
type Options struct {
	Logger           *slog.Logger
	Clock            sdk.Clock
	Bus              sdk.Bus
	LLM              sdk.LLM
	Memory           sdk.Memory
	Vision           sdk.Vision
	Memes            sdk.MemeStore
	Storage          StorageFactory
	Config           ConfigLoader
	Timeout          time.Duration
	SchedulerWorkers int
	SchedulerBuffer  int
	TraceLimit       int
	DedupTTL         time.Duration
	DedupMax         int
}

// PluginInfo 是插件状态快照。
type PluginInfo struct {
	ID              string
	Version         string
	Kind            sdk.Kind
	Critical        bool
	Health          sdk.Health
	MissingOptional []string
}

// Kernel 是插件注册、生命周期与 capability 解析的微内核骨架。
type Kernel struct {
	opts       Options
	reg        *registry
	tools      *toolRegistry
	actions    *actionRouter
	engine     *TurnEngine
	observers  []sdk.Observer
	scheduler  *Scheduler
	adapters   []sdk.Adapter
	engineMu   sync.RWMutex
	turnMu     sync.RWMutex
	schedMu    sync.RWMutex
	controlMu  sync.Mutex
	stateMu    sync.RWMutex
	healthMu   sync.Mutex
	dedupMu    sync.Mutex
	seen       map[sdk.EventID]time.Time
	dedupTTL   time.Duration
	dedupMax   int
	disabled   map[string]bool
	inited     bool
	traceMu    sync.Mutex
	traces     []sdk.Trace
	traceLimit int
	health     map[string]sdk.Health
	metrics    *Metrics
	startedIDs []string
	started    bool
}

// New 创建内核；零值 Options 会被补上默认 logger/clock/timeout。
func New(opts Options) *Kernel {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Clock == nil {
		opts.Clock = realClock{}
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}
	if opts.TraceLimit <= 0 {
		opts.TraceLimit = 200
	}
	if opts.DedupTTL <= 0 {
		opts.DedupTTL = 10 * time.Minute
	}
	if opts.DedupMax <= 0 {
		opts.DedupMax = 10000
	}
	metrics := &Metrics{}
	return &Kernel{
		opts:       opts,
		reg:        newRegistry(),
		tools:      newToolRegistry(opts.Logger, metrics),
		actions:    newActionRouter(opts.Logger, metrics),
		traceLimit: opts.TraceLimit,
		health:     make(map[string]sdk.Health),
		metrics:    metrics,
		disabled:   make(map[string]bool),
		seen:       make(map[sdk.EventID]time.Time),
		dedupTTL:   opts.DedupTTL,
		dedupMax:   opts.DedupMax,
	}
}

// Register 注册一个显式插件。
func (k *Kernel) Register(specs ...sdk.PluginSpec) error {
	k.controlMu.Lock()
	defer k.controlMu.Unlock()
	if k.started || k.inited {
		return fmt.Errorf("cannot register plugins after kernel initialized")
	}
	for _, spec := range specs {
		if err := k.reg.register(spec); err != nil {
			return err
		}
	}
	return nil
}

// Init 解析依赖并初始化所有插件。可重复调用，但只有第一次生效。
func (k *Kernel) Init(ctx context.Context) error {
	k.controlMu.Lock()
	defer k.controlMu.Unlock()
	if k.inited {
		return nil
	}
	order, err := k.reg.resolve()
	if err != nil {
		return err
	}
	initialized := make([]string, 0, len(order))
	cleanup := func() {
		for _, id := range initialized {
			k.tools.UnregisterOwner(id)
			if rec := k.reg.records[id]; rec != nil {
				if ap, ok := rec.plugin.(sdk.AdapterProvider); ok {
					if adapter := ap.Adapter(); adapter != nil {
						k.actions.UnregisterAdapter(adapter.ID())
					}
				}
				_ = k.stopOne(context.Background(), rec)
			}
		}
	}
	for _, id := range order {
		rec := k.reg.records[id]
		if err := k.initOne(ctx, rec); err != nil {
			k.setHealth(id, sdk.Health{Status: sdk.HealthFailed, Message: err.Error(), CheckedAt: time.Now()})
			cleanup()
			return fmt.Errorf("init plugin %q: %w", id, err)
		}
		initialized = append(initialized, id)
		if err := k.registerPluginCapabilities(id, rec); err != nil {
			cleanup()
			return err
		}
		k.setHealth(id, sdk.Healthy())
	}
	if err := k.buildEngine(ctx); err != nil {
		cleanup()
		return fmt.Errorf("build turn engine: %w", err)
	}
	k.inited = true
	return nil
}

// buildEngine 从已 Init 的插件中收集回合依赖。
func (k *Kernel) buildEngine(ctx context.Context) error {
	k.stateMu.RLock()
	disabled := make(map[string]bool, len(k.disabled))
	for id, off := range k.disabled {
		disabled[id] = off
	}
	k.stateMu.RUnlock()

	var (
		persona        sdk.Persona
		hasPersona     bool
		decider        sdk.Decider
		renderer       sdk.Renderer
		providers      []sdk.ContextProvider
		hooks          []sdk.TurnHook
		policies       []sdk.Policy
		outputPolicies []sdk.OutputPolicy
		observers      []sdk.Observer
	)
	for _, id := range k.reg.order {
		rec := k.reg.records[id]
		if rec == nil || disabled[id] {
			continue
		}
		if pp, ok := rec.plugin.(sdk.PersonaProvider); ok {
			if hasPersona {
				return fmt.Errorf("multiple persona providers (last %q)", id)
			}
			p, err := pp.Persona(ctx)
			if err != nil {
				return fmt.Errorf("persona %q: %w", id, err)
			}
			persona, hasPersona = p, true
		}
		if d, ok := rec.plugin.(sdk.Decider); ok {
			if decider != nil {
				return fmt.Errorf("multiple deciders (last %q)", id)
			}
			decider = d
		}
		if r, ok := rec.plugin.(sdk.Renderer); ok {
			if renderer != nil {
				return fmt.Errorf("multiple renderers (last %q)", id)
			}
			renderer = r
		}
		if p, ok := rec.plugin.(sdk.ContextProvider); ok {
			providers = append(providers, p)
		}
		if h, ok := rec.plugin.(sdk.TurnHook); ok {
			hooks = append(hooks, h)
		}
		if p, ok := rec.plugin.(sdk.Policy); ok {
			policies = append(policies, p)
		}
		if p, ok := rec.plugin.(sdk.OutputPolicy); ok {
			outputPolicies = append(outputPolicies, p)
		}
		if o, ok := rec.plugin.(sdk.Observer); ok {
			observers = append(observers, o)
		}
	}
	engine := NewTurnEngine(TurnOptions{
		Logger:         k.opts.Logger,
		Clock:          k.opts.Clock,
		LLM:            k.opts.LLM,
		Actions:        k.actions,
		Tools:          k.tools,
		Persona:        persona,
		Providers:      providers,
		Hooks:          hooks,
		Policies:       policies,
		OutputPolicies: outputPolicies,
		Decider:        decider,
		Renderer:       renderer,
		Timeout:        k.opts.Timeout,
	})
	k.engineMu.Lock()
	k.observers = observers
	k.engine = engine
	k.engineMu.Unlock()
	return nil
}

// HandleEvent 先让 Observer 记录/观察，再执行一个回合。
func (k *Kernel) HandleEvent(ctx context.Context, ev sdk.Event) (sdk.TurnResult, error) {
	k.turnMu.RLock()
	defer k.turnMu.RUnlock()

	k.engineMu.RLock()
	engine := k.engine
	observers := append([]sdk.Observer(nil), k.observers...)
	k.engineMu.RUnlock()
	if engine == nil {
		return sdk.TurnResult{}, fmt.Errorf("kernel is not initialized")
	}
	if !k.markSeen(ev.ID) {
		return sdk.TurnResult{Decision: sdk.Decision{Kind: sdk.DecisionSilent, Reason: "duplicate"}}, nil
	}
	for _, o := range observers {
		k.observe(ctx, o, ev)
	}
	k.metrics.Turns.Add(1)
	res, err := engine.Run(ctx, ev)
	if err != nil {
		k.metrics.TurnErrors.Add(1)
	}
	if res.Trace != nil {
		k.addTrace(*res.Trace)
	}
	return res, err
}

// Engine 返回当前回合引擎（可能为 nil）。
func (k *Kernel) Engine() *TurnEngine {
	k.engineMu.RLock()
	defer k.engineMu.RUnlock()
	return k.engine
}

func (k *Kernel) markSeen(id sdk.EventID) bool {
	if id == "" {
		return true
	}
	now := k.opts.Clock.Now()

	k.dedupMu.Lock()
	defer k.dedupMu.Unlock()
	if seen, ok := k.seen[id]; ok && now.Sub(seen) < k.dedupTTL {
		return false
	}
	if len(k.seen) >= k.dedupMax {
		for eventID, seenAt := range k.seen {
			if now.Sub(seenAt) >= k.dedupTTL {
				delete(k.seen, eventID)
			}
		}
	}
	if len(k.seen) >= k.dedupMax {
		var oldestID sdk.EventID
		var oldest time.Time
		for eventID, seenAt := range k.seen {
			if oldest.IsZero() || seenAt.Before(oldest) {
				oldestID, oldest = eventID, seenAt
			}
		}
		delete(k.seen, oldestID)
	}
	k.seen[id] = now
	return true
}

func (k *Kernel) addTrace(t sdk.Trace) {
	k.traceMu.Lock()
	defer k.traceMu.Unlock()
	k.traces = append(k.traces, t)
	if len(k.traces) > k.traceLimit {
		k.traces = k.traces[len(k.traces)-k.traceLimit:]
	}
}

// SetPluginEnabled 在运行时启用/停用非关键插件，并重建回合引擎。
func (k *Kernel) SetPluginEnabled(ctx context.Context, id string, enabled bool) error {
	k.controlMu.Lock()
	defer k.controlMu.Unlock()
	if !k.inited {
		return fmt.Errorf("kernel is not initialized")
	}
	if !k.started {
		return fmt.Errorf("kernel is not started")
	}
	k.turnMu.Lock()
	defer k.turnMu.Unlock()

	rec := k.reg.records[id]
	if rec == nil {
		return fmt.Errorf("unknown plugin %q", id)
	}
	if enabled {
		k.stateMu.RLock()
		alreadyEnabled := !k.disabled[id]
		k.stateMu.RUnlock()
		if alreadyEnabled {
			return nil
		}
		if err := k.initOne(ctx, rec); err != nil {
			return fmt.Errorf("enable plugin %q: %w", id, err)
		}
		if err := k.startOne(ctx, rec); err != nil {
			_ = k.stopOne(ctx, rec)
			return fmt.Errorf("enable plugin %q: %w", id, err)
		}
		if err := k.registerPluginCapabilities(id, rec); err != nil {
			_ = k.stopOne(ctx, rec)
			k.tools.UnregisterOwner(id)
			return err
		}
		k.stateMu.Lock()
		delete(k.disabled, id)
		if k.started {
			k.startedIDs = append(k.startedIDs, id)
		}
		k.stateMu.Unlock()
		k.setHealth(id, sdk.Healthy())
		if err := k.buildEngine(ctx); err != nil {
			_ = k.stopOne(ctx, rec)
			k.tools.UnregisterOwner(id)
			k.stateMu.Lock()
			k.disabled[id] = true
			k.removeStartedID(id)
			k.stateMu.Unlock()
			return err
		}
		return nil
	}

	k.stateMu.RLock()
	alreadyDisabled := k.disabled[id]
	k.stateMu.RUnlock()
	if alreadyDisabled {
		return nil
	}
	if rec.spec.Critical {
		return fmt.Errorf("critical plugin %q cannot be disabled", id)
	}
	if err := k.stopOne(ctx, rec); err != nil {
		return fmt.Errorf("disable plugin %q: %w", id, err)
	}
	k.tools.UnregisterOwner(id)
	if ap, ok := rec.plugin.(sdk.AdapterProvider); ok {
		if adapter := ap.Adapter(); adapter != nil {
			k.actions.UnregisterAdapter(adapter.ID())
		}
	}
	k.stateMu.Lock()
	k.disabled[id] = true
	k.removeStartedID(id)
	k.stateMu.Unlock()
	k.setHealth(id, sdk.Degraded("disabled"))
	return k.buildEngine(ctx)
}

func validateManifestTools(rec *pluginRecord, tools []sdk.Tool) error {
	if len(rec.manifest.Tools) == 0 {
		return nil
	}
	actual := make(map[string]sdk.Tool, len(tools))
	for _, tool := range tools {
		actual[tool.Name] = tool
	}
	for _, declared := range rec.manifest.Tools {
		tool, ok := actual[declared.Name]
		if !ok {
			return fmt.Errorf("plugin %q manifest declares tool %q but runtime did not provide it", rec.manifest.ID, declared.Name)
		}
		for _, permission := range declared.Permissions {
			if !hasPermission(tool.Permissions, permission) {
				return fmt.Errorf("plugin %q tool %q lacks declared permission %q", rec.manifest.ID, declared.Name, permission)
			}
		}
		for _, permission := range tool.Permissions {
			if permission != "" && !hasPermission(declared.Permissions, permission) {
				return fmt.Errorf("plugin %q tool %q has undeclared permission %q", rec.manifest.ID, declared.Name, permission)
			}
		}
	}
	return nil
}

// ReloadPlugin 通过停用再启用让插件重新读取配置。关键插件无法热重载。
func (k *Kernel) ReloadPlugin(ctx context.Context, id string) error {
	if id == "interface.webui" {
		return fmt.Errorf("cannot reload interface.webui from itself")
	}
	if err := k.SetPluginEnabled(ctx, id, false); err != nil {
		return err
	}
	return k.SetPluginEnabled(ctx, id, true)
}

func (k *Kernel) registerPluginCapabilities(id string, rec *pluginRecord) error {
	if tp, ok := rec.plugin.(sdk.ToolProvider); ok {
		tools := tp.Tools()
		if err := validateManifestTools(rec, tools); err != nil {
			return err
		}
		declared := make(map[string]sdk.ToolSpec, len(rec.manifest.Tools))
		for _, spec := range rec.manifest.Tools {
			declared[spec.Name] = spec
		}
		for i := range tools {
			if spec, ok := declared[tools[i].Name]; ok && tools[i].Timeout <= 0 && spec.TimeoutMS > 0 {
				tools[i].Timeout = time.Duration(spec.TimeoutMS) * time.Millisecond
			}
			if err := k.tools.RegisterOwned(id, tools[i]); err != nil {
				return fmt.Errorf("register tools for %q: %w", id, err)
			}
		}
	}
	if ap, ok := rec.plugin.(sdk.AdapterProvider); ok {
		if err := k.actions.RegisterAdapter(ap.Adapter()); err != nil {
			return fmt.Errorf("register adapter for %q: %w", id, err)
		}
	}
	return nil
}

// PluginStatuses 实现 sdk.AdminView。
func (k *Kernel) PluginStatuses() []sdk.PluginStatus {
	k.stateMu.RLock()
	defer k.stateMu.RUnlock()
	out := make([]sdk.PluginStatus, 0, len(k.reg.order))
	for _, id := range k.reg.order {
		rec := k.reg.records[id]
		if rec == nil {
			continue
		}
		_, hasManual := rec.plugin.(sdk.ManualProvider)
		out = append(out, sdk.PluginStatus{
			ID:          id,
			Version:     rec.manifest.Version,
			Name:        rec.manifest.Name,
			Description: rec.manifest.Description,
			Kind:        rec.manifest.Kind,
			Critical:    rec.spec.Critical,
			HasManual:   hasManual,
			Enabled:     !k.disabled[id],
			Health:      k.getHealth(id),
		})
	}
	return out
}

// Metrics 实现 sdk.AdminView。
func (k *Kernel) Metrics() map[string]int64 { return k.metrics.Snapshot() }

func (k *Kernel) removeStartedID(id string) {
	kept := k.startedIDs[:0]
	for _, started := range k.startedIDs {
		if started != id {
			kept = append(kept, started)
		}
	}
	k.startedIDs = kept
}

func (k *Kernel) setHealth(id string, h sdk.Health) {
	k.healthMu.Lock()
	k.health[id] = h
	k.healthMu.Unlock()
}

func (k *Kernel) getHealth(id string) sdk.Health {
	k.healthMu.Lock()
	defer k.healthMu.Unlock()
	return k.health[id]
}

// PluginManual 实现 sdk.AdminView。
func (k *Kernel) PluginManual(id string) (manual string, ok bool) {
	rec := k.reg.records[id]
	if rec == nil {
		return "", false
	}
	provider, ok := rec.plugin.(sdk.ManualProvider)
	if !ok {
		return "", false
	}
	defer func() {
		if recover() != nil {
			manual, ok = "", false
		}
	}()
	return provider.Manual(), true
}

// ToolManual 实现 sdk.AdminView。
func (k *Kernel) ToolManual(name string) (string, bool) {
	t, ok := k.tools.Get(name)
	if !ok || t.Manual == "" {
		return "", false
	}
	return t.Manual, true
}

// ToolSchema 实现 sdk.AdminView。
func (k *Kernel) ToolSchema(name string) (json.RawMessage, bool) {
	t, ok := k.tools.Get(name)
	if !ok || len(t.Schema) == 0 {
		return nil, false
	}
	return append(json.RawMessage(nil), t.Schema...), true
}

// RecentTraces 返回最近 limit 条 Trace，最新在前。
func (k *Kernel) RecentTraces(limit int) []sdk.Trace {
	k.traceMu.Lock()
	defer k.traceMu.Unlock()
	if limit <= 0 || limit > len(k.traces) {
		limit = len(k.traces)
	}
	out := make([]sdk.Trace, 0, limit)
	for i := len(k.traces) - 1; i >= len(k.traces)-limit; i-- {
		out = append(out, k.traces[i])
	}
	return out
}

var _ sdk.AdminView = (*Kernel)(nil)
var _ sdk.PluginController = (*Kernel)(nil)

func (k *Kernel) observe(ctx context.Context, o sdk.Observer, ev sdk.Event) {
	defer func() {
		if r := recover(); r != nil {
			k.opts.Logger.Error("observer panic", "panic", r)
		}
	}()
	o.Observe(ctx, ev)
}

// Start 按依赖顺序启动插件。关键插件失败会回滚已启动插件。
func (k *Kernel) Start(ctx context.Context) error {
	k.controlMu.Lock()
	defer k.controlMu.Unlock()
	if !k.inited {
		return fmt.Errorf("kernel is not initialized")
	}
	if k.started {
		return nil
	}
	k.stateMu.Lock()
	k.startedIDs = k.startedIDs[:0]
	k.stateMu.Unlock()
	for _, id := range k.reg.order {
		rec := k.reg.records[id]
		k.stateMu.RLock()
		disabled := k.disabled[id]
		k.stateMu.RUnlock()
		if disabled {
			continue
		}
		if err := k.startOne(ctx, rec); err != nil {
			k.setHealth(id, sdk.Health{Status: sdk.HealthFailed, Message: err.Error(), CheckedAt: time.Now()})
			k.stateMu.RLock()
			started := append([]string(nil), k.startedIDs...)
			k.stateMu.RUnlock()
			if rec.spec.Critical {
				_ = k.stopIDs(context.Background(), started)
				k.stateMu.Lock()
				k.startedIDs = nil
				k.stateMu.Unlock()
				k.started = false
				return fmt.Errorf("start critical plugin %q: %w", id, err)
			}
			k.setHealth(id, sdk.Degraded(err.Error()))
			continue
		}
		k.stateMu.Lock()
		k.startedIDs = append(k.startedIDs, id)
		k.stateMu.Unlock()
		k.setHealth(id, sdk.Healthy())
	}
	fail := func() {
		k.stopAdapters(context.Background())
		k.stateMu.RLock()
		started := append([]string(nil), k.startedIDs...)
		k.stateMu.RUnlock()
		_ = k.stopIDs(context.Background(), started)
		k.stateMu.Lock()
		k.startedIDs = nil
		k.stateMu.Unlock()
		k.started = false
	}
	if err := k.startAdapters(ctx); err != nil {
		fail()
		return err
	}
	if err := k.buildEngine(ctx); err != nil {
		fail()
		return fmt.Errorf("build turn engine: %w", err)
	}
	k.started = true
	return nil
}

func (k *Kernel) startAdapters(ctx context.Context) error {
	// 调度器使用独立生命周期：进程收到 SIGTERM 时先停止 adapter，再由 Stop 排空队列；
	// 若直接继承 signal ctx，worker 会在 Stop 前退出并丢弃积压事件。
	scheduler := NewScheduler(context.Background(), k.opts.SchedulerWorkers, k.opts.SchedulerBuffer, func(ctx context.Context, ev sdk.Event) error {
		_, err := k.HandleEvent(ctx, ev)
		return err
	})
	k.schedMu.Lock()
	k.scheduler = scheduler
	k.schedMu.Unlock()
	for _, a := range k.actions.List() {
		adapter := a
		if err := adapter.Start(ctx, func(ctx context.Context, ev sdk.Event) {
			k.submitEvent(ctx, ev)
		}); err != nil {
			return fmt.Errorf("start adapter %q: %w", adapter.ID(), err)
		}
		k.adapters = append(k.adapters, adapter)
	}
	return nil
}

func (k *Kernel) submitEvent(ctx context.Context, ev sdk.Event) {
	k.schedMu.RLock()
	scheduler := k.scheduler
	k.schedMu.RUnlock()
	if scheduler != nil {
		if err := scheduler.Submit(ctx, ev); err != nil && err != ErrSchedulerClosed {
			k.opts.Logger.Warn("event dropped", "conversation", ev.Conversation.ID, "error", err)
		}
	}
}

func (k *Kernel) stopAdapters(ctx context.Context) {
	// 先排空调度器（回合仍可向 adapter 发送），再关闭 adapter，避免在途回复丢消息。
	k.schedMu.Lock()
	scheduler := k.scheduler
	k.scheduler = nil
	k.schedMu.Unlock()
	if scheduler != nil {
		_ = scheduler.Close(ctx)
	}
	for _, a := range k.adapters {
		_ = a.Close()
	}
	k.adapters = nil
}

// Stop 先停 adapter/排空调度器，再逆序停止插件。可安全重复调用。
func (k *Kernel) Stop(ctx context.Context) error {
	k.controlMu.Lock()
	defer k.controlMu.Unlock()

	// 先断开 adapter 并排空调度器；此时运行中的回合仍持有 turnMu 读锁。
	k.stopAdapters(ctx)

	// 等直接 HandleEvent 调用结束，再停插件，避免 Stop 与回合并发。
	k.turnMu.Lock()
	defer k.turnMu.Unlock()
	k.stateMu.Lock()
	if len(k.startedIDs) == 0 {
		k.stateMu.Unlock()
		k.started = false
		k.engineMu.Lock()
		k.engine = nil
		k.observers = nil
		k.engineMu.Unlock()
		return nil
	}
	ids := append([]string(nil), k.startedIDs...)
	k.startedIDs = nil
	k.stateMu.Unlock()
	k.started = false
	err := k.stopIDs(ctx, ids)
	k.engineMu.Lock()
	k.engine = nil
	k.observers = nil
	k.engineMu.Unlock()
	return err
}

func (k *Kernel) stopIDs(ctx context.Context, ids []string) error {
	var firstErr error
	for i := len(ids) - 1; i >= 0; i-- {
		id := ids[i]
		rec := k.reg.records[id]
		if rec == nil {
			continue
		}
		if err := k.stopOne(ctx, rec); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("stop plugin %q: %w", id, err)
			}
			k.setHealth(id, sdk.Health{Status: sdk.HealthFailed, Message: err.Error(), CheckedAt: time.Now()})
			continue
		}
		k.setHealth(id, sdk.Healthy())
	}
	return firstErr
}

// Health 聚合插件健康状态。
func (k *Kernel) Health(ctx context.Context) sdk.Health {
	k.stateMu.RLock()
	ids := make([]string, 0, len(k.reg.order))
	for _, id := range k.reg.order {
		if !k.disabled[id] {
			ids = append(ids, id)
		}
	}
	k.stateMu.RUnlock()

	overall := sdk.Healthy()
	for _, id := range ids {
		h := k.pluginHealth(ctx, id)
		k.setHealth(id, h)
		switch h.Status {
		case sdk.HealthFailed:
			overall = sdk.Health{Status: sdk.HealthFailed, Message: "plugin " + id + " failed", CheckedAt: time.Now()}
		case sdk.HealthDegraded:
			if overall.Status == sdk.HealthOK {
				overall = sdk.Health{Status: sdk.HealthDegraded, Message: "plugin " + id + " degraded", CheckedAt: time.Now()}
			}
		}
	}
	return overall
}

func (k *Kernel) pluginHealth(ctx context.Context, id string) (h sdk.Health) {
	rec := k.reg.records[id]
	if rec == nil {
		return sdk.Health{Status: sdk.HealthFailed, Message: "unknown plugin", CheckedAt: time.Now()}
	}
	defer func() {
		if r := recover(); r != nil {
			h = sdk.Health{Status: sdk.HealthFailed, Message: fmt.Sprintf("health panic: %v", r), CheckedAt: time.Now()}
		}
	}()
	h = rec.plugin.Health(ctx)
	if h.CheckedAt.IsZero() {
		h.CheckedAt = time.Now()
	}
	if h.Status == "" {
		h.Status = sdk.HealthOK
	}
	return h
}

// Plugins 返回插件状态快照。
func (k *Kernel) Plugins() []PluginInfo {
	k.stateMu.RLock()
	defer k.stateMu.RUnlock()
	out := make([]PluginInfo, 0, len(k.reg.order))
	for _, id := range k.reg.order {
		rec := k.reg.records[id]
		if rec == nil {
			continue
		}
		out = append(out, PluginInfo{
			ID:              id,
			Version:         rec.manifest.Version,
			Kind:            rec.manifest.Kind,
			Critical:        rec.spec.Critical,
			Health:          k.getHealth(id),
			MissingOptional: append([]string(nil), rec.missingOptional...),
		})
	}
	return out
}

func (k *Kernel) initOne(ctx context.Context, rec *pluginRecord) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &sdk.Error{Code: sdk.ErrPluginPanic, PluginID: rec.manifest.ID, Message: fmt.Sprintf("init panic: %v", r)}
		}
	}()
	c, cancel := context.WithTimeout(ctx, k.opts.Timeout)
	defer cancel()
	return rec.plugin.Init(c, &host{kernel: k, pluginID: rec.manifest.ID})
}

func (k *Kernel) startOne(ctx context.Context, rec *pluginRecord) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &sdk.Error{Code: sdk.ErrPluginPanic, PluginID: rec.manifest.ID, Message: fmt.Sprintf("start panic: %v", r)}
		}
	}()
	c, cancel := context.WithTimeout(ctx, k.opts.Timeout)
	defer cancel()
	return rec.plugin.Start(c)
}

func (k *Kernel) stopOne(ctx context.Context, rec *pluginRecord) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &sdk.Error{Code: sdk.ErrPluginPanic, PluginID: rec.manifest.ID, Message: fmt.Sprintf("stop panic: %v", r)}
		}
	}()
	c, cancel := context.WithTimeout(ctx, k.opts.Timeout)
	defer cancel()
	return rec.plugin.Stop(c)
}
