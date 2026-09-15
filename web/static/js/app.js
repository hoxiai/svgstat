function createEmptyProjectStats() {
    return {
        projectId: '',
        date: '',
        pv: 0,
        uv: 0,
        requests: 0,
        bots: 0,
        referrers: {},
        countries: {},
        regions: {},
        cities: {},
        devices: {},
        browsers: {},
        paths: {},
        ips: {},
        visitors: []
    };
}

function createEmptyVisitorPage() {
    return {
        projectId: '',
        date: '',
        page: 1,
        pageSize: 20,
        total: 0,
        totalPages: 0,
        items: []
    };
}

function createEmptyTrend(days = 30) {
    return {
        projectId: '',
        days,
        startDate: '',
        endDate: '',
        points: [],
        totals: { pv: 0, uv: 0, requests: 0, bots: 0 }
    };
}

function createEmptyInstallation() {
    return { projectId: '', status: 'pending', firstSeenAt: null, lastSeenAt: null };
}

function createEmptyRealtime() {
	return { projectId: '', pv5: 0, pv30: 0, visitors5: 0, visitors30: 0, lastSeenAt: null };
}

function createEmptyAnalysis(days = 30) {
	return {
		projectId: '', days, startDate: '', endDate: '',
		current: { pv: 0, uv: 0, requests: 0, bots: 0 },
		previous: { pv: 0, uv: 0, requests: 0, bots: 0 },
		changes: { pv: null, uv: null, requests: null, bots: null },
		breakdowns: { paths: {}, referrers: {}, countries: {}, devices: {}, browsers: {}, sources: {}, mediums: {}, campaigns: {} }
	};
}

function createEmptySessionQuality(days = 30) {
	return {
		projectId: '', days, startDate: '', endDate: '',
		totals: { sessions: 0, bounces: 0, pageviews: 0, durationSeconds: 0, bounceRate: 0, averagePages: 0, averageDurationSeconds: 0 },
		entrances: [], exits: [], flows: [], segments: { source: [], medium: [], campaign: [], device: [], country: [] }
	};
}

function createEmptyEventReport(days = 30) {
	return {
		projectId: '', days, startDate: '', endDate: '', visitors: 0, events: [],
		quality: { status: 'no_data', vitals: [], javascript: { count: 0, visitors: 0 }, resources: { count: 0, visitors: 0 }, errorDetails: {} }
	};
}

function createEmptyIssueReport(days = 30) {
	return { projectId: '', days, status: 'insufficient_data', evaluatedAt: '', issues: [] };
}

function createEmptyDiagnostics() {
	return { projectId: '', status: 'warning', accepted: 0, rejected: 0, testEvents: 0, lastAcceptedAt: null, lastRejectedAt: null, issues: [], entries: [] };
}

function createDefaultVisitorFilters() {
    return {
        device: '',
        browser: '',
        path: '',
        sort: 'last_seen_desc',
        pageSize: 20
    };
}

