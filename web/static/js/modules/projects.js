import { createEmptyInstallation } from './state.js';

export function createProjectMethods() {
    return {
        async loadProjects() {
            this.loading = true;
            try {
                const res = await fetch('/api/v1/projects', { credentials: 'same-origin' });
                const data = await res.json();
                if (data.success) {
                    this.projects = data.data;
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
        }
    };
}
