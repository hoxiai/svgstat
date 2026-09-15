const state = {
    user: null,
    page: "overview",
    overview: null,
    users: { items: [], page: 1, pageSize: 20, total: 0, totalPages: 0 },
    projects: { items: [], page: 1, pageSize: 20, total: 0, totalPages: 0 },
    filters: { users: { q: "", status: "" }, projects: { q: "", status: "" } },
};

const elements = {};
let toastTimer;
let searchTimer;

document.addEventListener("DOMContentLoaded", () => {
    ["auth-screen", "forbidden-screen", "console", "login-form", "login-email", "login-password", "login-button", "login-error",
        "forbidden-logout", "logout-button", "refresh-button", "menu-button", "page-title", "page-subtitle", "loading", "error-banner",
        "summary-cards", "trend-chart", "health-list", "top-projects", "recent-users", "recent-projects", "users-table", "projects-table",
        "users-pagination", "projects-pagination", "user-search", "user-status", "project-search", "project-status", "admin-name", "admin-email",
        "admin-avatar", "drawer", "drawer-backdrop", "drawer-content", "drawer-close", "toast"].forEach(id => elements[id] = document.getElementById(id));
    bindEvents();
    checkSession();
});

function bindEvents() {
    elements["login-form"].addEventListener("submit", login);
    elements["logout-button"].addEventListener("click", logout);
    elements["forbidden-logout"].addEventListener("click", logout);
    elements["refresh-button"].addEventListener("click", () => loadPage(true));
    elements["menu-button"].addEventListener("click", () => document.querySelector(".sidebar").classList.toggle("open"));
    document.querySelectorAll(".nav-item").forEach(button => button.addEventListener("click", () => navigate(button.dataset.page)));
    document.querySelectorAll("[data-goto]").forEach(button => button.addEventListener("click", () => navigate(button.dataset.goto)));
    elements["user-status"].addEventListener("change", event => { state.filters.users.status = event.target.value; state.users.page = 1; loadUsers(); });
    elements["project-status"].addEventListener("change", event => { state.filters.projects.status = event.target.value; state.projects.page = 1; loadProjects(); });
    elements["user-search"].addEventListener("input", event => debounceSearch("users", event.target.value));
    elements["project-search"].addEventListener("input", event => debounceSearch("projects", event.target.value));
    elements["drawer-close"].addEventListener("click", closeDrawer);
    elements["drawer-backdrop"].addEventListener("click", closeDrawer);
    document.addEventListener("keydown", event => { if (event.key === "Escape") closeDrawer(); });
}

async function api(path, options = {}) {
    const response = await fetch(path, { credentials: "same-origin", headers: { "Content-Type": "application/json", ...(options.headers || {}) }, ...options });
    let payload = {};
    try { payload = await response.json(); } catch (_) {}
    if (!response.ok) {
        const error = new Error(payload.error || "请求失败");
        error.status = response.status;
        throw error;
    }
    return payload.data;
}

async function checkSession() {
    try {
        const user = await api("/api/v1/admin/me");
        enterConsole(user);
    } catch (error) {
        if (error.status === 403) showOnly("forbidden-screen");
        else showOnly("auth-screen");
    }
}

async function login(event) {
    event.preventDefault();
    elements["login-error"].classList.add("hidden");
    elements["login-button"].disabled = true;
    elements["login-button"].textContent = "登录中…";
    try {
        await api("/api/v1/auth/login", { method: "POST", body: JSON.stringify({ email: elements["login-email"].value, password: elements["login-password"].value }) });
        const user = await api("/api/v1/admin/me");
        enterConsole(user);
    } catch (error) {
        if (error.status === 403) showOnly("forbidden-screen");
        else {
            elements["login-error"].textContent = error.message === "Invalid credentials" ? "邮箱或密码不正确" : error.message;
            elements["login-error"].classList.remove("hidden");
        }
    } finally {
        elements["login-button"].disabled = false;
        elements["login-button"].textContent = "登录管理端";
    }
}