// Define the Alpine component
function spaApp() {
    return {
        lang: localStorage.getItem('svgstat-lang') || ((navigator.language || 'en').toLowerCase().startsWith('en') ? 'en' : 'zh'),
        user: null,
        currentPage: 'home',
        projects: [],
        loading: false,
        creating: false,
        showCreateModal: false,
        showCodeModal: false,
        onboardingMode: false,
        embedFormat: 'website',
		websiteDomainsInput: '',
		websiteTrackingEnabled: true,
		savingWebsiteTracking: false,
        installation: createEmptyInstallation(),
        loadingInstallation: false,
        installationPollTimer: null,
        installationRequestId: 0,
        dashboardRefreshTimer: null,
        expandedVisitorId: null,
        selectedProject: null,
        lastLoadedStatsProjectId: '',
        lastLoadedVisitorsRequestKey: '',
        projectStats: createEmptyProjectStats(),
		projectTrend: createEmptyTrend(),
		projectRealtime: createEmptyRealtime(),
		projectAnalysis: createEmptyAnalysis(),
		sessionQuality: createEmptySessionQuality(),
		eventReport: createEmptyEventReport(),
		issueReport: createEmptyIssueReport(),
		loadingIssues: false,
		issueRequestId: 0,
		conversionGoals: [],
		funnels: [],
		funnelReports: {},
		newGoal: { name: '', eventName: '' },
		newFunnel: { name: '', stepsText: '' },
		loadingConversions: false,
		diagnostics: createEmptyDiagnostics(),
		loadingDiagnostics: false,
		diagnosticsPollTimer: null,
		selectedEventName: '',
		conversionDimension: 'source',
		funnelDimension: 'source',
		sessionQualityDimension: 'source',
        trendDays: 30,
        visitorsPage: createEmptyVisitorPage(),
        visitorFilters: createDefaultVisitorFilters(),
        loadingStats: false,
		loadingTrend: false,
		loadingRealtime: false,
		realtimeRequestId: 0,
		loadingAnalysis: false,
		analysisRequestId: 0,
		sessionQualityRequestId: 0,
		loadingSessionQuality: false,
        trendRequestId: 0,
        trendRequestKey: '',
        loadingVisitors: false,
        freePageId: '',
        toast: { show: false, message: '', type: 'success', timer: null },
        codeSettings: {
            counterName: 'visits',
            counterLabel: 'Visits',
            counterColor: '',
            badgeName: 'requests',
            badgeLabel: 'Requests',
            badgeColor: '',
            badgeStyle: '',
            homepageUrl: ''
        },
        newProject: {
            name: '',
            slug: '',
            description: ''
        },
        loginForm: {
            email: '',
            password: ''
        },
        registerForm: {
            name: '',
            email: '',
            password: ''
        },
        loginLoading: false,
        registerLoading: false,
        loginError: null,
        registerError: null,
        registerSuccess: null,

        t(key) {
            return translations[this.lang]?.[key] || translations.en[key] || key;
        },

        init() {
            this.$watch('lang', (val) => {
                localStorage.setItem('svgstat-lang', val);
                document.documentElement.lang = val;
            });
            document.documentElement.lang = this.lang;
            this.parseRoute();
            window.addEventListener('popstate', () => this.parseRoute());
            this.checkAuth();
            if (this.currentPage === 'dashboard' || this.currentPage === 'project-detail') {
                this.loadProjects();
            }
        },

        parseRoute() {
            this.stopDashboardAutoRefresh();
			this.stopDiagnosticsPolling();
            this.stopInstallationPolling();
            const path = window.location.pathname;
            if (path === '/login') {
                this.currentPage = 'login';
                this.selectedProject = null;
                this.lastLoadedStatsProjectId = '';
                this.lastLoadedVisitorsRequestKey = '';
                this.showCodeModal = false;
                this.expandedVisitorId = null;
                this.projectStats = createEmptyProjectStats();
                this.visitorsPage = createEmptyVisitorPage();
                this.visitorFilters = createDefaultVisitorFilters();
            } else if (path === '/register') {
                this.currentPage = 'register';
                this.selectedProject = null;
                this.lastLoadedStatsProjectId = '';
                this.lastLoadedVisitorsRequestKey = '';
                this.showCodeModal = false;
                this.expandedVisitorId = null;
                this.projectStats = createEmptyProjectStats();
                this.visitorsPage = createEmptyVisitorPage();
                this.visitorFilters = createDefaultVisitorFilters();
            } else if (path === '/dashboard') {
                this.currentPage = 'dashboard';
                this.selectedProject = null;
                this.lastLoadedStatsProjectId = '';
                this.lastLoadedVisitorsRequestKey = '';
                this.showCodeModal = false;
                this.expandedVisitorId = null;
                this.projectStats = createEmptyProjectStats();
                this.visitorsPage = createEmptyVisitorPage();
                this.visitorFilters = createDefaultVisitorFilters();
            } else if (path.startsWith('/dashboard/')) {
                // 处理项目详情路由
                const slug = path.substring('/dashboard/'.length);
                this.currentPage = 'project-detail';
                this.selectedProject = null;
                this.installation = createEmptyInstallation();
                this.showCodeModal = false;
                this.expandedVisitorId = null;
                this.lastLoadedVisitorsRequestKey = '';
                this.visitorFilters = createDefaultVisitorFilters();
                // 如果项目已经加载过，则查找对应的项目
                if (this.projects.length > 0) {
                    const project = this.projects.find(p => p.slug === slug);
                    if (project) {
                        this.selectedProject = project;
                        this.installation = createEmptyInstallation();
                        this.loadStats(project.id);
						this.loadTrend(project.id);
						this.loadRealtime(project.id);
						this.loadAnalysis(project.id);
						this.loadSessionQuality(project.id);
						this.loadConversions(project.id);
						this.loadIssues(project.id);
						this.loadDiagnostics(project.id);
                        this.loadVisitors(project.id, 1);
                        this.loadInstallation(project.id);
                        this.startDashboardAutoRefresh(project.id);
                    }
                }
            } else {
                this.currentPage = 'home';
                this.selectedProject = null;
                this.lastLoadedStatsProjectId = '';
                this.lastLoadedVisitorsRequestKey = '';
                this.showCodeModal = false;
                this.expandedVisitorId = null;
                this.projectStats = createEmptyProjectStats();
                this.visitorsPage = createEmptyVisitorPage();
                this.visitorFilters = createDefaultVisitorFilters();
            }
        },

        navigate(path) {
            window.history.pushState({}, '', path);
            this.parseRoute();
            window.scrollTo(0, 0);
            if (this.currentPage === 'dashboard' || this.currentPage === 'project-detail') {
                this.loadProjects();
            }
        },

        async checkAuth() {
            try {
                const res = await fetch('/api/v1/auth/me', { credentials: 'same-origin' });
                const data = await res.json();
                if (data.success) {
                    this.user = data.data;
                }
            } catch (e) {
                console.error('Auth check failed', e);
            }
        },

        async login() {
            this.loginLoading = true;
            this.loginError = null;

            try {
                const res = await fetch('/api/v1/auth/login', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(this.loginForm),
                    credentials: 'same-origin'
                });

                const data = await res.json();

                if (data.success) {
                    this.user = data.data.user;
                    this.loginForm = { email: '', password: '' };
                    this.navigate('/dashboard');
                } else {
                    this.loginError = data.error || (this.lang === 'zh' ? '登录失败' : 'Invalid credentials');
                }
            } catch (e) {
                this.loginError = this.lang === 'zh' ? '发生错误，请重试' : 'Something went wrong. Please try again.';
                console.error(e);
            } finally {
                this.loginLoading = false;
            }
        },

        async register() {
            this.registerLoading = true;
            this.registerError = null;
            this.registerSuccess = null;

            try {
                const res = await fetch('/api/v1/auth/register', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(this.registerForm),
                    credentials: 'same-origin'
                });

                const data = await res.json();

                if (data.success) {
                    this.user = data.data.user;
                    this.registerForm = { name: '', email: '', password: '' };
                    this.registerSuccess = this.lang === 'zh' ? '账户创建成功！正在跳转...' : 'Account created! Redirecting...';
                    setTimeout(() => {
                        this.navigate('/dashboard');
                    }, 1000);
                } else {
                    this.registerError = data.error || (this.lang === 'zh' ? '发生错误' : 'Something went wrong');
                }
            } catch (e) {
                this.registerError = this.lang === 'zh' ? '发生错误，请重试' : 'Something went wrong. Please try again.';
                console.error(e);
            } finally {
                this.registerLoading = false;
            }
        },

        async logout() {
            try {
                await fetch('/api/v1/auth/logout', {
                    method: 'POST',
                    credentials: 'same-origin'
                });
                this.user = null;
                this.navigate('/');
            } catch (e) {
                console.error('Logout failed', e);
            }
        },

        async loadProjects() {
            this.loading = true;
            try {
                const res = await fetch('/api/v1/projects', { credentials: 'same-origin' });
                const data = await res.json();
                if (data.success) {
                    this.projects = data.data;
                    // 如果当前是项目详情页面，查找对应的项目
                    if (this.currentPage === 'project-detail') {
                        const path = window.location.pathname;
                        const slug = path.substring('/dashboard/'.length);
                        const project = this.projects.find(p => p.slug === slug);
                        if (project) {
                            this.selectedProject = project;
                            this.installation = createEmptyInstallation();
                            this.loadStats(project.id);
							this.loadTrend(project.id);
							this.loadRealtime(project.id);
							this.loadAnalysis(project.id);
							this.loadSessionQuality(project.id);
							this.loadConversions(project.id);
							this.loadIssues(project.id);
							this.loadDiagnostics(project.id);
                            this.loadVisitors(project.id, 1);
                            this.loadInstallation(project.id);
                            this.startDashboardAutoRefresh(project.id);
                        }
                    }
                }
            } catch (e) {
                console.error('Failed to load projects', e);
            } finally {
                this.loading = false;
            }
        },

        async createProject() {
            this.creating = true;
            try {
                const res = await fetch('/api/v1/projects', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(this.newProject),
                    credentials: 'same-origin'
                });

                const data = await res.json();
                if (data.success) {
                    const project = data.data;
                    this.showCreateModal = false;
                    this.newProject = { name: '', slug: '', description: '' };
                    await this.loadProjects();
                    this.openCodeModal(project, true);
                } else {
                    this.showToast(data.error || this.t('errorCreate'), 'error');
                }
            } catch (e) {
                console.error(e);
                this.showToast(this.t('errorGeneric'), 'error');
            } finally {
                this.creating = false;
            }
        },

        async deleteProject(id) {
            if (!confirm(this.t('confirmDelete'))) return;

            try {
                const res = await fetch(`/api/v1/projects/${id}`, {
                    method: 'DELETE',
                    credentials: 'same-origin'
                });

                if (res.ok) {
                    await this.loadProjects();
                }
            } catch (e) {
                console.error(e);
            }
        },

        viewProjectStats(project) {
            this.navigate(`/dashboard/${project.slug}`);
        },

        resetCodeSettings(project = this.selectedProject) {
            this.codeSettings = {
                counterName: 'visits',
                counterLabel: this.lang === 'zh' ? '访问量' : 'Visits',
                counterColor: '',
                badgeName: 'requests',
                badgeLabel: this.lang === 'zh' ? '请求数' : 'Requests',
                badgeColor: '',
                badgeStyle: '',
                homepageUrl: '',
                pageId: project ? `project/${project.slug}` : ''
            };
        },

        openCodeModal(project, onboarding = false) {
            this.stopInstallationPolling();
            this.selectedProject = project;
            this.onboardingMode = onboarding;
			this.embedFormat = 'website';
			this.websiteDomainsInput = (project.websiteDomains || []).join('\n');
			this.websiteTrackingEnabled = project.websiteTrackingEnabled !== false;
            this.installation = createEmptyInstallation();
            this.resetCodeSettings(project);
            this.showCodeModal = true;
            this.loadInstallation(project.id, true);
        },

        closeCodeModal() {
            this.showCodeModal = false;
            this.onboardingMode = false;
            this.stopInstallationPolling();
            if (this.currentPage !== 'project-detail') {
                this.selectedProject = null;
            }
        },

        getQueryString(params) {
            const search = new URLSearchParams();
            Object.entries(params || {}).forEach(([key, value]) => {
                if (value !== null && value !== undefined && String(value).trim() !== '') {
                    search.set(key, String(value).trim());
                }
            });
            const result = search.toString();
            return result ? `?${result}` : '';
        },

        getCounterSvgPath(project = this.selectedProject, preview = false) {
            if (!project) return '';
            const name = this.codeSettings.counterName || 'visits';
            const query = this.getQueryString({
                label: this.codeSettings.counterLabel,
                color: this.codeSettings.counterColor,
                homepage: this.getHomepageLink(),
                page_id: this.codeSettings.pageId,
                preview: preview ? '1' : ''
            });
            return `/svg/${project.slug}/counter/${name}.svg${query}`;
        },

        getBadgeSvgPath(project = this.selectedProject, preview = false) {
            if (!project) return '';
            const name = this.codeSettings.badgeName || 'requests';
            const query = this.getQueryString({
                label: this.codeSettings.badgeLabel,
                color: this.codeSettings.badgeColor,
                style: this.codeSettings.badgeStyle,
                homepage: this.getHomepageLink(),
                page_id: this.codeSettings.pageId,
                preview: preview ? '1' : ''
            });
            return `/svg/${project.slug}/badge/${name}.svg${query}`;
        },

        getCounterSvgUrl(project = this.selectedProject) {
            return this.getAbsoluteUrl(this.getCounterSvgPath(project));
        },

        getBadgeSvgUrl(project = this.selectedProject) {
            return this.getAbsoluteUrl(this.getBadgeSvgPath(project));
        },

        getCounterMarkdown(project = this.selectedProject) {
            const label = this.codeSettings.counterLabel || this.codeSettings.counterName || 'Visits';
            const url = this.getCounterSvgUrl(project);
            const homepage = this.getHomepageLink();
            if (!url) return '';
            return homepage ? `[![${label}](${url})](${homepage})` : `![${label}](${url})`;
        },

        getBadgeMarkdown(project = this.selectedProject) {
            const label = this.codeSettings.badgeLabel || this.codeSettings.badgeName || 'Requests';
            const url = this.getBadgeSvgUrl(project);
            const homepage = this.getHomepageLink();
            if (!url) return '';
            return homepage ? `[![${label}](${url})](${homepage})` : `![${label}](${url})`;
        },

        getCounterHtml(project = this.selectedProject) {
            const label = this.codeSettings.counterLabel || this.codeSettings.counterName || 'Visits';
            const image = `<img src="${this.escapeHtmlAttribute(this.getCounterSvgUrl(project))}" alt="${this.escapeHtmlAttribute(label)}">`;
            const homepage = this.getHomepageLink();
            return homepage ? `<a href="${this.escapeHtmlAttribute(homepage)}">${image}</a>` : image;
        },

        getRecommendedEmbed() {
			if (this.embedFormat === 'website') return this.getWebsiteSnippet();
			return this.embedFormat === 'html' ? this.getCounterHtml() : this.getCounterMarkdown();
        },

		getWebsiteSnippet(project = this.selectedProject) {
			if (!project) return '';
			return `<script defer src="${this.escapeHtmlAttribute(this.getAbsoluteUrl('/sdk.js'))}" data-project="${this.escapeHtmlAttribute(project.slug)}"><\/script>`;
		},

		getTestWebsiteSnippet(project = this.selectedProject) {
			if (!project) return '';
			return `<script defer src="${this.escapeHtmlAttribute(this.getAbsoluteUrl('/sdk.js'))}" data-project="${this.escapeHtmlAttribute(project.slug)}" data-mode="test"><\/script>`;
		},

		async saveWebsiteTracking() {
			if (!this.selectedProject || this.savingWebsiteTracking) return;
			this.savingWebsiteTracking = true;
			const domains = this.websiteDomainsInput.split(/[\n,]+/).map(value => value.trim()).filter(Boolean);
			try {
				const res = await fetch(`/api/v1/projects/${this.selectedProject.id}/website`, {
					method: 'PUT',
					headers: { 'Content-Type': 'application/json' },
					credentials: 'same-origin',
					body: JSON.stringify({ enabled: this.websiteTrackingEnabled, domains })
				});
				const data = await res.json();
				if (!data.success) {
					this.showToast(data.error || this.t('errorGeneric'), 'error');
					return;
				}
				this.selectedProject = data.data;
				this.websiteTrackingEnabled = data.data.websiteTrackingEnabled !== false;
				this.websiteDomainsInput = (data.data.websiteDomains || []).join('\n');
				this.projects = this.projects.map(project => project.id === data.data.id ? data.data : project);
				this.showToast(this.t('websiteSettingsSaved'));
			} catch (e) {
				console.error('Failed to save website tracking settings', e);
				this.showToast(this.t('errorGeneric'), 'error');
			} finally {
				this.savingWebsiteTracking = false;
			}
		},

        escapeHtmlAttribute(value) {
            return String(value || '').replaceAll('&', '&amp;').replaceAll('"', '&quot;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
        },

        getHomepageLink() {
            const value = String(this.codeSettings.homepageUrl || '').trim();
            if (!value) return '';
            try {
                const normalized = value.includes('://') ? value : `https://${value}`;
                const target = new URL(normalized);
                return `${target.protocol}//${target.host}/`;
            } catch (e) {
                return '';
            }
        },

        getPublicBaseUrl() {
            return window.location.origin;
        },

        getAbsoluteUrl(path) {
            return path ? `${this.getPublicBaseUrl()}${path}` : '';
        },

        getDemoCounterUrl() {
            const path = `/svg/demo/counter/visits.svg${this.getQueryString({
                label: this.lang === 'zh' ? '访问量' : 'Visits',
                color: '7c3aed'
            })}`;
            return this.getAbsoluteUrl(path);
        },

        getDemoBadgeUrl() {
            const path = `/svg/demo/badge/requests.svg${this.getQueryString({
                label: this.lang === 'zh' ? '请求数' : 'Requests',
                color: '0ea5e9',
                style: 'flat'
            })}`;
            return this.getAbsoluteUrl(path);
        },

        getDemoMarkdown() {
            const label = this.lang === 'zh' ? '访问量' : 'Visits';
            return `![${label}](${this.getDemoCounterUrl()})`;
        },

        getFreeBadgePath() {
            return `/svg/free/badge/visitor.svg${this.getQueryString({
                label: this.lang === 'zh' ? '访客' : 'visitors',
                page_id: this.freePageId
            })}`;
        },

        getFreeBadgeUrl() {
            return this.getAbsoluteUrl(this.getFreeBadgePath());
        },

        getFreeMarkdown() {
            return `![visitors](${this.getFreeBadgeUrl()})`;
        },

        getFreeHtml() {
            return `<img src="${this.getFreeBadgeUrl()}" alt="visitors">`;
        },

        showToast(message, type = 'success') {
            clearTimeout(this.toast.timer);
            this.toast.message = message;
            this.toast.type = type;
            this.toast.show = true;
            this.toast.timer = setTimeout(() => {
                this.toast.show = false;
            }, 2200);
        },

        copyText(text) {
            navigator.clipboard.writeText(text).then(() => {
                this.showToast(this.t('copied'));
            });
        },

        getSortedEntries(record, limit = null) {
            const entries = Object.entries(record || {}).sort((a, b) => b[1] - a[1]);
            return limit ? entries.slice(0, limit) : entries;
        },

        getBarStyle(count, record) {
            const values = Object.values(record || {});
            const max = values.length ? Math.max(...values) : 0;
            const width = max > 0 ? (count / max) * 100 : 0;
            return `width: ${width}%`;
        },

        getTrendMax() {
            const values = (this.projectTrend.points || []).flatMap(point => [point.pv || 0, point.uv || 0, point.requests || 0]);
            return Math.max(1, ...values);
        },

        getTrendPoints(metric) {
            const points = this.projectTrend.points || [];
            if (!points.length) return '';
            const width = 1000;
            const height = 220;
            const max = this.getTrendMax();
            return points.map((point, index) => {
                const x = points.length === 1 ? width / 2 : (index / (points.length - 1)) * width;
                const y = height - ((point[metric] || 0) / max) * height;
                return `${x.toFixed(1)},${y.toFixed(1)}`;
            }).join(' ');
        },

        formatTrendDate(value) {
            if (!value) return '-';
            const date = new Date(`${value}T00:00:00Z`);
            return date.toLocaleDateString(this.lang === 'zh' ? 'zh-CN' : 'en-US', { month: 'short', day: 'numeric', timeZone: 'UTC' });
        },

        setTrendDays(days) {
            if (this.trendDays === days || !this.selectedProject) return;
            this.trendDays = days;
            this.loadTrend(this.selectedProject.id, true);
			this.loadAnalysis(this.selectedProject.id, true);
			this.loadSessionQuality(this.selectedProject.id, true);
			this.loadConversions(this.selectedProject.id);
			this.loadIssues(this.selectedProject.id, true);
        },

		formatChange(value) {
			if (value === null || value === undefined) return '—';
			const rounded = Math.round(value * 10) / 10;
			return `${rounded > 0 ? '+' : ''}${rounded}%`;
		},

        shortVisitorId(visitorId) {
            if (!visitorId) return '-';
            return visitorId.length > 12 ? `${visitorId.slice(0, 12)}...` : visitorId;
        },

        formatBadgePath(path) {
            if (!path) return '-';
            const match = path.match(/^\/svg\/[^/]+\/(counter|badge)\/(.+)\.svg$/);
            return match ? `${match[1]}/${match[2]}` : path;
        },

        formatDateTime(value) {
            if (!value) return '-';
            const date = new Date(value);
            if (Number.isNaN(date.getTime())) return '-';
            return date.toLocaleString(this.lang === 'zh' ? 'zh-CN' : 'en-US');
        },

        formatVisitorLocation(visitor) {
            const parts = [visitor.country, visitor.region, visitor.city].filter(Boolean);
            return parts.length ? parts.join(' / ') : '-';
        },

        getVisitorPaginationText() {
            return this.t('visitorPagination')
                .replace('{page}', this.visitorsPage.page || 1)
                .replace('{totalPages}', this.visitorsPage.totalPages || 1);
        },

        getVisitorQueryString(page = 1) {
            const params = new URLSearchParams();
            params.set('page', String(page));
            params.set('page_size', String(this.visitorFilters.pageSize || 20));
            if (this.visitorFilters.device) params.set('device', this.visitorFilters.device);
            if (this.visitorFilters.browser) params.set('browser', this.visitorFilters.browser);
            if (this.visitorFilters.path) params.set('path', this.visitorFilters.path);
            if (this.visitorFilters.sort) params.set('sort', this.visitorFilters.sort);
            return params.toString();
        },

        applyVisitorFilters() {
            if (!this.selectedProject) return;
            this.lastLoadedVisitorsRequestKey = '';
            this.loadVisitors(this.selectedProject.id, 1);
        },

        resetVisitorFilters() {
            this.visitorFilters = createDefaultVisitorFilters();
            this.applyVisitorFilters();
        },

        toggleVisitorDetail(visitorId) {
            this.expandedVisitorId = this.expandedVisitorId === visitorId ? null : visitorId;
        },

        async loadVisitors(projectId, page = 1) {
            if (!projectId) return;

            const pageSize = this.visitorFilters.pageSize || 20;
            const requestKey = `${projectId}:${page}:${pageSize}`;
            const queryString = this.getVisitorQueryString(page);
            const fullRequestKey = `${projectId}:${queryString}`;
            if (this.lastLoadedVisitorsRequestKey === fullRequestKey) return;

            this.lastLoadedVisitorsRequestKey = fullRequestKey;
            this.loadingVisitors = true;
            this.expandedVisitorId = null;
            this.visitorsPage = {
                ...createEmptyVisitorPage(),
                page,
                pageSize
            };

            try {
                const res = await fetch(`/api/v1/projects/${projectId}/visitors?${queryString}`, { credentials: 'same-origin' });
                const data = await res.json();
                if (data.success) {
                    this.visitorsPage = {
                        ...createEmptyVisitorPage(),
                        ...data.data
                    };
                }
            } catch (e) {
                this.lastLoadedVisitorsRequestKey = '';
                console.error('Failed to load visitors', e);
            } finally {
                this.loadingVisitors = false;
            }
        },

        changeVisitorPage(nextPage) {
            if (!this.selectedProject) return;
            if (nextPage < 1) return;
            if (this.visitorsPage.totalPages > 0 && nextPage > this.visitorsPage.totalPages) return;
            this.loadVisitors(this.selectedProject.id, nextPage);
        },
        
        async loadStats(projectId, force = false) {
            if (!projectId) return;
            if (!force && this.lastLoadedStatsProjectId === projectId) return;

            this.lastLoadedStatsProjectId = projectId;
            this.loadingStats = true;
            this.expandedVisitorId = null;
            this.projectStats = createEmptyProjectStats();
            try {
                const res = await fetch(`/api/v1/projects/${projectId}/stats`, { credentials: 'same-origin' });
                const data = await res.json();
                if (data.success) {
                    this.projectStats = {
                        ...createEmptyProjectStats(),
                        ...data.data
                    };
                }
            } catch (e) {
                this.lastLoadedStatsProjectId = '';
                console.error('Failed to load stats', e);
            } finally {
                this.loadingStats = false;
            }
        },

        async loadTrend(projectId, force = false) {
            if (!projectId) return;
            const requestedDays = this.trendDays;
            const requestKey = `${projectId}:${requestedDays}`;
            if (!force && this.loadingTrend && this.trendRequestKey === requestKey) return;
            const requestId = ++this.trendRequestId;
            this.trendRequestKey = requestKey;
            this.loadingTrend = true;
            this.projectTrend = createEmptyTrend(requestedDays);
            try {
                const res = await fetch(`/api/v1/projects/${projectId}/stats/trend?days=${requestedDays}`, { credentials: 'same-origin' });
                const data = await res.json();
                if (requestId !== this.trendRequestId) return;
                if (data.success) {
                    this.projectTrend = {
                        ...createEmptyTrend(requestedDays),
                        ...data.data,
                        totals: { ...createEmptyTrend(requestedDays).totals, ...(data.data.totals || {}) }
                    };
                } else {
                    this.showToast(data.error || this.t('errorGeneric'), 'error');
                }
            } catch (e) {
                console.error('Failed to load trend', e);
            } finally {
                if (requestId === this.trendRequestId) {
                    this.loadingTrend = false;
                    this.trendRequestKey = '';
                }
            }
        },

        async loadInstallation(projectId, poll = false) {
            if (!projectId || this.loadingInstallation) return;
            const requestId = ++this.installationRequestId;
            this.loadingInstallation = true;
            try {
                const res = await fetch(`/api/v1/projects/${projectId}/installation`, { credentials: 'same-origin' });
                const data = await res.json();
                if (requestId !== this.installationRequestId || this.selectedProject?.id !== projectId) return;
                if (data.success) {
                    this.installation = { ...createEmptyInstallation(), ...data.data };
                    if (this.installation.status === 'installed') {
                        this.stopInstallationPolling();
                    } else if (poll && this.showCodeModal) {
                        this.startInstallationPolling(projectId);
                    }
                }
            } catch (e) {
                console.error('Failed to load installation status', e);
            } finally {
                if (requestId === this.installationRequestId) this.loadingInstallation = false;
            }
        },

		async loadRealtime(projectId) {
			if (!projectId) return;
			const requestId = ++this.realtimeRequestId;
			this.loadingRealtime = true;
			try {
				const res = await fetch(`/api/v1/projects/${projectId}/stats/realtime`, { credentials: 'same-origin' });
				const data = await res.json();
				if (requestId === this.realtimeRequestId && data.success && this.selectedProject?.id === projectId) {
					this.projectRealtime = { ...createEmptyRealtime(), ...data.data };
				}
			} catch (e) {
				console.error('Failed to load realtime statistics', e);
			} finally {
				if (requestId === this.realtimeRequestId) this.loadingRealtime = false;
			}
		},

		async loadAnalysis(projectId, force = false) {
			if (!projectId) return;
			const requestId = ++this.analysisRequestId;
			const requestedDays = this.trendDays;
			if (!force) this.projectAnalysis = createEmptyAnalysis(requestedDays);
			this.loadingAnalysis = true;
			try {
				const res = await fetch(`/api/v1/projects/${projectId}/analysis?days=${requestedDays}`, { credentials: 'same-origin' });
				const data = await res.json();
				if (requestId !== this.analysisRequestId || this.selectedProject?.id !== projectId) return;
				if (data.success) {
					this.projectAnalysis = {
						...createEmptyAnalysis(requestedDays), ...data.data,
						current: { ...createEmptyAnalysis(requestedDays).current, ...(data.data.current || {}) },
						previous: { ...createEmptyAnalysis(requestedDays).previous, ...(data.data.previous || {}) },
						changes: { ...createEmptyAnalysis(requestedDays).changes, ...(data.data.changes || {}) },
						breakdowns: { ...createEmptyAnalysis(requestedDays).breakdowns, ...(data.data.breakdowns || {}) }
					};
				}
			} catch (e) {
				console.error('Failed to load project analysis', e);
			} finally {
				if (requestId === this.analysisRequestId) this.loadingAnalysis = false;
			}
		},

		async loadSessionQuality(projectId, force = false) {
			if (!projectId) return;
			const requestId = ++this.sessionQualityRequestId;
			const requestedDays = this.trendDays;
			if (!force) this.sessionQuality = createEmptySessionQuality(requestedDays);
			this.loadingSessionQuality = true;
			try {
				const response = await fetch(`/api/v1/projects/${projectId}/session-quality?days=${requestedDays}`, { credentials: 'same-origin' });
				const data = await response.json();
				if (requestId !== this.sessionQualityRequestId || this.selectedProject?.id !== projectId) return;
				if (data.success) {
					this.sessionQuality = {
						...createEmptySessionQuality(requestedDays), ...data.data,
						totals: { ...createEmptySessionQuality(requestedDays).totals, ...(data.data.totals || {}) },
						segments: { ...createEmptySessionQuality(requestedDays).segments, ...(data.data.segments || {}) }
					};
				}
			} catch (e) {
				console.error('Failed to load session quality', e);
			} finally {
				if (requestId === this.sessionQualityRequestId) this.loadingSessionQuality = false;
			}
		},

		async loadConversions(projectId) {
			if (!projectId) return;
			this.loadingConversions = true;
			try {
				const [eventsResponse, goalsResponse, funnelsResponse] = await Promise.all([
					fetch(`/api/v1/projects/${projectId}/events?days=${this.trendDays}`, { credentials: 'same-origin' }),
					fetch(`/api/v1/projects/${projectId}/goals`, { credentials: 'same-origin' }),
					fetch(`/api/v1/projects/${projectId}/funnels`, { credentials: 'same-origin' })
				]);
				const [events, goals, funnels] = await Promise.all([eventsResponse.json(), goalsResponse.json(), funnelsResponse.json()]);
				if (this.selectedProject?.id !== projectId) return;
				if (events.success) {
					const emptyReport = createEmptyEventReport(this.trendDays);
					this.eventReport = { ...emptyReport, ...events.data, quality: { ...emptyReport.quality, ...(events.data.quality || {}) } };
					if (!this.eventReport.events.some(event => event.name === this.selectedEventName)) this.selectedEventName = this.eventReport.events[0]?.name || '';
				}
				if (goals.success) this.conversionGoals = goals.data || [];
				if (funnels.success) this.funnels = funnels.data || [];
				const reports = await Promise.all(this.funnels.map(async funnel => {
					const response = await fetch(`/api/v1/projects/${projectId}/funnels/${funnel.id}/analysis?days=${this.trendDays}`, { credentials: 'same-origin' });
					const data = await response.json();
					return [funnel.id, data.success ? data.data : { steps: [] }];
				}));
				this.funnelReports = Object.fromEntries(reports);
			} catch (e) { console.error('Failed to load conversions', e); }
			finally { this.loadingConversions = false; }
		},

		async loadIssues(projectId, force = false) {
			if (!projectId) return;
			const requestId = ++this.issueRequestId;
			if (!force) this.issueReport = createEmptyIssueReport(this.trendDays);
			this.loadingIssues = true;
			try {
				const response = await fetch(`/api/v1/projects/${projectId}/issues?days=${this.trendDays}`, { credentials: 'same-origin' });
				const data = await response.json();
				if (requestId !== this.issueRequestId || this.selectedProject?.id !== projectId) return;
				if (data.success) this.issueReport = { ...createEmptyIssueReport(this.trendDays), ...data.data };
			} catch (error) {
				console.error('Failed to load issue report', error);
			} finally {
				if (requestId === this.issueRequestId) this.loadingIssues = false;
			}
		},

		async loadDiagnostics(projectId) {
			if (!projectId || this.loadingDiagnostics) return;
			this.loadingDiagnostics = true;
			try {
				const response = await fetch(`/api/v1/projects/${projectId}/diagnostics`, { credentials: 'same-origin' });
				const data = await response.json();
				if (data.success && this.selectedProject?.id === projectId) this.diagnostics = { ...createEmptyDiagnostics(), ...data.data };
			} catch (e) { console.error('Failed to load diagnostics', e); }
			finally { this.loadingDiagnostics = false; }
		},

		async clearDiagnostics() {
			if (!this.selectedProject) return;
			await fetch(`/api/v1/projects/${this.selectedProject.id}/diagnostics`, { method: 'DELETE', credentials: 'same-origin' });
			this.diagnostics = createEmptyDiagnostics();
			await this.loadDiagnostics(this.selectedProject.id);
		},

		startDiagnosticsPolling(projectId) {
			this.stopDiagnosticsPolling();
			this.diagnosticsPollTimer = setInterval(() => {
				if (this.currentPage === 'project-detail' && this.selectedProject?.id === projectId) this.loadDiagnostics(projectId);
			}, 5000);
		},

		stopDiagnosticsPolling() {
			if (this.diagnosticsPollTimer) clearInterval(this.diagnosticsPollTimer);
			this.diagnosticsPollTimer = null;
		},

		getDiagnosticStatusClass() {
			return this.diagnostics.status === 'healthy' ? 'border-green-200 bg-green-50 text-green-800' : (this.diagnostics.status === 'error' ? 'border-red-200 bg-red-50 text-red-800' : 'border-amber-200 bg-amber-50 text-amber-800');
		},

		getDiagnosticType(entry) { return entry.type === 'event' ? (entry.eventName || 'event') : 'pageview'; },

		async createGoal() {
			if (!this.selectedProject) return;
			const response = await fetch(`/api/v1/projects/${this.selectedProject.id}/goals`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, credentials: 'same-origin', body: JSON.stringify(this.newGoal) });
			const data = await response.json();
			if (!data.success) { this.showToast(data.error || this.t('errorGeneric'), 'error'); return; }
			this.newGoal = { name: '', eventName: '' };
			await this.loadConversions(this.selectedProject.id);
		},

		async deleteGoal(id) {
			if (!this.selectedProject) return;
			await fetch(`/api/v1/projects/${this.selectedProject.id}/goals/${id}`, { method: 'DELETE', credentials: 'same-origin' });
			await this.loadConversions(this.selectedProject.id);
		},

		async createFunnel() {
			if (!this.selectedProject) return;
			const steps = this.newFunnel.stepsText.split(/[\n,]+/).map(value => value.trim()).filter(Boolean);
			const response = await fetch(`/api/v1/projects/${this.selectedProject.id}/funnels`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, credentials: 'same-origin', body: JSON.stringify({ name: this.newFunnel.name, steps }) });
			const data = await response.json();
			if (!data.success) { this.showToast(data.error || this.t('errorGeneric'), 'error'); return; }
			this.newFunnel = { name: '', stepsText: '' };
			await this.loadConversions(this.selectedProject.id);
		},

		async deleteFunnel(id) {
			if (!this.selectedProject) return;
			await fetch(`/api/v1/projects/${this.selectedProject.id}/funnels/${id}`, { method: 'DELETE', credentials: 'same-origin' });
			await this.loadConversions(this.selectedProject.id);
		},

		getEvent(name) { return (this.eventReport.events || []).find(event => event.name === name) || { count: 0, visitors: 0, rate: 0, sources: {}, campaigns: {}, values: {}, details: {} }; },
		getSelectedEvent() { return this.getEvent(this.selectedEventName); },
		getSelectedEventDetails() { return Object.entries(this.getSelectedEvent().details || {}).sort(([left], [right]) => left.localeCompare(right)); },
		getWebVital(name) { return (this.eventReport.quality?.vitals || []).find(vital => vital.name === name) || { name, average: 0, samples: 0, visitors: 0, rating: 'no_data', distribution: [] }; },
		getQualityDetails(key) { return (this.eventReport.quality?.errorDetails?.[key] || []).slice(0, 3); },
		getQualityStatusClass(status) {
			return status === 'good' ? 'border-green-200 bg-green-50 text-green-700' : (status === 'poor' ? 'border-red-200 bg-red-50 text-red-700' : (status === 'needs_improvement' ? 'border-amber-200 bg-amber-50 text-amber-700' : 'border-gray-200 bg-gray-50 text-gray-500'));
		},
		formatVital(vital) {
			if (!vital?.samples) return '-';
			return vital.name === 'cls' ? this.formatDecimal(vital.average) : `${Math.round(vital.average)} ms`;
		},
		formatQualityDetails(key) { return this.getQualityDetails(key).map(detail => `${detail.value} (${detail.visitors})`).join(' · '); },
		getIssueClass(severity) { return severity === 'critical' ? 'border-red-200 bg-red-50' : 'border-amber-200 bg-amber-50'; },
		getIssueStatusClass(status) { return status === 'healthy' ? 'border-green-200 bg-green-50 text-green-700' : (status === 'critical' ? 'border-red-200 bg-red-50 text-red-700' : (status === 'warning' ? 'border-amber-200 bg-amber-50 text-amber-700' : 'border-gray-200 bg-gray-50 text-gray-500')); },
		formatIssueValue(issue, field) {
			const value = issue?.[field] || 0;
			if (issue.unit === 'percent') return this.formatPercent(value);
			if (issue.unit === 'milliseconds') return `${Math.round(value)} ms`;
			if (issue.unit === 'score') return this.formatDecimal(value);
			return Math.round(value).toLocaleString();
		},
		getIssueTitle(issue) { return this.t(`issue_${issue.code}_title`).replace('{subject}', issue.subject || ''); },
		getIssueAdvice(issue) { return this.t(`issue_${issue.code}_advice`).replace('{subject}', issue.subject || ''); },
		getEventTrendMax() { return Math.max(1, ...(this.getSelectedEvent().trend || []).flatMap(point => [point.count || 0, point.visitors || 0])); },
		getEventTrendPoints(metric) {
			const points = this.getSelectedEvent().trend || [];
			if (!points.length) return '';
			const max = this.getEventTrendMax();
			return points.map((point, index) => `${points.length === 1 ? 500 : (index / (points.length - 1)) * 1000},${220 - ((point[metric] || 0) / max) * 210}`).join(' ');
		},
		getConversionSegments(eventName) { return this.getEvent(eventName).segments?.[this.conversionDimension] || []; },
		getFunnelSegments(funnelId) { return this.funnelReports[funnelId]?.segments?.[this.funnelDimension] || []; },
		formatFunnelSteps(values) { return (values || []).join(' → '); },
		getTopKey(record) { const entries = this.getSortedEntries(record, 1); return entries.length ? entries[0][0] : '-'; },
		formatPercent(value) { return `${Math.round((value || 0) * 10) / 10}%`; },
		formatDecimal(value) { return Math.round((value || 0) * 10) / 10; },
		formatDurationSeconds(value) {
			const seconds = Math.max(0, Math.round(value || 0));
			if (seconds < 60) return `${seconds}s`;
			const minutes = Math.floor(seconds / 60);
			return seconds % 60 ? `${minutes}m ${seconds % 60}s` : `${minutes}m`;
		},
		getSessionQualitySegments() { return this.sessionQuality.segments?.[this.sessionQualityDimension] || []; },
		formatEventValues(values) { return Object.entries(values || {}).map(([currency, value]) => `${currency} ${Math.round(value * 100) / 100}`).join(' · ') || '-'; },

        startInstallationPolling(projectId) {
            if (this.installationPollTimer) return;
            this.installationPollTimer = setInterval(() => this.loadInstallation(projectId, true), 5000);
        },

        stopInstallationPolling() {
            if (this.installationPollTimer) clearInterval(this.installationPollTimer);
            this.installationPollTimer = null;
            this.installationRequestId++;
            this.loadingInstallation = false;
        },

        startDashboardAutoRefresh(projectId) {
            this.stopDashboardAutoRefresh();
            this.dashboardRefreshTimer = setInterval(() => {
                if (this.currentPage !== 'project-detail' || this.selectedProject?.id !== projectId) return;
                this.loadStats(projectId, true);
				this.loadTrend(projectId, true);
				this.loadRealtime(projectId);
				this.loadAnalysis(projectId, true);
				this.loadSessionQuality(projectId, true);
				this.loadConversions(projectId);
				this.loadIssues(projectId, true);
                this.loadInstallation(projectId);
            }, 30000);
			this.startDiagnosticsPolling(projectId);
        },

        stopDashboardAutoRefresh() {
            if (this.dashboardRefreshTimer) clearInterval(this.dashboardRefreshTimer);
            this.dashboardRefreshTimer = null;
        }
    };
}

