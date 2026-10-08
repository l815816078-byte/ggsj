/* ============ FY 卡密管理 - 管理员后台 ============ */
const app = {
  view: 'dashboard',
  me: { username: '' },
  stats: {},
  gen: { count: 1, days: 30, max_windows: 3, remark: '', project: '' },
  genLoading: false,
  genResult: null,
  projects: [],
  curProj: { id: 0, name: '' },
  cardQuery: { search: '', status: 'all', page: 1, page_size: 20 },
  cards: [], cardTotal: 0,
  windows: [],
  logQuery: { actor: '', action: '', page: 1, page_size: 50 },
  logs: [], logTotal: 0,
  agents: [],
  renew: { code: '', days: 30 },
  createAgent: { username: '', password: '', quota: 100 },
  recharge: { username: '', quota: 0, amount: 100 },
};

const MENUS = [
  { id: 'dashboard', label: '仪表盘', icon: 'M4 13h6V3H4v10zm0 8h6v-6H4v6zm8 0h6V11h-6v10zm0-18v6h6V3h-6z' },
  { id: 'generate', label: '发卡', icon: 'M13 2L3 14h7l-1 8 10-12h-7l1-8z' },
  { id: 'cards', label: '卡密列表', icon: 'M4 6h16v2H4V6zm0 5h16v2H4v-2zm0 5h10v2H4v-2z' },
  { id: 'windows', label: '在线统计', icon: 'M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z' },
  { id: 'logs', label: '操作日志', icon: 'M14 2H6c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V8l-6-6zm2 16H8v-2h8v2zm0-4H8v-2h8v2zm-3-5V3.5L18.5 9H13z' },
  { id: 'projects', label: '项目管理', icon: 'M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z' },
  { id: 'hotupdate', label: '热更新', icon: 'M19.35 10.04C18.67 6.59 15.64 4 12 4 9.11 4 6.6 5.64 5.35 8.04 2.34 8.36 0 10.91 0 14c0 3.31 2.69 6 6 6h13c2.76 0 5-2.24 5-5 0-2.64-2.05-4.78-4.65-4.96zM14 13v4h-4v-4H7l5-5 5 5h-3z' },
  { id: 'agents', label: '代理管理', icon: 'M16 11c1.66 0 2.99-1.34 2.99-3S17.66 5 16 5s-3 1.34-3 3 1.34 3 3 3zm-8 0c1.66 0 2.99-1.34 2.99-3S9.66 5 8 5 5 6.34 5 8s1.34 3 3 3zm0 2c-2.33 0-7 1.17-7 3.5V19h14v-2.5c0-2.33-4.67-3.5-7-3.5zm8 0c-.29 0-.62.02-.97.05 1.16.84 1.97 1.97 1.97 3.45V19h6v-2.5c0-2.33-4.67-3.5-7-3.5z' },
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
function openModal(html) { document.getElementById('modalBox').innerHTML = html; document.getElementById('modalMask').classList.add('show'); }
function closeModal() { document.getElementById('modalMask').classList.remove('show'); }
function confirmModal(title, msg) {
  return new Promise(resolve => {
    window.__confirmOk = () => { closeModal(); resolve(true); };
    openModal('<h3>' + esc(title) + '</h3><div class="tip">' + esc(msg) + '</div><div class="actions">' +
      '<button class="btn ghost" onclick="closeModal()">取消</button>' +
      '<button class="btn" onclick="window.__confirmOk()">确认</button></div>');
  });
}
function statusOf(row) {
  if (row.status === 'disabled') return 'disabled';
  if (row.status === 'active' && row.expireAt && row.expireAt * 1000 < Date.now()) return 'expired';
  if (row.status === 'active') return 'active';
  return 'unused';
}
const STATUS_TEXT = { active: '已激活', unused: '未使用', disabled: '已禁用', expired: '已过期' };
const STATUS_CLS = { active: 'active', unused: 'unused', disabled: 'disabled', expired: 'expired' };
function pagerHtml(total, page, size, kind) {
  const pages = Math.max(1, Math.ceil(total / size));
  let h = '<span class="total">共 ' + total + ' 条</span>';
  h += '<span class="pg" onclick="goPage(\'' + kind + '\',' + (page - 1) + ')">‹</span>';
  let s = Math.max(1, page - 2), e = Math.min(pages, page + 2);
  for (let i = s; i <= e; i++) h += '<span class="pg' + (i === page ? ' on' : '') + '" onclick="goPage(\'' + kind + '\',' + i + ')">' + i + '</span>';
  h += '<span class="pg" onclick="goPage(\'' + kind + '\',' + (page + 1) + ')">›</span>';
  return h;
}
function goPage(kind, p) {
  if (kind === 'cards') { if (p < 1 || p > Math.ceil(app.cardTotal / app.cardQuery.page_size)) return; app.cardQuery.page = p; loadCards(); }
  else if (kind === 'logs') { if (p < 1 || p > Math.ceil(app.logTotal / app.logQuery.page_size)) return; app.logQuery.page = p; loadLogs(); }
}
function pageHead(title, sub, extra) {
  return '<div class="page-head"><div><h2>' + title + '</h2><div class="sub">' + sub + '</div></div>' + (extra || '') + '</div>';
}

/* ---------- 菜单 ---------- */
function renderMenu() {
  document.getElementById('menu').innerHTML = MENUS.map(m =>
    '<div class="menu-item' + (app.view === m.id ? ' active' : '') + '" onclick="switchView(\'' + m.id + '\')">' +
    '<svg viewBox="0 0 24 24"><path d="' + m.icon + '"/></svg><span>' + m.label + '</span></div>').join('');
}
// 每个视图切换过来时自动刷新对应数据（避免显示旧缓存）
const VIEW_LOADERS = {
  dashboard: refreshStats,
  generate: loadProjects,
  cards: loadCards,
  windows: loadWindows,
  logs: loadLogs,
  projects: loadProjects,
  hotupdate: loadHotUpdate,
  agents: loadAgents,
};
function switchView(id) {
  app.view = id;
  renderMenu();
  render();
  const l = VIEW_LOADERS[id];
  if (l) l();
  // 只有仪表盘/在线统计页才持续轮询，离开即停（避免不在页面也一直打接口）
  if (id === 'dashboard' || id === 'windows') startPolling();
  else stopPolling();
}

/* ---------- 视图渲染 ---------- */
function render() {
  const v = app.view;
  const main = document.getElementById('main');
  if (v === 'dashboard') main.innerHTML = viewDashboard();
  else if (v === 'generate') main.innerHTML = viewGenerate();
  else if (v === 'cards') main.innerHTML = viewCards();
  else if (v === 'windows') main.innerHTML = viewWindows();
  else if (v === 'logs') main.innerHTML = viewLogs();
  else if (v === 'projects') main.innerHTML = viewProjects();
  else if (v === 'hotupdate') main.innerHTML = viewHotUpdate();
  else if (v === 'agents') main.innerHTML = viewAgents();
}

function viewDashboard() {
  const s = app.stats;
  const items = [
    { n: s.total || 0, l: '总卡数' }, { n: s.active || 0, l: '已激活' },
    { n: s.today || 0, l: '今日新增激活' }, { n: s.online || 0, l: '当前在线窗口' },
    { n: s.agents || 0, l: '代理总数' }, { n: s.unused || 0, l: '未使用卡' },
  ];
  return pageHead('仪表盘', '卡密系统运行概况', '<button class="btn ghost" onclick="refreshStats()">刷新</button>') +
    '<div class="stat-grid">' + items.map(i => '<div class="stat"><div class="num">' + i.n + '</div><div class="lab">' + i.l + '</div></div>').join('') + '</div>';
}

function viewGenerate() {
  const g = app.gen;
  let h = pageHead('发卡', '批量生成卡密，可一键复制或导出');
  h += '<div class="card"><div class="card-title"><span class="bar"></span>生成参数</div><div class="form-grid">';
  h += '<div class="field"><label>项目（分类）</label><select class="select" style="width:100%" onfocus="refreshGenProjects(this)" onchange="app.gen.project=this.value">' +
    '<option value="">请选择项目</option>' + app.projects.map(p => '<option value="' + esc(p.name) + '"' + (g.project === p.name ? ' selected' : '') + '>' + esc(p.name) + '</option>').join('') + '</select></div>';
  h += '<div class="field"><label>数量（最多 100 张）</label><input class="input" type="number" min="1" max="100" value="' + g.count + '" oninput="app.gen.count=clampInt(this.value,1,100,1)"></div>';
  h += '<div class="field"><label>时长（天）</label><input class="input" type="number" min="1" max="3650" value="' + g.days + '" oninput="app.gen.days=clampInt(this.value,1,3650,30)"></div>';
  h += '<div class="field"><label>窗口数（1-100）</label><input class="input" type="number" min="1" max="100" value="' + g.max_windows + '" oninput="app.gen.max_windows=clampInt(this.value,1,100,3)"></div>';
  h += '<div class="field"><label>备注</label><input class="input" placeholder="如：卖给老王" value="' + esc(g.remark) + '" oninput="app.gen.remark=this.value"></div>';
  h += '</div><div class="btn-group"><button class="btn" onclick="doGenerate()"' + (app.genLoading ? ' disabled' : '') + '>' + (app.genLoading ? '生成中...' : '生 成') + '</button></div></div>';
  if (app.genResult) {
    const r = app.genResult;
    h += '<div class="card"><div class="card-title"><span class="bar"></span>生成结果（' + r.cards.length + ' 张）</div>';
    h += '<div class="batch-tip">批次号：' + r.batch_id + (r.quota_left != null ? '，剩余额度：' + r.quota_left : '') + '</div>';
    h += '<div class="code-box">' + esc(r.cards.join('\n')) + '</div>';
    h += '<div class="btn-group"><button class="btn" onclick="copyAll()">一键复制全部</button><button class="btn ghost" onclick="exportFile(\'txt\')">下载 txt</button><button class="btn ghost" onclick="exportFile(\'csv\')">下载 CSV</button></div></div>';
  }
  return h;
}

function viewCards() {
  const q = app.cardQuery;
  let h = pageHead('卡密列表', '搜索、筛选与卡密管理操作');
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
    let ops = '';
    if (st !== 'disabled') ops += '<button class="btn sm danger" onclick="cardAction(\'disable\',\'' + r.code + '\')">禁用</button>';
    else ops += '<button class="btn sm" onclick="cardAction(\'enable\',\'' + r.code + '\')">启用</button>';
    if (st === 'active') ops += '<button class="btn sm" onclick="cardAction(\'unbind\',\'' + r.code + '\')">解绑</button>';
    ops += '<button class="btn sm" onclick="openRenew(\'' + r.code + '\',' + (r.days || 30) + ')">延期</button>';
    ops += '<button class="btn sm danger" onclick="cardAction(\'delete\',\'' + r.code + '\')">删除</button>';
    const onlineStr = (r.online || 0) > 0
      ? '<span class="badge active">' + r.online + '/' + r.maxWindows + ' 在线</span>'
      : '<span style="color:var(--text-muted)">0</span>';
    return '<tr><td class="mono">' + esc(r.code) + '</td><td><span class="badge ' + STATUS_CLS[st] + '">' + STATUS_TEXT[st] + '</span></td>' +
      '<td>' + esc(r.project || '-') + '</td>' +
      '<td>' + r.days + '</td><td>' + r.maxWindows + ' 窗</td>' +
      '<td>' + onlineStr + '</td>' +
      '<td>' + (r.activateAt ? fmtTime(r.activateAt) : '-') + '</td>' +
      '<td>' + (r.expireAt ? fmtTime(r.expireAt) : '-') + '</td>' +
      '<td>' + esc(r.owner || '-') + '</td><td>' + esc(r.remark || '') + '</td><td>' + ops + '</td></tr>';
  }).join('');
  h += '<div class="table-wrap"><table><thead><tr><th>卡号</th><th>状态</th><th>项目</th><th>时长（天）</th><th>窗口</th><th>在线</th><th>激活时间</th><th>到期时间</th><th>归属</th><th>备注</th><th>操作</th></tr></thead>' +
    '<tbody>' + (rows || '<tr><td colspan="11" style="text-align:center;color:var(--text-muted);padding:24px">暂无数据</td></tr>') + '</tbody></table></div>';
  h += '<div class="pager">' + pagerHtml(app.cardTotal, q.page, q.page_size, 'cards') + '</div></div>';
  return h;
}

