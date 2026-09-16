import { createInitialState } from './modules/state.js';
import { createFormatterMethods } from './modules/formatters.js';
import { createRouterMethods } from './modules/router.js';
import { createAuthMethods } from './modules/auth.js';
import { createProjectMethods } from './modules/projects.js';
import { createAnalyticsMethods } from './modules/analytics.js';
import { translations } from './translations.js';

export function spaApp() {
    return {
        ...createInitialState(),
        ...createFormatterMethods(),
        ...createRouterMethods(),
        ...createAuthMethods(),
        ...createProjectMethods(),
        ...createAnalyticsMethods(),

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
        }
    };
}

if (typeof window !== 'undefined') {
    window.spaApp = spaApp;
    document.addEventListener('alpine:init', () => {
        if (window.Alpine) {
            window.Alpine.data('spaApp', spaApp);
        }
    });
}
