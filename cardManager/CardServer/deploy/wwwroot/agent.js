/* ============ FY 卡密管理 - 代理中心 ============ */
const app = {
  view: 'balance',
  me: { username: '', quota: 0, issued_count: 0 },
  gen: { count: 1, days: 30, max_windows: 3, remark: '', project: '' },
  genLoading: false,
  genResult: null,
  cardQuery: { search: '', status: 'all', page: 1, page_size: 20 },
  cards: [], cardTotal: 0,
  projects: [],
};

const MENUS = [
  { id: 'balance', label: '我的余额', icon: 'M11 21h-1l1-7H7.5c-.58 0-.57-.32-.38-.66.19-.34.05-.08.07-.12C8.48 10.94 10.42 7.54 13 3h1l-1 7h3.5c.49 0 .56.33.47.51l-.07.15C12.96 17.55 11 21 11 21z' },
  { id: 'generate', label: '发卡', icon: 'M13 2L3 14h7l-1 8 10-12h-7l1-8z' },
  { id: 'cards', label: '名下卡列表', icon: 'M4 6h16v2H4V6zm0 5h16v2H4v-2zm0 5h10v2H4v-2z' },
];

/* ---------- 主题切换（默认暗色，localStorage 记忆） ---------- */
const ICON_MOON = '<svg viewBox="0 0 24 24"><path d="M12.3 4.9c.4-.2.6-.7.3-1.1-.2-.4-.7-.5-1.1-.3C8.4 5.2 6.1 8.6 6.1 12.5c0 5 4 9 9 9 2.3 0 4.4-.8 6-2.2.3-.3.4-.7.2-1.1-.2-.4-.7-.5-1.1-.3-1 .6-2.1.9-3.3.9-3.9 0-7-3.1-7-7 0-2.5 1.3-4.8 3.4-5.8z"/></svg>';
const ICON_SUN = '<svg viewBox="0 0 24 24"><path d="M12 7c-2.76 0-5 2.24-5 5s2.24 5 5 5 5-2.24 5-5-2.24-5-5-5zM2 13h2c.55 0 1-.45 1-1s-.45-1-1-1H2c-.55 0-1 .45-1 1s.45 1 1 1zm18 0h2c.55 0 1-.45 1-1s-.45-1-1-1h-2c-.55 0-1 .45-1 1s.45 1 1 1zM11 2v2c0 .55.45 1 1 1s1-.45 1-1V2c0-.55-.45-1-1-1s-1 .45-1 1zm0 18v2c0 .55.45 1 1 1s1-.45 1-1v-2c0-.55-.45-1-1-1s-1 .45-1 1zM5.99 4.58c-.39-.39-1.03-.39-1.41 0-.39.39-.39 1.03 0 1.41l1.06 1.06c.39.39 1.03.39 1.41 0s.39-1.03 0-1.41L5.99 4.58zm12.37 12.37c-.39-.39-1.03-.39-1.41 0-.39.39-.39 1.03 0 1.41l1.06 1.06c.39.39 1.03.39 1.41 0 .39-.39.39-1.03 0-1.41l-1.06-1.06zm1.06-10.96c.39-.39.39-1.03 0-1.41-.39-.39-1.03-.39-1.41 0l-1.06 1.06c-.39-.39-.39 1.03 0 1.41s1.03.39 1.41 0l1.06-1.06zM7.05 18.36c.39-.39.39-1.03 0-1.41-.39-.39-1.03-.39-1.41 0l-1.06 1.06c-.39-.39-.39 1.03 0 1.41s1.03.39 1.41 0l1.06-1.06z"/></svg>';
function applyTheme(t) {
  document.documentElement.setAttribute('data-theme', t);
  localStorage.setItem('fy_theme', t);
  const btn = document.getElementById('themeBtn');
  if (btn) btn.innerHTML = (t === 'dark' ? ICON_SUN : ICON_MOON) + '<span>' + (t === 'dark' ? '切换浅色模式' : '切换暗色模式') + '</span>';
}
function toggleTheme() {
  applyTheme(document.documentElement.getAttribute('data-theme') === 'dark' ? 'light' : 'dark');
}

