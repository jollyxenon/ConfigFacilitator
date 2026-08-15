/* cfgfc web —— 调用真实后端（/api/snapshot、/api/command、/api/preview）。
   导航模型：Project 段只组织资源（Column / Setting）；Mode 段只决定什么生效。
   (Current) 是运行时状态节点，来自 snapshot 中每个 Project 的 current（含
   relation / mappings）；后端推导 relation，前端不提交 relation/mappings。
   规划（线路预览、执行 DIFF、阻塞提示）一律走后端 /api/preview；
   revision 冲突（409 revision_conflict）时提示重新加载并保留草稿。 */
"use strict";

const $  = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => [...r.querySelectorAll(s)];
const esc = s => String(s).replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const clone = v => JSON.parse(JSON.stringify(v));
const P = name => S.snap && S.snap.projects[name];

const SCRATCH = "__scratch__";

/* ================= 运行状态 ================= */
const S = {
  sel:    { seg: "mode", project: null, mode: null },   /* mode: null = (Current) */
  ptab:   "targets",
  treeAll: false,  /* true = 侧栏只点了 COLUMN 分组头（资源树/编辑器列表展示全部 Column）；false = 侧栏点了具体 Column（只展示它） */
  res:    { column: null, setting: null },
  ed:     { column: null, setting: null, path: null },  /* path: 目录 Setting 内打开的相对文件 */
  draft:  {},        /* project -> (mode|"__scratch__") -> { col: {settings[],strategy} } */
  preview: null,     /* { project, mappings[], errors[], forceTargets[] } 来自 /api/preview */
  content: {},       /* project -> "col|set" -> { entries, content, encoding, bytes, loaded } */
  hover:  null,
  navOpen: {},        /* 侧栏折叠状态：{ [project]: { open, cols: {[col]:bool}, modes: {[mode]:bool} } } */
  snap:   null,      /* 最近一次完整 snapshot（含 revision） */
  error:  null       /* 加载错误 */
};
const cur = () => P(S.sel.project);
const dKey = () => S.sel.mode === null ? SCRATCH : S.sel.mode;

const currentMappingsOf = name => (P(name) && P(name).current && P(name).current.mappings) || [];
/* 目标路径实况（来自 snapshot.targets；未知一律按 free 处理） */
const tState = path => (S.snap && S.snap.targets && S.snap.targets[path]) || "free";
const TARGET_BADGE = {
  ok:       ["b-ok",   "已链接"],
  drift:    ["b-warn", "目标漂移"],
  occupied: ["b-bad",  "目标被占用"],
  free:     ["b-idle", "空位"],
  unmanaged:["b-idle", "未管理"]
};
function badge(cls, text) { return '<span class="badge ' + cls + '">' + esc(text) + "</span>"; }
function targetBadge(path) { const [c, t] = TARGET_BADGE[tState(path)] || ["b-idle", "未知"]; return badge(c, t); }
const lineClass = path => ({ ok: "s-ok", drift: "s-warn", occupied: "s-bad", free: "" }[tState(path)] || "");

/* 同一来源的多个目标取最差实况（badge / 线路左边框） */
const STATE_RANK = { "b-bad": 3, "b-warn": 2, "b-ok": 1, "b-idle": 0 };
function worstTargetBadge(paths) {
  let cls = "b-idle", text = "未知", rank = -1;
  for (const p of paths) {
    const [c, t] = TARGET_BADGE[tState(p)] || ["b-idle", "未知"];
    if ((STATE_RANK[c] || 0) > rank) { rank = STATE_RANK[c]; cls = c; text = t; }
  }
  return badge(cls, text);
}
function worstLineClass(paths) {
  let cls = "", rank = -1;
  const lineRank = p => ({ "s-bad": 3, "s-warn": 2, "s-ok": 1, "": 0 }[lineClass(p)] || 0);
  for (const p of paths) { const r = lineRank(p); if (r > rank) { rank = r; cls = lineClass(p); } }
  return cls;
}
const sourceSegsMatch = (source, colName, setName) => {
  const segs = String(source).split("/");
  return segs.length >= 2 && segs[segs.length - 2] === colName && segs[segs.length - 1] === setName;
};

/* ================= API 层 ================= */
async function fetchJSON(url, options) {
  let response;
  try {
    response = await fetch(url, options);
  } catch (e) {
    return { status: 0, ok: false, data: null, error: { code: "network_error", message: "无法连接本地服务：" + e.message } };
  }
  let envelope = null;
  try {
    envelope = await response.json();
  } catch (e) {
    return { status: response.status, ok: false, data: null, error: { code: "bad_response", message: "服务返回了无法解析的响应" } };
  }
  return { status: response.status, ok: !!envelope.ok, data: envelope.data, error: envelope.error };
}
const apiSnapshot = () => fetchJSON("/api/snapshot", { method: "GET", headers: { "Accept": "application/json" } });
const apiCommand = payload => fetchJSON("/api/command", {
  method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload)
});
const apiPreview = payload => fetchJSON("/api/preview", {
  method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload)
});

/* ================= snapshot 生命周期 ================= */
function applySnapshot(snap) {
  S.snap = snap;
  S.preview = null;
  const names = Object.keys(snap.projects || {});
  if (!names.length) { S.sel.project = null; return; }
  if (!names.includes(S.sel.project)) S.sel.project = names[0];
  const p = P(S.sel.project);
  if (S.sel.mode !== null && !(p.modes || {})[S.sel.mode]) S.sel.mode = null;
  if (!(p.columns || {})[S.res.column])
    S.res = { column: Object.keys(p.columns || {})[0] || null, setting: null };
  else if (S.res.column && !(p.columns[S.res.column].settings || {})[S.res.setting])
    S.res.setting = null;
  if (!(p.columns || {})[S.ed.column]) S.ed = firstSettingOf(p);
}

async function loadSnapshot() {
  const res = await apiSnapshot();
  if (!res.ok) {
    S.error = (res.error && res.error.message) || "加载失败";
    showBootError(S.error);
    renderAll();
    return;
  }
  S.error = null;
  $("#bootError").hidden = true;
  applySnapshot(res.data);
  renderAll();
  schedulePreview();
}

/* ================= 规划：目标路径解析（仅供展示提示；权威规划走 /api/preview） ================= */
/* 与 internal/planner 的 effectiveTargetParts 一致：目标 = 展开(dir) + "/" + 名字；
   名字为空时兜底为 Setting 名；目录为空才是错误。 */
function settingTargets(projectName, colName, setName) {
  const col = P(projectName) && P(projectName).columns[colName];
  const set = col && col.settings[setName];
  if (!col || !set) return { error: 'setting "' + setName + '" 无法解析', targets: [] };
  const n = col.targetNumber;
  const targets = [];
  for (let i = 0; i < n; i++) {
    const dir  = (set.targetDir[i]  || "") || col.defaultTargetDir[i]  || "";
    const name = (set.targetName[i] || "") || col.defaultTargetName[i] || setName;
    if (!dir)  return { error: 'setting "' + setName + '" 第 ' + i + " 个目标目录为空", targets: [] };
    targets.push({ path: dir + "/" + name, inherited: !set.targetDir[i] && !set.targetName[i], index: i });
  }
  return { error: null, targets };
}

/* ================= 运行时状态判定（基于 snapshot 的 current/relation） ================= */
function liveState(projectName) {
  const p = P(projectName);
  if (!p) return { kind: "none", text: "—", cls: "off", badge: "b-idle", recorded: [] };
  if (!p.available) {
    return { kind: "unavailable", text: "不可用", cls: "drift", badge: "b-warn", recorded: [],
             error: (p.error && p.error.message) || "当前状态不可用" };
  }
  const c = p.current;
  if (!c) return { kind: "none", text: "None", cls: "off", badge: "b-idle", recorded: [] };
  const recorded = c.mappings || [];
  const rel = c.relation;
  if (rel && (rel.kind === "following" || rel.kind === "detached") && rel.originMode) {
    const temporary = rel.kind === "detached";
    if (!(p.modes || {})[rel.originMode])
      return { kind: "none", text: "None", cls: "off", badge: "b-idle", recorded };
    return { kind: "mode", mode: rel.originMode, temporary, forkedFrom: rel.originMode, recorded,
             text: temporary ? "临时 Mode（从 " + rel.originMode + " 分叉）" : "跟随 mode " + rel.originMode,
             cls: "", badge: "b-ok" };
  }
  const n = Object.keys(c.columns || {}).length;
  return { kind: "independent", recorded,
           text: n ? "独立选择（" + n + "C）" : "None", cls: "", badge: "b-ok" };
}

/* Setting 在资源视图里的状态（用真实已记录映射 + snapshot.targets） */
function settingState(projectName, colName, setName) {
  const set = P(projectName) && P(projectName).columns[colName] && P(projectName).columns[colName].settings[setName];
  if (!set) return ["b-idle", "未管理"];
  const mine = currentMappingsOf(projectName).filter(m => sourceSegsMatch(m.source, colName, setName));
  if (!mine.length) return ["b-idle", "未管理"];
  const states = mine.map(m => tState(m.target));
  if (states.every(s => s === "ok")) return ["b-ok", "已链接"];
  if (states.some(s => s === "occupied")) return ["b-bad", "目标被占用"];
  if (states.some(s => s === "ok")) return ["b-warn", "部分链接"];
  return ["b-idle", "未管理"];
}

/* ================= 草稿 ================= */
function baseDecls() {
  const p = cur();
  if (!p) return {};
  if (S.sel.mode !== null) return clone((p.modes[S.sel.mode] || {}).columns || {});
  return clone((p.current && p.current.columns) || {});
}
function decls() {
  if (!S.sel.project) return {};
  S.draft[S.sel.project] = S.draft[S.sel.project] || {};
  if (!(dKey() in S.draft[S.sel.project])) S.draft[S.sel.project][dKey()] = baseDecls();
  return S.draft[S.sel.project][dKey()];
}
const dirty = () => JSON.stringify(decls()) !== JSON.stringify(baseDecls());
function resetDraft() { if (S.draft[S.sel.project]) delete S.draft[S.sel.project][dKey()]; }

/* ================= 状态栏 ================= */
function renderStatusbar() {
  if (!S.snap) {
    $("#sbRoot").textContent = ""; $("#sbScope").textContent = ""; $("#sbLive").textContent = "";
    $("#sbMappings").textContent = "0"; $("#sbTx").hidden = true;
    return;
  }
  $("#sbRoot").textContent = S.snap.root;
  $("#sbScope").textContent = S.sel.seg === "project"
    ? "Project " + S.sel.project
    : "Mode " + S.sel.project + " / " + (S.sel.mode === null ? "（Current）" : S.sel.mode);
  const live = liveState(S.sel.project);
  const el = $("#sbLive");
  el.textContent = live.text;
  el.style.color = live.badge === "b-ok" ? "var(--ok)" : live.badge === "b-warn" ? "var(--warn)" : "var(--ink-muted)";
  $("#sbMappings").textContent = live.recorded.length;
  const tx = $("#sbTx");
  const pending = (S.snap.transactions || []).length;
  tx.hidden = !pending;
  if (pending) $("span", tx).textContent = pending + " 个未完成事务（下次变更前自动恢复）";
}

/* ================= 侧栏 ================= */
const navLabel = (disp, id) => (!disp || disp === id) ? esc(id) : esc(disp) + ' <span class="nav-sub">[' + esc(id) + "]</span>";

