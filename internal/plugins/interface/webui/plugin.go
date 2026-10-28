package webui

import (
	"context"
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"icarosu/internal/manifest"
	"icarosu/internal/sdk"
)

//go:embed manifest.yaml
var manifestYAML []byte

//go:embed index.html
var indexHTML string

//go:embed MANUAL.md
var manual string

var parsedManifest = manifest.MustParse(manifestYAML)

type Config struct {
	Enabled bool   `json:"enabled"`
	Addr    string `json:"addr"`
	Token   string `json:"token"`
}

type Plugin struct {
	view       sdk.AdminView
	controller sdk.PluginController
	pluginsDir string
	cfg        Config
	logger     *slog.Logger
	mu         sync.Mutex
	server     *http.Server
	started    bool
}

func New(view sdk.AdminView, controller sdk.PluginController, pluginsDir string, logger *slog.Logger) sdk.Plugin {
	return NewWithConfig(view, controller, pluginsDir, Config{Enabled: true, Addr: "127.0.0.1:8081"}, logger)
}

// NewWithConfig 允许测试/嵌入方显式指定 token/addr。
func NewWithConfig(view sdk.AdminView, controller sdk.PluginController, pluginsDir string, cfg Config, logger *slog.Logger) sdk.Plugin {
	if logger == nil {
		logger = slog.Default()
	}
	return &Plugin{view: view, controller: controller, pluginsDir: pluginsDir, cfg: cfg, logger: logger}
}

func (p *Plugin) Manifest() sdk.Manifest { return parsedManifest }

func (p *Plugin) Init(ctx context.Context, host sdk.Host) error {
	if host != nil {
		_ = host.Config("interface.webui", &p.cfg)
	}
	if p.cfg.Addr == "" {
		p.cfg.Addr = "127.0.0.1:8081"
	}
	return nil
}

func (p *Plugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return nil
	}
	if !p.cfg.Enabled {
		return nil
	}
	if p.cfg.Token == "" && !isLoopbackAddr(p.cfg.Addr) {
		return fmt.Errorf("webui token is required when listening on non-loopback address %q", p.cfg.Addr)
	}
	ln, err := net.Listen("tcp", p.cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen webui %q: %w", p.cfg.Addr, err)
	}
	p.server = &http.Server{
		Addr:              p.cfg.Addr,
		Handler:           p.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	p.started = true
	server := p.server
	p.logger.Info("webui plugin started", "addr", p.cfg.Addr)
	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			p.logger.Error("webui plugin server error", "error", err)
		}
	}()
	return nil
}

func (p *Plugin) Stop(ctx context.Context) error {
	p.mu.Lock()
	server := p.server
	p.server = nil
	p.started = false
	p.mu.Unlock()
	if server == nil {
		return nil
	}
	if err := server.Shutdown(ctx); err != nil {
		return server.Close()
	}
	return nil
}

func (p *Plugin) Health(ctx context.Context) sdk.Health {
	p.mu.Lock()
	started, enabled := p.started, p.cfg.Enabled
	p.mu.Unlock()
	if !enabled {
		return sdk.Degraded("disabled")
	}
	if !started {
		return sdk.Degraded("not started")
	}
	return sdk.Healthy()
}

// Handler 返回 HTTP 路由，便于测试或嵌入。
func (p *Plugin) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleIndex)
	mux.HandleFunc("/healthz", p.handleHealth)
	mux.HandleFunc("/api/status", p.handleStatus)
	mux.HandleFunc("/api/plugins", p.handlePlugins)
	mux.HandleFunc("/api/plugins/manual", p.handlePluginManual)
	mux.HandleFunc("/api/tools/manual", p.handleToolManual)
	mux.HandleFunc("/api/tools/schema", p.handleToolSchema)
	mux.HandleFunc("/api/plugins/toggle", p.handlePluginToggle)
	mux.HandleFunc("/api/plugins/config", p.handlePluginConfig)
	mux.HandleFunc("/api/traces", p.handleTraces)
	mux.HandleFunc("/metrics", p.handleMetrics)
	return p.auth(mux)
}

