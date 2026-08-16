package webui

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"icarosu/internal/config"
	"icarosu/internal/logger"
)

// Server 提供本地 WebUI，用于查看/编辑配置、热重载和查看日志。
type Server struct {
	manager *config.Manager
	logger  *slog.Logger
	logs    *logger.Buffer
	http    *http.Server
	handler http.Handler
}

// Option 用于配置 WebUI。
type Option func(*Server)

// WithLogBuffer 设置日志查看用的内存缓冲。
func WithLogBuffer(buf *logger.Buffer) Option {
	return func(s *Server) {
		s.logs = buf
	}
}

func New(manager *config.Manager, logger *slog.Logger, opts ...Option) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{manager: manager, logger: logger}
	for _, opt := range opts {
		opt(s)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/config/raw", s.handleConfigRaw)
	mux.HandleFunc("/api/reload", s.handleReload)
	mux.HandleFunc("/api/logs", s.handleLogs)
	s.handler = mux
	s.http = &http.Server{
		Addr:              manager.Get().WebUI.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

// Handler 返回 HTTP 路由，便于测试或嵌入其他服务。
func (s *Server) Handler() http.Handler {
	return s.handler
}

// Start 在后台启动 HTTP 服务。
func (s *Server) Start() error {
	addr := s.manager.Get().WebUI.Addr
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	s.http.Addr = addr
	s.logger.Info("webui started", "addr", addr)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("webui server error", "error", err)
		}
	}()
	return nil
}

// Close 关闭 HTTP 服务。
func (s *Server) Close() error {
	if s.http == nil {
		return nil
	}
	return s.http.Close()
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.writeJSON(w, s.manager.Get())
}

func (s *Server) handleConfigRaw(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile(s.manager.Path())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)
	case http.MethodPost:
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(string(body)) == "" {
			http.Error(w, "config cannot be empty", http.StatusBadRequest)
			return
		}

		path := s.manager.Path()
		old, _ := os.ReadFile(path)
		if err := os.WriteFile(path, body, 0o644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.manager.Reload(); err != nil {
			_ = os.WriteFile(path, old, 0o644)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.logger.Info("config updated and reloaded via webui")
		s.writeJSON(w, map[string]any{
			"ok":     true,
			"config": s.manager.Get(),
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.manager.Reload(); err != nil {
		s.logger.Warn("config reload failed", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.logger.Info("config reloaded via webui")
	s.writeJSON(w, map[string]any{
		"ok":     true,
		"config": s.manager.Get(),
	})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if s.logs == nil {
		s.writeJSON(w, map[string]any{"logs": []string{}})
		return
	}
	s.writeJSON(w, map[string]any{"logs": s.logs.Recent(limit)})
}

func (s *Server) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.logger.Warn("write json failed", "error", err)
	}
}

const indexHTML = `<!doctype html>
<html lang="zh">
<head>
<meta charset="utf-8">
<title>Icarosu WebUI</title>
<style>
body { font-family: system-ui, sans-serif; margin: 2rem; background: #f7f7f7; }
section { margin-bottom: 1.5rem; }
pre, textarea { background: #fff; border: 1px solid #ddd; padding: 1rem; overflow: auto; }
textarea { width: 100%; box-sizing: border-box; font-family: ui-monospace, monospace; }
button { padding: .5rem 1rem; margin-right: .5rem; }
h2 { margin-bottom: .5rem; }
</style>
</head>
<body>
<h1>Icarosu 控制面板</h1>

<section>
<h2>配置</h2>
<p>
  <button onclick="loadConfig()">查看当前配置(JSON)</button>
  <button onclick="loadRawConfig()">编辑配置(YAML)</button>
  <button onclick="saveRawConfig()">保存并热重载</button>
  <button onclick="reloadConfig()">仅热重载</button>
</p>
<textarea id="rawConfig" rows="18" placeholder="点击“编辑配置(YAML)”后在此修改"></textarea>
<pre id="output">点击按钮查看或编辑配置。</pre>
</section>

<section>
<h2>日志</h2>
<p><button onclick="loadLogs()">刷新日志</button> <label><input type="checkbox" id="autoLog" checked> 自动刷新(3秒)</label></p>
<pre id="logOutput">点击“刷新日志”查看。</pre>
</section>

<script>
async function loadConfig() {
  const res = await fetch('/api/config');
  document.getElementById('output').textContent = JSON.stringify(await res.json(), null, 2);
}
async function loadRawConfig() {
  const res = await fetch('/api/config/raw');
  const text = await res.text();
  document.getElementById('rawConfig').value = text;
  document.getElementById('output').textContent = '已载入 YAML，修改后点击“保存并热重载”。';
}
async function saveRawConfig() {
  const text = document.getElementById('rawConfig').value;
  const res = await fetch('/api/config/raw', {
    method: 'POST',
    headers: {'Content-Type': 'text/plain'},
    body: text,
  });
  const data = await res.json();
  document.getElementById('output').textContent = JSON.stringify(data, null, 2);
  if (!res.ok) alert('保存失败：' + (data.error || res.statusText));
}
async function reloadConfig() {
  const res = await fetch('/api/reload', {method: 'POST'});
  const data = await res.json();
  document.getElementById('output').textContent = JSON.stringify(data, null, 2);
}
async function loadLogs() {
  const res = await fetch('/api/logs?limit=300');
  const data = await res.json();
  document.getElementById('logOutput').textContent = (data.logs || []).join('\n');
}
setInterval(() => {
  if (document.getElementById('autoLog').checked) loadLogs();
}, 3000);
loadLogs();
</script>
</body>
</html>
`