/* 展开/收起箭头：expandable 节点显示 ▸/▾（整行可点切换展开），叶子只占位保持列对齐 */
const caret = (expanded, expandable) =>
  '<span class="caret' + (expanded ? " open" : "") + '" aria-expanded="' + (expandable ? expanded : false) + '">' +
  (expandable ? (expanded ? "▾" : "▸") : "") + "</span>";

function renderNav() {
  const projects = S.snap ? Object.keys(S.snap.projects) : [];
  /* 每个 Project 一棵可折叠树（默认缩起）：Project 整行展开 → Column/Mode 分组头
     可展开 → 条目（叶子，点击导航）。字体按上下级从重到轻 */
  $("#navProjects").innerHTML = projects.map(name => {
    const p = P(name);
    const ns = S.navOpen[name] || (S.navOpen[name] = { open: false, cols: false, modes: false });
    const nCol = Object.keys(p.columns || {}).length;
    const nSet = Object.values(p.columns || {}).reduce((a, c) => a + Object.keys(c.settings || {}).length, 0);
    const on = S.sel.seg === "project" && S.sel.project === name;
    const live = liveState(name);
    let html = '<button class="nav-item lv1" data-nav="project" data-project="' + esc(name) + '" aria-current="' + on + '">' +
      '<span class="bar"></span>' + caret(ns.open, true) +
      '<span class="nav-name">' + navLabel(p.displayName, name) + "</span>" +
      '<span class="nav-sub">' + nCol + "C · " + nSet + "S</span></button>";
    html += "<div class='nav-subtree'" + (ns.open ? "" : " hidden") + ">";
    /* Column 分组：组头整行可展开；点击还把主区切到「全部 Column」资源视图 */
    const allOn = S.sel.seg === "project" && S.sel.project === name && S.treeAll;
    html += '<button class="nav-group" data-nav-group="cols|' + esc(name) + '" aria-expanded="' + ns.cols + '"' +
      (allOn ? ' aria-current="true"' : "") + '>' +
      '<span class="gname">Column</span><span class="gid">' + nCol + "</span>" +
      '<span class="caret">' + (ns.cols ? "▾" : "▸") + "</span></button>";
    html += "<div class='nav-subtree'" + (ns.cols ? "" : " hidden") + ">" +
      Object.keys(p.columns || {}).map(colName => {
        const colOn = S.sel.seg === "project" && S.sel.project === name && S.res.column === colName;
        const nS = Object.keys(p.columns[colName].settings || {}).length;
        return '<button class="nav-item lv2" data-nav="column" data-project="' + esc(name) + '"' +
          ' data-column="' + esc(colName) + '" aria-current="' + colOn + '">' +
          '<span class="bar"></span><span class="caret"></span>' +
          '<span class="nav-name">' + navLabel(p.columns[colName].displayName, colName) + "</span>" +
          '<span class="nav-sub">' + nS + "S</span></button>";
      }).join("") + "</div>";
    /* Mode 分组：组头整行可展开 */
    html += '<button class="nav-group" data-nav-group="modes|' + esc(name) + '" aria-expanded="' + ns.modes + '">' +
      '<span class="gname">Mode</span><span class="caret">' + (ns.modes ? "▾" : "▸") + "</span></button>";
    html += "<div class='nav-subtree'" + (ns.modes ? "" : " hidden") + ">";
    const curOn = S.sel.seg === "mode" && S.sel.project === name && S.sel.mode === null;
    html += '<button class="nav-item lv2 current" data-nav="mode" data-project="' + esc(name) + '" aria-current="' + curOn + '">' +
      '<span class="bar"></span><span class="caret"></span>' +
      '<span style="display:flex;align-items:center;gap:7px;min-width:0">' +
      '<span class="live ' + live.cls + '"></span>' +
      '<span class="nav-name">（Current）</span></span>' +
      '<span class="nav-sub">' + esc(live.text) + "</span></button>";
    for (const m of Object.keys(p.modes || {})) {
      if (m === SCRATCH) continue;
      const on2 = S.sel.seg === "mode" && S.sel.project === name && S.sel.mode === m;
      const isLive = live.kind === "mode" && live.mode === m && !live.temporary;
      const n = Object.keys(p.modes[m].columns || {}).length;
      html += '<button class="nav-item lv2" data-nav="mode" data-project="' + esc(name) + '"' +
        ' data-mode="' + esc(m) + '" aria-current="' + on2 + '">' +
        '<span class="bar"></span><span class="caret"></span>' +
        '<span class="nav-name">' + navLabel(p.modes[m].displayName, m) + "</span>" +
        (isLive ? badge("b-ok", "生效中") : '<span class="nav-sub">' + n + "C</span>") + "</button>";
    }
    html += "</div></div>";
    return html;
  }).join("");
}

function renderCrumb() {
  const c = $("#crumb");
  if (S.sel.seg === "project")
    c.innerHTML = "<span>Project</span><span class='sep'>/</span><b>" + esc(S.sel.project) +
      "</b><span class='sep'>/</span><span class='cn'>资源组织</span>";
  else
    c.innerHTML = "<span>Mode</span><span class='sep'>/</span><b>" + esc(S.sel.project) +
      "</b><span class='sep'>/</span><b>" + (S.sel.mode === null ? "（Current）" : esc(S.sel.mode)) +
      "</b><span class='sep'>/</span><span class='cn'>" +
      (S.sel.mode === null ? "运行时状态" : "Column 选择") + "</span>";
  $("#scope-project").hidden = S.sel.seg !== "project";
  $("#scope-mode").hidden = S.sel.seg !== "mode";
}

/* ================= Project 作用域 ================= */
/* 单个 Column 的树块：Column 头行 + 其全部 Setting 行（全部视图与单选视图共用） */
function treeBlockFor(colName) {
  const p = cur();
  const col = p.columns[colName];
  const sets = Object.entries(col.settings || {}).map(([setName, set]) => {
    const [bc, bt] = settingState(S.sel.project, colName, setName);
    return '<button class="tree-set" data-column="' + esc(colName) + '" data-setting="' + esc(setName) + '"' +
      ' aria-current="' + (S.res.column === colName && S.res.setting === setName) + '">' +
      '<span class="glyph">' + (set.kind === "directory" ? "▤" : "▪") + "</span>" +
      '<span style="flex:1">' + esc(setName) + "</span>" + badge(bc, bt) + "</button>";
  }).join("");
  return '<button class="tree-col" data-column="' + esc(colName) + '" aria-current="' +
    (S.res.column === colName && S.res.setting === null) + '">' +
    '<span class="glyph">▾</span><span style="flex:1" class="cn">' + esc(col.displayName || colName) + "</span>" +
    '<span class="nav-sub">' + col.targetNumber + " target</span></button>" + sets;
}

function renderTree() {
  const p = cur();
  if (!p) { $("#tree").innerHTML = "<div class='empty'>从侧边栏选择一个 Column。</div>"; return; }
  /* 侧栏只点了 COLUMN 分组头 → 展示全部 Column 及其 Setting；侧栏点了具体 Column → 只展示它 */
  const colNames = S.treeAll
    ? Object.keys(p.columns || {})
    : (S.res.column && p.columns[S.res.column] ? [S.res.column] : []);
  if (!colNames.length) { $("#tree").innerHTML = "<div class='empty'>从侧边栏选择一个 Column。</div>"; return; }
  $("#tree").innerHTML = colNames.map(treeBlockFor).join("");
}

function renderResPanel() {
  const p = cur();
  const col = p && p.columns[S.res.column];
  if (!col) { $("#resPanel").innerHTML = '<div class="empty">选择一个 Column。</div>'; return; }

  if (S.res.setting === null) {
    const rows = (col.defaultTargetDir || []).map((dir, i) =>
      "<tr><td class='num'>" + i + "</td><td>" + esc(dir) + "</td><td>" +
      ((col.defaultTargetName || [])[i] ? esc(col.defaultTargetName[i])
        : "<span class='inherit'>空 → 每个 Setting 必须自己指定</span>") +
      "</td></tr>").join("");
    const refs = Object.entries(p.modes || {}).filter(([, m]) => (m.columns || {})[S.res.column]).map(([n]) => n);
    $("#resPanel").innerHTML =
      "<h2>COLUMN</h2><div class='title-lg'>" + esc(col.displayName || S.res.column) + "</div>" +
      "<dl class='kv'><dt>canonical</dt><dd>" + esc(S.res.column) + "</dd>" +
      "<dt>描述</dt><dd class='cn'>" + esc(col.description || "—") + "</dd>" +
      "<dt>Setting 数</dt><dd>" + Object.keys(col.settings || {}).length + "</dd>" +
      "<dt>被引用</dt><dd class='cn'>" + (refs.length ? "Mode " + refs.map(esc).join("、") : "无 Mode 引用") + "</dd></dl>" +
      "<h3>目标位置（Column 默认）</h3><table class='grid'><thead><tr><th>#</th><th>目标目录</th><th>目标名</th></tr></thead><tbody>" +
      rows + "</tbody></table>" +
      "<div class='btnrow' style='margin-top:12px'>" +
      "<button class='btn' data-index-edit='column'>编辑 Index</button>" +
      "<button class='btn' data-index-rename='column'>重命名</button>" +
      "<button class='btn btn-danger' data-danger='column'>delete column…</button></div>";
    return;
  }

  const setName = S.res.setting;
  const set = col.settings[setName];
  if (!set) { $("#resPanel").innerHTML = '<div class="empty">选择一个 Setting。</div>'; return; }
  const r = settingTargets(S.sel.project, S.res.column, setName);
  const mine = currentMappingsOf(S.sel.project).filter(m => sourceSegsMatch(m.source, S.res.column, setName));
  const rows = (col.defaultTargetDir || []).map((dir, i) => {
    const ovDir = (set.targetDir || [])[i] || "", ovName = (set.targetName || [])[i] || "";
    const t = r.targets[i];
    const liveBadge = mine.length
      ? targetBadge(mine[0].target)
      : (t ? targetBadge(t.path) : badge("b-bad", "错误"));
    return "<tr><td class='num'>" + i + "</td>" +
      "<td>" + esc(dir) + "<br /><span class='inherit'>" +
      ((col.defaultTargetName || [])[i] ? esc(col.defaultTargetName[i]) : "（空）") + "</span></td>" +
      "<td>" + (ovDir || ovName
        ? esc((ovDir || "·") + " / " + (ovName || "·"))
        : "<span class='inherit'>继承</span>") + "</td>" +
      "<td>" + (t ? "<span style='color:var(--signal)'>" + esc(t.path) + "</span>"
        : "<span style='color:var(--bad)'>无法解析</span>") + "</td>" +
      "<td>" + liveBadge + "</td></tr>";
  }).join("");
  const usedBy = Object.entries(p.modes || {}).filter(([, m]) => {
    const d = (m.columns || {})[S.res.column];
    return d && (d.strategy === "full" || (d.settings || []).includes(setName));
  }).map(([n]) => n);
  const [bc, bt] = settingState(S.sel.project, S.res.column, setName);
  $("#resPanel").innerHTML =
    "<h2>SETTING</h2><div class='title-lg'>" + esc(set.displayName || setName) + "</div>" +
    (r.error ? "<div class='diag'>目标无法解析：" + esc(r.error) + "</div>" : "") +
    "<dl class='kv'><dt>canonical</dt><dd>" + esc(setName) + "</dd>" +
    "<dt>Column</dt><dd>" + esc(S.res.column) + "</dd>" +
    "<dt>来源</dt><dd>" + esc(S.sel.project + "/Column/" + S.res.column + "/" + setName) + "</dd>" +
    "<dt>类型</dt><dd>" + esc(set.kind) + "</dd>" +
    "<dt>状态</dt><dd>" + badge(bc, bt) + "</dd>" +
    "<dt>被引用</dt><dd class='cn'>" + (usedBy.length ? "Mode " + usedBy.map(esc).join("、") : "无 Mode 引用") + "</dd></dl>" +
    "<h3>目标挂载点</h3><table class='grid'><thead><tr><th>#</th><th>Column 默认</th><th>Setting 覆盖</th><th>解析结果</th><th>实况</th></tr></thead><tbody>" +
    rows + "</tbody></table>" +
    "<div class='btnrow' style='margin-top:12px'>" +
    "<button class='btn' data-index-edit='setting'>编辑 Index</button>" +
    "<button class='btn' data-index-rename='setting'>重命名</button>" +
    "<button class='btn' data-goto-editor='1'>编辑内容</button>" +
    "<button class='btn btn-danger' data-danger='setting'>delete setting…</button></div>";
}