function viewWindows() {
  let h = pageHead('在线统计', '每张卡每个窗口的实时在线明细', '<button class="btn ghost" onclick="loadWindows()">刷新</button>');
  if (!app.windows.length) return h + '<div class="card" style="text-align:center;color:var(--text-muted)">暂无在线设备</div>';
  h += app.windows.map(item => {
    let wins = item.windows.map(w => '<div class="win-row" style="display:flex;align-items:center;gap:14px;padding:10px 4px;border-bottom:1px solid var(--border)">' +
      '<span class="mono">' + esc(w.windowID) + '</span><span style="color:var(--text-muted);font-family:Consolas,monospace;font-size:12px">FP: ' + esc(w.fp) + '</span>' +
      '<span style="color:var(--text-muted);font-size:12px">最后心跳：' + fmtTime(w.lastSeen) + '</span><span class="badge active">在线</span></div>').join('');
    if (!wins) wins = '<div style="color:var(--text-muted);font-size:13px;padding:8px 0">暂无在线窗口</div>';
    return '<div class="card"><div class="card-title"><span class="bar"></span>' + esc(item.code) +
      '<span class="badge active" style="margin-left:10px">' + item.online + '/' + item.maxWindows + ' 在线</span>' +
      '<span style="margin-left:10px;color:var(--text-muted);font-size:12px">归属：' + esc(item.owner || '-') + '</span></div>' + wins + '</div>';
  }).join('');
  return h;
}

