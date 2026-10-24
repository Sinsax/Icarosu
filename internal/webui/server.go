package webui

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
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

// Server 鎻愪緵鏈湴 WebUI锛岀敤浜庢煡鐪?缂栬緫閰嶇疆銆佺儹閲嶈浇鍜屾煡鐪嬫棩蹇椼€?
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
	http       *http.Server
	handler    http.Handler
}

// Option 鐢ㄤ簬閰嶇疆 WebUI銆?
type Option func(*Server)

// WithLogBuffer 璁剧疆鏃ュ織鏌ョ湅鐢ㄧ殑鍐呭瓨缂撳啿銆?
func WithLogBuffer(buf *logger.Buffer) Option {
	return func(s *Server) {
		s.logs = buf
	}
}

// WithStore 璁剧疆闀挎湡瀛樺偍锛岀敤浜庣姸鎬侀潰鏉跨粺璁°€?
func WithStore(st store.FullStore) Option {
	return func(s *Server) {
		s.store = st
	}
}

// WithAttention 璁剧疆娉ㄦ剰鍔涚郴缁燂紝鐢ㄤ簬鐘舵€侀潰鏉垮睍绀恒€?
func WithAttention(at *attention.Tracker) Option {
	return func(s *Server) {
		s.attention = at
	}
}

// WithLLM 璁剧疆 LLM 瀹㈡埛绔紝鐢ㄤ簬鐘舵€侀潰鏉垮睍绀鸿皟鐢ㄧ粺璁°€?
func WithLLM(client *llm.Client) Option {
	return func(s *Server) {
		s.llm = client
	}
}

// WithMemeStore 璁剧疆琛ㄦ儏鍖呭簱锛岀敤浜庤〃鎯呯鐞嗛〉闈€?
func WithMemeStore(ms *meme.Store) Option {
	return func(s *Server) {
		s.memes = ms
	}
}

// WithPluginManager 璁剧疆鎻掍欢绠＄悊鍣紝鐢ㄤ簬鎻掍欢鐑紑鍏抽〉闈€?
func WithPluginManager(pm *plugin.Manager) Option {
	return func(s *Server) {
		s.plugins = pm
	}
}

// WithMemory 璁剧疆鐭湡璁板繂锛岀敤浜庣鐞嗘搷浣溿€?
func WithMemory(mem *memory.Memory) Option {
	return func(s *Server) {
		s.mem = mem
	}
}

// WithSummarizer 璁剧疆鎬荤粨鎻掍欢锛岀敤浜庢墜鍔ㄨЕ鍙戞€荤粨銆?
func WithSummarizer(sm *summarizer.SummarizerPlugin) Option {
	return func(s *Server) {
		s.summarizer = sm
	}
}

// WithBot 璁剧疆鏈哄櫒浜哄彂閫佽兘鍔涳紝鐢ㄤ簬鍙戦€佹祴璇曟秷鎭€?
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
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

// Handler 杩斿洖 HTTP 璺敱锛屼究浜庢祴璇曟垨宓屽叆鍏朵粬鏈嶅姟銆?
func (s *Server) Handler() http.Handler {
	return s.handler
}

// Start 鍦ㄥ悗鍙板惎鍔?HTTP 鏈嶅姟銆?
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

// Close 鍏抽棴 HTTP 鏈嶅姟銆?
func (s *Server) Close() error {
	if s.http == nil {
		return nil
	}
	return s.http.Close()
}

// auth 淇濇姢鎵€鏈?/api 鎺ュ彛锛涢厤缃簡 token 鏃舵湭鎺堟潈璇锋眰杩斿洖 401銆?
// 棣栭〉 / 淇濇寔鍙闂紝鍓嶇浼氭彁绀鸿緭鍏ヤ护鐗屻€?
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/healthz" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		token := s.manager.Get().WebUI.Token
		if token != "" && !s.authorized(r, token) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="webui"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authorized(r *http.Request, token string) bool {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") && strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")) == token {
		return true
	}
	return r.URL.Query().Get("token") == token
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