/* ---------- 工具函数 ---------- */
async function api(url, options = {}) {
  const resp = await fetch(url, { credentials: 'include', headers: { 'Content-Type': 'application/json' }, ...options });
  if (resp.status === 401) { location.href = '/login.html'; throw new Error('未登录'); }
  return resp.json();
}
function fmtTime(ts) {
  if (!ts) return '-';
  const d = new Date(ts * 1000);
  const p = n => String(n).padStart(2, '0');
  return d.getFullYear() + '-' + p(d.getMonth() + 1) + '-' + p(d.getDate()) + ' ' + p(d.getHours()) + ':' + p(d.getMinutes());
}
function esc(s) { return String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c])); }
function clampInt(v, min, max, def) { const n = parseInt(v, 10); if (isNaN(n)) return def; return Math.max(min, Math.min(max, n)); }
function toast(msg, ok) {
  const w = document.getElementById('toastWrap');
  const el = document.createElement('div');
  el.className = 'toast ' + (ok === false ? 'err' : 'ok');
  el.textContent = msg;
  w.appendChild(el);
  setTimeout(() => el.remove(), 2600);
}
function pageHead(title, sub, extra) {
  return '<div class="page-head"><div><h2>' + title + '</h2><div class="sub">' + sub + '</div></div>' + (extra || '') + '</div>';
}
function statusOf(row) {
  if (row.status === 'disabled') return 'disabled';
  if (row.status === 'active' && row.expireAt && row.expireAt * 1000 < Date.now()) return 'expired';
  if (row.status === 'active') return 'active';
  return 'unused';
}
const STATUS_TEXT = { active: '已激活', unused: '未使用', disabled: '已禁用', expired: '已过期' };
const STATUS_CLS = { active: 'active', unused: 'unused', disabled: 'disabled', expired: 'expired' };
function pagerHtml(total, page, size) {
  const pages = Math.max(1, Math.ceil(total / size));
  let h = '<span class="total">共 ' + total + ' 条</span>';
  h += '<span class="pg" onclick="goPage(' + (page - 1) + ')">‹</span>';
  let s = Math.max(1, page - 2), e = Math.min(pages, page + 2);
  for (let i = s; i <= e; i++) h += '<span class="pg' + (i === page ? ' on' : '') + '" onclick="goPage(' + i + ')">' + i + '</span>';
  h += '<span class="pg" onclick="goPage(' + (page + 1) + ')">›</span>';
  return h;
}
function goPage(p) {
  const pages = Math.max(1, Math.ceil(app.cardTotal / app.cardQuery.page_size));
  if (p < 1 || p > pages) return;
  app.cardQuery.page = p;
  loadCards();
}

/* ---------- 菜单 ---------- */
function renderMenu() {
  document.getElementById('menu').innerHTML = MENUS.map(m =>
    '<div class="menu-item' + (app.view === m.id ? ' active' : '') + '" onclick="switchView(\'' + m.id + '\')">' +
    '<svg viewBox="0 0 24 24"><path d="' + m.icon + '"/></svg><span>' + m.label + '</span></div>').join('');
}
function switchView(id) { app.view = id; renderMenu(); render(); }

/* ---------- 视图渲染 ---------- */
function render() {
  const v = app.view;
  const main = document.getElementById('main');
  if (v === 'balance') main.innerHTML = viewBalance();
  else if (v === 'generate') main.innerHTML = viewGenerate();
  else if (v === 'cards') main.innerHTML = viewCards();
}

function viewBalance() {
  const m = app.me;
  let h = pageHead('我的余额', '剩余可发卡数', '<button class="btn ghost" onclick="loadMe()">刷新</button>');
  h += '<div class="card" style="background:linear-gradient(135deg,#065F46,#059669,#34D399);border:none;color:#fff;padding:28px 32px">' +
    '<div style="font-size:14px;opacity:.85">剩余可发卡额度</div>' +
    '<div style="font-size:48px;font-weight:700;margin-top:8px;font-family:Consolas,monospace">' + (m.quota || 0) + '<span style="font-size:18px;font-weight:400;margin-left:6px">张</span></div>' +
    '<div style="font-size:13px;opacity:.8;margin-top:8px">已累计发卡：' + (m.issued_count || 0) + ' 张</div></div>';
  h += '<div class="stat-grid" style="grid-template-columns:repeat(2,1fr)">' +
    '<div class="stat" style="background:linear-gradient(135deg,#4F46E5,#818CF8);border:none;color:#fff"><div class="num" style="color:#fff">' + (m.issued_count || 0) + '</div><div class="lab" style="opacity:.85">已发卡数</div></div>' +
    '<div class="stat" style="background:linear-gradient(135deg,#3B82F6,#60A5FA);border:none;color:#fff"><div class="num" style="color:#fff;font-size:18px;line-height:32px">' + fmtTime(m.created_at) + '</div><div class="lab" style="opacity:.85">创建时间</div></div>' +
    '</div>';
  return h;
}

