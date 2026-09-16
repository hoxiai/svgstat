import {
    createEmptyInstallation,
    createEmptyProjectStats,
    createEmptyVisitorPage,
    createDefaultVisitorFilters,
    createDefaultBreakdownPages
} from './state.js';

export function createRouterMethods() {
    return {
        handleHashChange() {
            if (this.currentPage !== 'project-detail' || !this.selectedProject) return;
            const rawHash = window.location.hash.replace(/^#/, '');
            const validTabs = ['overview', 'growth', 'quality', 'visitors', 'diagnostics'];
            const targetTab = validTabs.includes(rawHash) ? rawHash : 'overview';
            if (this.projectTab !== targetTab) {
                this.setProjectTab(targetTab);
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
                const slug = path.substring('/dashboard/'.length);
                this.currentPage = 'project-detail';
                this.selectedProject = null;
                this.installation = createEmptyInstallation();
                this.showCodeModal = false;
                this.expandedVisitorId = null;
                this.lastLoadedVisitorsRequestKey = '';
                this.visitorFilters = createDefaultVisitorFilters();
                this.breakdownPages = createDefaultBreakdownPages();
                this.diagnosticsPage = 1;
                this.eventsPage = 1;
                this.sessionSegmentsPage = 1;

                const rawHash = window.location.hash.replace(/^#/, '');
                const validTabs = ['overview', 'growth', 'quality', 'visitors', 'diagnostics'];
                this.projectTab = validTabs.includes(rawHash) ? rawHash : 'overview';

                if (this.projects.length > 0) {
                    const project = this.projects.find(p => p.slug === slug);
                    if (project) {
                        this.selectedProject = project;
                        this.installation = createEmptyInstallation();
                        this.loadTabAnalytics(project.id, this.projectTab);
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