function viewLogs() {
  const q = app.logQuery;
  const actions = ['登录', '登出', '发卡', '禁用', '启用', '解绑', '延期', '删除卡密', '充额度', '创建代理', '创建项目', '重命名项目', '删除项目'];
  let h = pageHead('操作日志', '所有管理操作留痕可查');
  h += '<div class="card"><div class="toolbar">';
  h += '<input class="input" style="width:180px" placeholder="按操作者筛选" value="' + esc(q.actor) + '" oninput="app.logQuery.actor=this.value" onkeydown="if(event.key===\'Enter\'){app.logQuery.page=1;loadLogs()}">';
  h += '<select class="select" style="width:150px" onchange="app.logQuery.action=this.value;app.logQuery.page=1;loadLogs()"><option value="">全部动作</option>' +
    actions.map(a => '<option value="' + a + '"' + (q.action === a ? ' selected' : '') + '>' + a + '</option>').join('') + '</select>';
  h += '<button class="btn ghost" onclick="app.logQuery.page=1;loadLogs()">查询</button>';
  h += '<div class="spacer"></div><span class="total">共 ' + app.logTotal + ' 条</span></div>';
  const rows = app.logs.map(r => '<tr><td style="white-space:nowrap">' + fmtTime(r.time) + '</td><td>' + esc(r.actor) + '</td><td>' + esc(r.action) + '</td><td>' + esc(r.detail || '') + '</td></tr>').join('');
  h += '<div class="table-wrap"><table><thead><tr><th>时间</th><th>操作者</th><th>动作</th><th>详情</th></tr></thead>' +
    '<tbody>' + (rows || '<tr><td colspan="4" style="text-align:center;color:var(--text-muted);padding:24px">暂无数据</td></tr>') + '</tbody></table></div>';
  h += '<div class="pager">' + pagerHtml(app.logTotal, q.page, q.page_size, 'logs') + '</div></div>';
  return h;
}