async function logout() {
    try { await api("/api/v1/auth/logout", { method: "POST" }); } catch (_) {}
    state.user = null;
    showOnly("auth-screen");
}

function enterConsole(user) {
    state.user = user;
    elements["admin-name"].textContent = user.name || "管理员";
    elements["admin-email"].textContent = user.email;
    elements["admin-avatar"].textContent = (user.name || user.email || "A")[0].toUpperCase();
    showOnly("console");
    navigate("overview");
}

function showOnly(id) {
    ["auth-screen", "forbidden-screen", "console"].forEach(item => elements[item].classList.toggle("hidden", item !== id));
}

function navigate(page) {
    state.page = page;
    const metadata = {
        overview: ["管理概览", "掌握平台运行和增长情况"],
        users: ["用户管理", "查看账号、项目归属与访问状态"],
        projects: ["项目管理", "管理项目运行状态和功能能力"],
    };
    elements["page-title"].textContent = metadata[page][0];
    elements["page-subtitle"].textContent = metadata[page][1];
    document.querySelectorAll(".nav-item").forEach(item => item.classList.toggle("active", item.dataset.page === page));
    document.querySelectorAll(".page").forEach(item => item.classList.add("hidden"));
    document.getElementById(`${page}-page`).classList.remove("hidden");
    document.querySelector(".sidebar").classList.remove("open");
    loadPage();
}

function loadPage(force = false) {
    if (state.page === "overview") return loadOverview(force);
    if (state.page === "users") return loadUsers();
    return loadProjects();
}

async function withLoading(loader) {
    elements.loading.classList.remove("hidden");
    document.querySelectorAll(".page").forEach(page => page.style.opacity = ".45");
    elements["error-banner"].classList.add("hidden");
    try { await loader(); }
    catch (error) {
        if (error.status === 401 || error.status === 403) return checkSession();
        elements["error-banner"].textContent = error.message;
        elements["error-banner"].classList.remove("hidden");
    } finally {
        elements.loading.classList.add("hidden");
        document.querySelectorAll(".page").forEach(page => page.style.opacity = "1");
    }
}

async function loadOverview(force = false) {
    if (state.overview && !force) return renderOverview();
    await withLoading(async () => { state.overview = await api("/api/v1/admin/overview"); renderOverview(); });
}

function renderOverview() {
    const data = state.overview;
    const cards = [
        ["用户总数", data.summary.users, `今日新增 ${formatNumber(data.summary.newUsersToday)}`],
        ["项目总数", data.summary.projects, `${formatNumber(data.summary.activeProjects)} 个运行中`],
        ["今日请求", data.summary.requests, `${formatNumber(data.summary.pv)} PV · ${formatNumber(data.summary.uv)} UV`],
        ["机器人请求", data.summary.bots, "今日已识别流量"],
    ];
    elements["summary-cards"].innerHTML = cards.map(card => `<article class="summary-card"><p>${card[0]}</p><strong>${formatNumber(card[1])}</strong><small>${card[2]}</small></article>`).join("");
    const max = Math.max(...data.trend.map(point => point.requests), 1);
    elements["trend-chart"].innerHTML = data.trend.map(point => `<div class="bar-column"><span class="bar-value">${compactNumber(point.requests)}</span><span class="bar" style="height:${Math.max(point.requests / max * 82, 2)}%" title="${escapeHTML(point.date)}：${formatNumber(point.requests)}"></span><span class="bar-label">${formatDay(point.date)}</span></div>`).join("");
    const health = [["PostgreSQL", data.health.database], ["Redis", data.health.redis]];
    elements["health-list"].innerHTML = health.map(item => `<div class="health-row"><span class="dot ${item[1] === "ok" ? "" : "error"}"></span><div><strong>${item[0]}</strong><small>${item[1] === "ok" ? "运行正常" : "连接异常"}</small></div><span class="badge ${item[1] === "ok" ? "active" : "disabled"}">${item[1] === "ok" ? "正常" : "异常"}</span></div>`).join("");
    elements["top-projects"].innerHTML = data.topProjects.length ? data.topProjects.map((item, index) => `<div class="rank-row"><span class="rank-number">${index + 1}</span><div><strong>${escapeHTML(item.name)}</strong><small>/${escapeHTML(item.slug)}</small></div><b>${formatNumber(item.requests)}</b></div>`).join("") : empty("暂无项目数据");
    elements["recent-users"].innerHTML = data.recentUsers.length ? data.recentUsers.map(item => `<div class="compact-row"><span class="avatar">${initial(item.name || item.email)}</span><div><strong>${escapeHTML(item.name || "未填写名称")}</strong><small>${escapeHTML(item.email)}</small></div><span class="badge ${item.status}">${statusLabel(item.status)}</span></div>`).join("") : empty("暂无用户");
    elements["recent-projects"].innerHTML = data.recentProjects.length ? data.recentProjects.map(item => `<div class="compact-row"><span class="avatar">${initial(item.name)}</span><div><strong>${escapeHTML(item.name)}</strong><small>${escapeHTML(item.ownerEmail || "公共项目")}</small></div><span class="badge ${item.status}">${statusLabel(item.status)}</span></div>`).join("") : empty("暂无项目");
}

