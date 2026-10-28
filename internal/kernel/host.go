package kernel

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"icarosu/internal/sdk"
)

type host struct {
	kernel   *Kernel
	pluginID string
}

func (h *host) ID() string { return h.pluginID }

func (h *host) Has(capability string) bool {
	return h.kernel.reg.caps.has(capability)
}

func (h *host) Bus() sdk.Bus {
	if h.kernel.opts.Bus == nil {
		return nopBus{}
	}
	return h.kernel.opts.Bus
}
func (h *host) LLM() sdk.LLM {
	if !h.allowed("llm.chat") {
		return nil
	}
	return h.kernel.opts.LLM
}

func (h *host) allowed(permission string) bool {
	rec := h.kernel.reg.records[h.pluginID]
	return rec == nil || hasPermission(rec.manifest.Permissions, permission)
}

func (h *host) Memory() sdk.Memory {
	if !h.allowed("memory.read") || !h.allowed("memory.write") {
		return deniedMemory{pluginID: h.pluginID}
	}
	if h.kernel.opts.Memory == nil {
		return nopMemory{}
	}
	return h.kernel.opts.Memory
}
func (h *host) Tools() sdk.ToolRegistry {
	allowed := make(map[string]bool)
	if rec := h.kernel.reg.records[h.pluginID]; rec != nil {
		for _, t := range rec.manifest.Tools {
			allowed[t.Name] = true
		}
	}
	return pluginTools{kernel: h.kernel, pluginID: h.pluginID, allowed: allowed}
}
func (h *host) Actions() sdk.ActionSink {
	rec := h.kernel.reg.records[h.pluginID]
	if rec != nil && !hasPermission(rec.manifest.Permissions, "messaging.send") {
		return deniedActions{pluginID: h.pluginID}
	}
	return h.kernel.actions
}

func (h *host) Storage(namespace string) (sdk.Storage, error) {
	if h.kernel.opts.Storage == nil {
		return nil, fmt.Errorf("storage capability is not configured")
	}
	if namespace != h.pluginID && !strings.HasPrefix(namespace, h.pluginID+"/") {
		return nil, &sdk.Error{Code: sdk.ErrPermission, PluginID: h.pluginID, Message: "storage namespace is outside plugin scope"}
	}
	return h.kernel.opts.Storage(namespace)
}

func (h *host) Config(section string, dst any) error {
	if h.kernel.opts.Config == nil {
		return fmt.Errorf("config section %q is not configured", section)
	}
	return h.kernel.opts.Config(section, dst)
}

func (h *host) Logger() *slog.Logger { return h.kernel.opts.Logger }
func (h *host) Clock() sdk.Clock     { return h.kernel.opts.Clock }

func (h *host) Members() sdk.MemberDirectory {
	if !h.allowed("members.read") {
		return nil
	}
	return memberDirectory{kernel: h.kernel}
}
func (h *host) Vision() sdk.Vision {
	if !h.allowed("vision.analyze") {
		return nil
	}
	return h.kernel.opts.Vision
}
func (h *host) Memes() sdk.MemeStore {
	if !h.allowed("memes.write") {
		return nil
	}
	return h.kernel.opts.Memes
}

type pluginTools struct {
	kernel   *Kernel
	pluginID string
	allowed  map[string]bool
}

func (p pluginTools) Register(t sdk.Tool) error {
	if !p.allowed[t.Name] {
		return &sdk.Error{Code: sdk.ErrPermission, PluginID: p.pluginID, Message: "tool is not declared in manifest"}
	}
	return p.kernel.tools.RegisterOwned(p.pluginID, t)
}

func (p pluginTools) List() []sdk.ToolSpec {
	all := p.kernel.tools.List()
	out := make([]sdk.ToolSpec, 0, len(all))
	for _, t := range all {
		if p.allowed[t.Name] {
			out = append(out, t)
		}
	}
	return out
}

type deniedMemory struct{ pluginID string }

func (d deniedMemory) Recall(ctx context.Context, query string, limit int) ([]sdk.MemoryItem, error) {
	return nil, &sdk.Error{Code: sdk.ErrPermission, PluginID: d.pluginID, Message: "plugin lacks memory.read/write permission"}
}
func (d deniedMemory) Remember(ctx context.Context, item sdk.MemoryItem) error {
	return &sdk.Error{Code: sdk.ErrPermission, PluginID: d.pluginID, Message: "plugin lacks memory.read/write permission"}
}

type nopMemory struct{}

func (nopMemory) Recall(ctx context.Context, query string, limit int) ([]sdk.MemoryItem, error) {
	return nil, nil
}
func (nopMemory) Remember(ctx context.Context, item sdk.MemoryItem) error { return nil }

type nopBus struct{}

func (nopBus) Publish(ctx context.Context, ev sdk.Event) error { return nil }
func (nopBus) Subscribe(kind string, fn sdk.Observer) (func(), error) {
	return func() {}, nil
}

type deniedActions struct{ pluginID string }

func (d deniedActions) Send(ctx context.Context, a sdk.Action) error {
	return &sdk.Error{Code: sdk.ErrPermission, PluginID: d.pluginID, Message: "plugin lacks messaging.send permission"}
}

type memberDirectory struct {
	kernel *Kernel
}

func (m memberDirectory) ListMembers(ctx context.Context, conversation sdk.Conversation) ([]sdk.Member, error) {
	adapter, ok := m.kernel.actions.Adapter(string(conversation.AdapterID))
	if !ok {
		return nil, fmt.Errorf("unknown adapter %q", conversation.AdapterID)
	}
	directory, ok := adapter.(sdk.MemberDirectory)
	if !ok {
		return nil, fmt.Errorf("adapter %q does not support member directory", conversation.AdapterID)
	}
	return directory.ListMembers(ctx, conversation)
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

var _ sdk.Clock = realClock{}
var _ sdk.Host = (*host)(nil)
var _ sdk.MemberHost = (*host)(nil)
var _ sdk.VisionHost = (*host)(nil)
var _ sdk.MemeHost = (*host)(nil)
