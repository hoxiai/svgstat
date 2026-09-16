import {
    createEmptyInstallation,
    createEmptyProjectStats,
    createEmptyVisitorPage,
    createDefaultVisitorFilters
} from './state.js';

export function createRouterMethods() {
    return {
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
                const slug = path.substring('/dashboard/'.length);
                this.currentPage = 'project-detail';
                this.selectedProject = null;
                this.installation = createEmptyInstallation();
                this.showCodeModal = false;
                this.expandedVisitorId = null;
                this.lastLoadedVisitorsRequestKey = '';
                this.visitorFilters = createDefaultVisitorFilters();
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
        }
    };
}