async function loadUsers() {
    await withLoading(async () => {
        const query = new URLSearchParams({ q: state.filters.users.q, status: state.filters.users.status, page: state.users.page, pageSize: 20 });
        state.users = await api(`/api/v1/admin/users?${query}`);
        renderUsers();
    });
}

function renderUsers() {
    elements["users-table"].innerHTML = state.users.items.length ? state.users.items.map(user => `<tr>
        <td><div class="primary-cell"><span class="avatar">${initial(user.name || user.email)}</span><div><strong>${escapeHTML(user.name || "未填写名称")}</strong><small>${escapeHTML(user.email)}</small></div></div></td>
        <td><span class="badge ${user.role}">${user.role === "admin" ? "管理员" : "用户"}</span></td><td>${formatNumber(user.projectCount)}</td>
        <td>${formatDate(user.lastLoginAt)}</td><td>${formatDate(user.createdAt)}</td><td><span class="badge ${user.status}">${statusLabel(user.status)}</span></td>
        <td><div class="actions"><button class="small-button" onclick="openUser('${user.id}')">详情</button><button class="small-button" ${state.user.id === user.id ? "disabled" : ""} onclick="toggleUser('${user.id}','${user.status}')">${user.status === "active" ? "停用" : "启用"}</button></div></td></tr>`).join("") : `<tr><td colspan="7">${empty("没有找到符合条件的用户")}</td></tr>`;
    renderPagination("users");
}

async function loadProjects() {
    await withLoading(async () => {
        const query = new URLSearchParams({ q: state.filters.projects.q, status: state.filters.projects.status, page: state.projects.page, pageSize: 20 });
        state.projects = await api(`/api/v1/admin/projects?${query}`);
        renderProjects();
    });
}

function renderProjects() {
    elements["projects-table"].innerHTML = state.projects.items.length ? state.projects.items.map(project => `<tr>
        <td><div class="primary-cell"><span class="avatar">${initial(project.name)}</span><div><strong>${escapeHTML(project.name)}</strong><small>/${escapeHTML(project.slug)}</small></div></div></td>
        <td>${escapeHTML(project.ownerEmail || "公共项目")}</td><td><strong>${formatNumber(project.requests7d)}</strong></td>
        <td><div class="caps">${capBadge(project, "renderEnabled", "渲染")}${capBadge(project, "badgeEnabled", "徽章")}${capBadge(project, "widgetEnabled", "组件")}${capBadge(project, "chartEnabled", "图表")}</div></td>
        <td><span class="badge ${project.status}">${statusLabel(project.status)}</span></td>
        <td><div class="actions"><button class="small-button" onclick="openProject('${project.id}')">详情</button><button class="small-button" ${(project.id === "free" || project.id === "demo") ? "disabled" : ""} onclick="toggleProjectStatus('${project.id}','${project.status}')">${project.status === "active" ? "停用" : "启用"}</button></div></td></tr>`).join("") : `<tr><td colspan="6">${empty("没有找到符合条件的项目")}</td></tr>`;
    renderPagination("projects");
}