function viewAgents() {
  let h = pageHead('代理管理', '创建下级代理、充额度、查看已发卡数', '<button class="btn" onclick="openCreateAgent()">创建代理</button>');
  const rows = app.agents.map(r => '<tr><td>' + esc(r.username) + '</td><td><b style="color:var(--primary)">' + r.quota + '</b> 张</td>' +
    '<td>' + r.issued_count + ' 张</td><td>' + esc(r.parent || '-') + '</td>' +
    '<td style="white-space:nowrap">' + fmtTime(r.createdAt) + '</td>' +
    '<td><button class="btn sm" onclick="openRecharge(\'' + esc(r.username) + '\',' + r.quota + ')">充额度</button></td></tr>').join('');
  h += '<div class="card"><div class="table-wrap"><table><thead><tr><th>用户名</th><th>剩余额度</th><th>已发卡数</th><th>上级</th><th>创建时间</th><th>操作</th></tr></thead>' +
    '<tbody>' + (rows || '<tr><td colspan="6" style="text-align:center;color:var(--text-muted);padding:24px">暂无代理</td></tr>') + '</tbody></table></div></div>';
  return h;
}

function viewProjects() {
  let h = pageHead('项目管理', '项目名称即分类，发卡时必须选择项目', '<button class="btn" onclick="openCreateProject()">新建项目</button>');
  const rows = app.projects.map(p =>
    '<tr><td><b>' + esc(p.name) + '</b></td><td>' + p.card_count + ' 张</td>' +
    '<td style="white-space:nowrap">' + fmtTime(p.createdAt) + '</td>' +
    '<td><button class="btn sm" onclick="openRenameProject(' + p.id + ')">改名</button>' +
    '<button class="btn sm danger" onclick="delProject(' + p.id + ')">删除</button></td></tr>').join('');
  h += '<div class="card"><div class="table-wrap"><table><thead><tr><th>项目名称</th><th>卡密数</th><th>创建时间</th><th>操作</th></tr></thead>' +
    '<tbody>' + (rows || '<tr><td colspan="4" style="text-align:center;color:var(--text-muted);padding:24px">暂无项目，先新建一个</td></tr>') + '</tbody></table></div></div>';
  return h;
}

