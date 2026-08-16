package webui_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"icarosu/internal/config"
	"icarosu/internal/logger"
	"icarosu/internal/webui"
)

func writeTestConfig(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", ".webui-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	path := filepath.Join(dir, "config.yaml")
	content := `version: 12
llm_config_file: ""
bot:
  self_id: 1
  name: "` + name + `"
qq:
  onebot:
    ws_url: "ws://127.0.0.1:6700"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestWebUIConfigEndpoint(t *testing.T) {
	m, err := config.NewManager(writeTestConfig(t, "old"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	s := webui.New(m, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var data map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	bot, _ := data["Bot"].(map[string]any)
	if bot["Name"] != "old" {
		t.Fatalf("expected Bot.Name old, got %v", bot["Name"])
	}
}

func TestWebUIReloadEndpoint(t *testing.T) {
	path := writeTestConfig(t, "old")
	m, err := config.NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	s := webui.New(m, nil)

	// 修改磁盘配置后通过 WebUI 触发重载
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	updated := strings.Replace(string(data), `name: "old"`, `name: "new"`, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatalf("update config: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/reload", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if m.Get().Bot.Name != "new" {
		t.Fatalf("expected reloaded name new, got %q", m.Get().Bot.Name)
	}
}

func TestWebUIConfigJSONUpdate(t *testing.T) {
	path := writeTestConfig(t, "old")
	m, err := config.NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	s := webui.New(m, nil)

	// 读取当前 JSON 配置并修改 bot.name
	getReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get config failed: %d", getRec.Code)
	}
	var data map[string]any
	if err := json.Unmarshal(getRec.Body.Bytes(), &data); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	bot := data["Bot"].(map[string]any)
	bot["Name"] = "new"
	body, _ := json.Marshal(data)

	postReq := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(string(body)))
	postReq.Header.Set("Content-Type", "application/json")
	postRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusOK {
		t.Fatalf("update config failed: %d %s", postRec.Code, postRec.Body.String())
	}
	if m.Get().Bot.Name != "new" {
		t.Fatalf("expected updated name new, got %q", m.Get().Bot.Name)
	}
}

func TestWebUIConfigRawUpdate(t *testing.T) {
	path := writeTestConfig(t, "old")
	m, err := config.NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	s := webui.New(m, nil)

	// GET 原始 YAML
	req := httptest.NewRequest(http.MethodGet, "/api/config/raw", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `name: "old"`) {
		t.Fatalf("unexpected raw config response: %d %s", rec.Code, rec.Body.String())
	}

	// POST 修改后的 YAML 并触发重载
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	updated := strings.Replace(string(data), `name: "old"`, `name: "new"`, 1)
	req = httptest.NewRequest(http.MethodPost, "/api/config/raw", strings.NewReader(updated))
	req.Header.Set("Content-Type", "text/plain")
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if m.Get().Bot.Name != "new" {
		t.Fatalf("expected updated name new, got %q", m.Get().Bot.Name)
	}
}

func TestWebUILogsEndpoint(t *testing.T) {
	m, err := config.NewManager(writeTestConfig(t, "old"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	buf := logger.NewBuffer(10)
	_, _ = buf.Write([]byte("line1\nline2\n"))
	s := webui.New(m, nil, webui.WithLogBuffer(buf))

	req := httptest.NewRequest(http.MethodGet, "/api/logs?limit=10", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var data struct {
		Logs []string `json:"logs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("decode logs: %v", err)
	}
	if len(data.Logs) != 2 || data.Logs[0] != "line1" || data.Logs[1] != "line2" {
		t.Fatalf("unexpected logs: %v", data.Logs)
	}
}