func (p *Plugin) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		if p.cfg.Token == "" {
			if isWriteMethod(r.Method) {
				http.Error(w, "webui token is required for writes", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		got := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if !strings.HasPrefix(auth, "Bearer ") || len(got) != len(p.cfg.Token) || subtle.ConstantTimeCompare([]byte(got), []byte(p.cfg.Token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="webui"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if isWriteMethod(r.Method) && !sameOrigin(r) {
			http.Error(w, "cross-origin write rejected", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (p *Plugin) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (p *Plugin) handleHealth(w http.ResponseWriter, r *http.Request) {
	p.writeJSON(w, p.view.Health(r.Context()))
}

func (p *Plugin) handleStatus(w http.ResponseWriter, r *http.Request) {
	p.writeJSON(w, map[string]any{
		"health":  p.view.Health(r.Context()),
		"plugins": p.view.PluginStatuses(),
		"traces":  len(p.view.RecentTraces(200)),
		"metrics": p.view.Metrics(),
	})
}

func (p *Plugin) handlePlugins(w http.ResponseWriter, r *http.Request) {
	p.writeJSON(w, p.view.PluginStatuses())
}

func (p *Plugin) handlePluginManual(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	manual, ok := p.view.PluginManual(id)
	if !ok {
		http.Error(w, "manual not found", http.StatusNotFound)
		return
	}
	p.writeJSON(w, map[string]string{"id": id, "manual": manual})
}

func (p *Plugin) handleToolManual(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	manual, ok := p.view.ToolManual(name)
	if !ok {
		http.Error(w, "tool manual not found", http.StatusNotFound)
		return
	}
	p.writeJSON(w, map[string]string{"name": name, "manual": manual})
}

func (p *Plugin) handleToolSchema(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	schema, ok := p.view.ToolSchema(name)
	if !ok {
		http.Error(w, "tool schema not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(schema)
}

func (p *Plugin) handlePluginToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if p.controller == nil {
		http.Error(w, "plugin controller is not configured", http.StatusNotImplemented)
		return
	}
	var req map[string]any
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, _ := req["id"].(string)
	enabled, ok := req["enabled"].(bool)
	if id == "" || !ok {
		http.Error(w, "id and enabled are required", http.StatusBadRequest)
		return
	}
	if id == "interface.webui" && !enabled {
		http.Error(w, "cannot disable the active webui plugin from itself", http.StatusConflict)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := p.controller.SetPluginEnabled(ctx, id, enabled); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p.writeJSON(w, map[string]any{"ok": true, "id": id, "enabled": enabled})
}

var pluginIDRE = regexp.MustCompile(`^[a-z][a-z0-9._-]*$`)

func (p *Plugin) handlePluginConfig(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if !pluginIDRE.MatchString(id) {
		http.Error(w, "invalid plugin id", http.StatusBadRequest)
		return
	}
	if p.pluginsDir == "" {
		http.Error(w, "plugins dir is not configured", http.StatusNotImplemented)
		return
	}
	path := filepath.Join(p.pluginsDir, id+".yaml")
	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(data)
	case http.MethodPost:
		data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(data) == 0 {
			http.Error(w, "config cannot be empty", http.StatusBadRequest)
			return
		}
		var parsed any
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			http.Error(w, "invalid yaml: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.MkdirAll(p.pluginsDir, 0o755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmp, err := os.CreateTemp(p.pluginsDir, id+".*.tmp")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmpName := tmp.Name()
		defer func() { _ = os.Remove(tmpName) }()
		if err := tmp.Chmod(0o600); err != nil {
			_ = tmp.Close()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := tmp.Write(data); err != nil {
			_ = tmp.Close()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tmp.Sync(); err != nil {
			_ = tmp.Close()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tmp.Close(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.Rename(tmpName, path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var reloadErr string
		if p.controller != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := p.controller.ReloadPlugin(ctx, id); err != nil {
				reloadErr = err.Error()
			}
		}
		p.writeJSON(w, map[string]any{"ok": true, "id": id, "reload_error": reloadErr})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (p *Plugin) handleTraces(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	p.writeJSON(w, p.view.RecentTraces(limit))
}

func (p *Plugin) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	metrics := p.view.Metrics()
	keys := make([]string, 0, len(metrics))
	for k := range metrics {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		_, _ = fmt.Fprintf(w, "%s %d\n", k, metrics[k])
	}
}

func (p *Plugin) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, fmt.Sprintf("encode: %v", err), http.StatusInternalServerError)
	}
}

func (p *Plugin) Manual() string { return manual }

var _ sdk.Plugin = (*Plugin)(nil)
