package webui

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"icarosu/internal/attention"
	"icarosu/internal/config"
	"icarosu/internal/llm"
	"icarosu/internal/logger"
	"icarosu/internal/meme"
	"icarosu/internal/memory"
	"icarosu/internal/plugin"
	"icarosu/internal/plugin/summarizer"
	"icarosu/internal/store"
)

// Server 提供本地 WebUI，用于查看/编辑配置、热重载和查看日志。
type Server struct {
	manager    *config.Manager
	logger     *slog.Logger
	logs       *logger.Buffer
	store      store.FullStore
	attention  *attention.Tracker
	llm        *llm.Client
	memes      *meme.Store
	plugins    *plugin.Manager
	mem        *memory.Memory
	summarizer *summarizer.SummarizerPlugin
	bot        plugin.Bot
	startedAt  time.Time
	mu         sync.Mutex
	configMu   sync.Mutex
	started    bool
	http       *http.Server
	handler    http.Handler
}

// Option 鐢ㄤ簬閰嶇疆 WebUI銆?
type Option func(*Server)

// WithLogBuffer 设置日志查看用的内存缓冲。
func WithLogBuffer(buf *logger.Buffer) Option {
	return func(s *Server) {
		s.logs = buf
	}
}

// WithStore 设置长期存储，用于状态面板统计。
func WithStore(st store.FullStore) Option {
	return func(s *Server) {
		s.store = st
	}
}

// WithAttention 设置注意力系统，用于状态面板展示。
func WithAttention(at *attention.Tracker) Option {
	return func(s *Server) {
		s.attention = at
	}
}

// WithLLM 设置 LLM 客户端，用于状态面板展示调用统计。
func WithLLM(client *llm.Client) Option {
	return func(s *Server) {
		s.llm = client
	}
}

// WithMemeStore 设置表情包库，用于表情管理页面。
func WithMemeStore(ms *meme.Store) Option {
	return func(s *Server) {
		s.memes = ms
	}
}

// WithPluginManager 设置插件管理器，用于插件热开关页面。
func WithPluginManager(pm *plugin.Manager) Option {
	return func(s *Server) {
		s.plugins = pm
	}
}

// WithMemory 设置短期记忆，用于管理操作。
func WithMemory(mem *memory.Memory) Option {
	return func(s *Server) {
		s.mem = mem
	}
}

// WithSummarizer 设置总结插件，用于手动触发总结。
func WithSummarizer(sm *summarizer.SummarizerPlugin) Option {
	return func(s *Server) {
		s.summarizer = sm
	}
}

// WithBot 设置机器人发送能力，用于发送测试消息。
func WithBot(b plugin.Bot) Option {
	return func(s *Server) {
		s.bot = b
	}
}

func New(manager *config.Manager, logger *slog.Logger, opts ...Option) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{manager: manager, logger: logger, startedAt: time.Now()}
	for _, opt := range opts {
		opt(s)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/config/raw", s.handleConfigRaw)
	mux.HandleFunc("/api/reload", s.handleReload)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/logs/stream", s.handleLogStream)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/api/memories", s.handleMemories)
	mux.HandleFunc("/api/memories/delete", s.handleMemoryDelete)
	mux.HandleFunc("/api/memes", s.handleMemes)
	mux.HandleFunc("/api/memes/add", s.handleMemeAdd)
	mux.HandleFunc("/api/memes/delete", s.handleMemeDelete)
	mux.HandleFunc("/api/memes/file", s.handleMemeFile)
	mux.HandleFunc("/api/plugins", s.handlePlugins)
	mux.HandleFunc("/api/plugins/toggle", s.handlePluginToggle)
	mux.HandleFunc("/api/admin/clear", s.handleAdminClear)
	mux.HandleFunc("/api/admin/summarize", s.handleAdminSummarize)
	mux.HandleFunc("/api/admin/send", s.handleAdminSend)
	mux.HandleFunc("/api/groups/styles", s.handleGroupStyles)
	mux.HandleFunc("/api/groups/jargons", s.handleGroupJargons)
	mux.HandleFunc("/api/groups/profiles", s.handleGroupProfiles)
	s.handler = s.auth(mux)
	s.http = &http.Server{
		Addr:              manager.Get().WebUI.Addr,
		Handler:           s.handler,
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	addr := s.manager.Get().WebUI.Addr
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	token := s.manager.Get().WebUI.Token
	if token == "" {
		if !isLoopbackAddr(addr) {
			return fmt.Errorf("webui token is required when listening on non-loopback address %q", addr)
		}
		s.logger.Warn("webui writes with no Origin are allowed: webui.token is empty", "addr", addr)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen webui %q: %w", addr, err)
	}
	s.started = true
	s.logger.Info("webui started", "addr", addr)
	go func() {
		if err := s.http.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.logger.Error("webui server error", "error", err)
		}
	}()
	return nil
}

// Close 关闭 HTTP 服务。
func (s *Server) Close() error {
	s.mu.Lock()
	s.started = false
	s.mu.Unlock()
	if s.http == nil {
		return nil
	}
	return s.http.Close()
}

// auth 保护除首页/健康检查外的接口；配置 token 时未授权请求返回 401。
// 首页保持可访问，前端会提示输入令牌。
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/healthz" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		token := s.manager.Get().WebUI.Token
		if token == "" {
			if isWriteMethod(r.Method) && !sameOrigin(r) {
				http.Error(w, "cross-origin write rejected", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if !s.authorized(r, token) {
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

func (s *Server) authorized(r *http.Request, token string) bool {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return false
	}
	got := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	if len(got) != len(token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
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

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (s *Server) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.logger.Warn("write json failed", "error", err)
	}
}
