package kernel

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"icarosu/internal/sdk"
)

// actionRouter 按 Conversation.AdapterID 把 Action 路由给平台 adapter。
type actionRouter struct {
	mu       sync.RWMutex
	adapters map[string]sdk.Adapter
	logger   *slog.Logger
	metrics  *Metrics
}

func newActionRouter(logger *slog.Logger, metrics *Metrics) *actionRouter {
	if logger == nil {
		logger = slog.Default()
	}
	return &actionRouter{adapters: make(map[string]sdk.Adapter), logger: logger, metrics: metrics}
}

func (r *actionRouter) RegisterAdapter(a sdk.Adapter) error {
	if a == nil {
		return fmt.Errorf("adapter is nil")
	}
	id := a.ID()
	if id == "" {
		return fmt.Errorf("adapter id is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.adapters[id]; ok {
		return fmt.Errorf("duplicate adapter %q", id)
	}
	r.adapters[id] = a
	return nil
}

func (r *actionRouter) Adapter(id string) (sdk.Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[id]
	return a, ok
}

func (r *actionRouter) UnregisterAdapter(id string) {
	r.mu.Lock()
	delete(r.adapters, id)
	r.mu.Unlock()
}

func (r *actionRouter) List() []sdk.Adapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]sdk.Adapter, 0, len(r.adapters))
	for _, a := range r.adapters {
		out = append(out, a)
	}
	return out
}

func (r *actionRouter) Send(ctx context.Context, action sdk.Action) error {
	if action.Silent {
		return nil
	}
	adapterID := string(action.Conversation.AdapterID)
	if adapterID == "" {
		return fmt.Errorf("action has no adapter id")
	}
	a, ok := r.Adapter(adapterID)
	if !ok {
		r.logger.Warn("action has unknown adapter", "adapter", adapterID)
		return fmt.Errorf("unknown adapter %q", adapterID)
	}
	if r.metrics != nil {
		r.metrics.Actions.Add(1)
	}
	var sendErr error
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				sendErr = &sdk.Error{Code: sdk.ErrPluginPanic, Message: fmt.Sprintf("adapter %q panic: %v", adapterID, rec)}
			}
		}()
		sendErr = a.Send(ctx, action)
	}()
	if sendErr != nil {
		if r.metrics != nil {
			r.metrics.ActionErrors.Add(1)
		}
		r.logger.Warn("action send failed", "adapter", adapterID, "parts", len(action.Parts), "error", sendErr)
		return sendErr
	}
	r.logger.Debug("action sent", "adapter", adapterID, "parts", len(action.Parts))
	return nil
}

var _ sdk.ActionSink = (*actionRouter)(nil)
