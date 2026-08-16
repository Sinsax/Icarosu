package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"icarosu/internal/llm"
)

// ExecFunc 执行一个工具，入参是已经解析好的 JSON 参数。
type ExecFunc func(ctx context.Context, args map[string]any) (string, error)

// Tool 是一个可注册的工具。
type Tool struct {
	Definition llm.Tool
	Execute    ExecFunc
}

// Registry 是工具注册表。
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	order []string
}

func New() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register 注册工具，工具名必须唯一。
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.tools[t.Definition.Function.Name]; ok {
		panic(fmt.Sprintf("tools: duplicate tool name %q", t.Definition.Function.Name))
	}
	r.tools[t.Definition.Function.Name] = t
	r.order = append(r.order, t.Definition.Function.Name)
}

// Definitions 返回所有工具的 LLM 定义。
func (r *Registry) Definitions() []llm.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]llm.Tool, 0, len(r.order))
	for _, name := range r.order {
		defs = append(defs, r.tools[name].Definition)
	}
	return defs
}

// Execute 执行指定名称的工具。
func (r *Registry) Execute(ctx context.Context, name, arguments string) (string, error) {
	r.mu.RLock()
	t, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}

	var args map[string]any
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("parse arguments for tool %q: %w", name, err)
		}
	}

	return t.Execute(ctx, args)
}