function renderEdList() {
  const p = cur();
  if (!p) { $("#edList").innerHTML = ""; return; }
  /* 与资源树同一作用域：全部 Column（侧栏只点了 COLUMN）或当前 Column（侧栏点了具体 Column） */
  const cols = S.treeAll
    ? Object.entries(p.columns || {})
    : (S.res.column && p.columns[S.res.column] ? [[S.res.column, p.columns[S.res.column]]] : []);
  $("#edList").innerHTML = cols.map(([colName, col]) =>
    "<div class='ed-group'>" + esc(colName) + "</div>" +
    Object.entries(col.settings || {}).map(([setName, set]) => {
      const info = S.content[S.sel.project] && S.content[S.sel.project][colName + "|" + setName];
      const bytes = info && info.bytes != null ? info.bytes + "B" : (set.kind === "directory" ? "dir" : "—");
      return "<button class='ed-file' data-ed-column='" + esc(colName) + "' data-ed-setting='" + esc(setName) + "'" +
        " aria-current='" + (S.ed.column === colName && S.ed.setting === setName) + "'>" +
        "<span class='glyph'>" + (set.kind === "directory" ? "▤" : "▪") + "</span>" +
        "<span style='flex:1'>" + esc(setName) + "</span>" +
        "<span class='num'>" + esc(bytes) + "</span></button>";
    }).join("")).join("");
}

function renderEditor() {
  const p = cur();
  const col = p && p.columns[S.ed.column];
  const set = col && col.settings[S.ed.setting];
  if (!set) {
    $("#edPath").textContent = ""; $("#edText").value = "";
    $("#edText").disabled = true; $("#edSave").disabled = true;
    $("#edDirty").hidden = true; $("#edHint").textContent = "";
    $("#edEntries").innerHTML = "";
    syncGutter();
    return;
  }
  const key = S.ed.column + "|" + S.ed.setting;
  const info = (S.content[S.sel.project] || {})[key] || {};
  $("#edPath").textContent = S.ed.column + "/" + S.ed.setting + (S.ed.path ? "/" + S.ed.path : "");
  renderEntries(set, info);
  const binary = info.encoding === "base64";
  const loaded = !!info.loaded;
  $("#edText").value = loaded ? (info.content || "") : "";
  $("#edText").disabled = !loaded || binary;
  $("#edSave").disabled = !loaded || binary;
  $("#edDirty").hidden = true;
  $("#edHint").textContent = binary
    ? "二进制文件（" + info.bytes + " 字节），只读显示，不可编辑。"
    : (loaded ? "" : (set.kind === "directory" ? "选择一个目录条目读取内容。" : "未加载。"));
  syncGutter();
}

function renderEntries(set, info) {
  const box = $("#edEntries");
  if (set.kind !== "directory") { box.innerHTML = ""; return; }
  const entries = info.entries || [];
  if (!entries.length) { box.innerHTML = "<div class='e-head'>（目录为空）</div>"; return; }
  const rows = entries.map(e =>
    "<div class='e-row'>" +
    "<span class='glyph'>" + (e.kind === "directory" ? "▤" : "▪") + "</span>" +
    (e.kind === "file"
      ? "<button class='e-open' data-content-open='" + esc(e.path) + "' aria-current='" + (S.ed.path === e.path) + "'>" + esc(e.path) + "</button>"
      : "<span class='e-path'>" + esc(e.path) + "</span>") +
    "<span class='e-size'>" + (e.kind === "file" ? e.size + "B" : "dir") + "</span>" +
    (e.kind === "file"
      ? "<span class='btnrow'><button class='btn btn-sm' data-content-move='" + esc(e.path) + "'>move</button>" +
        "<button class='btn btn-sm btn-danger' data-content-del='" + esc(e.path) + "'>delete</button></span>"
      : "") +
    "</div>").join("");
  box.innerHTML = "<div class='e-head'><span style='flex:1'>目录条目（" + entries.length + "）</span>" +
    "<button class='btn btn-sm' data-content-mkdir='1'>mkdir</button></div>" + rows;
}

function syncGutter() {
  const n = $("#edText").value.split("\n").length;
  $("#edGutter").textContent = Array.from({ length: n }, (_, i) => i + 1).join("\n");
}

/* ================= Mode 作用域 ================= */
const STRATEGIES = [
  ["cover",     "只用选中的 Setting，替换这个 Column 之前的映射"],
  ["increment", "在这个 Column 已记录的映射上叠加，同目标才替换"],
  ["full",      "这个 Column 的全部 Setting，不用手选"],
  ["none",      "清空这个 Column，一条映射都不建立"]
];

const previewBlocked = () => !!(S.preview && S.preview.errors && S.preview.errors.length);

/* 阻塞原因是否为重复目标（只有这种阻塞可以强制应用） */
const duplicateBlocked = () => {
  const pre = S.preview && S.preview.project === S.sel.project ? S.preview : null;
  return !!(pre && pre.errors && pre.errors.some(e => /duplicate target/i.test(e)));
};

function renderModeScope() {
  const p = cur();
  const isCurrent = S.sel.mode === null;
    if (!p) { $("#modeCallout").innerHTML = ""; $("#modeLiveTable").innerHTML = ""; $("#modeBody").innerHTML = ""; $("#modeFoot").innerHTML = ""; $("#wireHead").textContent = "线路预览"; return; }
  const live = liveState(S.sel.project);
  $("#modeTitle").textContent = isCurrent ? "（Current）" : ((p.modes[S.sel.mode] || {}).displayName || S.sel.mode);
  $("#modeSubtitle").textContent = isCurrent
    ? "这个 Project 此刻真正生效的状态（snapshot current）。"
    : "编辑这个 Mode 选哪些 Column、各用哪些 Setting，再应用到（Current）。";

  const callout = $("#modeCallout");
  if (!p.available) {
    callout.innerHTML = '<div class="callout warn">Project 当前状态不可用：' + esc((live.error) || "未知原因") + "</div>";
    $("#modeLiveTable").innerHTML = ""; $("#modeBody").innerHTML = ""; $("#modeFoot").innerHTML = ""; $("#wireHead").textContent = "线路预览";
  }
  if (isCurrent) {
    let message;
    if (live.temporary) {
      message = "当前生效的是<b>临时 Mode</b>，从 <b>" + esc(live.forkedFrom || "当前状态") +
        "</b> 分叉。继续修改只会更新临时 Mode，不会改回原 Mode。";
    } else if (live.kind === "mode") {
      message = "当前跟随 <b>mode " + esc(live.mode) + "</b>。保存该 Mode 的选择会自动同步到这里；" +
        "在这里编辑会创建临时 Mode，不会改动它。";
    } else if (live.kind === "independent") {
      message = "当前是独立选择，不跟随任何 Mode。在这里编辑会创建临时 Mode。";
    } else {
      message = "当前没有可跟随的 Mode；在这里编辑会创建临时 Mode。";
    }
    callout.innerHTML = '<div class="callout">' + message + "</div>";
  } else {
    const isLive = live.kind === "mode" && live.mode === S.sel.mode && !live.temporary;
    const text = isLive
      ? "这个 Mode 正在驱动（Current）。保存选择只写 ModeIndex；应用后（Current）才跟随它。"
      : "这个 Mode 未生效。保存只写 ModeIndex；应用才会让（Current）跟随它。";
    callout.innerHTML = '<div class="callout' + (isLive ? "" : " warn") + '">' + text + "</div>";
  }
  renderLiveTable(isCurrent, live);
  renderModeBody();
  renderModeFoot(isCurrent);
}

function renderLiveTable(isCurrent, live) {
  const box = $("#modeLiveTable");
  if (!isCurrent || !live.recorded.length) { box.innerHTML = ""; return; }
  /* 同一来源的多个目标合并为一行，避免逐条占用阅读空间 */
  const groups = new Map();
  for (const m of live.recorded) {
    if (!groups.has(m.source)) groups.set(m.source, []);
    groups.get(m.source).push(m.target);
  }
  box.innerHTML = "<div class='maptable'><table class='grid'><thead><tr>" +
    "<th>已记录来源</th><th>目标</th><th>实况</th></tr></thead><tbody>" +
    [...groups.entries()].map(([source, targets]) => "<tr><td>" + esc(source) + "</td>" +
      "<td>" + targets.map(t => "<span style='color:var(--signal)'>" + esc(t) + "</span>").join("<span style='color:var(--ink-faint)'> · </span>") + "</td>" +
      "<td>" + worstTargetBadge(targets) + "</td></tr>").join("") +
    "</tbody></table></div>";
}

/* 单个 Column 的配置卡片 HTML（可被任意块复用） */
function cardHtmlForColumn(colName) {
  const p = cur();
  const d = decls();
  const col = p.columns[colName];
  const decl = d[colName];
  const on = !!decl;
  const strategy = decl ? (decl.strategy || "cover") : "cover";
  let html = "<div class='card" + (on ? " on" : "") + (S.hover === colName ? " hot" : "") +
    "' data-card='" + esc(colName) + "'>" +
    "<div class='card-head'>" +
    "<button class='toggle' aria-pressed='" + on + "' data-toggle='" + esc(colName) + "'></button>" +
    "<span><span class='cname'>" + esc(col.displayName || colName) + "</span> " +
    "<span class='cid'>" + esc(colName) + "</span></span>" +
    "<span class='nav-sub'>" + col.targetNumber + " target</span>" +
    "</div>";
  if (on) {
    html += "<div class='card-body'>";
    html += "<div class='field'><label>策略</label><div>" +
      "<div class='seg'>" + STRATEGIES.map(([k]) =>
        "<button data-strategy='" + esc(colName) + "|" + k + "' aria-pressed='" + (strategy === k) + "'>" +
        k + "</button>").join("") + "</div>" +
      "<div class='seg-why'>" + esc(STRATEGIES.find(s => s[0] === strategy)[1]) + "</div></div></div>";
    const needPick = strategy === "cover" || strategy === "increment";
    html += "<div class='field'><label>Setting</label><div class='setlist'>";
    for (const setName of Object.keys(col.settings || {})) {
      const checked = strategy === "full" ? true : (decl.settings || []).includes(setName);
      const mine = currentMappingsOf(S.sel.project).filter(m => sourceSegsMatch(m.source, colName, setName));
      const r = settingTargets(S.sel.project, colName, setName);
      const tgt = mine.length
        ? mine.map(m => m.target).join("  ·  ")
        : (r.error ? r.error : "未应用");
      html += "<label class='setrow'>" +
        "<input type='checkbox'" + (checked ? " checked" : "") +
        (needPick ? "" : " disabled") +
        " data-pick='" + esc(colName) + "|" + esc(setName) + "' />" +
        "<span class='sname'>" + esc(setName) + "</span>" +
        "<span class='stgt'>" + esc(tgt) + "</span>" +
        (mine.length ? targetBadge(mine[0].target)
          : (r.error ? badge("b-bad", "目标无法解析") : badge("b-idle", "未应用"))) + "</label>";
    }
    html += "</div></div>";
    if (strategy === "full")
      html += "<div class='seg-why'>full 不看勾选，Column 里新增的 Setting 会自动进来；有无法解析的目标就整体失败。</div>";
    html += "</div>";
  }
  html += "</div>";
  return html;
}