function capBadge(project, field, label) {
    return `<button class="cap ${project[field] ? "on" : ""}" title="点击切换" onclick="toggleCapability('${project.id}','${field}',${project[field]})">${label}</button>`;
}

function renderPagination(type) {
    const page = state[type];
    elements[`${type}-pagination`].innerHTML = `<span>共 ${formatNumber(page.total)} 条，第 ${page.page || 1} / ${page.totalPages || 1} 页</span><div><button class="small-button" ${page.page <= 1 ? "disabled" : ""} onclick="changePage('${type}',-1)">上一页</button><button class="small-button" ${page.totalPages === 0 || page.page >= page.totalPages ? "disabled" : ""} onclick="changePage('${type}',1)">下一页</button></div>`;
}

function changePage(type, offset) { state[type].page += offset; type === "users" ? loadUsers() : loadProjects(); }

function debounceSearch(type, value) {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => { state.filters[type].q = value.trim(); state[type].page = 1; type === "users" ? loadUsers() : loadProjects(); }, 320);
}

async function openUser(id) {
    openDrawer("正在加载用户详情…");
    try {
        const data = await api(`/api/v1/admin/users/${id}`);
        const user = data.user;
        elements["drawer-content"].innerHTML = `<p class="eyebrow">用户详情</p><h2>${escapeHTML(user.name || "未填写名称")}</h2><p class="muted">${escapeHTML(user.email)}</p>
            <div class="detail-section"><div class="detail-grid"><div class="detail-item"><small>账号状态</small><strong>${statusLabel(user.status)}</strong></div><div class="detail-item"><small>角色</small><strong>${user.role === "admin" ? "管理员" : "普通用户"}</strong></div><div class="detail-item"><small>项目数量</small><strong>${formatNumber(user.projectCount)}</strong></div><div class="detail-item"><small>注册时间</small><strong>${formatDate(user.createdAt)}</strong></div></div></div>
            <div class="detail-section"><h3>拥有的项目</h3>${data.projects.length ? data.projects.map(project => `<div class="project-mini"><div><strong>${escapeHTML(project.name)}</strong><div class="muted">/${escapeHTML(project.slug)}</div></div><span class="badge ${project.status}">${statusLabel(project.status)}</span></div>`).join("") : empty("该用户还没有项目")}</div>
            <div class="button-row"><button class="button ${user.status === "active" ? "danger" : "primary"}" ${state.user.id === user.id ? "disabled" : ""} onclick="toggleUser('${user.id}','${user.status}',true)">${user.status === "active" ? "停用账号" : "启用账号"}</button></div>`;
    } catch (error) { elements["drawer-content"].innerHTML = empty(error.message); }
}

async function openProject(id) {
    openDrawer("正在加载项目详情…");
    try {
        const project = await api(`/api/v1/admin/projects/${id}`);
        elements["drawer-content"].innerHTML = `<p class="eyebrow">项目详情</p><h2>${escapeHTML(project.name)}</h2><p class="muted">/${escapeHTML(project.slug)} · ${escapeHTML(project.ownerEmail || "公共项目")}</p>
            <div class="detail-section"><div class="detail-grid"><div class="detail-item"><small>项目状态</small><strong>${statusLabel(project.status)}</strong></div><div class="detail-item"><small>近 7 天请求</small><strong>${formatNumber(project.requests7d)}</strong></div><div class="detail-item"><small>可见性</small><strong>${project.visibility}</strong></div><div class="detail-item"><small>创建时间</small><strong>${formatDate(project.createdAt)}</strong></div></div></div>
            <div class="detail-section"><h3>能力开关</h3>${switchRow(project, "renderEnabled", "SVG 渲染", "控制 Counter 渲染")}${switchRow(project, "badgeEnabled", "徽章", "控制 Badge 渲染")}${switchRow(project, "widgetEnabled", "组件", "控制 Widget 能力")}${switchRow(project, "chartEnabled", "图表", "控制 Chart 能力")}</div>
            <div class="button-row"><a class="button secondary" href="/svg/${encodeURIComponent(project.slug)}/badge/visits.svg" target="_blank">打开 SVG ↗</a><button class="button ${project.status === "active" ? "danger" : "primary"}" ${(project.id === "free" || project.id === "demo") ? "disabled" : ""} onclick="toggleProjectStatus('${project.id}','${project.status}',true)">${project.status === "active" ? "停用项目" : "启用项目"}</button></div>`;
    } catch (error) { elements["drawer-content"].innerHTML = empty(error.message); }
}

