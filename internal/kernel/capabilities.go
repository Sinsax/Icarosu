package kernel

import "fmt"

// capabilityRegistry 是内核预置 capability + 插件 provides 的注册表。
type capabilityRegistry struct {
	providers map[string]string // capability -> plugin id（"kernel" 表示内核预置）
}

func newCapabilityRegistry() *capabilityRegistry {
	c := &capabilityRegistry{providers: make(map[string]string)}
	for _, name := range []string{
		"kernel.bus", "kernel.llm", "kernel.memory", "kernel.tools",
		"kernel.actions", "kernel.storage", "kernel.config",
		"kernel.log", "kernel.trace", "kernel.clock",
	} {
		c.providers[name] = "kernel"
	}
	return c
}

func (c *capabilityRegistry) has(name string) bool {
	_, ok := c.providers[name]
	return ok
}

func (c *capabilityRegistry) add(pluginID string, capabilities []string) error {
	for _, name := range capabilities {
		if owner, ok := c.providers[name]; ok && owner != pluginID {
			return fmt.Errorf("capability %q already provided by %q", name, owner)
		}
		c.providers[name] = pluginID
	}
	return nil
}

func (c *capabilityRegistry) provider(name string) (string, bool) {
	owner, ok := c.providers[name]
	return owner, ok
}
