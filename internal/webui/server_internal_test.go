package webui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"icarosu/internal/config"
)

func writeInternalTestConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: 12
llm_config_file: ""
bot:
  self_id: 1
  name: "old"
llm:
  api_key: "secret-key"
qq:
  onebot:
    ws_url: "ws://127.0.0.1:6700"
webui:
  enabled: true
  token: "secret-token"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestHTTPHandlerWiresAuthAndRedactsConfig(t *testing.T) {
	m, err := config.NewManager(writeInternalTestConfig(t))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	s := New(m, nil)
	if s.http.Handler == nil {
		t.Fatal("http server handler is nil")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	s.http.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec = httptest.NewRecorder()
	s.http.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with token, got %d: %s", rec.Code, rec.Body.String())
	}
	var view struct {
		LLM struct {
			APIKey string `json:"APIKey"`
		}
		Secrets map[string]bool `json:"_secrets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode config view: %v", err)
	}
	if view.LLM.APIKey != "" {
		t.Fatalf("api key should be redacted, got %q", view.LLM.APIKey)
	}
	if !view.Secrets["llm_api_key"] {
		t.Fatalf("expected llm_api_key secret flag, got %+v", view.Secrets)
	}

	rawReq := httptest.NewRequest(http.MethodGet, "/api/config/raw", nil)
	rawReq.Header.Set("Authorization", "Bearer secret-token")
	rawRec := httptest.NewRecorder()
	s.http.Handler.ServeHTTP(rawRec, rawReq)
	if rawRec.Code != http.StatusOK {
		t.Fatalf("raw config failed: %d", rawRec.Code)
	}
	if strings.Contains(rawRec.Body.String(), "secret-key") {
		t.Fatalf("raw config leaked api key: %s", rawRec.Body.String())
	}
}

func TestConfigPostPreservesRedactedSecrets(t *testing.T) {
	m, err := config.NewManager(writeInternalTestConfig(t))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	s := New(m, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	s.http.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get config failed: %d", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	bot := payload["Bot"].(map[string]any)
	bot["Name"] = "new"
	body, _ := json.Marshal(payload)

	postReq := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(body))
	postReq.Header.Set("Authorization", "Bearer secret-token")
	postReq.Header.Set("Content-Type", "application/json")
	postRec := httptest.NewRecorder()
	s.http.Handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusOK {
		t.Fatalf("post config failed: %d: %s", postRec.Code, postRec.Body.String())
	}
	if got := m.Get().LLM.APIKey; got != "secret-key" {
		t.Fatalf("api key should be preserved, got %q", got)
	}
	if got := m.Get().Bot.Name; got != "new" {
		t.Fatalf("bot name should be updated, got %q", got)
	}
}
