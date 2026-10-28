package sdk

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Kind 是插件种类。
type Kind string

const (
	KindAdapter    Kind = "adapter"
	KindStorage    Kind = "storage"
	KindMemory     Kind = "memory"
	KindPersona    Kind = "persona"
	KindRenderer   Kind = "renderer"
	KindDrive      Kind = "drive"
	KindTool       Kind = "tool"
	KindReflection Kind = "reflection"
	KindGovernance Kind = "governance"
	KindInterface  Kind = "interface"
	KindModel      Kind = "model"
	KindSense      Kind = "sense"
)

// Manifest 是插件的唯一事实源。
type Manifest struct {
	APIVersion       int        `yaml:"api_version" json:"api_version"`
	ID               string     `yaml:"id" json:"id"`
	Version          string     `yaml:"version" json:"version"`
	Kind             Kind       `yaml:"kind" json:"kind"`
	Name             string     `yaml:"name" json:"name"`
	Description      string     `yaml:"description" json:"description"`
	Provides         []string   `yaml:"provides" json:"provides"`
	Requires         []string   `yaml:"requires" json:"requires"`
	OptionalRequires []string   `yaml:"optional_requires" json:"optional_requires"`
	ConfigSchema     string     `yaml:"config_schema" json:"config_schema"`
	Permissions      []string   `yaml:"permissions" json:"permissions"`
	Tools            []ToolSpec `yaml:"tools" json:"tools"`
	Manual           string     `yaml:"manual" json:"manual"`
}

var (
	semverRE = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$`)
	kindRE   = regexp.MustCompile(`^[a-z][a-z0-9.-]*$`)
)

func validateAssetPath(p string) error {
	if p == "" {
		return nil
	}
	if filepath.IsAbs(p) {
		return fmt.Errorf("path %q must be relative", p)
	}
	clean := filepath.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes plugin directory", p)
	}
	return nil
}

func validKind(kind Kind) bool {
	switch kind {
	case KindAdapter, KindStorage, KindMemory, KindPersona, KindRenderer, KindDrive,
		KindTool, KindReflection, KindGovernance, KindInterface, KindModel, KindSense:
		return true
	default:
		return false
	}
}

// Validate 校验 manifest 的必填字段与命名。
func (m Manifest) Validate() error {
	if m.APIVersion != 1 {
		return fmt.Errorf("api_version must be 1, got %d", m.APIVersion)
	}
	if m.Kind == "" {
		return fmt.Errorf("kind is empty")
	}
	if !validKind(m.Kind) {
		return fmt.Errorf("unknown plugin kind %q", m.Kind)
	}
	if !kindRE.MatchString(m.ID) {
		return fmt.Errorf("invalid plugin id %q", m.ID)
	}
	if !strings.HasPrefix(m.ID, string(m.Kind)+".") {
		return fmt.Errorf("plugin id %q must start with %q", m.ID, string(m.Kind)+".")
	}
	if !semverRE.MatchString(m.Version) {
		return fmt.Errorf("invalid plugin version %q (want semver)", m.Version)
	}
	if len(m.Provides) == 0 {
		return fmt.Errorf("provides is empty")
	}
	seen := make(map[string]bool)
	for _, p := range m.Provides {
		if strings.TrimSpace(p) == "" {
			return fmt.Errorf("provides contains empty capability")
		}
		if seen[p] {
			return fmt.Errorf("duplicate provides %q", p)
		}
		seen[p] = true
	}
	requiredSeen := make(map[string]bool)
	for _, r := range append(append([]string{}, m.Requires...), m.OptionalRequires...) {
		if strings.TrimSpace(r) == "" {
			return fmt.Errorf("requires contains empty capability")
		}
		if r == m.ID {
			return fmt.Errorf("plugin %q requires itself", m.ID)
		}
		if requiredSeen[r] {
			return fmt.Errorf("duplicate requires %q", r)
		}
		requiredSeen[r] = true
	}
	if err := validateAssetPath(m.ConfigSchema); err != nil {
		return fmt.Errorf("config_schema: %w", err)
	}
	if err := validateAssetPath(m.Manual); err != nil {
		return fmt.Errorf("manual: %w", err)
	}

	toolNames := make(map[string]bool)
	for _, t := range m.Tools {
		if err := ValidateToolName(t.Name); err != nil {
			return fmt.Errorf("tool %q: %w", t.Name, err)
		}
		if toolNames[t.Name] {
			return fmt.Errorf("duplicate tool %q", t.Name)
		}
		if t.TimeoutMS < 0 {
			return fmt.Errorf("tool %q has negative timeout_ms", t.Name)
		}
		if err := validateAssetPath(t.Schema); err != nil {
			return fmt.Errorf("tool %q schema: %w", t.Name, err)
		}
		if err := validateAssetPath(t.Manual); err != nil {
			return fmt.Errorf("tool %q manual: %w", t.Name, err)
		}
		toolNames[t.Name] = true
	}
	return nil
}

// Plugin 是所有第一方插件实现的生命周期接口。
type Plugin interface {
	Manifest() Manifest
	Init(ctx context.Context, host Host) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health(ctx context.Context) Health
}

// PluginSpec 是 bundle 里的显式声明。
type PluginSpec struct {
	New      func() Plugin
	Critical bool
}

// ToolProvider 是可选接口：插件向 LLM 暴露工具。
type ToolProvider interface {
	Tools() []Tool
}

// HealthStatus 是插件健康状态。
type HealthStatus string

const (
	HealthOK       HealthStatus = "ok"
	HealthDegraded HealthStatus = "degraded"
	HealthFailed   HealthStatus = "failed"
)

// Health 是插件健康快照。
type Health struct {
	Status    HealthStatus
	Message   string
	CheckedAt time.Time
}

// Healthy 返回健康状态。
func Healthy() Health { return Health{Status: HealthOK, CheckedAt: time.Now()} }

// Degraded 返回降级状态。
func Degraded(msg string) Health {
	return Health{Status: HealthDegraded, Message: msg, CheckedAt: time.Now()}
}
