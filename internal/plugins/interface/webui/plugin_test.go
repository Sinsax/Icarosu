package webui_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"icarosu/internal/plugins/interface/webui"
	"icarosu/internal/sdk"
	"icarosu/internal/sdk/testkit"
)

type fakeView struct{}

func (fakeView) PluginStatuses() []sdk.PluginStatus {
	return []sdk.PluginStatus{{ID: "persona.default", Kind: sdk.KindPersona, Health: sdk.Healthy()}}
}
func (fakeView) PluginManual(id string) (string, bool) { return "manual for " + id, true }
func (fakeView) ToolManual(name string) (string, bool) { return "tool manual for " + name, true }
func (fakeView) ToolSchema(name string) (json.RawMessage, bool) {
	return json.RawMessage("{\"type\":\"object\"}"), true
}
func (fakeView) Metrics() map[string]int64             { return map[string]int64{"turns_total": 1} }
func (fakeView) Health(ctx context.Context) sdk.Health { return sdk.Healthy() }
func (fakeView) RecentTraces(limit int) []sdk.Trace {
	return []sdk.Trace{{TurnID: "t1", Decision: sdk.Decision{Kind: sdk.DecisionReply}}}
}

type fakeController struct {
	id      string
	enabled bool
	err     error
}

func (c *fakeController) SetPluginEnabled(ctx context.Context, id string, enabled bool) error {
	c.id = id
	c.enabled = enabled
	return c.err
}

func (c *fakeController) ReloadPlugin(ctx context.Context, id string) error {
	c.id = id
	return c.err
}

func TestWebUITogglePlugin(t *testing.T) {
	ctrl := &fakeController{}
	p := webui.NewWithConfig(fakeView{}, ctrl, "", webui.Config{Enabled: true, Addr: "127.0.0.1:0", Token: "secret"}, nil).(*webui.Plugin)
	if err := p.Init(context.Background(), testkit.NewFakeHost()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	srv := httptest.NewServer(p.Handler())
	defer srv.Close()

	body := strings.NewReader(`{"id":"reflection.steal","enabled":true}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/plugins/toggle", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST toggle: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ctrl.id != "reflection.steal" || !ctrl.enabled {
		t.Fatalf("unexpected controller call: %+v", ctrl)
	}
}

func TestWebUIPluginConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "persona.default.yaml"), []byte("version: 1\nconfig:\n  name: old\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ctrl := &fakeController{}
	p := webui.NewWithConfig(fakeView{}, ctrl, dir, webui.Config{Enabled: true, Addr: "127.0.0.1:0", Token: "secret"}, nil).(*webui.Plugin)
	if err := p.Init(context.Background(), testkit.NewFakeHost()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	srv := httptest.NewServer(p.Handler())
	defer srv.Close()

	reqGet, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/plugins/config?id=persona.default", nil)
	reqGet.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(reqGet)
	if err != nil {
		t.Fatalf("GET config: %v", err)
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(data), "name: old") {
		t.Fatalf("unexpected config: %s", data)
	}

	body := strings.NewReader("version: 1\nconfig:\n  name: new\n")
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/plugins/config?id=persona.default", body)
	req.Header.Set("Content-Type", "application/yaml")
	req.Header.Set("Authorization", "Bearer secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST config: %v", err)
	}
	resp.Body.Close()
	updated, _ := os.ReadFile(filepath.Join(dir, "persona.default.yaml"))
	if !strings.Contains(string(updated), "name: new") {
		t.Fatalf("config not updated: %s", updated)
	}
	if ctrl.id != "persona.default" {
		t.Fatalf("expected reload call, got %+v", ctrl)
	}
}

func TestWebUIRejectsWritesWithoutToken(t *testing.T) {
	p := webui.New(fakeView{}, &fakeController{}, "", nil).(*webui.Plugin)
	if err := p.Init(context.Background(), testkit.NewFakeHost()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	srv := httptest.NewServer(p.Handler())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/plugins/toggle", strings.NewReader(`{"id":"x","enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 without token, got %d", resp.StatusCode)
	}
}

func TestWebUIStartReportsListenError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	p := webui.NewWithConfig(fakeView{}, nil, "", webui.Config{Enabled: true, Addr: ln.Addr().String(), Token: "secret"}, nil).(*webui.Plugin)
	if err := p.Init(context.Background(), testkit.NewFakeHost()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := p.Start(context.Background()); err == nil {
		t.Fatal("expected listen error")
	}
}

func TestWebUIRequiresTokenOnNonLoopback(t *testing.T) {
	p := webui.NewWithConfig(fakeView{}, nil, "", webui.Config{Enabled: true, Addr: "0.0.0.0:0"}, nil).(*webui.Plugin)
	if err := p.Init(context.Background(), testkit.NewFakeHost()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := p.Start(context.Background()); err == nil {
		t.Fatal("expected non-loopback token error")
	}
}

func TestWebUIEndpoints(t *testing.T) {
	p := webui.New(fakeView{}, nil, "", nil).(*webui.Plugin)
	if err := p.Init(context.Background(), testkit.NewFakeHost()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	srv := httptest.NewServer(p.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/plugins")
	if err != nil {
		t.Fatalf("GET plugins: %v", err)
	}
	defer resp.Body.Close()
	var plugins []sdk.PluginStatus
	if err := json.NewDecoder(resp.Body).Decode(&plugins); err != nil {
		t.Fatalf("decode plugins: %v", err)
	}
	if len(plugins) != 1 || plugins[0].ID != "persona.default" {
		t.Fatalf("unexpected plugins: %+v", plugins)
	}

	resp2, err := http.Get(srv.URL + "/api/traces?limit=5")
	if err != nil {
		t.Fatalf("GET traces: %v", err)
	}
	defer resp2.Body.Close()
	var traces []sdk.Trace
	if err := json.NewDecoder(resp2.Body).Decode(&traces); err != nil {
		t.Fatalf("decode traces: %v", err)
	}
	if len(traces) != 1 || traces[0].TurnID != "t1" {
		t.Fatalf("unexpected traces: %+v", traces)
	}

	resp3, err := http.Get(srv.URL + "/api/tools/manual?name=time_now")
	if err != nil {
		t.Fatalf("GET tool manual: %v", err)
	}
	defer resp3.Body.Close()
	var manual map[string]string
	if err := json.NewDecoder(resp3.Body).Decode(&manual); err != nil {
		t.Fatalf("decode tool manual: %v", err)
	}
	if !strings.Contains(manual["manual"], "tool manual") {
		t.Fatalf("unexpected tool manual: %+v", manual)
	}
}
