package kernel

import (
	"fmt"
	"sort"

	"icarosu/internal/sdk"
)

type pluginRecord struct {
	spec            sdk.PluginSpec
	plugin          sdk.Plugin
	manifest        sdk.Manifest
	health          sdk.Health
	missingOptional []string
}

type registry struct {
	records map[string]*pluginRecord
	caps    *capabilityRegistry
	order   []string
}

func newRegistry() *registry {
	return &registry{
		records: make(map[string]*pluginRecord),
		caps:    newCapabilityRegistry(),
	}
}

func (r *registry) register(spec sdk.PluginSpec) (err error) {
	if spec.New == nil {
		return fmt.Errorf("plugin spec has nil New")
	}
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("plugin constructor panic: %v", rec)
		}
	}()

	p := spec.New()
	if p == nil {
		return fmt.Errorf("plugin constructor returned nil")
	}
	m := p.Manifest()
	if err := m.Validate(); err != nil {
		return fmt.Errorf("invalid manifest %q: %w", m.ID, err)
	}
	if _, ok := r.records[m.ID]; ok {
		return fmt.Errorf("duplicate plugin id %q", m.ID)
	}
	r.records[m.ID] = &pluginRecord{
		spec:     spec,
		plugin:   p,
		manifest: m,
		health:   sdk.Healthy(),
	}
	return nil
}

// resolve 注册 capability、检查依赖并返回拓扑排序后的插件顺序。
func (r *registry) resolve() ([]string, error) {
	ids := make([]string, 0, len(r.records))
	for id := range r.records {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		rec := r.records[id]
		if err := r.caps.add(id, rec.manifest.Provides); err != nil {
			return nil, fmt.Errorf("plugin %q: %w", id, err)
		}
	}

	deps := make(map[string][]string, len(r.records))
	for _, id := range ids {
		rec := r.records[id]
		for _, req := range rec.manifest.Requires {
			if !r.caps.has(req) {
				return nil, fmt.Errorf("plugin %q requires unresolved capability %q", id, req)
			}
			owner, _ := r.caps.provider(req)
			if owner != "kernel" && owner != id {
				deps[id] = append(deps[id], owner)
			}
		}
		for _, req := range rec.manifest.OptionalRequires {
			if !r.caps.has(req) {
				rec.missingOptional = append(rec.missingOptional, req)
			} else if owner, _ := r.caps.provider(req); owner != "kernel" && owner != id {
				deps[id] = append(deps[id], owner)
			}
		}
	}

	order, err := topoSort(ids, deps)
	if err != nil {
		return nil, err
	}
	r.order = order
	return order, nil
}

func topoSort(ids []string, deps map[string][]string) ([]string, error) {
	const (
		unvisited = 0
		visiting  = 1
		done      = 2
	)
	state := make(map[string]int, len(ids))
	order := make([]string, 0, len(ids))

	var visit func(string, []string) error
	visit = func(id string, stack []string) error {
		switch state[id] {
		case done:
			return nil
		case visiting:
			return fmt.Errorf("dependency cycle: %v -> %s", stack, id)
		}
		state[id] = visiting
		for _, dep := range deps[id] {
			if err := visit(dep, append(stack, id)); err != nil {
				return err
			}
		}
		state[id] = done
		order = append(order, id)
		return nil
	}

	for _, id := range ids {
		if err := visit(id, nil); err != nil {
			return nil, err
		}
	}
	return order, nil
}