function viewGenerate() {
  const g = app.gen;
  let h = pageHead('发卡', '从自己额度内生成卡密');
  h += '<div class="card"><div class="card-title"><span class="bar"></span>生成参数</div><div class="form-grid">';
  h += '<div class="field"><label>项目（分类）</label><select class="select" style="width:100%" onchange="app.gen.project=this.value">' +
    '<option value="">请选择项目</option>' + app.projects.map(p => '<option value="' + esc(p.name) + '"' + (g.project === p.name ? ' selected' : '') + '>' + esc(p.name) + '</option>').join('') + '</select></div>';
  h += '<div class="field"><label>数量（剩余额度 ' + (app.me.quota || 0) + '）</label><input class="input" type="number" min="1" max="100" value="' + g.count + '" oninput="app.gen.count=clampInt(this.value,1,100,1)"></div>';
  h += '<div class="field"><label>时长（天）</label><input class="input" type="number" min="1" max="3650" value="' + g.days + '" oninput="app.gen.days=clampInt(this.value,1,3650,30)"></div>';
  h += '<div class="field"><label>窗口数（1-100）</label><input class="input" type="number" min="1" max="100" value="' + g.max_windows + '" oninput="app.gen.max_windows=clampInt(this.value,1,100,3)"></div>';
  h += '<div class="field"><label>备注</label><input class="input" placeholder="如：客户A" value="' + esc(g.remark) + '" oninput="app.gen.remark=this.value"></div>';
  h += '</div><div class="btn-group"><button class="btn" onclick="doGenerate()"' + (app.genLoading ? ' disabled' : '') + '>' + (app.genLoading ? '生成中...' : '生 成') + '</button></div></div>';
  if (app.genResult) {
    const r = app.genResult;
    h += '<div class="card"><div class="card-title"><span class="bar"></span>生成结果（' + r.cards.length + ' 张）</div>';
    h += '<div class="batch-tip">批次号：' + r.batch_id + '，剩余额度：' + r.quota_left + '</div>';
    h += '<div class="code-box">' + esc(r.cards.join('\n')) + '</div>';
    h += '<div class="btn-group"><button class="btn" onclick="copyAll()">一键复制全部</button><button class="btn ghost" onclick="exportFile(\'txt\')">下载 txt</button><button class="btn ghost" onclick="exportFile(\'csv\')">下载 CSV</button></div></div>';
  }
  return h;
}