function switchRow(project, field, title, description) {
    return `<div class="switch-row"><div><strong>${title}</strong><div class="muted">${description}</div></div><button class="switch ${project[field] ? "on" : ""}" aria-label="切换${title}" onclick="toggleCapability('${project.id}','${field}',${project[field]},true)"></button></div>`;
}

async function toggleUser(id, status, drawer = false) {
    const next = status === "active" ? "disabled" : "active";
    if (!confirm(next === "disabled" ? "停用后该用户的所有登录会话会立即失效，确认继续？" : "确认重新启用该用户？")) return;
    try {
        await api(`/api/v1/admin/users/${id}/status`, { method: "PATCH", body: JSON.stringify({ status: next }) });
        showToast(next === "disabled" ? "用户已停用" : "用户已启用");
        await loadUsers();
        if (drawer) openUser(id);
        state.overview = null;
    } catch (error) { showToast(error.message, true); }
}

async function toggleProjectStatus(id, status, drawer = false) {
    const next = status === "active" ? "disabled" : "active";
    if (!confirm(next === "disabled" ? "停用后该项目的 SVG 将停止服务，确认继续？" : "确认重新启用该项目？")) return;
    try {
        await api(`/api/v1/admin/projects/${id}/status`, { method: "PATCH", body: JSON.stringify({ status: next }) });
        showToast(next === "disabled" ? "项目已停用" : "项目已启用");
        await loadProjects();
        if (drawer) openProject(id);
        state.overview = null;
    } catch (error) { showToast(error.message, true); }
}

async function toggleCapability(id, field, current, drawer = false) {
    try {
        await api(`/api/v1/admin/projects/${id}/capabilities`, { method: "PATCH", body: JSON.stringify({ [field]: !current }) });
        showToast("项目能力已更新");
        await loadProjects();
        if (drawer) openProject(id);
    } catch (error) { showToast(error.message, true); }
}

function openDrawer(content) {
    elements["drawer-content"].innerHTML = `<div class="empty">${content}</div>`;
    elements.drawer.classList.remove("hidden");
    elements["drawer-backdrop"].classList.remove("hidden");
}
function closeDrawer() { elements.drawer.classList.add("hidden"); elements["drawer-backdrop"].classList.add("hidden"); }
function showToast(message, error = false) { clearTimeout(toastTimer); elements.toast.textContent = message; elements.toast.style.background = error ? "#a93636" : "#242a38"; elements.toast.classList.remove("hidden"); toastTimer = setTimeout(() => elements.toast.classList.add("hidden"), 2600); }
function statusLabel(status) { return ({ active: "正常", disabled: "已停用", archived: "已归档", pending: "待处理", grace: "宽限期" })[status] || status; }
function formatNumber(value) { return new Intl.NumberFormat("zh-CN").format(Number(value || 0)); }
function compactNumber(value) { return new Intl.NumberFormat("zh-CN", { notation: "compact", maximumFractionDigits: 1 }).format(Number(value || 0)); }
function formatDate(value) { return value ? new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value)) : "—"; }
function formatDay(value) { const date = new Date(`${value}T00:00:00Z`); return `${date.getUTCMonth() + 1}/${date.getUTCDate()}`; }
function initial(value) { return escapeHTML((value || "?").trim().slice(0, 1).toUpperCase()); }
function empty(message) { return `<div class="empty">${escapeHTML(message)}</div>`; }
function escapeHTML(value) { return String(value ?? "").replace(/[&<>'"]/g, char => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;" })[char]); }
