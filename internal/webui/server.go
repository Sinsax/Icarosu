package webui

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"icarosu/internal/config"
)

// Server 提供简单的本地 WebUI，用于查看配置和触发热重载。
type Server struct {
	manager *config.Manager
	logger  *slog.Logger
	http    *http.Server
	handler http.Handler
}

func New(manager *config.Manager, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{manager: manager, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/reload", s.handleReload)
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
pre { background: #fff; border: 1px solid #ddd; padding: 1rem; overflow: auto; }
button { padding: .5rem 1rem; margin-right: .5rem; }
</style>
</head>
<body>
<h1>Icarosu 配置面板</h1>
<p><button onclick="loadConfig()">查看配置</button> <button onclick="reloadConfig()">热重载</button></p>
<pre id="output">点击按钮查看当前配置。</pre>
<script>
async function loadConfig() {
  const res = await fetch('/api/config');
  document.getElementById('output').textContent = JSON.stringify(await res.json(), null, 2);
}
async function reloadConfig() {
  const res = await fetch('/api/reload', {method: 'POST'});
  const data = await res.json();
  document.getElementById('output').textContent = JSON.stringify(data, null, 2);
}
</script>
</body>
</html>
`
