import {
    createEmptyInstallation,
    createEmptyRealtime,
    createEmptyAnalysis,
    createEmptySessionQuality,
    createEmptyEventReport,
    createEmptyIssueReport,
    createEmptyDiagnostics,
    createEmptyProjectStats,
    createEmptyProjectOverview,
    createEmptyTrend,
    createEmptyVisitorPage,
    createDefaultVisitorFilters
} from './state.js';

export function createAnalyticsMethods() {
    return {
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

        async loadProjectOverview(projectId, force = false, isSilent = false) {
            if (!projectId) return;
            const requestId = ++this.overviewRequestId;
            if (!isSilent && !this.projectOverview) {
                this.loadingOverview = true;
            }
            try {
                const res = await fetch(`/api/v1/projects/${projectId}/stats/overview`, { credentials: 'same-origin' });
                const data = await res.json();
                if (requestId !== this.overviewRequestId || this.selectedProject?.id !== projectId) return;
                if (data.success) {
                    this.projectOverview = {
                        ...createEmptyProjectOverview(),
                        ...data.data,
                        yesterdayFull: { ...createEmptyProjectOverview().yesterdayFull, ...(data.data.yesterdayFull || {}) },
                        yesterdaySamePeriod: { ...createEmptyProjectOverview().yesterdaySamePeriod, ...(data.data.yesterdaySamePeriod || {}) },
                        changes: { ...createEmptyProjectOverview().changes, ...(data.data.changes || {}) },
                        todayHourly: data.data.todayHourly || {},
                        yesterdayHourly: data.data.yesterdayHourly || {}
                    };
                }
            } catch (e) {
                console.error('Failed to load project overview', e);
            } finally {
                if (requestId === this.overviewRequestId) this.loadingOverview = false;
            }
        },

        async loadVisitStream(projectId, isSilent = false) {
            if (!projectId) return;
            const requestId = ++this.visitStreamRequestId;
            if (!isSilent && (!this.visitStream || this.visitStream.length === 0)) {
                this.loadingVisitStream = true;
            }
            try {
                const res = await fetch(`/api/v1/projects/${projectId}/visit-stream?limit=50`, { credentials: 'same-origin' });
                const data = await res.json();
                if (requestId !== this.visitStreamRequestId || this.selectedProject?.id !== projectId) return;
                if (data.success) {
                    const incoming = data.data || [];
                    if (this.lastLoadedVisitStreamProjectId !== projectId || !this.visitStream || this.visitStream.length === 0) {
                        this.lastLoadedVisitStreamProjectId = projectId;
                        this.visitStream = incoming;
                    } else {
                        const existingIds = new Set(this.visitStream.map(item => item.id));
                        const newItems = incoming.filter(item => !existingIds.has(item.id));
                        if (newItems.length > 0) {
                            this.visitStream = [...newItems, ...this.visitStream].slice(0, 50);
                        }
                    }
                }
            } catch (e) {
                console.error('Failed to load visit stream', e);
            } finally {
                if (requestId === this.visitStreamRequestId) this.loadingVisitStream = false;
            }
        },

        startVisitStreamPolling(projectId) {
            this.stopVisitStreamPolling();
            if (!this.autoRefreshStream) return;
            this.streamInterval = setInterval(() => {
                if (this.currentPage === 'project-detail' && this.selectedProject?.id === projectId && this.projectTab === 'visitors' && this.visitorViewMode === 'stream' && this.autoRefreshStream) {
                    this.loadVisitStream(projectId, true);
                }
            }, 5000);
        },

        stopVisitStreamPolling() {
            if (this.streamInterval) {
                clearInterval(this.streamInterval);
                this.streamInterval = null;
            }
        },

        toggleAutoRefreshStream() {
            this.autoRefreshStream = !this.autoRefreshStream;
            if (this.selectedProject) {
                if (this.autoRefreshStream && this.projectTab === 'visitors' && this.visitorViewMode === 'stream') {
                    this.loadVisitStream(this.selectedProject.id);
                    this.startVisitStreamPolling(this.selectedProject.id);
                } else {
                    this.stopVisitStreamPolling();
                }
            }
        },

        switchVisitorViewMode(mode) {
            this.visitorViewMode = mode;
            if (!this.selectedProject) return;
            if (mode === 'ip') {
                this.stopVisitStreamPolling();
                this.loadVisitors(this.selectedProject.id);
            } else if (mode === 'stream') {
                if (!this.visitStream || this.visitStream.length === 0) {
                    this.loadVisitStream(this.selectedProject.id);
                }
                if (this.autoRefreshStream) {
                    this.startVisitStreamPolling(this.selectedProject.id);
                }
            }
        },

        getFilteredVisitStream() {
            if (!this.visitStream || !Array.isArray(this.visitStream)) return [];
            if (this.streamFilter === 'ai') {
                return this.visitStream.filter(item => Boolean(item.isAiAgent));
            }
            if (this.streamFilter === 'human') {
                return this.visitStream.filter(item => !item.isAiAgent);
            }
            return this.visitStream;
        },

        async loadAnalysis(projectId, force = false, isSilent = false) {
            if (!projectId) return;
            const requestId = ++this.analysisRequestId;
            const requestedDays = this.trendDays;
            if (!isSilent && !this.projectAnalysis) {
                this.loadingAnalysis = true;
            }
            if (!this.projectAnalysis) {
                this.projectAnalysis = createEmptyAnalysis(requestedDays);
            }
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
        getConversionSegments(eventName) { return this.getEvent(eventName).segments?.[this.conversionDimension] || []; },
        getFunnelSegments(funnelId) { return this.funnelReports[funnelId]?.segments?.[this.funnelDimension] || []; },
        getSessionQualitySegments() { return this.sessionQuality.segments?.[this.sessionQualityDimension] || []; },

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

        setProjectTab(tab) {
            if (!tab) return;
            const validTabs = ['overview', 'growth', 'quality', 'visitors', 'diagnostics'];
            const targetTab = validTabs.includes(tab) ? tab : 'overview';
            this.projectTab = targetTab;
            if (window.location.hash !== `#${targetTab}`) {
                window.location.hash = targetTab;
            }
            if (this.selectedProject) {
                this.loadTabAnalytics(this.selectedProject.id, targetTab);
                if (targetTab === 'diagnostics') {
                    this.startDiagnosticsPolling(this.selectedProject.id);
                } else {
                    this.stopDiagnosticsPolling();
                }
                if (targetTab === 'visitors' && this.visitorViewMode === 'stream' && this.autoRefreshStream) {
                    this.startVisitStreamPolling(this.selectedProject.id);
                } else {
                    this.stopVisitStreamPolling();
                }
            }
        },

        loadTabAnalytics(projectId, tab = 'overview', isRefresh = false) {
            if (!projectId) return;
            this.loadInstallation(projectId);
            this.loadRealtime(projectId);

            if (tab === 'overview') {
                this.loadProjectOverview(projectId, isRefresh);
                this.loadStats(projectId, isRefresh);
                this.loadTrend(projectId, isRefresh);
                this.loadAnalysis(projectId, isRefresh);
                this.loadIssues(projectId, isRefresh);
            } else if (tab === 'growth') {
                this.loadConversions(projectId);
                this.loadAnalysis(projectId, isRefresh);
            } else if (tab === 'quality') {
                this.loadSessionQuality(projectId, isRefresh);
                this.loadConversions(projectId);
            } else if (tab === 'visitors') {
                if (this.visitorViewMode === 'ip') {
                    this.loadVisitors(projectId, 1, isRefresh);
                } else {
                    this.loadVisitStream(projectId);
                    if (this.autoRefreshStream) {
                        this.startVisitStreamPolling(projectId);
                    }
                }
            } else if (tab === 'diagnostics') {
                this.loadDiagnostics(projectId);
                this.loadStats(projectId, isRefresh);
            }
        },

        startDashboardAutoRefresh(projectId) {
            this.stopDashboardAutoRefresh();
            this.dashboardRefreshTimer = setInterval(() => {
                if (this.currentPage !== 'project-detail' || this.selectedProject?.id !== projectId) return;
                this.loadRealtime(projectId);
                this.loadInstallation(projectId);
                if (this.projectTab === 'overview') {
                    this.loadProjectOverview(projectId, true, true);
                    this.loadStats(projectId, true, true);
                    this.loadTrend(projectId, true, true);
                    this.loadAnalysis(projectId, true, true);
                    this.loadIssues(projectId, true);
                } else if (this.projectTab === 'growth') {
                    this.loadConversions(projectId);
                    this.loadAnalysis(projectId, true, true);
                } else if (this.projectTab === 'quality') {
                    this.loadSessionQuality(projectId, true);
                    this.loadConversions(projectId);
                } else if (this.projectTab === 'diagnostics') {
                    this.loadDiagnostics(projectId);
                } else if (this.projectTab === 'visitors') {
                    if (this.visitorViewMode === 'ip') {
                        this.loadVisitors(projectId, this.visitorsPage?.page || 1, true);
                    } else {
                        this.loadVisitStream(projectId, true);
                    }
                }
            }, 30000);
            if (this.projectTab === 'diagnostics') {
                this.startDiagnosticsPolling(projectId);
            }
            if (this.projectTab === 'visitors' && this.visitorViewMode === 'stream' && this.autoRefreshStream) {
                this.startVisitStreamPolling(projectId);
            }
        },

        stopDashboardAutoRefresh() {
            if (this.dashboardRefreshTimer) clearInterval(this.dashboardRefreshTimer);
            this.dashboardRefreshTimer = null;
            this.stopVisitStreamPolling();
        },

        setTrendDays(days) {
            if (this.trendDays === days || !this.selectedProject) return;
            this.trendDays = days;
            const pid = this.selectedProject.id;
            this.loadTabAnalytics(pid, this.projectTab, true);
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

        async loadVisitors(projectId, page = 1, force = false) {
            if (!projectId) return;

            const pageSize = this.visitorFilters?.pageSize || 20;
            const queryString = this.getVisitorQueryString ? this.getVisitorQueryString(page) : `page=${page}&page_size=${pageSize}`;
            const fullRequestKey = `${projectId}:${queryString}`;
            if (!force && this.lastLoadedVisitorsRequestKey === fullRequestKey) return;

            this.lastLoadedVisitorsRequestKey = fullRequestKey;
            this.loadingVisitors = true;
            this.loadingVisitorsPage = true;
            this.expandedVisitorId = null;
            if (!this.visitorsPage || !this.visitorsPage.items || this.visitorsPage.items.length === 0) {
                this.visitorsPage = {
                    ...createEmptyVisitorPage(),
                    page,
                    pageSize
                };
            }

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
                this.loadingVisitorsPage = false;
            }
        },

        changeVisitorPage(nextPage) {
            if (!this.selectedProject) return;
            if (nextPage < 1) return;
            if (this.visitorsPage.totalPages > 0 && nextPage > this.visitorsPage.totalPages) return;
            this.loadVisitors(this.selectedProject.id, nextPage);
        },
        
        async loadStats(projectId, force = false, isSilent = false) {
            if (!projectId) return;
            if (!force && this.lastLoadedStatsProjectId === projectId) return;

            this.lastLoadedStatsProjectId = projectId;
            if (!isSilent && (!this.projectStats || !this.projectStats.pv)) {
                this.loadingStats = true;
            }
            if (!this.projectStats) {
                this.projectStats = createEmptyProjectStats();
            }
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

        async loadTrend(projectId, force = false, isSilent = false) {
            if (!projectId) return;
            const requestedDays = this.trendDays;
            const requestKey = `${projectId}:${requestedDays}`;
            if (!force && this.loadingTrend && this.trendRequestKey === requestKey) return;
            const requestId = ++this.trendRequestId;
            this.trendRequestKey = requestKey;
            if (!isSilent && (!this.projectTrend || !this.projectTrend.totals)) {
                this.loadingTrend = true;
            }
            if (!this.projectTrend) {
                this.projectTrend = createEmptyTrend(requestedDays);
            }
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
        }
    };
}
