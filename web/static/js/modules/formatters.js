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

        // Channels are keyed "medium\u001fsource"; with no channel selected
        // every source is shown.
        getChannelSources() {
            const breakdowns = this.projectAnalysis.breakdowns || {};
            if (!this.sourceChannel) return breakdowns.sources || {};
            const prefix = this.sourceChannel + '\u001f';
            const result = {};
            Object.entries(breakdowns.channels || {}).forEach(([key, count]) => {
                if (key.startsWith(prefix)) result[key.slice(prefix.length)] = count;
            });
            return result;
        },

        selectSourceChannel(channel) {
            this.sourceChannel = channel;
            this.breakdownPages.channelSources = 1;
        },

        mediumLabel(medium) {
            const key = `medium_${medium}`;
            const label = this.t(key);
            return label === key ? medium : label;
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

        formatPercentageChange(val) {
            if (val === null || val === undefined) return '—';
            const num = Number(val);
            if (Number.isNaN(num)) return '—';
            const rounded = Math.round(num * 10) / 10;
            return `${rounded > 0 ? '+' : ''}${rounded}%`;
        },

        formatPercentageChangeWithArrow(val) {
            if (val === null || val === undefined) return '—';
            const num = Number(val);
            if (Number.isNaN(num)) return '—';
            const rounded = Math.round(num * 10) / 10;
            if (rounded > 0) return `↑ ${rounded}%`;
            if (rounded < 0) return `↓ ${Math.abs(rounded)}%`;
            return '0%';
        },

        getPercentageChangeColorClass(val) {
            if (val === null || val === undefined) return 'border-gray-200 bg-gray-50 text-gray-500';
            const num = Number(val);
            if (Number.isNaN(num)) return 'border-gray-200 bg-gray-50 text-gray-500';
            if (num > 0) return 'border-emerald-200 bg-emerald-50 text-emerald-700';
            if (num < 0) return 'border-rose-200 bg-rose-50 text-rose-700';
            return 'border-gray-200 bg-gray-50 text-gray-500';
        },

        getHourlyPoints(metric, todayHourly, yesterdayHourly, currentHour) {
            metric = metric || this.hourlyMetric || 'pv';
            const overview = this.projectOverview;
            todayHourly = todayHourly || overview?.todayHourly || {};
            yesterdayHourly = yesterdayHourly || overview?.yesterdayHourly || {};
            if (currentHour === undefined || currentHour === null) {
                currentHour = overview?.currentHour ?? 23;
            }

            const todayVals = [];
            for (let h = 0; h <= currentHour && h < 24; h++) {
                const k = String(h).padStart(2, '0');
                todayVals.push(todayHourly[k]?.[metric] || 0);
            }
            const yesterdayVals = [];
            for (let h = 0; h < 24; h++) {
                const k = String(h).padStart(2, '0');
                yesterdayVals.push(yesterdayHourly[k]?.[metric] || 0);
            }

            const max = Math.max(1, ...todayVals, ...yesterdayVals);

            const width = 1000;
            const height = 205;
            const chartHeight = 175;

            const getY = (val) => (height - (val / max) * chartHeight);
            const getX = (h) => (h / 23) * width;

            const todayPoints = [];
            for (let h = 0; h <= currentHour && h < 24; h++) {
                const k = String(h).padStart(2, '0');
                const val = todayHourly[k]?.[metric] || 0;
                todayPoints.push(`${getX(h).toFixed(1)},${getY(val).toFixed(1)}`);
            }

            const yesterdayPoints = [];
            for (let h = 0; h < 24; h++) {
                const k = String(h).padStart(2, '0');
                const val = yesterdayHourly[k]?.[metric] || 0;
                yesterdayPoints.push(`${getX(h).toFixed(1)},${getY(val).toFixed(1)}`);
            }

            return {
                today: todayPoints.join(' '),
                yesterday: yesterdayPoints.join(' '),
                max,
                currentHour
            };
        },

        getHourlyTodayPoints(metric) {
            return this.getHourlyPoints(metric).today;
        },

        getHourlyYesterdayPoints(metric) {
            return this.getHourlyPoints(metric).yesterday;
        },

        getHourlyMax(metric) {
            return this.getHourlyPoints(metric).max;
        },

        setHourlyMetric(metric) {
            this.hourlyMetric = metric;
        },

        formatSourceName(name) {
            if (!name || name === 'none' || name === 'direct' || name === 'Direct') return this.t('directTraffic');
            const key = `search_${name.toLowerCase()}`;
            const translated = this.t(key);
            if (translated !== key) return translated;
            return name;
        },

        formatSourceCategory(cat) {
            const key = `cat_${cat}`;
            const translated = this.t(key);
            return translated !== key ? translated : (cat || this.t('cat_other'));
        },

        formatSourceCategoryBadge(cat) {
            switch (cat) {
                case 'search': return 'bg-blue-50 text-blue-700 border-blue-200';
                case 'referral': return 'bg-emerald-50 text-emerald-700 border-emerald-200';
                case 'direct': return 'bg-gray-100 text-gray-700 border-gray-200';
                case 'social': return 'bg-purple-50 text-purple-700 border-purple-200';
                case 'ai': return 'bg-indigo-50 text-indigo-700 border-indigo-200';
                default: return 'bg-amber-50 text-amber-700 border-amber-200';
            }
        },

        formatLocationParts(country, region, city) {
            const parts = [country, region, city].filter(Boolean);
            return parts.length ? parts.join(' · ') : '—';
        },

        formatTimeOnly(value) {
            if (!value) return '-';
            const date = new Date(value);
            if (Number.isNaN(date.getTime())) return String(value);
            return date.toLocaleTimeString(this.lang === 'zh' ? 'zh-CN' : 'en-US', { hour12: false });
        },

        formatDateOnly(value) {
            if (!value) return '';
            const date = new Date(value);
            if (Number.isNaN(date.getTime())) return '';
            return date.toLocaleDateString(this.lang === 'zh' ? 'zh-CN' : 'en-US', { month: '2-digit', day: '2-digit' });
        },

        getTrafficCategories() {
            const breakdowns = this.projectAnalysis?.breakdowns || {};
            const channels = breakdowns.channels || {};
            const mediums = breakdowns.mediums || {};

            let search = 0;
            let referral = 0;
            let direct = 0;
            let other = 0;

            const searchEngines = new Set(['baidu', 'google', 'bing', '360', 'sogou', 'shenma', 'yahoo', 'duckduckgo', 'yandex', 'brave', 'naver', 'ecosia', 'toutiao']);

            if (Object.keys(channels).length > 0) {
                Object.entries(channels).forEach(([ch, count]) => {
                    const [med, src] = ch.split('\u001f');
                    const medium = (med || '').toLowerCase();
                    const source = (src || '').toLowerCase();
                    if (medium === 'organic' || medium === 'search' || searchEngines.has(source)) {
                        search += count;
                    } else if (medium === 'referral') {
                        referral += count;
                    } else if (medium === 'direct' || medium === 'none' || source === 'direct' || source === 'none' || !source) {
                        direct += count;
                    } else {
                        other += count;
                    }
                });
            } else {
                Object.entries(mediums).forEach(([med, count]) => {
                    const m = med.toLowerCase();
                    if (m === 'organic' || m === 'search') search += count;
                    else if (m === 'referral') referral += count;
                    else if (m === 'direct' || m === 'none' || m === '') direct += count;
                    else other += count;
                });
            }

            const total = search + referral + direct + other;
            const calcPct = (count) => (total > 0 ? Math.round((count / total) * 1000) / 10 : 0);

            return {
                total,
                search: { count: search, pct: calcPct(search) },
                referral: { count: referral, pct: calcPct(referral) },
                direct: { count: direct, pct: calcPct(direct) },
                other: { count: other, pct: calcPct(other) }
            };
        },

        getTopSearchEngines(limit = 5) {
            const breakdowns = this.projectAnalysis?.breakdowns || {};
            const sources = breakdowns.sources || {};
            const searchEngines = ['baidu', 'google', 'bing', '360', 'sogou', 'shenma', 'yahoo', 'duckduckgo', 'yandex', 'brave', 'naver'];

            const items = [];
            Object.entries(sources).forEach(([src, count]) => {
                const lower = src.toLowerCase();
                if (searchEngines.includes(lower)) {
                    items.push({
                        key: lower,
                        name: this.formatSourceName(lower),
                        count
                    });
                }
            });

            items.sort((a, b) => b.count - a.count);
            const top = items.slice(0, limit);
            const max = top.length ? top[0].count : 1;
            return top.map(item => ({
                ...item,
                pct: max > 0 ? (item.count / max) * 100 : 0
            }));
        },

        getTopExternalReferrers(limit = 10) {
            const breakdowns = this.projectAnalysis?.breakdowns || {};
            const referrers = breakdowns.referrers || {};
            const items = [];

            Object.entries(referrers).forEach(([url, count]) => {
                if (!url || url === 'Direct' || url === 'direct' || url === 'none') return;
                let domain = url;
                try {
                    const parsed = new URL(url.includes('://') ? url : `https://${url}`);
                    domain = parsed.hostname;
                } catch (_) {}

                items.push({
                    url,
                    domain,
                    count
                });
            });

            items.sort((a, b) => b.count - a.count);
            const top = items.slice(0, limit);
            const max = top.length ? top[0].count : 1;
            return top.map(item => ({
                ...item,
                pct: max > 0 ? (item.count / max) * 100 : 0
            }));
        },

        getTopSearchKeywords(limit = 10) {
            const breakdowns = this.projectAnalysis?.breakdowns || {};
            const terms = breakdowns.terms || {};
            const entries = Object.entries(terms).sort((a, b) => b[1] - a[1]);
            return entries.slice(0, limit).map(([term, count]) => ({ term, count }));
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
