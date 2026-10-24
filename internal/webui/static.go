package webui

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
.stats-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 14px; margin-bottom: 20px; }
.stat-card {
  background: rgba(15,23,42,.5); border: 1px solid var(--border); border-radius: 12px; padding: 16px;
}
.stat-card .label { font-size: 13px; color: var(--muted); margin-bottom: 6px; }
.stat-card .value { font-size: 28px; font-weight: 700; color: var(--text); }
.stat-card .sub { font-size: 12px; color: var(--muted); margin-top: 4px; word-break: break-all; }
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
    <button class="tab" id="tab-status" onclick="switchTab('status')">状态</button>
    <button class="tab" id="tab-memories" onclick="switchTab('memories')">记忆</button>
    <button class="tab" id="tab-memes" onclick="switchTab('memes')">表情</button>
    <button class="tab" id="tab-plugins" onclick="switchTab('plugins')">插件</button>
    <button class="tab" id="tab-groups" onclick="switchTab('groups')">群数据</button>
    <button class="tab" id="tab-admin" onclick="switchTab('admin')">管理</button>
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

  <section id="panel-status" class="card hidden">
    <div class="toolbar">
      <button class="primary" onclick="loadStatus()">刷新状态</button>
      <span style="color:var(--muted); font-size:13px;">每 5 秒自动刷新</span>
    </div>
    <div class="stats-grid" id="statusCards"></div>
    <pre id="statusDetail" style="background:#020617; border:1px solid var(--border); border-radius:12px; padding:14px; color:#cbd5e1; font-size:13px; max-height:300px; overflow:auto;">加载中...</pre>
  </section>

  <section id="panel-memories" class="card hidden">
    <div class="toolbar">
      <input id="memorySearch" type="text" placeholder="搜索记忆内容..." style="background:#0f172a; border:1px solid var(--border); color:var(--text); border-radius:8px; padding:9px 12px; width:260px;">
      <button class="primary" onclick="loadMemories()">搜索</button>
      <button onclick="loadMemories()">刷新</button>
      <span style="color:var(--muted); font-size:13px;">支持按内容模糊搜索</span>
    </div>
    <div style="overflow:auto;">
      <table style="width:100%; border-collapse:collapse; font-size:14px;">
        <thead>
          <tr style="color:var(--muted); text-align:left;">
            <th style="padding:8px; border-bottom:1px solid var(--border);">ID</th>
            <th style="padding:8px; border-bottom:1px solid var(--border);">群</th>
            <th style="padding:8px; border-bottom:1px solid var(--border);">用户</th>
            <th style="padding:8px; border-bottom:1px solid var(--border);">内容</th>
            <th style="padding:8px; border-bottom:1px solid var(--border);">时间</th>
            <th style="padding:8px; border-bottom:1px solid var(--border);">操作</th>
          </tr>
        </thead>
        <tbody id="memoriesTable"></tbody>
      </table>
    </div>
  </section>

  <section id="panel-memes" class="card hidden">
    <div class="toolbar">
      <input id="memeUrl" type="text" placeholder="图片 URL，添加表情包..." style="background:#0f172a; border:1px solid var(--border); color:var(--text); border-radius:8px; padding:9px 12px; width:360px;">
      <button class="primary" onclick="addMeme()">添加</button>
      <button onclick="loadMemes()">刷新</button>
      <span style="color:var(--muted); font-size:13px;">点击图片可预览</span>
    </div>
    <div id="memeGrid" style="display:grid; grid-template-columns:repeat(auto-fill,minmax(140px,1fr)); gap:12px;"></div>
  </section>

  <section id="panel-plugins" class="card hidden">
    <div class="toolbar">
      <button class="primary" onclick="loadPlugins()">刷新插件</button>
      <span style="color:var(--muted); font-size:13px;">热开关即时生效，无需重启</span>
    </div>
    <div style="overflow:auto;">
      <table style="width:100%; border-collapse:collapse; font-size:14px;">
        <thead>
          <tr style="color:var(--muted); text-align:left;">
            <th style="padding:8px; border-bottom:1px solid var(--border);">名称</th>
            <th style="padding:8px; border-bottom:1px solid var(--border);">阶段</th>
            <th style="padding:8px; border-bottom:1px solid var(--border);">优先级</th>
            <th style="padding:8px; border-bottom:1px solid var(--border);">说明</th>
            <th style="padding:8px; border-bottom:1px solid var(--border);">状态</th>
            <th style="padding:8px; border-bottom:1px solid var(--border);">操作</th>
          </tr>
        </thead>
        <tbody id="pluginsTable"></tbody>
      </table>
    </div>
  </section>

  <section id="panel-groups" class="card hidden">
    <div class="toolbar">
      <button class="primary" onclick="loadGroupData()">刷新群数据</button>
      <span style="color:var(--muted); font-size:13px;">查看每个群学习到的表达方式、黑话和用户画像</span>
    </div>
    <pre id="groupDataOutput" style="background:#020617; border:1px solid var(--border); border-radius:12px; padding:14px; color:#cbd5e1; font-size:13px; max-height:60vh; overflow:auto;">加载中...</pre>
  </section>

  <section id="panel-admin" class="card hidden">
    <div style="display:grid; grid-template-columns:repeat(auto-fit,minmax(280px,1fr)); gap:16px;">
      <div style="background:rgba(15,23,42,.5); border:1px solid var(--border); border-radius:12px; padding:16px;">
        <h3 style="margin-top:0;">清空短期上下文</h3>
        <div class="field"><label>群 ID</label><input id="clearGroup" type="number" placeholder="群号"></div>
        <div class="field"><label>用户 ID（私聊）</label><input id="clearUser" type="number" placeholder="QQ 号"></div>
        <button class="danger" onclick="adminClear()" style="margin-top:12px;">清空</button>
      </div>
      <div style="background:rgba(15,23,42,.5); border:1px solid var(--border); border-radius:12px; padding:16px;">
        <h3 style="margin-top:0;">手动触发群总结</h3>
        <div class="field"><label>群 ID</label><input id="summarizeGroup" type="number" placeholder="群号"></div>
        <button class="primary" onclick="adminSummarize()" style="margin-top:12px;">总结</button>
      </div>
      <div style="background:rgba(15,23,42,.5); border:1px solid var(--border); border-radius:12px; padding:16px;">
        <h3 style="margin-top:0;">发送测试消息</h3>
        <div class="field"><label>群 ID</label><input id="sendGroup" type="number" placeholder="群号"></div>
        <div class="field"><label>用户 ID（私聊）</label><input id="sendUser" type="number" placeholder="QQ 号"></div>
        <div class="field"><label>内容</label><input id="sendMessage" type="text" placeholder="要发送的消息"></div>
        <button class="primary" onclick="adminSend()" style="margin-top:12px;">发送</button>
      </div>
    </div>
  </section>

  <section id="panel-logs" class="card hidden">
    <div class="toolbar">
      <button class="primary" onclick="loadLogs()">刷新日志</button>
      <label style="color:var(--muted); font-size:14px; display:flex; align-items:center; gap:6px;">
        <input type="checkbox" id="autoLog" checked onchange="toggleLogStream()" style="accent-color:var(--accent); width:16px; height:16px;"> 实时推送
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
let webuiToken = localStorage.getItem('webui_token') || '';

