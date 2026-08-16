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

	"gopkg.in/yaml.v3"

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
	switch r.Method {
	case http.MethodGet:
		s.writeJSON(w, s.manager.Get())
	case http.MethodPost, http.MethodPut:
		var cfg config.Config
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if cfg.Version == 0 {
			cfg.Version = config.CurrentVersion
		}
		if cfg.LLMConfigFile == "" {
			cfg.LLMConfigFile = s.manager.Get().LLMConfigFile
		}

		data, err := yaml.Marshal(&cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		path := s.manager.Path()
		old, _ := os.ReadFile(path)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.manager.Reload(); err != nil {
			_ = os.WriteFile(path, old, 0o644)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.logger.Info("config updated via webui form")
		s.writeJSON(w, map[string]any{
			"ok":     true,
			"config": s.manager.Get(),
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
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
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Icarosu 控制面板</title>
<style>
:root {
  --bg: #0f172a;
  --card: #1e293b;
  --card-2: #273549;
  --text: #e2e8f0;
  --muted: #94a3b8;
  --accent: #38bdf8;
  --accent-2: #818cf8;
  --ok: #34d399;
  --err: #f87171;
  --border: #334155;
  --radius: 14px;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  font-family: "Segoe UI", system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
  background: linear-gradient(135deg, #0f172a 0%, #1e1b4b 50%, #0f172a 100%);
  color: var(--text);
  min-height: 100vh;
}
.app { max-width: 1200px; margin: 0 auto; padding: 24px 16px 48px; }
header {
  display: flex; align-items: center; justify-content: space-between;
  background: rgba(30,41,59,.72); backdrop-filter: blur(12px);
  border: 1px solid var(--border); border-radius: var(--radius);
  padding: 18px 24px; margin-bottom: 20px;
  box-shadow: 0 8px 30px rgba(0,0,0,.25);
}
header h1 { margin: 0; font-size: 22px; background: linear-gradient(90deg, var(--accent), var(--accent-2)); -webkit-background-clip: text; background-clip: text; color: transparent; }
.status { font-size: 13px; color: var(--muted); }
.status .dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: var(--ok); margin-right: 6px; }
nav.tabs { display: flex; gap: 8px; margin-bottom: 18px; }
.tab {
  border: 1px solid var(--border); background: var(--card); color: var(--muted);
  padding: 10px 20px; border-radius: 999px; cursor: pointer; font-size: 14px; transition: .2s;
}
.tab:hover { color: var(--text); border-color: var(--accent); }
.tab.active { background: linear-gradient(90deg, var(--accent), var(--accent-2)); color: #0f172a; font-weight: 600; border-color: transparent; }
.card {
  background: rgba(30,41,59,.78); backdrop-filter: blur(8px);
  border: 1px solid var(--border); border-radius: var(--radius);
  padding: 22px; box-shadow: 0 8px 30px rgba(0,0,0,.2);
}
.toolbar { display: flex; flex-wrap: wrap; gap: 10px; margin-bottom: 18px; align-items: center; }
button {
  border: 1px solid var(--border); background: var(--card-2); color: var(--text);
  padding: 9px 16px; border-radius: 10px; cursor: pointer; font-size: 14px; transition: .2s;
}
button:hover { border-color: var(--accent); transform: translateY(-1px); }
button.primary { background: linear-gradient(90deg, var(--accent), var(--accent-2)); color: #0f172a; font-weight: 600; border: none; }
button.danger { background: rgba(248,113,113,.12); color: var(--err); }
button.small { padding: 6px 12px; font-size: 13px; }
.form-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 14px; }
fieldset {
  border: 1px solid var(--border); border-radius: 12px; padding: 14px 16px; margin: 0;
  background: rgba(15,23,42,.35); grid-column: 1 / -1;
}
legend { padding: 0 8px; font-size: 15px; font-weight: 600; color: var(--accent); }
.field { display: flex; flex-direction: column; gap: 6px; }
.field label { font-size: 13px; color: var(--muted); }
.field input, .field select {
  background: #0f172a; border: 1px solid var(--border); color: var(--text);
  border-radius: 8px; padding: 9px 10px; font-size: 14px; outline: none; transition: .2s; width: 100%;
}
.field input:focus, .field select:focus { border-color: var(--accent); box-shadow: 0 0 0 3px rgba(56,189,248,.15); }
.field.check { flex-direction: row; align-items: center; gap: 10px; }
.field.check input { width: 18px; height: 18px; accent-color: var(--accent); }
.log-box {
  background: #020617; border: 1px solid var(--border); border-radius: 12px;
  padding: 16px; height: 60vh; overflow: auto; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px; line-height: 1.6; color: #cbd5e1; white-space: pre-wrap; word-break: break-all;
}
.toast {
  position: fixed; top: 20px; right: 20px; z-index: 99; padding: 12px 18px; border-radius: 10px;
  font-size: 14px; box-shadow: 0 8px 30px rgba(0,0,0,.3); opacity: 0; transform: translateY(-8px); transition: .25s; pointer-events: none;
}
.toast.show { opacity: 1; transform: translateY(0); }
.toast.ok { background: #065f46; color: #d1fae5; border: 1px solid #10b981; }
.toast.err { background: #7f1d1d; color: #fee2e2; border: 1px solid #ef4444; }
.hidden { display: none; }
</style>
</head>
<body>
<div class="app">
  <header>
    <h1>⚡ Icarosu 控制面板</h1>
    <div class="status"><span class="dot"></span>运行中</div>
  </header>

  <nav class="tabs">
    <button class="tab active" id="tab-config" onclick="switchTab('config')">配置编辑</button>
    <button class="tab" id="tab-logs" onclick="switchTab('logs')">日志查看</button>
  </nav>

  <section id="panel-config" class="card">
    <div class="toolbar">
      <button class="primary" onclick="loadConfig()">刷新</button>
      <button onclick="saveConfig()">保存并热重载</button>
      <button onclick="reloadConfig()">仅热重载</button>
      <span style="color:var(--muted); font-size:13px;">修改后记得保存</span>
    </div>
    <div id="configForm" class="form-grid">
      <p style="color:var(--muted); grid-column:1/-1;">正在自动加载配置...</p>
    </div>
  </section>

  <section id="panel-logs" class="card hidden">
    <div class="toolbar">
      <button class="primary" onclick="loadLogs()">刷新日志</button>
      <label style="color:var(--muted); font-size:14px; display:flex; align-items:center; gap:6px;">
        <input type="checkbox" id="autoLog" checked style="accent-color:var(--accent); width:16px; height:16px;"> 自动刷新(3秒)
      </label>
    </div>
    <div class="log-box" id="logOutput">加载中...</div>
  </section>
</div>

<div class="toast" id="toast"></div>

<script>
const LABELS = {
  Version:'版本', LLMConfigFile:'LLM 配置文件', Bot:'机器人', Filter:'过滤器', Memory:'记忆', Store:'存储',
  VLM:'图片识别', Meme:'表情包', Attention:'注意力', WebUI:'Web 控制台', QQ:'QQ', LLM:'语言模型', Log:'日志',
  SelfID:'机器人QQ', AdminIDs:'管理员QQ', Name:'名字', Nicknames:'昵称', Personality:'性格', SpeakingStyle:'说话风格',
  Background:'背景', Interests:'兴趣', MentionReply:'被@必回', Keywords:'关键词', RandomProbability:'随机概率',
  InterestReply:'兴趣回复', CooldownSeconds:'冷却秒数', GroupRules:'群规则', Forbidden:'禁止事项', SystemPrompt:'系统提示词',
  Enabled:'启用', Mode:'模式', GroupIDs:'群ID', UserIDs:'用户ID', ShortTermSize:'短期记忆条数', LongTermEnabled:'长期记忆',
  ContextMaxAgeMinutes:'上下文时效(分钟)', SummaryThreshold:'总结阈值', SummaryIntervalMinutes:'总结间隔(分钟)',
  Driver:'驱动', DSN:'数据库路径', Provider:'供应商', BaseURL:'接口地址', APIKey:'API密钥', Model:'模型',
  Temperature:'温度', MaxTokens:'最大Token', TimeoutSeconds:'超时(秒)', CachePrefix:'缓存前缀', CacheMaxTokens:'缓存最大Token',
  LogCache:'缓存日志', Directory:'目录', StealProbability:'偷图概率', MaxScore:'最大分数', DecayPerMinute:'每分钟衰减',
  BoostAtBot:'@提升', BoostMention:'提名称提升', BoostInterest:'兴趣提升', BoostBotReplied:'回复后提升',
  TripHoldSeconds:'熔断秒数', Addr:'地址', WSURL:'WebSocket地址', AccessToken:'访问令牌', Prompt:'提示词', Level:'日志级别',
  OneBot:'OneBot'
};
function prettyKey(key) {
  if (LABELS[key]) return LABELS[key];
  return key.replace(/([A-Z])/g, ' $1').replace(/^./, s => s.toUpperCase());
}
let state = null;

function switchTab(name) {
  document.getElementById('tab-config').classList.toggle('active', name === 'config');
  document.getElementById('tab-logs').classList.toggle('active', name === 'logs');
  document.getElementById('panel-config').classList.toggle('hidden', name !== 'config');
  document.getElementById('panel-logs').classList.toggle('hidden', name !== 'logs');
  if (name === 'logs') loadLogs();
}

function toast(msg, ok) {
  const el = document.getElementById('toast');
  el.textContent = msg;
  el.className = 'toast show ' + (ok ? 'ok' : 'err');
  clearTimeout(el._timer);
  el._timer = setTimeout(() => el.className = 'toast', 2600);
}

function setByPath(obj, path, value) {
  const parts = path.split('.');
  let cur = obj;
  for (let i = 0; i < parts.length - 1; i++) {
    if (!cur[parts[i]]) cur[parts[i]] = {};
    cur = cur[parts[i]];
  }
  cur[parts[parts.length - 1]] = value;
}

function renderForm(obj, container, prefix) {
  container.innerHTML = '';
  for (const key of Object.keys(obj)) {
    const value = obj[key];
    const path = prefix ? prefix + '.' + key : key;
    const label = prettyKey(key);
    const wrap = document.createElement('div');
    wrap.className = 'field';

    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      const fieldset = document.createElement('fieldset');
      const legend = document.createElement('legend');
      legend.textContent = label;
      const inner = document.createElement('div');
      inner.className = 'form-grid';
      fieldset.appendChild(legend);
      fieldset.appendChild(inner);
      container.appendChild(fieldset);
      renderForm(value, inner, path);
      continue;
    }

    if (Array.isArray(value)) {
      const lab = document.createElement('label');
      lab.textContent = label;
      const input = document.createElement('input');
      input.type = 'text';
      input.value = value.join(', ');
      input.placeholder = '多个值用逗号分隔';
      input.dataset.path = path;
      input.dataset.kind = 'array';
      const isNum = value.length > 0 && typeof value[0] === 'number';
      input.dataset.arrType = isNum ? 'number' : 'string';
      input.addEventListener('change', () => {
        const raw = input.value.split(',').map(s => s.trim()).filter(Boolean);
        const val = input.dataset.arrType === 'number' ? raw.map(Number) : raw;
        setByPath(state, path, val);
      });
      wrap.appendChild(lab);
      wrap.appendChild(input);
      container.appendChild(wrap);
      continue;
    }

    const lab = document.createElement('label');
    lab.textContent = label;
    const input = document.createElement('input');

    if (typeof value === 'boolean') {
      input.type = 'checkbox';
      input.checked = value;
      input.dataset.path = path;
      input.dataset.kind = 'bool';
      input.addEventListener('change', () => setByPath(state, path, input.checked));
      wrap.classList.add('check');
      wrap.appendChild(input);
      wrap.appendChild(lab);
      container.appendChild(wrap);
      continue;
    }

    if (typeof value === 'number') {
      input.type = 'number';
      input.step = Number.isInteger(value) ? '1' : 'any';
      input.value = value;
      input.dataset.path = path;
      input.dataset.kind = 'number';
      input.addEventListener('change', () => setByPath(state, path, input.value === '' ? 0 : Number(input.value)));
    } else {
      input.type = 'text';
      input.value = value || '';
      input.dataset.path = path;
      input.dataset.kind = 'string';
      input.addEventListener('change', () => setByPath(state, path, input.value));
    }

    wrap.appendChild(lab);
    wrap.appendChild(input);
    container.appendChild(wrap);
  }
}

async function loadConfig() {
  const res = await fetch('/api/config');
  if (!res.ok) { toast('加载配置失败', false); return; }
  state = await res.json();
  renderForm(state, document.getElementById('configForm'));
  toast('配置已加载', true);
}

async function saveConfig() {
  if (!state) { toast('请先加载配置', false); return; }
  const res = await fetch('/api/config', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(state)
  });
  const data = await res.json();
  if (!res.ok) {
    toast('保存失败：' + (data.error || res.statusText), false);
    return;
  }
  state = data.config;
  renderForm(state, document.getElementById('configForm'));
  toast('已保存并热重载', true);
}

async function reloadConfig() {
  const res = await fetch('/api/reload', {method: 'POST'});
  const data = await res.json();
  if (!res.ok) { toast('热重载失败', false); return; }
  state = data.config;
  renderForm(state, document.getElementById('configForm'));
  toast('已热重载', true);
}

async function loadLogs() {
  const res = await fetch('/api/logs?limit=300');
  const data = await res.json();
  document.getElementById('logOutput').textContent = (data.logs || []).join('\n') || '暂无日志';
}

setInterval(() => {
  if (document.getElementById('autoLog').checked) loadLogs();
}, 3000);
loadConfig();
loadLogs();
</script>
</body>
</html>
`