function viewCards() {
  const q = app.cardQuery;
  let h = pageHead('名下卡列表', '查看自己发出的卡密状态');
  h += '<div class="card"><div class="toolbar">';
  h += '<input class="input" style="width:220px" placeholder="搜索卡号" value="' + esc(q.search) + '" oninput="app.cardQuery.search=this.value" onkeydown="if(event.key===\'Enter\'){app.cardQuery.page=1;loadCards()}">';
  h += '<select class="select" style="width:140px" onchange="app.cardQuery.status=this.value;app.cardQuery.page=1;loadCards()">' +
    '<option value="all"' + (q.status === 'all' ? ' selected' : '') + '>全部状态</option>' +
    '<option value="unused"' + (q.status === 'unused' ? ' selected' : '') + '>未使用</option>' +
    '<option value="active"' + (q.status === 'active' ? ' selected' : '') + '>已激活</option>' +
    '<option value="disabled"' + (q.status === 'disabled' ? ' selected' : '') + '>已禁用</option></select>';
  h += '<button class="btn ghost" onclick="app.cardQuery.page=1;loadCards()">查询</button>';
  h += '<div class="spacer"></div><span class="total">共 ' + app.cardTotal + ' 张</span></div>';
  const rows = app.cards.map(r => {
    const st = statusOf(r);
    return '<tr><td class="mono">' + esc(r.code) + '</td><td><span class="badge ' + STATUS_CLS[st] + '">' + STATUS_TEXT[st] + '</span></td>' +
      '<td>' + esc(r.project || '-') + '</td>' +
      '<td>' + r.days + '</td><td>' + r.maxWindows + ' 窗</td>' +
      '<td>' + (r.activateAt ? fmtTime(r.activateAt) : '-') + '</td>' +
      '<td>' + (r.expireAt ? fmtTime(r.expireAt) : '-') + '</td>' +
      '<td>' + esc(r.remark || '') + '</td></tr>';
  }).join('');
  h += '<div class="table-wrap"><table><thead><tr><th>卡号</th><th>状态</th><th>项目</th><th>时长（天）</th><th>窗口</th><th>激活时间</th><th>到期时间</th><th>备注</th></tr></thead>' +
    '<tbody>' + (rows || '<tr><td colspan="8" style="text-align:center;color:var(--text-muted);padding:24px">暂无数据</td></tr>') + '</tbody></table></div>';
  h += '<div class="pager">' + pagerHtml(app.cardTotal, q.page, q.page_size) + '</div></div>';
  return h;
}

/* ---------- 数据操作 ---------- */
async function loadMe() {
  const d = await api('/api/agent/me');
  if (d.code === 0) { app.me = d; if (app.view === 'balance') render(); }
}
async function doGenerate() {
  if (!app.gen.project) { toast('请先选择项目', false); return; }
  if (app.gen.count > (app.me.quota || 0)) { toast('余额不足', false); return; }
  app.genLoading = true; render();
  try {
    const d = await api('/api/agent/generate', { method: 'POST', body: JSON.stringify(app.gen) });
    if (d.code !== 0) { toast(d.msg || '生成失败', false); return; }
    app.genResult = d;
    app.me.quota = d.quota_left;
    toast('成功生成 ' + d.count + ' 张卡密，剩余额度 ' + d.quota_left + ' 张');
    render();
  } finally { app.genLoading = false; }
}
async function copyAll() {
  if (!app.genResult) return;
  await navigator.clipboard.writeText(app.genResult.cards.join('\n'));
  toast('已复制全部卡号');
}
function exportFile(type) {
  if (!app.genResult) return;
  const cards = app.genResult.cards;
  let content, mime, ext;
  if (type === 'txt') { content = cards.join('\n'); mime = 'text/plain'; ext = 'txt'; }
  else {
    content = '卡号,项目,批次,时长(天),窗口数\n' + cards.map(c => c + ',' + app.gen.project + ',' + app.genResult.batch_id + ',' + app.gen.days + ',' + app.gen.max_windows).join('\n');
    mime = 'text/csv'; ext = 'csv';
  }
  const blob = new Blob([content], { type: mime });
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = '卡密批次' + app.genResult.batch_id + '.' + ext;
  a.click();
  URL.revokeObjectURL(a.href);
}
async function loadCards() {
  const q = new URLSearchParams();
  if (app.cardQuery.search) q.set('search', app.cardQuery.search);
  if (app.cardQuery.status !== 'all') q.set('status', app.cardQuery.status);
  q.set('page', app.cardQuery.page);
  q.set('page_size', app.cardQuery.page_size);
  const d = await api('/api/agent/cards?' + q);
  if (d.code === 0) { app.cards = d.list; app.cardTotal = d.total; }
  if (app.view === 'cards') render();
}
async function loadProjects() {
  const d = await api('/api/agent/projects');
  if (d.code === 0) { app.projects = d.list; if (app.view === 'generate') render(); }
}
async function logout() {
  await api('/api/auth/logout', { method: 'POST' });
  location.href = '/login.html';
}

/* ---------- 初始化 ---------- */
(async function init() {
  applyTheme(localStorage.getItem('fy_theme') || 'dark');
  const me = await api('/api/auth/me');
  if (me.role !== 'agent') { location.href = '/login.html'; return; }
  app.me.username = me.username;
  document.getElementById('uname').textContent = me.username;
  renderMenu();
  render();
  loadMe();
  loadCards();
  loadProjects();
})();