async function apiFetch(path, options) {
  options = options || {};
  options.headers = Object.assign({}, options.headers || {});
  if (webuiToken) options.headers['Authorization'] = 'Bearer ' + webuiToken;
  let res = await fetch(path, options);
  if (res.status === 401) {
    const token = prompt('请输入 WebUI 访问令牌');
    if (!token) return res;
    webuiToken = token;
    localStorage.setItem('webui_token', token);
    options.headers['Authorization'] = 'Bearer ' + token;
    res = await fetch(path, options);
  }
  return res;
}

function switchTab(name) {
  document.getElementById('tab-config').classList.toggle('active', name === 'config');
  document.getElementById('tab-status').classList.toggle('active', name === 'status');
  document.getElementById('tab-memories').classList.toggle('active', name === 'memories');
  document.getElementById('tab-memes').classList.toggle('active', name === 'memes');
  document.getElementById('tab-plugins').classList.toggle('active', name === 'plugins');
  document.getElementById('tab-groups').classList.toggle('active', name === 'groups');
  document.getElementById('tab-admin').classList.toggle('active', name === 'admin');
  document.getElementById('tab-logs').classList.toggle('active', name === 'logs');
  document.getElementById('panel-config').classList.toggle('hidden', name !== 'config');
  document.getElementById('panel-status').classList.toggle('hidden', name !== 'status');
  document.getElementById('panel-memories').classList.toggle('hidden', name !== 'memories');
  document.getElementById('panel-memes').classList.toggle('hidden', name !== 'memes');
  document.getElementById('panel-plugins').classList.toggle('hidden', name !== 'plugins');
  document.getElementById('panel-groups').classList.toggle('hidden', name !== 'groups');
  document.getElementById('panel-admin').classList.toggle('hidden', name !== 'admin');
  document.getElementById('panel-logs').classList.toggle('hidden', name !== 'logs');
  if (name === 'status') loadStatus();
  if (name === 'memories') loadMemories();
  if (name === 'memes') loadMemes();
  if (name === 'plugins') loadPlugins();
  if (name === 'groups') loadGroupData();
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
  const res = await apiFetch('/api/config');
  if (!res.ok) { toast('加载配置失败', false); return; }
  state = await res.json();
  renderForm(state, document.getElementById('configForm'));
  toast('配置已加载', true);
}