function viewHotUpdate() {
  const s = app.hotupdate || {};
  let h = pageHead('热更新', '查看当前程序与版本号，上传新版后设备端启动时自动更新');
  // 当前状态卡：版本号 + 程序文件，标题栏右侧放"上传新的"
  const fileState = s.appExists
    ? '<span class="badge active">已上传</span><span style="margin-left:10px;color:var(--text-muted);font-size:13px">app · ' + fmtSize(s.appSize) + ' · ' + esc(s.appMtime || '') + '</span>'
    : '<span class="badge disabled">未上传</span><span style="margin-left:10px;color:var(--text-muted);font-size:13px">设备端拉取不到新程序</span>';
  h += '<div class="card"><div class="card-title"><span class="bar"></span>当前状态' +
    '<span style="margin-left:auto;display:flex;gap:8px"><button class="btn ghost" onclick="loadHotUpdate()">刷新</button><button class="btn" onclick="openUploadHotUpdate()">上传新的</button></span></div>' +
    '<div style="display:flex;gap:40px;flex-wrap:wrap;padding:6px 2px">' +
    '<div class="stat"><div class="num" style="font-size:28px">' + esc(s.version || '未设置') + '</div><div class="lab">当前版本号</div></div>' +
    '<div class="stat"><div class="num" style="font-size:20px;line-height:32px">' + fileState + '</div><div class="lab">程序文件（app）</div></div>' +
    '</div></div>';
  // 历史版本卡：上传/回滚前自动备份，最多保留 10 条
  const hist = app.hotupdateHistory || [];
  h += '<div class="card"><div class="card-title"><span class="bar"></span>历史版本（最多 10 条）' +
    (hist.length ? '<span style="margin-left:auto;font-size:13px;color:var(--text-muted)">每次上传或回滚前自动备份当前版本</span>' : '') + '</div>';
  if (!hist.length) {
    h += '<div style="padding:12px 2px;color:var(--text-muted);font-size:13px">暂无历史记录，上传新版时自动备份，可随时回滚</div>';
  } else {
    h += '<div class="table-wrap"><table><thead><tr><th>版本号</th><th>大小</th><th>备份时间</th><th style="width:140px">操作</th></tr></thead><tbody>' +
      hist.map(it => '<tr><td>' + esc(it.version || '未知') + '</td><td>' + fmtSize(it.size) + '</td><td style="white-space:nowrap">' + esc(it.time || '') + '</td>' +
        '<td><button class="btn sm" onclick="rollbackHotUpdate(\'' + it.id + '\',\'' + esc(it.version || '') + '\')">回滚</button>' +
        '<button class="btn sm danger" onclick="delHotUpdateHistory(\'' + it.id + '\',\'' + esc(it.version || '') + '\')">删除</button></td></tr>').join('') +
      '</tbody></table></div>';
  }
  h += '</div>';
  return h;
}
function openUploadHotUpdate() {
  const s = app.hotupdate || {};
  openModal(
    '<h3>上传新版程序</h3>' +
    '<div class="tip">选择编译好的设备端程序（Go 编译产物 app），上传后将覆盖当前版本，设备端下次启动自动更新。版本号已自动比当前多一点，可直接使用或修改。</div>' +
    '<div class="row"><label>选择文件</label><input type="file" id="huFile" class="input" style="padding:8px" onchange="document.getElementById(\'huFileName\').textContent=this.files.length?this.files[0].name:\'\'">' +
    '<span id="huFileName" style="color:var(--text-muted);font-size:13px;margin-left:10px"></span></div>' +
    '<div class="row"><label>版本号</label><input class="input" id="huVer" placeholder="如：1.1.1" value="' + esc(s.nextVersion || s.version || '') + '" maxlength="32"></div>' +
    '<div class="actions"><button class="btn ghost" onclick="closeModal()">取消</button><button class="btn" id="huUploadBtn" onclick="uploadHotUpdate()">上传并覆盖</button></div>'
  );
}
function fmtSize(n) {
  if (!n) return '0 B';
  const mb = 1024 * 1024;
  if (n >= mb) return (n / mb).toFixed(1) + ' MB';
  return n + ' B';
}
async function loadHotUpdate() {
  const [s, h] = await Promise.all([
    api('/api/admin/hotupdate/status'),
    api('/api/admin/hotupdate/history')
  ]);
  if (s.code === 0) app.hotupdate = s;
  if (h.code === 0) app.hotupdateHistory = h.list || [];
  if (app.view === 'hotupdate') render();
}
async function uploadHotUpdate() {
  const input = document.getElementById('huFile');
  const btn = document.getElementById('huUploadBtn');
  if (!input.files.length) { toast('请先选择文件', false); return; }
  const ver = document.getElementById('huVer').value.trim();
  if (!ver) { toast('请先填写版本号', false); return; }
  const fd = new FormData();
  fd.append('file', input.files[0]);
  fd.append('version', ver);
  btn.disabled = true; btn.textContent = '上传中...';
  try {
    const resp = await fetch('/api/admin/hotupdate/upload', { method: 'POST', body: fd, credentials: 'include' });
    if (resp.status === 401) { location.href = '/login.html'; return; }
    const d = await resp.json();
    if (d.code === 0) { closeModal(); toast('上传成功：' + fmtSize(d.size) + '，版本 ' + d.version); loadHotUpdate(); }
    else toast(d.msg || '上传失败', false);
  } catch (e) { toast('上传失败：网络错误', false); }
  finally { btn.disabled = false; btn.textContent = '上传并覆盖'; }
}
async function rollbackHotUpdate(id, ver) {
  if (!confirm('确定回滚到版本 "' + ver + '" 吗？\n回滚后设备端下次启动会自动更新到该版本。')) return;
  const d = await api('/api/admin/hotupdate/rollback', { method: 'POST', body: JSON.stringify({ id }) });
  if (d.code === 0) { toast('已回滚到版本 ' + (d.version || '')); loadHotUpdate(); }
  else toast(d.msg || '回滚失败', false);
}
async function delHotUpdateHistory(id, ver) {
  if (!confirm('确定删除历史版本 "' + ver + '" 吗？\n删除后不可恢复，仅影响历史备份，不影响当前版本。')) return;
  const d = await api('/api/admin/hotupdate/history/delete', { method: 'POST', body: JSON.stringify({ id }) });
  if (d.code === 0) { toast('已删除历史版本 ' + (ver || '')); loadHotUpdate(); }
  else toast(d.msg || '删除失败', false);
}