/* 配置与线路按 Column 配对成块：左卡片右线路，块顶对齐、下沿取较长者 */
function renderModeBody() {
  const box = $("#modeBody");
  const p = cur();
  if (!p) { box.innerHTML = ""; $("#wireHead").textContent = "线路预览"; return; }
  const pre = S.preview && S.preview.project === S.sel.project ? S.preview : null;
  const isCurrent = S.sel.mode === null;
  const live = liveState(S.sel.project);
  /* 线路按 Column 收集：col -> source -> targets[]（preview 未就绪时为空） */
  const wireByCol = new Map();
  if (pre) {
    for (const m of pre.mappings) {
      const segs = String(m.source).split("/");
      const col = segs[segs.length - 2] || "";
      if (!wireByCol.has(col)) wireByCol.set(col, new Map());
      const groups = wireByCol.get(col);
      if (!groups.has(m.source)) groups.set(m.source, []);
      groups.get(m.source).push(m.target);
    }
    $("#wireHead").textContent = "线路预览（" + pre.mappings.length + " 个来源）";
  } else {
    $("#wireHead").textContent = "线路预览（规划中…）";
  }
  let html = (pre ? pre.errors : []).map(e => "<div class='diag'>" + esc(e) + "</div>").join("");
  if (isCurrent && live.kind === "mode") {
    html += "<div class='note' style='padding:10px 16px 4px'>" +
      (live.temporary
        ? "当前正在使用临时 Mode <b>" + esc(live.text) + "</b>。修改只会继续改临时 Mode，不会改回原 Mode。"
        : "当前来自 <b>mode " + esc(live.mode) + "</b>。这里的修改会创建临时 Mode，不会修改原 Mode。") +
      "</div>";
  }
  const colNames = Object.keys(p.columns || {});
  if (!colNames.length) { box.innerHTML = html; return; }
  if (pre && !pre.mappings.length && !pre.errors.length) {
    box.innerHTML = html + "<div class='hint'>没有任何选择，应用后会解除全部受管链接。</div>";
    return;
  }
  const force = new Set((pre && pre.forceTargets) || []);
  html += colNames.map(colName => {
    const groups = wireByCol.get(colName);
    const wire = groups ? [...groups.entries()].map(([source, targets]) => {
      const segs = String(source).split("/");
      const col = segs[segs.length - 2] || "";
      const tag = segs.slice(-2).join(" · ");
      const forceAny = targets.some(t => force.has(t));
      return "<div class='wireline " + worstLineClass(targets) + (S.hover === col ? " hot" : "") +
        "' data-line-col='" + esc(col) + "'>" +
        "<div class='src'>" + esc(source) + "</div>" +
        "<div><span class='arr'>→</span>" +
        targets.map(t => "<span class='tgt'>" + esc(t) + "</span>").join("<span class='arr'> · </span>") + "</div>" +
        "<div class='tag'>" + esc(tag) + " · " + targets.length + " 目标" + (forceAny ? " · 需 --force-targets" : "") + "</div></div>";
    }).join("") : "";
    return "<div class='col-block'>" +
      "<div class='col-cards'>" + cardHtmlForColumn(colName) + "</div>" +
      "<div class='col-wire'>" + wire + "</div>" +
      "</div>";
  }).join("");
  if (!pre) html += "<div class='hint'>正在向后端请求规划…</div>";
  box.innerHTML = html;
}

/* ================= 预览（/api/preview，防抖 300ms） ================= */
let previewTimer = null, previewSeq = 0;

function schedulePreview() {
  clearTimeout(previewTimer);
  previewTimer = setTimeout(firePreview, 300);
}

async function firePreview() {
  if (S.sel.seg !== "mode" || !S.sel.project) return;
  const project = S.sel.project;
  const p = P(project);
  if (!p || !p.available) return;
  const columns = clone(decls());
  const seq = ++previewSeq;
  const res = await apiPreview({ command: "current.preview", project, columns });
  if (seq !== previewSeq || S.sel.project !== project) return;   /* 过期响应丢弃 */
  if (res.ok && res.data && res.data.details) {
    const d = res.data.details;
    S.preview = { project, mappings: d.mappings || [], errors: d.errors || [], forceTargets: d.forceTargets || [] };
  } else {
    S.preview = { project, mappings: [], errors: [(res.error && res.error.message) || "预览失败"], forceTargets: [] };
  }
  if (S.sel.seg === "mode" && S.sel.project === project) { renderModeBody(); renderModeFoot(S.sel.mode === null); renderDrawer(); }
}

async function flushPreview() {
  clearTimeout(previewTimer);
  await firePreview();
}

function renderModeFoot(isCurrent) {
  const pre = S.preview && S.preview.project === S.sel.project ? S.preview : null;
  const blocked = !!(pre && pre.errors.length);
  const forceable = !isCurrent && duplicateBlocked();
  const live = liveState(S.sel.project);
  const nCols = Object.keys(decls()).length;
  const nMaps = pre ? pre.mappings.length : "…";
  let html = "";
  /* 无法应用（重复目标阻塞）时，在按钮上方浮出强制应用条 */
  if (forceable) {
    html += "<div class='force-bar'>" +
      "<button class='btn btn-danger' data-cmd='apply-force'>强制应用</button>" +
      "<span class='why'>重复目标冲突：强制应用按「后声明的 Column 覆盖先声明」忽略冲突，可能丢弃部分映射。</span></div>";
  }
  html += "<span class='note'>选中 " + nCols + " 个 Column，产生 " +
    nMaps + " 条映射" + (blocked ? "，<b style='color:var(--bad)'>有 " + pre.errors.length + " 处阻塞</b>" : "") +
    "</span><div style='flex:1'></div><div class='btnrow'>";
  if (isCurrent) {
    html += "<button class='btn btn-primary' data-cmd='apply-temporary'" + (dirty() && !blocked ? "" : " disabled") + ">应用临时 Mode</button>" +
      "<button class='btn' data-cmd='discard'" + (dirty() ? "" : " disabled") + ">丢弃未应用改动</button>" +
      (live.temporary ? "<button class='btn btn-danger' data-cmd='discard-scratch'>退出临时 Mode</button>" : "") +
      "<button class='btn' data-cmd='refresh'>refresh</button>" +
      "<button class='btn' data-cmd='revert'>revert</button>" +
      "<button class='btn btn-danger' data-cmd='reset'>reset</button>";
  } else {
    /* 保存永远只写 ModeIndex，不触碰 Current 或目标链接；应用才同步 */
    html += "<button class='btn btn-primary' data-cmd='save-mode'" + (dirty() ? "" : " disabled") + ">" +
      "保存选择</button>" +
      "<button class='btn' data-cmd='discard'" + (dirty() ? "" : " disabled") + ">丢弃改动</button>" +
      "<button class='btn btn-primary' data-cmd='apply-mode'" + (blocked ? " disabled" : "") + ">应用到（Current）</button>" +
      "<button class='btn btn-danger' data-danger='mode'>delete mode…</button>";
  }
  $("#modeFoot").innerHTML = html + "</div>";
}

/* ================= 抽屉 ================= */
function renderDrawer() {
  const acts = $("#drawerActions");
  if (S.sel.seg === "project") {
    acts.innerHTML = "<button class='btn' data-cmd='sync'>sync</button><button class='btn' data-cmd='refresh'>refresh</button>";
  } else {
    const blocked = previewBlocked();
    const forceable = S.sel.mode !== null && duplicateBlocked();
    acts.innerHTML = S.sel.mode === null
      ? "<button class='btn btn-primary' data-cmd='apply-temporary'" + (dirty() && !blocked ? "" : " disabled") + ">应用临时 Mode</button>"
      : (forceable ? "<button class='btn btn-danger' data-cmd='apply-force'>强制应用</button>" : "") +
        "<button class='btn' data-cmd='save-mode'" + (dirty() ? "" : " disabled") + ">保存</button>" +
        "<button class='btn btn-primary' data-cmd='apply-mode'" + (blocked ? " disabled" : "") + ">应用到（Current）</button>";
  }
  renderPlan();
}

function renderPlan() {
  const box = $("#planDiff");
  const pill = $("#pendingPill");
  if (S.sel.seg === "project") {
    box.innerHTML = "<div class='empty'>Project 作用域只改资源元数据，不动目标路径。切到 Mode 看应用计划。</div>";
    pill.hidden = true;
    return;
  }
  const live = liveState(S.sel.project);
  const recorded = live.recorded || [];
  const pre = S.preview && S.preview.project === S.sel.project ? S.preview : null;
  const force = new Set((pre && pre.forceTargets) || []);
  const lines = [];
  for (const e of (pre ? pre.errors : [])) lines.push(["d-del", "!", e, "阻塞，apply 会失败"]);
  let keep = 0;
  if (pre) {
    const nowByTarget = new Map(recorded.map(m => [m.target, m.source]));
    const nextByTarget = new Map(pre.mappings.map(m => [m.target, m.source]));
    for (const m of pre.mappings) {
      const oldSrc = nowByTarget.get(m.target);
      const note = force.has(m.target)
        ? (tState(m.target) === "occupied" ? "目标被占用，需 --force-targets" : "目标漂移，需 --force-targets")
        : "建立受管符号链接";
      if (oldSrc === undefined) lines.push(["d-add", "+", m.source + " → " + m.target, note]);
      else if (oldSrc !== m.source) lines.push(["d-mod", "~", m.source + " → " + m.target, "同目标换源" + (force.has(m.target) ? "，需 --force-targets" : "")]);
      else keep++;
    }
    for (const m of recorded) if (!nextByTarget.has(m.target))
      lines.push(["d-del", "−", m.source + " → " + m.target, "解除并恢复原路径"]);
  }
  box.innerHTML = !pre
    ? "<div class='empty'>规划中…</div>"
    : (lines.length
      ? lines.map(([c, op, text, note]) => "<div class='" + c + "'><span class='op'>" + op + "</span>" +
          esc(text) + " <span class='tag' style='color:var(--ink-faint)'>· " + esc(note) + "</span></div>").join("") +
        (keep ? "<div class='d-keep'><span class='op'>=</span>" + keep + " 条映射保持不变</div>" : "")
      : "<div class='empty'>无变化：目标与已记录映射一致（" + keep + " 条保持）。</div>");
  pill.hidden = !dirty();
  if (dirty()) pill.textContent = S.sel.mode === null ? "临时 Mode 未应用" : "选择已改，未保存";
}