async function saveConfig() {
  if (!state) { toast('请先加载配置', false); return; }
  const res = await apiFetch('/api/config', {
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
  const res = await apiFetch('/api/reload', {method: 'POST'});
  const data = await res.json();
  if (!res.ok) { toast('热重载失败', false); return; }
  state = data.config;
  renderForm(state, document.getElementById('configForm'));
  toast('已热重载', true);
}

function esc(s) {
  return String(s).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
}

async function loadStatus() {
  const res = await apiFetch('/api/status');
  if (!res.ok) return;
  const data = await res.json();
  const cards = [
    ['运行状态', data.running ? '在线' : '离线', ''],
    ['群数量', data.groups ?? '-', (data.group_ids || []).join(', ')],
    ['成员数', data.members ?? '-', ''],
    ['消息数', data.messages ?? '-', ''],
    ['记忆数', data.memories ?? '-', ''],
    ['LLM 请求', data.llm ? data.llm.requests : '-', ''],
    ['LLM Tokens', data.llm ? data.llm.tokens : '-', ''],
    ['LLM 错误', data.llm ? data.llm.errors : '-', ''],
    ['鉴权', data.auth_enabled ? '已开启' : '未开启', ''],
    ['测试模式', data.dry_run ? '开启' : '关闭', data.dry_run ? '不会真正发送' : '']
  ];
  document.getElementById('statusCards').innerHTML = cards.map(c =>
    '<div class="stat-card"><div class="label">' + esc(c[0]) + '</div><div class="value">' + esc(c[1]) + '</div>' +
    (c[2] ? '<div class="sub">' + esc(c[2]) + '</div>' : '') + '</div>'
  ).join('');
  document.getElementById('statusDetail').textContent = JSON.stringify(data, null, 2);
}

async function loadMemories() {
  const q = document.getElementById('memorySearch').value.trim();
  const url = q ? '/api/memories?limit=100&q=' + encodeURIComponent(q) : '/api/memories?limit=100';
  const res = await apiFetch(url);
  if (!res.ok) return;
  const data = await res.json();
  const rows = (data.memories || []).map(m =>
    '<tr>' +
    '<td style="padding:8px; border-bottom:1px solid var(--border);">' + esc(m.ID) + '</td>' +
    '<td style="padding:8px; border-bottom:1px solid var(--border);">' + esc(m.GroupID) + '</td>' +
    '<td style="padding:8px; border-bottom:1px solid var(--border);">' + esc(m.UserID) + '</td>' +
    '<td style="padding:8px; border-bottom:1px solid var(--border);">' + esc(m.Content) + '</td>' +
    '<td style="padding:8px; border-bottom:1px solid var(--border);">' + esc(m.CreatedAt) + '</td>' +
    '<td style="padding:8px; border-bottom:1px solid var(--border);"><button class="small danger" onclick="deleteMemory(' + m.ID + ')">删除</button></td>' +
    '</tr>'
  ).join('');
  document.getElementById('memoriesTable').innerHTML = rows || '<tr><td colspan="6" style="padding:12px; color:var(--muted);">暂无记忆</td></tr>';
}

async function deleteMemory(id) {
  if (!confirm('确认删除记忆 #' + id + ' ？')) return;
  const res = await apiFetch('/api/memories/delete', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({id: id})
  });
  if (!res.ok) { toast('删除失败', false); return; }
  toast('已删除', true);
  loadMemories();
}

async function loadMemes() {
  const res = await apiFetch('/api/memes');
  if (!res.ok) return;
  const data = await res.json();
  const paths = data.memes || [];
  if (paths.length === 0) {
    document.getElementById('memeGrid').innerHTML = '<p style="color:var(--muted); grid-column:1/-1;">暂无表情包</p>';
    return;
  }
  document.getElementById('memeGrid').innerHTML = paths.map(p => {
    const name = p.split(/[\\/]/).pop();
    const url = '/api/memes/file?path=' + encodeURIComponent(p);
    return '<div style="background:rgba(15,23,42,.5); border:1px solid var(--border); border-radius:12px; padding:10px; text-align:center;">' +
      '<a href="' + url + '" target="_blank"><img src="' + url + '" alt="' + esc(name) + '" style="max-width:100%; height:100px; object-fit:contain; border-radius:8px; background:#0f172a;"></a>' +
      '<div style="font-size:12px; color:var(--muted); margin:6px 0; word-break:break-all;">' + esc(name) + '</div>' +
      '<button class="small danger" data-path="' + encodeURIComponent(p) + '" onclick="deleteMeme(this.dataset.path)">删除</button>' +
      '</div>';
  }).join('');
}

async function addMeme() {
  const url = document.getElementById('memeUrl').value.trim();
  if (!url) { toast('请输入图片 URL', false); return; }
  const res = await apiFetch('/api/memes/add', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({url: url})
  });
  if (!res.ok) { toast('添加失败', false); return; }
  document.getElementById('memeUrl').value = '';
  toast('已添加', true);
  loadMemes();
}

