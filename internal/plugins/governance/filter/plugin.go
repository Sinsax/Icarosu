package filter

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"icarosu/internal/manifest"
	"icarosu/internal/sdk"
)

//go:embed manifest.yaml
var manifestYAML []byte

var parsedManifest = manifest.MustParse(manifestYAML)

type Config struct {
	Enabled  bool    `json:"enabled"`
	Mode     string  `json:"mode"`
	GroupIDs []int64 `json:"group_ids"`
	UserIDs  []int64 `json:"user_ids"`
}

type Plugin struct {
	cfg Config
}

func New() sdk.Plugin { return &Plugin{} }

func (p *Plugin) Manifest() sdk.Manifest { return parsedManifest }

func (p *Plugin) Init(ctx context.Context, host sdk.Host) error {
	if host != nil {
		_ = host.Config("governance.filter", &p.cfg)
	}
	return nil
}

func (p *Plugin) Start(ctx context.Context) error       { return nil }
func (p *Plugin) Stop(ctx context.Context) error        { return nil }
func (p *Plugin) Health(ctx context.Context) sdk.Health { return sdk.Healthy() }

func (p *Plugin) Check(ctx context.Context, turn *sdk.Turn) error {
	if !p.cfg.Enabled {
		return nil
	}
	ev := turn.Event
	id := nativeID(ev.Conversation)
	var allowed bool
	switch ev.Conversation.Kind {
	case "group":
		allowed = p.allowed(id, p.cfg.GroupIDs)
	case "private":
		allowed = p.allowed(id, p.cfg.UserIDs)
	default:
		return nil
	}
	if !allowed {
		return fmt.Errorf("filtered by %s mode", p.cfg.Mode)
	}
	return nil
}

func (p *Plugin) allowed(id int64, ids []int64) bool {
	contains := false
	for _, v := range ids {
		if v == id {
			contains = true
			break
		}
	}
	if p.cfg.Mode == "deny" {
		return !contains
	}
	return contains
}

func nativeID(c sdk.Conversation) int64 {
	if v := c.Metadata["native_id"]; v != "" {
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	parts := strings.Split(string(c.ID), ":")
	if len(parts) == 0 {
		return 0
	}
	n, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	return n
}

var _ sdk.Plugin = (*Plugin)(nil)
var _ sdk.Policy = (*Plugin)(nil)