function log(line) {
  const body = $("#logBody");
  body.innerHTML += "<span class='ts'>" + new Date().toTimeString().slice(0, 8) + "</span>  " + esc(line) + "\n";
  body.parentElement.scrollTop = body.parentElement.scrollHeight;
}


/* ================= 入口 ================= */
function firstColumnOf(p) { return Object.keys((p && p.columns) || {})[0] || null; }
function firstSettingOf(p) {
  const colName = firstColumnOf(p);
  return { column: colName, setting: colName ? Object.keys(p.columns[colName].settings || {})[0] || null : null };
}

function renderAll() {
  renderStatusbar(); renderNav(); renderCrumb();
  if (S.sel.seg === "project") { renderTree(); renderResPanel(); renderEdList(); renderEditor(); }
  else renderModeScope();
  renderDrawer();
}

/* ================= 覆盖层（模态 / 错误横幅 / 样式，JS 注入，不动 index.html） ================= */
let modalResolve = null;

function injectOverlayUI() {
  const style = document.createElement("style");
  style.textContent =
    "#bootError{position:fixed;top:0;left:0;right:0;z-index:60;display:flex;gap:10px;align-items:center;" +
    "justify-content:center;padding:10px 16px;background:var(--bad-bg);color:var(--bad);" +
    "border-bottom:1px solid var(--line-strong);font-family:var(--font-cjk);font-size:13px}" +
    "#bootError[hidden]{display:none}" +
    "#modalHost{position:fixed;inset:0;z-index:70}#modalHost[hidden]{display:none}" +
    ".modal-backdrop{position:absolute;inset:0;background:rgba(26,26,24,.35);display:flex;align-items:center;justify-content:center}" +
    ".modal-box{background:var(--bg);border:1px solid var(--line-strong);border-radius:8px;box-shadow:0 12px 32px rgba(0,0,0,.18);" +
    "width:min(540px,92vw);max-height:80vh;overflow:auto;padding:16px 18px;font-family:var(--font-cjk)}" +
    ".modal-title{margin:0 0 8px;font-family:var(--font-disp);font-size:16px}" +
    ".modal-body{font-size:13px;color:var(--ink);line-height:1.65}" +
    ".modal-body code{font-family:var(--font-mono);background:var(--surface);padding:0 3px}" +
    ".modal-body .tgtlist{font-family:var(--font-mono);font-size:12px;color:var(--signal);display:block;margin:6px 0;white-space:pre-wrap}" +
    ".modal-foot{display:flex;justify-content:flex-end;gap:8px;margin-top:14px}" +
    ".ed-entries{border:1px solid var(--line);border-radius:6px;margin-bottom:10px;max-height:220px;overflow:auto;font-size:12px;font-family:var(--font-mono)}" +
    ".ed-entries .e-head{padding:6px 10px;background:var(--surface);color:var(--ink-muted);font-family:var(--font-cjk);" +
    "border-bottom:1px solid var(--line-hair);display:flex;gap:8px;align-items:center}" +
    ".ed-entries .e-row{display:flex;align-items:center;gap:8px;padding:4px 10px;border-bottom:1px solid var(--line-hair)}" +
    ".ed-entries .e-row:hover{background:var(--signal-bg)}" +
    ".ed-entries .e-open{flex:1;text-align:left;background:none;border:none;cursor:pointer;font-family:var(--font-mono);font-size:12px;color:var(--ink)}" +
    ".ed-entries .e-open:hover{color:var(--signal)}" +
    ".ed-entries .e-open[aria-current='true']{color:var(--signal);font-weight:600}" +
    ".ed-entries .e-path{flex:1;color:var(--ink-muted)}" +
    ".ed-entries .e-size{color:var(--ink-faint)}" +
    ".ed-hint{font-size:12px;color:var(--ink-muted);padding:6px 0;font-family:var(--font-cjk)}";
  document.head.appendChild(style);

  const host = document.createElement("div");
  host.id = "modalHost";
  host.hidden = true;
  document.body.appendChild(host);

  const banner = document.createElement("div");
  banner.id = "bootError";
  banner.hidden = true;
  banner.innerHTML = "<span class='msg'></span><button class='btn btn-sm' data-cmd='retry-snapshot'>重试</button>";
  document.body.insertBefore(banner, document.body.firstChild);
  $("[data-cmd='retry-snapshot']", banner).addEventListener("click", () => { banner.hidden = true; loadSnapshot(); });

  /* 侧栏 foot 增加 sync --all */
  const foot = $(".side-foot .btnrow");
  if (foot) {
    const b = document.createElement("button");
    b.className = "btn btn-sm";
    b.textContent = "sync --all";
    b.dataset.cmd = "sync-all";
    b.title = "全部 Project 对账（部分成功也报告）";
    foot.insertBefore(b, foot.firstChild);
  }

  /* 内容编辑器附加区：目录条目 + 提示行 */
  const edMain = $(".ed-main");
  if (edMain) {
    const entries = document.createElement("div");
    entries.id = "edEntries";
    entries.className = "ed-entries";
    edMain.insertBefore(entries, edMain.firstChild);
    const hint = document.createElement("div");
    hint.id = "edHint";
    hint.className = "ed-hint";
    edMain.appendChild(hint);
  }

  /* 危险对话框错误提示行 */
  const dlgBody = $("#dangerDlg .dlg-body");
  if (dlgBody) {
    const err = document.createElement("div");
    err.id = "dlgError";
    err.className = "diag";
    err.hidden = true;
    dlgBody.appendChild(err);
  }
}

function showBootError(message) {
  const b = $("#bootError");
  b.hidden = false;
  $(".msg", b).textContent = "仓库快照加载失败：" + message;
}

function openModal(title, html, buttons) {
  const host = $("#modalHost");
  host.innerHTML = "<div class='modal-backdrop'><div class='modal-box'><h3 class='modal-title'></h3>" +
    "<div class='modal-body'></div><div class='modal-foot'></div></div></div>";
  $(".modal-title", host).textContent = title;
  $(".modal-body", host).innerHTML = html;
  const foot = $(".modal-foot", host);
  for (const b of buttons) {
    const el = document.createElement("button");
    el.className = "btn" + (b.primary ? " btn-primary" : "");
    el.textContent = b.label;
    el.addEventListener("click", () => closeModal(b.value));
    foot.appendChild(el);
  }
  host.hidden = false;
  return new Promise(res => { modalResolve = res; });
}
function closeModal(value) {
  const host = $("#modalHost");
  host.hidden = true;
  host.innerHTML = "";
  if (modalResolve) { const r = modalResolve; modalResolve = null; r(value); }
}

function showConflictModal() {
  openModal("仓库已变化", "<p>仓库已被其他窗口或 CLI 修改（revision 冲突）。</p>" +
    "<p>当前草稿与未保存内容会保留；重新加载会以最新仓库状态刷新界面。</p>", [
      { label: "暂不", value: "later" },
      { label: "重新加载", value: "reload", primary: true }
    ]).then(v => { if (v === "reload") loadSnapshot(); });
}

function askForce(message) {
  return openModal("需要 --force-targets 授权", "<p>" + esc(message) + "</p>" +
    "<p>目标被占用或漂移时，回收需要强制授权；与它无关的路径不会被回收。</p>", [
      { label: "取消", value: "no" },
      { label: "强制授权重试", value: "yes", primary: true }
    ]).then(v => v === "yes");
}

/* 解析逗号分隔的 aliases；空白项不提交给后端。 */
function parseAliases(value) {
  if (!value.trim()) return [];
  return value.split(",").map(alias => alias.trim()).filter(Boolean);
}

/* 生成三类资源共用的创建表单。 */
function createFormHtml(kind, project, column) {
  const settingFields = kind === "setting" ?
    "<label class='create-field'><span>类型 <b>*</b></span><select name='kind'>" +
      "<option value='file'>file</option><option value='directory'>directory</option></select></label>" +
    "<label class='create-field create-content'><span>UTF-8 初始内容</span>" +
      "<textarea name='content' rows='8' spellcheck='false' placeholder='可留空，按原字节写入'></textarea>" +
      "<small>仅文件型 Setting 支持；目录型 Setting 创建为空目录。</small></label>" : "";
  const scope = kind === "setting"
    ? "Project <code>" + esc(project) + "</code> · Column <code>" + esc(column) + "</code>"
    : "Project <code>" + esc(project) + "</code>";
  return "<form id='resourceCreateForm' class='create-form' novalidate>" +
    "<p class='create-scope'>" + scope + "</p>" +
    "<label class='create-field'><span>canonical 名称 <b>*</b></span>" +
      "<input name='name' required autocomplete='off' autofocus placeholder='唯一、可用于路径的名称' /></label>" +
    "<label class='create-field'><span>展示名称</span><input name='displayName' autocomplete='off' placeholder='留空则使用 canonical 名称' /></label>" +
    "<label class='create-field'><span>描述</span><textarea name='description' rows='3'></textarea></label>" +
    "<label class='create-field'><span>Aliases</span><input name='aliases' autocomplete='off' placeholder='逗号分隔，例如 main, default' /></label>" +
    settingFields +
    "<div class='create-error' id='createError' role='alert' hidden></div>" +
    "<button class='btn btn-sm create-reload' id='createReload' type='button' hidden>重新加载最新快照</button>" +
    "</form>";
}

/* 创建失败时保留表单，并把后端错误显示在表单内。 */
function showCreateError(err) {
  const box = $("#createError");
  if (!box) return;
  box.hidden = false;
  box.textContent = err.code === "revision_conflict"
    ? "仓库已被其他窗口或 CLI 修改。请重新加载最新快照后再提交；当前输入会保留。"
    : ((err.message || "创建失败") + (err.code ? "（" + err.code + "）" : ""));
  const reload = $("#createReload");
  if (reload) reload.hidden = err.code !== "revision_conflict";
}

/* 创建成功后切换到新资源，并以服务端 snapshot 为唯一状态来源。 */
function focusCreatedResource(kind, project, column, name) {
  const ns = S.navOpen[project] || (S.navOpen[project] = { open: true, cols: false, modes: false });
  ns.open = true;
  if (kind === "column") {
    ns.cols = true;
    S.sel = { seg: "project", project, mode: undefined };
    S.res = { column: name, setting: null };
    S.ed = { column: name, setting: null, path: null };
  } else if (kind === "setting") {
    ns.cols = true;
    S.sel = { seg: "project", project, mode: undefined };
    S.res = { column, setting: name };
    S.ed = { column, setting: name, path: null };
    S.ptab = "editor";
  } else {
    ns.modes = true;
    S.sel = { seg: "mode", project, mode: name };
  }
  renderAll();
  if (kind === "setting") {
    switchPtab("editor");
    loadContent();
  }
  schedulePreview();
}