async function deleteMeme(encodedPath) {
  if (!confirm('确认删除这个表情包？')) return;
  const path = decodeURIComponent(encodedPath);
  const res = await apiFetch('/api/memes/delete', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({path: path})
  });
  if (!res.ok) { toast('删除失败', false); return; }
  toast('已删除', true);
  loadMemes();
}

async function loadPlugins() {
  const res = await apiFetch('/api/plugins');
  if (!res.ok) return;
  const data = await res.json();
  const rows = (data.plugins || []).map(p =>
    '<tr>' +
    '<td style="padding:8px; border-bottom:1px solid var(--border);">' + esc(p.name) + '</td>' +
    '<td style="padding:8px; border-bottom:1px solid var(--border);">' + esc(p.stage) + '</td>' +
    '<td style="padding:8px; border-bottom:1px solid var(--border);">' + esc(p.priority) + '</td>' +
    '<td style="padding:8px; border-bottom:1px solid var(--border);">' + esc(p.description) + '</td>' +
    '<td style="padding:8px; border-bottom:1px solid var(--border);">' + (p.enabled ? '<span style="color:var(--ok);">启用</span>' : '<span style="color:var(--err);">禁用</span>') + '</td>' +
    '<td style="padding:8px; border-bottom:1px solid var(--border);"><button class="small" onclick="togglePlugin(\'' + p.name.replace(/'/g, "\\'") + '\',' + !p.enabled + ')">' + (p.enabled ? '禁用' : '启用') + '</button></td>' +
    '</tr>'
  ).join('');
  document.getElementById('pluginsTable').innerHTML = rows || '<tr><td colspan="6" style="padding:12px; color:var(--muted);">暂无插件</td></tr>';
}

async function togglePlugin(name, enabled) {
  const res = await apiFetch('/api/plugins/toggle', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({name: name, enabled: enabled})
  });
  if (!res.ok) { toast('切换失败', false); return; }
  toast('已切换', true);
  loadPlugins();
}

