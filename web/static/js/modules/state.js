export function createEmptyProjectStats() {
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

export function createEmptyVisitorPage() {
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

export function createEmptyTrend(days = 30) {
    return {
        projectId: '',
        days,
        startDate: '',
        endDate: '',
        points: [],
        totals: { pv: 0, uv: 0, requests: 0, bots: 0 }
    };
}

export function createEmptyInstallation() {
    return { projectId: '', status: 'pending', firstSeenAt: null, lastSeenAt: null };
}

export function createEmptyRealtime() {
    return { projectId: '', pv5: 0, pv30: 0, visitors5: 0, visitors30: 0, lastSeenAt: null };
}

export function createEmptyAnalysis(days = 30) {
    return {
        projectId: '', days, startDate: '', endDate: '',
        current: { pv: 0, uv: 0, requests: 0, bots: 0 },
        previous: { pv: 0, uv: 0, requests: 0, bots: 0 },
        changes: { pv: null, uv: null, requests: null, bots: null },
        breakdowns: { paths: {}, referrers: {}, countries: {}, devices: {}, browsers: {}, sources: {}, mediums: {}, campaigns: {} }
    };
}

export function createEmptySessionQuality(days = 30) {
    return {
        projectId: '', days, startDate: '', endDate: '',
        totals: { sessions: 0, bounces: 0, pageviews: 0, durationSeconds: 0, bounceRate: 0, averagePages: 0, averageDurationSeconds: 0 },
        entrances: [], exits: [], flows: [], segments: { source: [], medium: [], campaign: [], device: [], country: [] }
    };
}

export function createEmptyEventReport(days = 30) {
    return {
        projectId: '', days, startDate: '', endDate: '', visitors: 0, events: [],
        quality: { status: 'no_data', vitals: [], javascript: { count: 0, visitors: 0 }, resources: { count: 0, visitors: 0 }, errorDetails: {} }
    };
}

export function createEmptyIssueReport(days = 30) {
    return { projectId: '', days, status: 'insufficient_data', evaluatedAt: '', issues: [] };
}

export function createEmptyDiagnostics() {
    return { projectId: '', status: 'warning', accepted: 0, rejected: 0, testEvents: 0, lastAcceptedAt: null, lastRejectedAt: null, issues: [], entries: [] };
}

export function createDefaultVisitorFilters() {
    return {
        device: '',
        browser: '',
        path: '',
        sort: 'last_seen_desc',
        pageSize: 20
    };
}

export function createInitialState() {
    const lang = localStorage.getItem('svgstat-lang')
        || ((navigator.language || 'en').toLowerCase().startsWith('en') ? 'en' : 'zh');

    return {
        lang,
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
    };
}