/* 打开一个资源创建 modal，并在失败时保留所有字段。 */
function openCreateResource(kind) {
  const project = S.sel.project;
  const column = S.res.column;
  if (!project) { log("未选择 Project"); return; }
  if (kind === "setting" && !column) { log("请先选择一个 Column"); return; }
  const labels = { column: "Column", setting: "Setting", mode: "Mode" };
  const host = $("#modalHost");
  host.innerHTML = "<div class='modal-backdrop'><div class='modal-box create-box'>" +
    "<h3 class='modal-title'>新建 " + labels[kind] + "</h3><div class='modal-body'>" +
    createFormHtml(kind, project, column) + "</div><div class='modal-foot'>" +
    "<button class='btn' type='button' data-create-cancel>取消</button>" +
    "<button class='btn btn-primary' type='submit' form='resourceCreateForm' data-create-submit>创建</button>" +
    "</div></div></div>";
  host.hidden = false;
  const form = $("#resourceCreateForm");
  const kindSelect = $("select[name='kind']", form);
  const contentField = $(".create-content", form);
  if (kindSelect) {
    kindSelect.addEventListener("change", () => {
      const directory = kindSelect.value === "directory";
      contentField.hidden = directory;
      if (directory) $("textarea[name='content']", form).value = "";
    });
  }
  $("[data-create-cancel]", host).addEventListener("click", () => closeModal("cancel"));
  $("#createReload", form).addEventListener("click", async () => {
    await loadSnapshot();
    const box = $("#createError");
    if (box) box.textContent = "已重新加载最新快照，可以保留当前输入重新提交。";
    $("#createReload", form).hidden = true;
  });
  form.addEventListener("submit", async event => {
    event.preventDefault();
    const name = form.elements.name.value;
    if (!name.trim()) { showCreateError({ code: "invalid_name", message: "canonical 名称不能为空" }); return; }
    const payload = {
      command: kind + ".create", project, name,
      displayName: form.elements.displayName.value,
      description: form.elements.description.value,
      aliases: parseAliases(form.elements.aliases.value)
    };
    if (kind === "setting") Object.assign(payload, {
      column, kind: form.elements.kind.value, content: form.elements.content.value, encoding: "utf8"
    });
    const submit = $("[data-create-submit]", host);
    submit.disabled = true;
    const ok = await execCommand(payload, {
      label: kind + " create " + name,
      forcePrompt: false,
      onError: showCreateError
    });
    submit.disabled = false;
    if (!ok) return;
    closeModal("created");
    focusCreatedResource(kind, project, column, name);
  });
  form.elements.name.focus();
}

/* 生成 Index 编辑表单中的 target 行。 */
function indexTargetRows(kind, project, column, setting) {
  const p = P(project);
  const col = p.columns[column];
  const count = col.targetNumber || 0;
  const rows = [];
  for (let i = 0; i < count; i++) {
    const dir = kind === "column" ? (col.defaultTargetDir[i] || "") : (setting.targetDir[i] || "");
    const name = kind === "column" ? (col.defaultTargetName[i] || "") : (setting.targetName[i] || "");
    const hint = kind === "column" && !name ? "来自 Setting 名称" : (kind === "setting" && !dir && !name ? "继承 Column 默认" : "");
    rows.push("<div class='index-target-row' data-index-row='" + i + "'>" +
      "<span class='num'>#" + i + "</span>" +
      "<input data-target-dir value='" + esc(dir) + "' placeholder='目标目录' />" +
      "<input data-target-name value='" + esc(name) + "' placeholder='" + (kind === "column" ? "空 = Setting 名称" : "空 = 继承") + "' />" +
      (hint ? "<small>" + esc(hint) + "</small>" : "") +
      "<button class='btn btn-sm' type='button' data-index-target-save>保存</button>" +
      "<button class='btn btn-sm btn-danger' type='button' data-index-target-delete>删除</button></div>");
  }
  return rows.join("") || "<div class='empty'>暂无 target。可在下方新增。</div>";
}

/* 在 Index modal 内显示后端错误，不清除用户正在编辑的字段。 */
function showIndexError(err) {
  const box = $("#indexError");
  if (!box) return;
  box.hidden = false;
  box.textContent = err.code === "revision_conflict"
    ? "仓库已变化，请重新加载最新快照后再提交；当前输入会保留。"
    : ((err.message || "Index 修改失败") + (err.code ? "（" + err.code + "）" : ""));
}

/* 打开 Column/Setting Index 编辑 modal；target 每行独立提交，避免多个位置互相覆盖。 */
function openIndexEditor(kind) {
  const project = S.sel.project;
  const column = S.res.column;
  const p = cur();
  const col = p && p.columns[column];
  const setting = kind === "setting" && col ? col.settings[S.res.setting] : null;
  if (!project || !col || (kind === "setting" && !setting)) { log("请先选择要编辑的资源"); return; }
  const name = kind === "column" ? column : S.res.setting;
  const item = kind === "column" ? col : setting;
  const aliases = (item.aliases || []).join(", ");
  const scope = kind === "column" ? "Project <code>" + esc(project) + "</code>" :
    "Project <code>" + esc(project) + "</code> · Column <code>" + esc(column) + "</code>";
  const host = $("#modalHost");
  host.innerHTML = "<div class='modal-backdrop'><div class='modal-box index-box'><h3 class='modal-title'>编辑 " + (kind === "column" ? "Column" : "Setting") + " Index</h3>" +
    "<div class='modal-body'><p class='create-scope'>" + scope + " · canonical <code>" + esc(name) + "</code></p>" +
    "<form id='indexMetadataForm' class='create-form'><label class='create-field'><span>展示名称</span><input name='displayName' value='" + esc(item.displayName || name) + "' /></label>" +
    "<label class='create-field'><span>描述</span><textarea name='description' rows='3'>" + esc(item.description || "") + "</textarea></label>" +
    "<label class='create-field'><span>Aliases</span><input name='aliases' value='" + esc(aliases) + "' placeholder='逗号分隔' /></label>" +
    "<div class='create-error' id='indexError' role='alert' hidden></div><button class='btn btn-primary' type='submit'>保存元数据</button></form>" +
    "<h4 class='index-heading'>" + (kind === "column" ? "Column 默认 targets" : "Setting target 覆盖") + "</h4>" +
    "<div class='index-targets' id='indexTargets'>" + indexTargetRows(kind, project, column, setting) + "</div>" +
    (kind === "column" ? "<form id='indexAddForm' class='index-add'><input name='targetDir' placeholder='新增目标目录' /><input name='targetName' placeholder='目标名；留空则使用 Setting 名称' /><label><input type='checkbox' name='nameFromSetting' /> 名称来自 Setting</label><button class='btn btn-sm' type='submit'>新增 target</button></form>" : "<p class='create-scope'>Setting 的空字段表示继承 Column 默认 target；每行可单独保存或恢复继承。</p>") +
    "</div><div class='modal-foot'><button class='btn' type='button' data-index-cancel>关闭</button></div></div></div>";
  host.hidden = false;

  const form = $("#indexMetadataForm", host);
  form.addEventListener("submit", async event => {
    event.preventDefault();
    const payload = { command: kind + ".set", project, name, displayName: form.elements.displayName.value, description: form.elements.description.value, aliases: parseAliases(form.elements.aliases.value) };
    if (kind === "setting") payload.column = column;
    const ok = await execCommand(payload, { label: kind + " set " + name, onError: showIndexError });
    if (ok) closeModal("index-saved");
  });

  $$('[data-index-target-save]', host).forEach(button => button.addEventListener("click", async () => {
    const row = button.closest("[data-index-row]");
    const dir = $("[data-target-dir]", row).value.trim();
    const targetName = $("[data-target-name]", row).value.trim();
    const payload = { command: kind === "column" ? "column.target.set" : "setting.target.set", project, name, targetIndex: Number(row.dataset.index), targetDir: dir, targetName, clearDir: !dir, nameFromSetting: !targetName, inheritDir: !dir, inheritName: !targetName };
    if (kind === "setting") payload.column = column;
    const ok = await execCommand(payload, { label: kind + " target set " + row.dataset.index, onError: showIndexError });
    if (ok) closeModal("target-saved");
  }));

  $$('[data-index-target-delete]', host).forEach(button => button.addEventListener("click", async () => {
    if (kind !== "column") return;
    const row = button.closest("[data-index-row]");
    const ok = await execCommand({ command: "column.target.delete", project, name, targetIndex: Number(row.dataset.index), yes: true }, { label: "column target delete", onError: showIndexError });
    if (ok) closeModal("target-deleted");
  }));

  const add = $("#indexAddForm", host);
  if (add) add.addEventListener("submit", async event => {
    event.preventDefault();
    const dir = add.elements.targetDir.value.trim();
    const targetName = add.elements.targetName.value.trim();
    const derived = add.elements.nameFromSetting.checked;
    if (!dir || (!targetName && !derived)) { showIndexError({ code: "invalid_target", message: "目标目录不能为空，且必须填写目标名或选择来自 Setting" }); return; }
    const ok = await execCommand({ command: "column.target.add", project, name, targetDir: dir, targetName, nameFromSetting: derived }, { label: "column target add", onError: showIndexError });
    if (ok) closeModal("target-added");
  });
  $("[data-index-cancel]", host).addEventListener("click", () => closeModal("cancel"));
}

/* 打开独立 canonical rename modal；rename 会移动来源并重写索引/引用。 */
function openIndexRename(kind) {
  const project = S.sel.project;
  const column = S.res.column;
  const oldName = kind === "column" ? column : S.res.setting;
  if (!project || !oldName) { log("请先选择要重命名的资源"); return; }
  const host = $("#modalHost");
  host.innerHTML = "<div class='modal-backdrop'><div class='modal-box create-box'><h3 class='modal-title'>重命名 " + (kind === "column" ? "Column" : "Setting") + "</h3><div class='modal-body'><p class='create-scope'>canonical 重命名会移动来源并更新相关引用。</p><form id='renameForm' class='create-form'><label class='create-field'><span>新 canonical 名称 <b>*</b></span><input name='newName' value='" + esc(oldName) + "' required /></label><div class='create-error' id='renameError' role='alert' hidden></div></form></div><div class='modal-foot'><button class='btn' type='button' data-rename-cancel>取消</button><button class='btn btn-primary' type='submit' form='renameForm'>重命名</button></div></div></div>";
  host.hidden = false;
  const form = $("#renameForm", host);
  form.addEventListener("submit", async event => {
    event.preventDefault();
    const newName = form.elements.newName.value.trim();
    if (!newName) { $("#renameError", host).hidden = false; $("#renameError", host).textContent = "canonical 名称不能为空"; return; }
    const payload = { command: kind + ".rename", project, name: oldName, newName };
    if (kind === "setting") payload.column = column;
    const ok = await execCommand(payload, { label: kind + " rename " + oldName, onError: err => { const box = $("#renameError", host); box.hidden = false; box.textContent = err.message || "重命名失败"; } });
    if (!ok) return;
    closeModal("renamed");
    if (kind === "column") {
      S.res = { column: newName, setting: null }; S.ed = firstSettingOf(P(project));
    } else {
      S.res = { column, setting: newName }; S.ed = { column, setting: newName, path: null };
    }
    renderAll();
    if (kind === "setting") loadContent();
  });
  $("[data-rename-cancel]", host).addEventListener("click", () => closeModal("cancel"));
  form.elements.newName.focus();
}

