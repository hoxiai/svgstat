export function createFormatterMethods() {
    return {
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

        getPaginatedEntries(record, page = 1, pageSize = 8) {
            const entries = this.getSortedEntries(record);
            const start = Math.max(0, (page - 1) * pageSize);
            return entries.slice(start, start + pageSize);
        },

        getPaginatedList(list, page = 1, pageSize = 8) {
            if (!Array.isArray(list)) return [];
            const start = Math.max(0, (page - 1) * pageSize);
            return list.slice(start, start + pageSize);
        },

        getTotalPages(totalItems, pageSize = 8) {
            if (!totalItems) return 1;
            const count = typeof totalItems === 'number'
                ? totalItems
                : (Array.isArray(totalItems) ? totalItems.length : Object.keys(totalItems).length);
            return Math.max(1, Math.ceil(count / pageSize));
        },

        getTotalCount(items) {
            if (!items) return 0;
            if (typeof items === 'number') return items;
            if (Array.isArray(items)) return items.length;
            return Object.keys(items).length;
        },

        changeBreakdownPage(key, delta, record, pageSize = 8) {
            const current = this.breakdownPages[key] || 1;
            const total = this.getTotalPages(record, pageSize);
            const target = current + delta;
            if (target >= 1 && target <= total) {
                this.breakdownPages[key] = target;
            }
        },

        changeListPage(pageProp, delta, list, pageSize = 8) {
            const current = this[pageProp] || 1;
            const total = this.getTotalPages(list, pageSize);
            const target = current + delta;
            if (target >= 1 && target <= total) {
                this[pageProp] = target;
            }
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

        getDiagnosticStatusClass() {
            return this.diagnostics.status === 'healthy' ? 'border-green-200 bg-green-50 text-green-800' : (this.diagnostics.status === 'error' ? 'border-red-200 bg-red-50 text-red-800' : 'border-amber-200 bg-amber-50 text-amber-800');
        },

        getDiagnosticType(entry) { return entry.type === 'event' ? (entry.eventName || 'event') : 'pageview'; },

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

        formatEventValues(values) { return Object.entries(values || {}).map(([currency, value]) => `${currency} ${Math.round(value * 100) / 100}`).join(' · ') || '-'; }
    };
}
