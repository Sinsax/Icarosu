package filter_test

import (
	"context"
	"testing"

	"icarosu/internal/config"
	"icarosu/internal/plugin"
	"icarosu/internal/plugin/filter"
)

func TestFilterDisabledPassesAll(t *testing.T) {
	p := filter.New(config.FilterConfig{
		Enabled:  false,
		Mode:     "allow",
		GroupIDs: []int64{100},
		UserIDs:  []int64{100},
	})

	if handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:    plugin.EventGroupMessage,
		GroupID: 999,
	}); handled {
		t.Fatal("disabled filter should not consume group events")
	}

	if handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:   plugin.EventPrivateMessage,
		UserID: 999,
	}); handled {
		t.Fatal("disabled filter should not consume private events")
	}
}

func TestFilterAllowModeForGroup(t *testing.T) {
	p := filter.New(config.FilterConfig{
		Enabled:  true,
		Mode:     "allow",
		GroupIDs: []int64{100, 200},
	})

	if handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:    plugin.EventGroupMessage,
		GroupID: 100,
	}); handled {
		t.Fatal("allowed group should not be filtered")
	}

	if handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:    plugin.EventGroupMessage,
		GroupID: 300,
	}); !handled {
		t.Fatal("non-allowed group should be filtered")
	}
}

func TestFilterAllowModeForPrivate(t *testing.T) {
	p := filter.New(config.FilterConfig{
		Enabled: true,
		Mode:    "allow",
		UserIDs: []int64{100, 200},
	})

	if handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:   plugin.EventPrivateMessage,
		UserID: 100,
	}); handled {
		t.Fatal("allowed user should not be filtered")
	}

	if handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:   plugin.EventPrivateMessage,
		UserID: 300,
	}); !handled {
		t.Fatal("non-allowed user should be filtered")
	}
}

func TestFilterDenyModeForGroup(t *testing.T) {
	p := filter.New(config.FilterConfig{
		Enabled:  true,
		Mode:     "deny",
		GroupIDs: []int64{100, 200},
	})

	if handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:    plugin.EventGroupMessage,
		GroupID: 100,
	}); !handled {
		t.Fatal("denied group should be filtered")
	}

	if handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:    plugin.EventGroupMessage,
		GroupID: 300,
	}); handled {
		t.Fatal("non-denied group should not be filtered")
	}
}

func TestFilterDenyModeForPrivate(t *testing.T) {
	p := filter.New(config.FilterConfig{
		Enabled: true,
		Mode:    "deny",
		UserIDs: []int64{100, 200},
	})

	if handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:   plugin.EventPrivateMessage,
		UserID: 100,
	}); !handled {
		t.Fatal("denied user should be filtered")
	}

	if handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:   plugin.EventPrivateMessage,
		UserID: 300,
	}); handled {
		t.Fatal("non-denied user should not be filtered")
	}
}