/* ================= 命令执行 ================= */
async function execCommand(payload, opts = {}) {
  const label = opts.label || payload.command;
  const forcePrompt = opts.forcePrompt !== false;
  const keepDraft = !!opts.keepDraft;
  if (!S.snap) { log(label + "：还没有仓库快照，无法执行"); return false; }
  const res = await apiCommand({ revision: S.snap.revision, ...payload });
  if (res.ok) {
    if (payload.command === "root") {
      S.draft = {};
      await loadSnapshot();                     /* root 的响应 snapshot 是旧根的，必须重新拉取 */
    } else if (res.data && res.data.snapshot) {
      applySnapshot(res.data.snapshot);
      if (!keepDraft) S.draft = {};             /* 快照已变，旧草稿基准失效 */
      renderAll();
      schedulePreview();
    }
    log(label + "：" + ((res.data && res.data.message) || "完成"));
    return true;
  }
  const err = res.error || {};
  if (err.code === "revision_conflict") {
    if (opts.onError) opts.onError(err); else showConflictModal();
    return false;
  }
  const needsForce = err.code === "unsafe_target" ||
    (err.message && /unmanaged|drift|被占用|occupied/i.test(err.message));
  if (needsForce && forcePrompt) {
    const ok2 = await askForce(err.message || "目标需要强制授权");
    if (ok2) return execCommand({ ...payload, forceTargets: true }, { ...opts, forcePrompt: false });
    return false;
  }
  log(label + " 失败（" + err.code + "）：" + (err.message || "未知错误"));
  if (opts.onError) opts.onError(err);
  return false;
}

/* 预览已标注 forceTargets 时，先请求强制授权再提交 */
async function applyWithForce(payload, label) {
  await flushPreview();
  const pre = S.preview && S.preview.project === S.sel.project ? S.preview : null;
  if (pre && pre.errors.length) { log(label + " 被阻塞：" + pre.errors[0]); return false; }
  let force = false;
  if (pre && pre.forceTargets.length) {
    force = await askForce("以下目标需要 --force-targets 授权：\n" + pre.forceTargets.join("\n"));
    if (!force) { log(label + " 已取消（需要 --force-targets 授权）"); return false; }
  }
  return execCommand({ ...payload, forceTargets: force }, { label });
}

async function runCommand(name) {
  const project = S.sel.project;
  if (!project) { log("未选择 Project"); return; }
  switch (name) {
    case "apply-mode":
      return applyWithForce({ command: "apply.mode", project, mode: S.sel.mode }, "apply mode " + S.sel.mode);
    case "save-mode":
      return execCommand({ command: "mode.replace", project, mode: S.sel.mode, columns: clone(decls()) }, { label: "save mode " + S.sel.mode });
    case "apply-temporary":
      return applyWithForce({ command: "current.replace", project, columns: clone(decls()) }, "apply temporary");
    case "apply-force":
      /* 强制应用：忽略重复目标冲突（后声明覆盖先声明），并带 --force-targets 回收授权 */
      return execCommand({ command: "apply.mode", project, mode: S.sel.mode, force: true, forceTargets: true },
        { label: "force apply mode " + S.sel.mode });
    case "discard-scratch": {
      const live = liveState(project);
      if (live.kind === "mode" && live.temporary && live.forkedFrom) {
        /* 退出临时 Mode = 重新跟随原 Mode */
        return applyWithForce({ command: "apply.mode", project, mode: live.forkedFrom }, "exit temporary（回到 " + live.forkedFrom + "）");
      }
      return execCommand({ command: "reset", project }, { label: "exit temporary（reset）" });
    }
    case "discard":
      resetDraft(); renderAll(); schedulePreview();
      log(S.sel.mode === null ? "已丢弃未应用的临时 Mode 改动" : "已丢弃未保存的 Mode 改动");
      return;
    case "reset":
      return execCommand({ command: "reset", project }, { label: "reset" });
    case "refresh":
      return execCommand({ command: "refresh", project }, { label: "refresh" });
    case "revert":
      return execCommand({ command: "revert", project }, { label: "revert" });
    case "sync":
      return execCommand({ command: "sync", project }, { label: "sync" });
    case "sync-all": {
      const res = await apiCommand({ command: "sync", revision: S.snap.revision, project, all: true });
      if (res.ok) {
        if (res.data && res.data.snapshot) { applySnapshot(res.data.snapshot); S.draft = {}; renderAll(); schedulePreview(); }
        const det = (res.data && res.data.details) || {};
        log("sync --all：" + (res.data && res.data.message));
        for (const n of det.succeeded || []) log("  ✓ " + n);
        for (const [n, e] of Object.entries(det.failed || {})) log("  ✗ " + n + "：" + ((e && e.message) || e));
      } else {
        const err = res.error || {};
        if (err.code === "revision_conflict") { showConflictModal(); return; }
        log("sync --all 失败（" + err.code + "）：" + (err.message || "未知错误"));
      }
      return;
    }
    case "root": {
      const path = window.prompt("仓库根目录路径", (S.snap && S.snap.root) || "");
      if (!path) return;
      return execCommand({ command: "root", path }, { label: "root" });
    }
    case "project-create": {
      const pname = window.prompt("新 Project 名（canonical）", "");
      if (!pname) return;
      const ok = await execCommand({ command: "project.create", name: pname }, { label: "project create" });
      if (ok && P(pname)) {
        S.sel = { seg: "project", project: pname, mode: undefined };
        S.res = { column: firstColumnOf(P(pname)), setting: null };
        S.ed = firstSettingOf(P(pname));
        renderAll(); schedulePreview();
      }
      return;
    }
    case "column-create":
      return openCreateResource("column");
    case "setting-create":
      return openCreateResource("setting");
    case "mode-create":
      return openCreateResource("mode");
    default:
      log("未知命令 " + name);
  }
}

/* ================= 内容编辑（Setting Content） ================= */
function contentKey() { return S.ed.column + "|" + S.ed.setting; }
function contentInfo() {
  S.content[S.sel.project] = S.content[S.sel.project] || {};
  const key = contentKey();
  S.content[S.sel.project][key] = S.content[S.sel.project][key] || { loaded: false, entries: null };
  return S.content[S.sel.project][key];
}

async function loadContent() {
  const p = cur();
  const col = p && p.columns[S.ed.column];
  const set = col && col.settings[S.ed.setting];
  if (!set) return;
  const info = contentInfo();
  const base = { command: "setting.content.list", project: S.sel.project, column: S.ed.column, setting: S.ed.setting };
  const listRes = await apiCommand(base);
  if (listRes.ok && listRes.data && listRes.data.details) {
    info.entries = listRes.data.details.entries || [];
  } else {
    info.entries = [];
    log("content list 失败：" + ((listRes.error && listRes.error.message) || "未知错误"));
  }
  if (set.kind === "file") {
    await readEntry(null);
  } else {
    const files = info.entries.filter(e => e.kind === "file");
    const target = S.ed.path || (files.some(f => f.path === "README.md") ? "README.md" : files[0] && files[0].path);
    if (target) await readEntry(target);
    else { info.loaded = false; info.content = ""; info.encoding = "utf8"; info.bytes = 0; }
  }
  renderEditor(); renderEdList();
}

async function readEntry(relPath) {
  const info = contentInfo();
  const r = await apiCommand({
    command: "setting.content.read", project: S.sel.project, column: S.ed.column, setting: S.ed.setting,
    path: relPath || ""
  });
  if (r.ok && r.data && r.data.details) {
    const d = r.data.details;
    info.content = d.content || "";
    info.encoding = d.encoding || "utf8";
    info.bytes = d.bytes || 0;
    info.loaded = true;
  } else {
    info.loaded = false;
    log("content read 失败（" + (relPath || "根") + "）：" + ((r.error && r.error.message) || "未知错误"));
  }
}

async function saveContent() {
  const set = cur().columns[S.ed.column].settings[S.ed.setting];
  if (!set || $("#edSave").disabled) return;
  const info = contentInfo();
  const text = $("#edText").value;
  /* 先更新本地缓存，execCommand 成功后会以最新缓存重绘 */
  info.content = text;
  info.encoding = "utf8";
  info.bytes = new TextEncoder().encode(text).length;
  info.loaded = true;
  const ok = await execCommand({
    command: "setting.content.write", project: S.sel.project, column: S.ed.column, setting: S.ed.setting,
    path: S.ed.path || "", content: text, encoding: "utf8"
  }, { label: "content write", keepDraft: true });
  if (ok) {
    $("#edDirty").hidden = true;
    if (set.kind === "directory") refreshEntries(false);
  }
}

async function refreshEntries(render = true) {
  const info = contentInfo();
  const r = await apiCommand({
    command: "setting.content.list", project: S.sel.project, column: S.ed.column, setting: S.ed.setting
  });
  if (r.ok && r.data && r.data.details) info.entries = r.data.details.entries || [];
  if (render) { renderEditor(); renderEdList(); }
}

async function execContentMkdir() {
  const rel = window.prompt("新建目录的相对路径（相对 Setting 根，如 sub/conf.d）", "");
  if (!rel) return;
  const ok = await execCommand({
    command: "setting.content.mkdir", project: S.sel.project, column: S.ed.column, setting: S.ed.setting, path: rel
  }, { label: "content mkdir", keepDraft: true });
  if (ok) refreshEntries();
}

async function execContentMove(entryPath) {
  const newPath = window.prompt("移动到（相对路径）", entryPath);
  if (!newPath || newPath === entryPath) return;
  const ok = await execCommand({
    command: "setting.content.move", project: S.sel.project, column: S.ed.column, setting: S.ed.setting,
    oldPath: entryPath, path: newPath
  }, { label: "content move", keepDraft: true });
  if (ok) refreshEntries();
}

async function execContentDelete(entryPath) {
  if (!window.confirm("删除内容条目 " + entryPath + " ？")) return;
  const ok = await execCommand({
    command: "setting.content.delete", project: S.sel.project, column: S.ed.column, setting: S.ed.setting,
    path: entryPath, yes: true
  }, { label: "content delete", keepDraft: true });
  if (ok) refreshEntries();
}

/* ================= 危险确认（删除：先提交，后端拒绝后再要求授权勾选） ================= */
let dangerCtx = null;

function dangerMeta(kind) {
  const p = cur();
  const project = S.sel.project;
  if (!p) return null;
  if (kind === "column") {
    const col = p.columns[S.res.column];
    const refs = Object.keys(p.modes || {}).filter(m => (p.modes[m].columns || {})[S.res.column]);
    return { kind, title: "删除 Column", target: S.res.column, project, command: "column.delete",
      column: S.res.column, label: "column " + S.res.column,
      desc: "删除 <code>" + esc(S.res.column) + "</code>（Column），含 " +
        Object.keys(col.settings || {}).length + " 个 Setting。" +
        (refs.length ? " 被 Mode " + refs.map(esc).join("、") + " 引用。" : "") };
  }
  if (kind === "setting") {
    const refs = Object.keys(p.modes || {}).filter(m => {
      const d = (p.modes[m].columns || {})[S.res.column];
      return d && (d.strategy === "full" || (d.settings || []).includes(S.res.setting));
    });
    return { kind, title: "删除 Setting", target: S.res.setting, project, command: "setting.delete",
      column: S.res.column, setting: S.res.setting, label: "setting " + S.res.setting,
      desc: "删除 <code>" + esc(S.res.column) + "/" + esc(S.res.setting) + "</code>（Setting）。" +
        (refs.length ? " 被 Mode " + refs.map(esc).join("、") + " 引用。" : "") };
  }
  if (kind === "mode") {
    const live = liveState(project);
    const isLive = live.kind === "mode" && live.mode === S.sel.mode && !live.temporary;
    return { kind, title: "删除 Mode", target: S.sel.mode, project, command: "mode.delete",
      mode: S.sel.mode, label: "mode " + S.sel.mode,
      desc: "删除 <code>" + esc(S.sel.mode) + "</code>（Mode）。" +
        (isLive ? " 它正在驱动（Current），删除后（Current）会退化。" : "") };
  }
  return null;
}