async function adminClear() {
  const group_id = Number(document.getElementById('clearGroup').value || 0);
  const user_id = Number(document.getElementById('clearUser').value || 0);
  if (!group_id && !user_id) { toast('请填写群 ID 或用户 ID', false); return; }
  const res = await apiFetch('/api/admin/clear', {
    method: 'POST', headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({group_id: group_id, user_id: user_id})
  });
  if (!res.ok) { toast('清空失败', false); return; }
  toast('已清空', true);
}

async function adminSummarize() {
  const group_id = Number(document.getElementById('summarizeGroup').value || 0);
  if (!group_id) { toast('请填写群 ID', false); return; }
  const res = await apiFetch('/api/admin/summarize', {
    method: 'POST', headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({group_id: group_id})
  });
  if (!res.ok) { toast('触发总结失败', false); return; }
  toast('已触发总结', true);
}

async function adminSend() {
  const group_id = Number(document.getElementById('sendGroup').value || 0);
  const user_id = Number(document.getElementById('sendUser').value || 0);
  const message = document.getElementById('sendMessage').value.trim();
  if (!message) { toast('请输入消息内容', false); return; }
  if (!group_id && !user_id) { toast('请填写群 ID 或用户 ID', false); return; }
  const res = await apiFetch('/api/admin/send', {
    method: 'POST', headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({group_id: group_id, user_id: user_id, message: message})
  });
  if (!res.ok) { toast('发送失败', false); return; }
  toast('已发送', true);
}

let logController = null;

function stopLogStream() {
  if (logController) {
    logController.abort();
    logController = null;
  }
}

async function loadGroupData() {
  const output = document.getElementById('groupDataOutput');
  output.textContent = '加载中...';
  try {
    const [stylesRes, jargonsRes, profilesRes] = await Promise.all([
      apiFetch('/api/groups/styles'),
      apiFetch('/api/groups/jargons?group_id=0'),
      apiFetch('/api/groups/profiles?group_id=0'),
    ]);
    const styles = await stylesRes.json();
    const jargons = await jargonsRes.json();
    const profiles = await profilesRes.json();
    output.textContent = JSON.stringify({styles, jargons, profiles}, null, 2);
  } catch (e) {
    output.textContent = '加载失败: ' + e;
  }
}

async function loadLogs() {
  const res = await apiFetch('/api/logs?limit=300');
  const data = await res.json();
  const el = document.getElementById('logOutput');
  el.textContent = (data.logs || []).join('\n') || '暂无日志';
  el.scrollTop = el.scrollHeight;
}

function appendLog(line) {
  const el = document.getElementById('logOutput');
  const text = el.textContent ? el.textContent + '\n' + line : line;
  const lines = text.split('\n');
  el.textContent = lines.length > 1000 ? lines.slice(lines.length - 1000).join('\n') : text;
  el.scrollTop = el.scrollHeight;
}

async function startLogStream() {
  stopLogStream();
  const controller = new AbortController();
  logController = controller;
  try {
    const res = await apiFetch('/api/logs/stream', {signal: controller.signal});
    if (!res.ok || !res.body) throw new Error('stream unavailable');
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = '';
    while (true) {
      const {done, value} = await reader.read();
      if (done) break;
      buf += decoder.decode(value, {stream: true});
      const parts = buf.split('\n\n');
      buf = parts.pop();
      for (const part of parts) {
        if (part.startsWith('data: ')) appendLog(part.slice(6));
      }
    }
  } catch (e) {
    if (e.name !== 'AbortError') loadLogs();
  } finally {
    if (logController === controller) logController = null;
  }
}

function toggleLogStream() {
  if (document.getElementById('autoLog').checked) startLogStream();
  else stopLogStream();
}

setInterval(() => {
  if (!document.getElementById('panel-status').classList.contains('hidden')) loadStatus();
}, 5000);
loadConfig();
loadStatus();
loadLogs();
if (document.getElementById('autoLog').checked) startLogStream();
</script>
</body>
</html>
`