// Load all components after DOM is ready
async function loadComponents() {
	// Function to load and insert a single component
	async function loadAndInsert(path, containerId) {
		try {
			const response = await fetch(path);
			const html = await response.text();
			const container = document.getElementById(containerId);
			if (container) {
				container.innerHTML = html;
			}
		} catch (e) {
			console.error('Failed to load component:', path, e);
		}
	}

	// Load navbar
	await loadAndInsert('/components/Navbar.html', 'navbar-container');

	// Load all pages
	const [home, login, register, dashboard, dashboardProject] = await Promise.all([
		fetch('/components/pages/Home.html').then(r => r.text()),
		fetch('/components/pages/Login.html').then(r => r.text()),
		fetch('/components/pages/Register.html').then(r => r.text()),
		fetch('/components/pages/Dashboard.html').then(r => r.text()),
		fetch('/components/pages/DashboardProject.html').then(r => r.text())
	]);
	const pageContainer = document.getElementById('page-container');
	if (pageContainer) {
		pageContainer.innerHTML = home + login + register + dashboard + dashboardProject;
	}

	// Load all modals
	const [createModal, detailModal] = await Promise.all([
		fetch('/components/CreateProjectModal.html').then(r => r.text()),
		fetch('/components/ProjectDetailModal.html').then(r => r.text())
	]);
	const modalsContainer = document.getElementById('modals-container');
	if (modalsContainer) {
		modalsContainer.innerHTML = createModal + detailModal;
	}
}

// Start loading components when DOM is ready
document.addEventListener('DOMContentLoaded', loadComponents);
