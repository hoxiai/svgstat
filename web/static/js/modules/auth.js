export function createAuthMethods() {
    return {
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
        }
    };
}