function openDanger(kind) {
  const meta = dangerMeta(kind);
  if (!meta) return;
  dangerCtx = { ...meta, need: { cascade: false, force: false }, error: null };
  $("#dlgTitle").textContent = dangerCtx.title;
  $("#dlgDesc").innerHTML = dangerCtx.desc;
  $("#dlgError").hidden = true;
  $("#reqYes").textContent = "本次必需";
  $("#reqCascade").textContent = "未知：先提交（不带 --cascade），后端拒绝后再勾选";
  $("#reqForce").textContent = "未知：先提交（不带 --force-targets），后端拒绝后再勾选";
  ["gYes", "gCascade", "gForce"].forEach(id => { $("#" + id).checked = false; });
  $("#dlgConfirm").disabled = true;
  $("#dangerDlg").showModal();
}

/* 后端拒绝后的重试：更新需要勾选的授权并显示错误 */
function reopenDangerWithError(err) {
  if (!dangerCtx) return;
  dangerCtx.need.cascade = dangerCtx.need.cascade || err.code === "dependencies_exist";
  dangerCtx.need.force = dangerCtx.need.force || err.code === "unsafe_target" ||
    (err.message && /unmanaged|drift|被占用|occupied/i.test(err.message));
  dangerCtx.error = (err.message || err.code);
  $("#dlgTitle").textContent = dangerCtx.title;
  $("#dlgDesc").innerHTML = dangerCtx.desc;
  const errEl = $("#dlgError");
  errEl.textContent = "上次尝试被拒绝：" + dangerCtx.error;
  errEl.hidden = false;
  $("#reqYes").textContent = "本次必需";
  $("#reqCascade").textContent = dangerCtx.need.cascade
    ? "后端要求：存在依赖引用，本次必需"
    : "未知：先提交，后端拒绝后再勾选";
  $("#reqForce").textContent = dangerCtx.need.force
    ? "后端要求：存在漂移/被占用目标，本次必需"
    : "未知：先提交，后端拒绝后再勾选";
  ["gYes", "gCascade", "gForce"].forEach(id => { $("#" + id).checked = false; });
  $("#dlgConfirm").disabled = true;
  $("#dangerDlg").showModal();
}

async function runDelete() {
  const ctx = dangerCtx;
  if (!ctx) return;
  const payload = {
    command: ctx.command, revision: S.snap.revision, project: ctx.project,
    column: ctx.column, setting: ctx.setting, mode: ctx.mode,
    yes: $("#gYes").checked, cascade: $("#gCascade").checked, forceTargets: $("#gForce").checked
  };
  const res = await apiCommand(payload);
  if (res.ok) {
    if (res.data && res.data.snapshot) { applySnapshot(res.data.snapshot); S.draft = {}; renderAll(); schedulePreview(); }
    log(ctx.label + " 已删除" + (payload.cascade ? "（--cascade）" : "") + (payload.forceTargets ? "（--force-targets）" : ""));
    dangerCtx = null;
    return;
  }
  const err = res.error || {};
  if (err.code === "revision_conflict") { dangerCtx = null; showConflictModal(); return; }
  reopenDangerWithError(err);
  log(ctx.label + " 删除被拒绝（" + err.code + "）：" + (err.message || "未知错误") + "，请在对话框勾选授权后重试");
}

/* ================= 标签切换（Column 与 Setting / 内容编辑） ================= */
function switchPtab(name) {
  S.ptab = name;
  $$("[data-ptab]").forEach(x => x.setAttribute("aria-selected", String(x.dataset.ptab === name)));
  $("#ptab-targets").hidden = name !== "targets";
  $("#ptab-editor").hidden = name !== "editor";
}

/* ================= 事件 ================= */
document.addEventListener("click", e => {
  const t = e.target;

  /* Column / Mode 分组头：整行点击展开/收起；COLUMN 分组头还会把主区切到「全部 Column」资源视图 */
  const grp = t.closest("[data-nav-group]");
  if (grp) {
    const [kind, project] = grp.dataset.navGroup.split("|");
    const ns = S.navOpen[project] || (S.navOpen[project] = { open: false, cols: false, modes: false });
    if (kind === "cols") {
      ns.cols = !ns.cols;
      const alreadyAll = S.sel.seg === "project" && S.sel.project === project && S.treeAll;
      if (!alreadyAll) {
        S.sel = { seg: "project", project, mode: undefined };
        S.treeAll = true;
        S.res = { column: firstColumnOf(P(project)), setting: null };
        S.ed = firstSettingOf(P(project));
        switchPtab("targets");
        renderAll(); schedulePreview();
      } else {
        renderNav();
      }
    } else {
      ns.modes = !ns.modes;
      renderNav();
    }
    return;
  }

  /* 可展开节点：Project 整行点击切换展开；Column/Mode 条目是叶子，点击只导航 */
  const nav = t.closest("[data-nav]");
  if (nav) {
    const project = nav.dataset.project;
    const kind = nav.dataset.nav;
    if (kind === "project") {
      const ns = S.navOpen[project] || (S.navOpen[project] = { open: false, cols: false, modes: false });
      /* 收起/展开 Project 都重置分组状态：每次点开只看到 Column/Mode 分组头，
         不保留上次的分组展开情况 */
      ns.open = !ns.open;
      ns.cols = false;
      ns.modes = false;
      S.sel = { seg: "project", project, mode: undefined };
      S.res = { column: firstColumnOf(P(project)), setting: null };
      S.ed = firstSettingOf(P(project));
    } else if (kind === "column") {
      /* 侧栏 Column 条目：跳转到 Project 资源视图，只展示该 Column 及其 Setting */
      S.sel = { seg: "project", project, mode: undefined };
      S.treeAll = false;
      S.res = { column: nav.dataset.column, setting: null };
      const col = P(project).columns[nav.dataset.column];
      S.ed = { column: nav.dataset.column, setting: col ? Object.keys(col.settings || {})[0] || null : null, path: null };
    } else {
      S.sel = { seg: "mode", project, mode: nav.dataset.mode !== undefined ? nav.dataset.mode : null };
    }
    S.hover = null; renderAll(); schedulePreview(); return;
  }

  const indexEdit = t.closest("[data-index-edit]");
  if (indexEdit) { openIndexEditor(indexEdit.dataset.indexEdit); return; }
  const indexRename = t.closest("[data-index-rename]");
  if (indexRename) { openIndexRename(indexRename.dataset.indexRename); return; }

  const ptab = t.closest("[data-ptab]");
  if (ptab) {
    switchPtab(ptab.dataset.ptab);
    if (S.ptab === "editor" && !contentInfo().loaded) loadContent();
    return;
  }

  const treeCol = t.closest(".tree-col");
  if (treeCol) { S.res = { column: treeCol.dataset.column, setting: null }; renderTree(); renderResPanel(); return; }
  const treeSet = t.closest(".tree-set");
  if (treeSet) { S.res = { column: treeSet.dataset.column, setting: treeSet.dataset.setting }; renderTree(); renderResPanel(); return; }

  if (t.closest("[data-goto-editor]")) {
    S.ed = { column: S.res.column, setting: S.res.setting, path: null };
    $("[data-ptab='editor']").click();
    renderEdList(); loadContent(); return;
  }
  const edFile = t.closest(".ed-file");
  if (edFile) {
    S.ed = { column: edFile.dataset.edColumn, setting: edFile.dataset.edSetting, path: null };
    renderEdList(); loadContent(); return;
  }

  const contentOpen = t.closest("[data-content-open]");
  if (contentOpen) {
    S.ed.path = contentOpen.dataset.contentOpen;
    readEntry(S.ed.path).then(() => renderEditor());
    return;
  }
  if (t.closest("[data-content-mkdir]")) { execContentMkdir(); return; }
  const contentMove = t.closest("[data-content-move]");
  if (contentMove) { execContentMove(contentMove.dataset.contentMove); return; }
  const contentDel = t.closest("[data-content-del]");
  if (contentDel) { execContentDelete(contentDel.dataset.contentDel); return; }

  if (t.id === "edSave") { saveContent(); return; }

  /* Column 开关 */
  const toggle = t.closest("[data-toggle]");
  if (toggle && !toggle.disabled) {
    const colName = toggle.dataset.toggle;
    const d = decls();
    if (colName in d) {
      delete d[colName];
    } else {
      const settings = Object.keys(cur().columns[colName].settings || {})
        .filter(setName => !settingTargets(S.sel.project, colName, setName).error);
      d[colName] = { settings, strategy: "cover" };
    }
    S.hover = colName; renderModeScope(); renderDrawer(); schedulePreview(); return;
  }

  /* 策略 */
  const strat = t.closest("[data-strategy]");
  if (strat) {
    const [colName, value] = strat.dataset.strategy.split("|");
    decls()[colName].strategy = value;
    S.hover = colName; renderModeScope(); renderDrawer(); schedulePreview(); return;
  }

  const danger = t.closest("[data-danger]");
  if (danger) { openDanger(danger.dataset.danger); return; }

  if (t.id === "drawerToggle") {
    const d = $("#drawer");
    const open = d.dataset.open !== "true";
    d.dataset.open = String(open);
    t.setAttribute("aria-expanded", String(open));
    t.textContent = open ? "▾" : "▸";
    return;
  }

  const cmd = t.closest("[data-cmd]");
  if (cmd && !cmd.disabled) runCommand(cmd.dataset.cmd);
});

/* Setting 勾选 */
document.addEventListener("change", e => {
  const pick = e.target.closest("[data-pick]");
  if (!pick) return;
  const [colName, setName] = pick.dataset.pick.split("|");
  const d = decls();
  if (!d[colName]) return;
  const list = d[colName].settings || (d[colName].settings = []);
  const i = list.indexOf(setName);
  if (pick.checked && i < 0) list.push(setName);
  if (!pick.checked && i >= 0) list.splice(i, 1);
  S.hover = colName; renderModeScope(); renderDrawer(); schedulePreview();
});

/* hover 联动：卡片 ↔ 线路 */
document.addEventListener("mouseover", e => {
  const card = e.target.closest("[data-card]");
  const line = e.target.closest("[data-line-col]");
  const colName = card ? card.dataset.card : line ? line.dataset.lineCol : null;
  if (colName === S.hover) return;
  S.hover = colName;
  $$("[data-card]").forEach(el => el.classList.toggle("hot", el.dataset.card === colName));
  $$("[data-line-col]").forEach(el => el.classList.toggle("hot", el.dataset.lineCol === colName));
});

$("#edText").addEventListener("input", () => { $("#edDirty").hidden = false; syncGutter(); });
$("#edText").addEventListener("scroll", () => { $("#edGutter").scrollTop = $("#edText").scrollTop; });

/* 危险确认对话框 */
$("#dangerDlg").addEventListener("change", () => {
  const n = dangerCtx ? dangerCtx.need : { cascade: false, force: false };
  $("#dlgConfirm").disabled = !($("#gYes").checked &&
    (!n.cascade || $("#gCascade").checked) && (!n.force || $("#gForce").checked));
});
$("#dangerDlg").addEventListener("close", () => {
  if ($("#dangerDlg").returnValue !== "ok") { if (dangerCtx) log(dangerCtx.kind + " delete 已取消"); return; }
  runDelete();
});

/* ================= 启动 ================= */
injectOverlayUI();
loadSnapshot();
log("cfgfc web 已加载（真实后端 /api/snapshot、/api/command、/api/preview）");