/* ---------- 数据操作 ---------- */
async function refreshStats() {
  const d = await api('/api/admin/stats');
  if (d.code === 0) { app.stats = d; if (app.view === 'dashboard') render(); }
}
async function doGenerate() {
  if (!app.gen.project) { toast('请先选择项目', false); return; }
  app.genLoading = true; render();
  try {
    const d = await api('/api/admin/generate', { method: 'POST', body: JSON.stringify(app.gen) });
    if (d.code !== 0) { toast(d.msg || '生成失败', false); return; }
    app.genResult = d;
    toast('成功生成 ' + d.count + ' 张卡密');
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
  const d = await api('/api/admin/cards?' + q);
  if (d.code === 0) { app.cards = d.list; app.cardTotal = d.total; }
  if (app.view === 'cards') render();
}
async function cardAction(action, code) {
  const map = {
    disable: { msg: '确定禁用卡 ' + code + ' 吗？禁用后立即失效', url: '/api/admin/cards/disable' },
    enable: { msg: '确定启用卡 ' + code + ' 吗？', url: '/api/admin/cards/enable' },
    unbind: { msg: '确定解绑卡 ' + code + ' 吗？解绑后客户需重新激活', url: '/api/admin/cards/unbind' },
    delete: { msg: '确定删除卡 ' + code + ' 吗？删除后永久消失、不可恢复！', url: '/api/admin/cards/delete' },
  };
  const ok = await confirmModal('确认操作', map[action].msg);
  if (!ok) return;
  const d = await api(map[action].url, { method: 'POST', body: JSON.stringify({ code }) });
  if (d.code === 0) { toast('操作成功'); loadCards(); }
  else toast(d.msg || '操作失败', false);
}
function openRenew(code, days) {
  app.renew = { code, days: days || 30 };
  openModal('<h3>手动延期</h3><div class="tip">卡号：<b>' + esc(code) + '</b>，为这张卡增加到期天数。</div>' +
    '<div class="row"><label>延期天数（1-3650）</label><input class="input" type="number" min="1" max="3650" value="' + app.renew.days + '" oninput="app.renew.days=clampInt(this.value,1,3650,30)"></div>' +
    '<div class="actions"><button class="btn ghost" onclick="closeModal()">取消</button><button class="btn" onclick="doRenew()">确认延期</button></div>');
}
async function doRenew() {
  const d = await api('/api/admin/cards/renew', { method: 'POST', body: JSON.stringify({ code: app.renew.code, days: app.renew.days }) });
  if (d.code === 0) { toast('已延期，新到期：' + fmtTime(d.expire)); closeModal(); loadCards(); }
  else toast(d.msg || '延期失败', false);
}
async function loadWindows() {
  const d = await api('/api/admin/windows');
  if (d.code === 0) { app.windows = d.list; if (app.view === 'windows') render(); }
}
async function loadLogs() {
  const q = new URLSearchParams();
  if (app.logQuery.actor) q.set('actor', app.logQuery.actor);
  if (app.logQuery.action) q.set('action', app.logQuery.action);
  q.set('page', app.logQuery.page);
  q.set('page_size', app.logQuery.page_size);
  const d = await api('/api/admin/logs?' + q);
  if (d.code === 0) { app.logs = d.list; app.logTotal = d.total; }
  if (app.view === 'logs') render();
}
async function loadAgents() {
  const d = await api('/api/admin/agents');
  if (d.code === 0) { app.agents = d.list; if (app.view === 'agents') render(); }
}
function openCreateAgent() {
  app.createAgent = { username: '', password: '', quota: 100 };
  openModal('<h3>创建代理</h3>' +
    '<div class="row"><label>用户名</label><input class="input" placeholder="代理登录名" value="" oninput="app.createAgent.username=this.value"></div>' +
    '<div class="row"><label>密码</label><input class="input" type="password" placeholder="登录密码" oninput="app.createAgent.password=this.value"></div>' +
    '<div class="row"><label>初始额度</label><input class="input" type="number" min="0" max="100000" value="100" oninput="app.createAgent.quota=clampInt(this.value,0,100000,100)"></div>' +
    '<div class="actions"><button class="btn ghost" onclick="closeModal()">取消</button><button class="btn" onclick="doCreateAgent()">创建</button></div>');
}
async function doCreateAgent() {
  const d = await api('/api/admin/agents/create', { method: 'POST', body: JSON.stringify(app.createAgent) });
  if (d.code === 0) { toast('代理创建成功'); closeModal(); loadAgents(); }
  else toast(d.msg || '创建失败', false);
}
function openRecharge(username, quota) {
  app.recharge = { username, quota, amount: 100 };
  openModal('<h3>充额度</h3><div class="tip">代理：<b>' + esc(username) + '</b>，当前剩余 <b>' + quota + '</b> 张。</div>' +
    '<div class="row"><label>充值数量</label><input class="input" type="number" min="1" max="100000" value="100" oninput="app.recharge.amount=clampInt(this.value,1,100000,100)"></div>' +
    '<div class="actions"><button class="btn ghost" onclick="closeModal()">取消</button><button class="btn" onclick="doRecharge()">确认充值</button></div>');
}
async function doRecharge() {
  const d = await api('/api/admin/agents/recharge', { method: 'POST', body: JSON.stringify({ username: app.recharge.username, quota: app.recharge.amount }) });
  if (d.code === 0) { toast('充值成功，剩余 ' + d.quota + ' 张'); closeModal(); loadAgents(); }
  else toast(d.msg || '充值失败', false);
}
async function loadProjects() {
  const d = await api('/api/admin/projects');
  if (d.code === 0) { app.projects = d.list; if (app.view === 'projects') render(); }
}
// 点开项目下拉框时实时请求最新项目列表，并只重建该下拉框选项（保留当前选中）
async function refreshGenProjects(sel) {
  try {
    const d = await api('/api/admin/projects');
    if (d.code !== 0) return;
    app.projects = d.list || [];
    if (sel) {
      const cur = sel.value;
      sel.innerHTML = '<option value="">请选择项目</option>' + app.projects.map(p => '<option value="' + esc(p.name) + '"' + (cur === p.name ? ' selected' : '') + '>' + esc(p.name) + '</option>').join('');
      sel.value = cur;
    }
  } catch (e) { /* 请求失败保留旧数据，不打断操作 */ }
}
function openCreateProject() {
  app.curProj = { id: 0, name: '' };
  openModal('<h3>新建项目</h3><div class="tip">项目名称即分类，发卡时必须选择。</div>' +
    '<div class="row"><label>项目名称</label><input class="input" placeholder="如：月卡 / 周卡 / 特价" oninput="app.curProj.name=this.value"></div>' +
    '<div class="actions"><button class="btn ghost" onclick="closeModal()">取消</button><button class="btn" onclick="doCreateProject()">创建</button></div>');
}
async function doCreateProject() {
  const d = await api('/api/admin/projects/create', { method: 'POST', body: JSON.stringify({ name: app.curProj.name }) });
  if (d.code === 0) { toast('项目创建成功'); closeModal(); loadProjects(); }
  else toast(d.msg || '创建失败', false);
}
function openRenameProject(id) {
  const p = app.projects.find(x => x.id === id);
  if (!p) return;
  app.curProj = { id, name: p.name };
  openModal('<h3>重命名项目</h3><div class="tip">改名后该项目下所有卡密的分类同步更新。</div>' +
    '<div class="row"><label>项目名称</label><input class="input" value="' + esc(p.name) + '" oninput="app.curProj.name=this.value"></div>' +
    '<div class="actions"><button class="btn ghost" onclick="closeModal()">取消</button><button class="btn" onclick="doRenameProject()">保存</button></div>');
}
async function doRenameProject() {
  const d = await api('/api/admin/projects/update', { method: 'POST', body: JSON.stringify({ id: app.curProj.id, name: app.curProj.name }) });
  if (d.code === 0) { toast('项目已重命名'); closeModal(); loadProjects(); }
  else toast(d.msg || '重命名失败', false);
}
async function delProject(id) {
  const p = app.projects.find(x => x.id === id);
  if (!p) return;
  const ok = await confirmModal('删除项目', '确定删除项目「' + p.name + '」吗？若项目下已有卡密将无法删除。');
  if (!ok) return;
  const d = await api('/api/admin/projects/delete', { method: 'POST', body: JSON.stringify({ id }) });
  if (d.code === 0) { toast('项目已删除'); loadProjects(); }
  else toast(d.msg || '删除失败', false);
}
async function logout() {
  await api('/api/auth/logout', { method: 'POST' });
  location.href = '/login.html';
}

/* ---------- 轮询控制（仅仪表盘/在线统计页轮询，离开即停） ---------- */
let pollTimer = null;
function startPolling() {
  stopPolling();
  pollTimer = setInterval(() => {
    if (app.view === 'windows') loadWindows();
    else if (app.view === 'dashboard') refreshStats();
  }, 15000);
}
function stopPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
}

/* ---------- 初始化 ---------- */
(async function init() {
  applyTheme(localStorage.getItem('fy_theme') || 'dark');
  const me = await api('/api/auth/me');
  app.me = me;
  document.getElementById('uname').textContent = me.username + (me.role === 'admin' ? '（管理员）' : '');
  renderMenu();
  render();
  // 初始视图为仪表盘：只加载当前页数据，其他视图切换时由 VIEW_LOADERS 按需加载
  refreshStats();
  startPolling();
})();
