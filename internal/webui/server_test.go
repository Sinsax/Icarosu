package webui_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"icarosu/internal/config"
	"icarosu/internal/logger"
	"icarosu/internal/meme"
	"icarosu/internal/memory"
	"icarosu/internal/plugin"
	"icarosu/internal/store"
	"icarosu/internal/webui"
)

type webuiMockBot struct {
	group   []string
	private []string
}

func (b *webuiMockBot) SendGroupMessage(_ context.Context, _ int64, message string) error {
	b.group = append(b.group, message)
	return nil
}

func (b *webuiMockBot) SendPrivateMessage(_ context.Context, _ int64, message string) error {
	b.private = append(b.private, message)
	return nil
}

func (b *webuiMockBot) SendGroupImage(_ context.Context, _ int64, _ string) error { return nil }
func (b *webuiMockBot) SendPrivateImage(_ context.Context, _ int64, _ string) error {
	return nil
}

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

func TestWebUIHealthAndMetrics(t *testing.T) {
	m, err := config.NewManager(writeTestConfig(t, "old"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	s := webui.New(m, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ok") {
		t.Fatalf("unexpected health response: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "bot_uptime_seconds") {
		t.Fatalf("unexpected metrics response: %d %s", rec.Code, rec.Body.String())
	}
}

func TestWebUIStatusEndpoint(t *testing.T) {
	m, err := config.NewManager(writeTestConfig(t, "old"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	s := webui.New(m, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var data map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if data["running"] != true {
		t.Fatalf("expected running true, got %v", data["running"])
	}
}

func TestWebUIMemoriesEndpoint(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".webui-memory-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	st, err := store.Open(config.StoreConfig{Driver: "sqlite", DSN: filepath.Join(dir, "bot.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.SaveMemory(100, 200, "小明喜欢看番"); err != nil {
		t.Fatalf("save memory: %v", err)
	}

	m, err := config.NewManager(writeTestConfig(t, "old"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	s := webui.New(m, nil, webui.WithStore(st))

	req := httptest.NewRequest(http.MethodGet, "/api/memories", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var data struct {
		Memories []store.MemoryRecord `json:"memories"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("decode memories: %v", err)
	}
	if len(data.Memories) != 1 || data.Memories[0].Content != "小明喜欢看番" {
		t.Fatalf("unexpected memories: %+v", data.Memories)
	}

	delBody := strings.NewReader(`{"id":` + fmt.Sprint(data.Memories[0].ID) + `}`)
	req = httptest.NewRequest(http.MethodPost, "/api/memories/delete", delBody)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete failed: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/memories", nil)
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	_ = json.Unmarshal(rec.Body.Bytes(), &data)
	if len(data.Memories) != 0 {
		t.Fatalf("expected memories empty after delete, got %+v", data.Memories)
	}
}

func TestWebUIMemesEndpoint(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".webui-meme-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if err := os.WriteFile(filepath.Join(dir, "a.png"), []byte("fake-image"), 0o644); err != nil {
		t.Fatalf("write meme: %v", err)
	}
	ms, err := meme.Open(dir)
	if err != nil {
		t.Fatalf("open meme store: %v", err)
	}

	m, err := config.NewManager(writeTestConfig(t, "old"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	s := webui.New(m, nil, webui.WithMemeStore(ms))

	req := httptest.NewRequest(http.MethodGet, "/api/memes", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list memes failed: %d", rec.Code)
	}
	var data struct {
		Memes []string `json:"memes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("decode memes: %v", err)
	}
	if len(data.Memes) != 1 {
		t.Fatalf("expected 1 meme, got %d", len(data.Memes))
	}

	fileReq := httptest.NewRequest(http.MethodGet, "/api/memes/file?path="+url.QueryEscape(data.Memes[0]), nil)
	fileRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(fileRec, fileReq)
	if fileRec.Code != http.StatusOK || fileRec.Body.String() != "fake-image" {
		t.Fatalf("unexpected file response: %d %q", fileRec.Code, fileRec.Body.String())
	}

	delBody := strings.NewReader(`{"path":` + fmt.Sprintf("%q", data.Memes[0]) + `}`)
	req = httptest.NewRequest(http.MethodPost, "/api/memes/delete", delBody)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete meme failed: %d %s", rec.Code, rec.Body.String())
	}
}

func TestWebUIPluginsEndpoint(t *testing.T) {
	m, err := config.NewManager(writeTestConfig(t, "old"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	pm := plugin.NewManager()
	s := webui.New(m, nil, webui.WithPluginManager(pm))

	req := httptest.NewRequest(http.MethodGet, "/api/plugins", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list plugins failed: %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/plugins/toggle", strings.NewReader(`{"name":"chat","enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle plugin failed: %d %s", rec.Code, rec.Body.String())
	}
}

func TestWebUIAdminActions(t *testing.T) {
	m, err := config.NewManager(writeTestConfig(t, "old"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	mem := memory.New(10, nil)
	bot := &webuiMockBot{}
	s := webui.New(m, nil, webui.WithMemory(mem), webui.WithBot(bot))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/clear", strings.NewReader(`{"group_id":123}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear failed: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/send", strings.NewReader(`{"group_id":123,"message":"测试消息"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("send failed: %d %s", rec.Code, rec.Body.String())
	}
	if len(bot.group) != 1 || bot.group[0] != "测试消息" {
		t.Fatalf("unexpected sent messages: %v", bot.group)
	}
}

func TestWebUIAuthRequired(t *testing.T) {
	path := writeTestConfig(t, "old")
	m, err := config.NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	// 给配置加上 webui token 后重载
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	withToken := string(data) + "\nwebui:\n  enabled: true\n  addr: \"127.0.0.1:8080\"\n  token: \"secret\"\n"
	if err := os.WriteFile(path, []byte(withToken), 0o644); err != nil {
		t.Fatalf("write token config: %v", err)
	}
	if err := m.Reload(); err != nil {
		t.Fatalf("reload with token: %v", err)
	}

	s := webui.New(m, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with token, got %d", rec.Code)
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

func TestWebUIRawConfigPreservesComments(t *testing.T) {
	path := writeTestConfig(t, "old")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	commented := "# keep this comment\n" + string(data)
	if err := os.WriteFile(path, []byte(commented), 0o644); err != nil {
		t.Fatalf("write commented config: %v", err)
	}
	m, err := config.NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	s := webui.New(m, nil)

	getReq := httptest.NewRequest(http.MethodGet, "/api/config/raw", nil)
	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET raw: %d", getRec.Code)
	}
	updated := strings.Replace(getRec.Body.String(), `name: "old"`, `name: "new"`, 1)
	postReq := httptest.NewRequest(http.MethodPost, "/api/config/raw", strings.NewReader(updated))
	postRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusOK {
		t.Fatalf("POST raw: %d %s", postRec.Code, postRec.Body.String())
	}
	saved, _ := os.ReadFile(path)
	if !strings.Contains(string(saved), "# keep this comment") {
		t.Fatalf("comment lost:\n%s", saved)
	}
	if !strings.Contains(string(saved), `name: "new"`) {
		t.Fatalf("name not updated:\n%s", saved)
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
