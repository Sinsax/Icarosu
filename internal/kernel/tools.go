package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"icarosu/internal/sdk"
)

const maxToolResultBytes = 16 << 10

// toolRegistry 是内核的工具注册表，负责校验、超时与结果截断。
type toolRegistry struct {
	mu      sync.RWMutex
	tools   map[string]sdk.Tool
	owners  map[string]string
	order   []string
	logger  *slog.Logger
	metrics *Metrics
}

func newToolRegistry(logger *slog.Logger, metrics *Metrics) *toolRegistry {
	if logger == nil {
		logger = slog.Default()
	}
	return &toolRegistry{tools: make(map[string]sdk.Tool), owners: make(map[string]string), logger: logger, metrics: metrics}
}

func (r *toolRegistry) Register(t sdk.Tool) error { return r.RegisterOwned("", t) }

func (r *toolRegistry) RegisterOwned(owner string, t sdk.Tool) error {
	if err := sdk.ValidateToolName(t.Name); err != nil {
		return err
	}
	if t.Execute == nil {
		return fmt.Errorf("tool %q has nil Execute", t.Name)
	}
	if len(t.Schema) > 0 && !json.Valid(t.Schema) {
		return fmt.Errorf("tool %q has invalid JSON schema", t.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tools[t.Name]; ok {
		return fmt.Errorf("duplicate tool %q", t.Name)
	}
	r.tools[t.Name] = t
	r.owners[t.Name] = owner
	r.order = append(r.order, t.Name)
	return nil
}

func (r *toolRegistry) UnregisterOwner(owner string) {
	if owner == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.order[:0]
	for _, name := range r.order {
		if r.owners[name] == owner {
			delete(r.tools, name)
			delete(r.owners, name)
			continue
		}
		kept = append(kept, name)
	}
	r.order = kept
}

func (r *toolRegistry) List() []sdk.ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]sdk.ToolSpec, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		out = append(out, sdk.ToolSpec{
			Name:        t.Name,
			Summary:     t.Summary,
			Permissions: append([]string(nil), t.Permissions...),
		})
	}
	return out
}

func (r *toolRegistry) Definitions() []sdk.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]sdk.ToolDefinition, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		out = append(out, sdk.ToolDefinition{
			Name:        t.Name,
			Description: t.Summary,
			Parameters:  t.Schema,
		})
	}
	return out
}

func (r *toolRegistry) Get(name string) (sdk.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *toolRegistry) Execute(ctx context.Context, call sdk.ToolCall) (sdk.ToolResult, error) {
	t, ok := r.Get(call.Name)
	if !ok {
		return sdk.ToolResult{}, fmt.Errorf("unknown tool %q", call.Name)
	}
	if hasPermission(t.Permissions, "admin") && !call.Actor.IsAdmin {
		return sdk.ToolResult{}, fmt.Errorf("permission denied: tool %q requires admin", call.Name)
	}
	timeout := t.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if r.metrics != nil {
		r.metrics.ToolCalls.Add(1)
	}
	started := time.Now()
	var res sdk.ToolResult
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = &sdk.Error{Code: sdk.ErrPluginPanic, Message: fmt.Sprintf("tool %q panic: %v", call.Name, r)}
			}
		}()
		res, err = t.Execute(c, call)
	}()
	if err != nil {
		if r.metrics != nil {
			r.metrics.ToolErrors.Add(1)
		}
		r.logger.Warn("tool call failed", "tool", call.Name, "duration_ms", time.Since(started).Milliseconds(), "error", err)
		return sdk.ToolResult{}, err
	}
	r.logger.Debug("tool call", "tool", call.Name, "duration_ms", time.Since(started).Milliseconds())
	if len(res.Text) > maxToolResultBytes {
		cut := maxToolResultBytes
		for cut > 0 && !utf8.ValidString(res.Text[:cut]) {
			cut--
		}
		res.Text = res.Text[:cut] + "\n[truncated]"
	}
	if len(res.Parts) > 8 {
		res.Parts = res.Parts[:8]
	}
	return res, nil
}

func hasPermission(permissions []string, want string) bool {
	for _, p := range permissions {
		if p == want {
			return true
		}
	}
	return false
}

func (r *toolRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := append([]string(nil), r.order...)
	sort.Strings(out)
	return out
}

// ResultText 把工具结果变成给 LLM 的文本。
func ResultText(res sdk.ToolResult) string {
	if res.Text != "" {
		return res.Text
	}
	if len(res.Parts) > 0 {
		var kinds []string
		for _, p := range res.Parts {
			kinds = append(kinds, string(p.Type))
		}
		return "tool returned parts: " + strings.Join(kinds, ",")
	}
	return "ok"
}
