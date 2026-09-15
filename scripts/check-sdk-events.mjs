import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const requests = [];
const storage = new Map();
const context = {
  URL, URLSearchParams, Date, Math, setTimeout: callback => callback(),
  location: { pathname: '/pricing', search: '?utm_source=newsletter&utm_medium=email&utm_campaign=launch', hash: '', origin: 'https://site.example' },
  document: { currentScript: { src: 'https://stats.example/sdk.js', getAttribute: name => name === 'data-project' ? 'demo' : null }, referrer: 'https://search.example/results' },
  navigator: { sendBeacon: (_url, body) => { requests.push(Object.fromEntries(body.entries())); return true; } },
  sessionStorage: { getItem: key => storage.get(key) || null, setItem: (key, value) => storage.set(key, value) },
  crypto: { randomUUID: () => 'visitor-1' },
  history: { pushState() {}, replaceState() {} },
  addEventListener() {},
  fetch() { throw new Error('sendBeacon should be used'); }
};
context.window = context;
vm.runInNewContext(fs.readFileSync(new URL('../web/static/js/sdk.js', import.meta.url), 'utf8'), context);

assert.equal(requests.length, 1);
assert.equal(requests[0].type, 'pageview');
assert.equal(requests[0].mode, 'live');
context.window.svgstat('event', 'signup');
context.window.svgstat('event', 'purchase', { value: 99, currency: 'CNY', nested: { ignored: true } });
assert.equal(requests.length, 3);
assert.equal(requests[1].event, 'signup');
assert.equal(requests[1].path, '/pricing?utm_source=newsletter&utm_medium=email&utm_campaign=launch');
assert.deepEqual(JSON.parse(requests[2].properties), { value: 99, currency: 'CNY' });
assert.equal(requests[2].visitor, 'visitor-1');
assert.equal(requests.filter(request => request.type === 'pageview').length, 1);

const testRequests = [];
const testContext = {
  ...context,
  document: { currentScript: { src: 'https://stats.example/sdk.js', getAttribute: name => name === 'data-project' ? 'test-project' : (name === 'data-mode' ? 'test' : null) }, referrer: '' },
  navigator: { sendBeacon: (_url, body) => { testRequests.push(Object.fromEntries(body.entries())); return true; } },
  sessionStorage: { getItem: () => null, setItem() {} },
  __svgstatProjects: {},
  svgstat: undefined
};
testContext.window = testContext;
vm.runInNewContext(fs.readFileSync(new URL('../web/static/js/sdk.js', import.meta.url), 'utf8'), testContext);
assert.equal(testRequests.length, 1);
assert.equal(testRequests[0].mode, 'test');

const invalidModeRequests = [];
const invalidModeContext = {
  ...context,
  document: { currentScript: { src: 'https://stats.example/sdk.js', getAttribute: name => name === 'data-project' ? 'invalid-mode-project' : (name === 'data-mode' ? 'tset' : null) }, referrer: '' },
  navigator: { sendBeacon: (_url, body) => { invalidModeRequests.push(Object.fromEntries(body.entries())); return true; } },
  sessionStorage: { getItem: () => null, setItem() {} },
  __svgstatProjects: {},
  svgstat: undefined
};
invalidModeContext.window = invalidModeContext;
vm.runInNewContext(fs.readFileSync(new URL('../web/static/js/sdk.js', import.meta.url), 'utf8'), invalidModeContext);
assert.equal(invalidModeRequests.length, 1);
assert.equal(invalidModeRequests[0].mode, 'tset');

function element(attributes = {}, selectors = []) {
  const normalized = Object.fromEntries(Object.entries(attributes).map(([key, value]) => [key.toLowerCase(), String(value)]));
  const node = {
    attributes: Object.entries(normalized).map(([name, value]) => ({ name, value })),
    getAttribute: name => normalized[name.toLowerCase()] ?? null,
    hasAttribute: name => Object.hasOwn(normalized, name.toLowerCase()),
    closest: selector => selectors.includes(selector) ? node : null
  };
  return node;
}

const autoRequests = [];
const autoListeners = {};
const autoWindowListeners = {};
const performanceObservers = {};
class MockPerformanceObserver {
  constructor(callback) { this.callback = callback; }
  observe(options) { performanceObservers[options.type] = this.callback; }
}
const autoContext = {
  ...context,
  location: { pathname: '/pricing', search: '', hash: '', origin: 'https://site.example', hostname: 'site.example' },
  document: {
    currentScript: { src: 'https://stats.example/sdk.js', getAttribute: name => name === 'data-project' ? 'auto-project' : null },
    referrer: '',
    addEventListener: (name, callback) => { autoListeners[name] = callback; }
  },
  navigator: { sendBeacon: (_url, body) => { autoRequests.push(Object.fromEntries(body.entries())); return true; } },
  PerformanceObserver: MockPerformanceObserver,
  addEventListener: (name, callback) => { autoWindowListeners[name] = callback; },
  sessionStorage: { getItem: () => null, setItem() {} },
  __svgstatProjects: {},
  svgstat: undefined
};
autoContext.window = autoContext;
vm.runInNewContext(fs.readFileSync(new URL('../web/static/js/sdk.js', import.meta.url), 'utf8'), autoContext);
assert.equal(typeof autoListeners.click, 'function');
assert.equal(typeof autoListeners.submit, 'function');
assert.equal(typeof autoWindowListeners.error, 'function');
assert.equal(typeof autoWindowListeners.unhandledrejection, 'function');

autoListeners.click({ target: element({ href: 'https://docs.example/guide?email=private@example.com#secret' }, ['a[href]']) });
autoListeners.click({ target: element({ href: '/files/report.PDF?token=secret' }, ['a[href]']) });
autoListeners.click({ target: element({ href: 'mailto:private@example.com' }, ['a[href]']) });
autoListeners.click({ target: element({ href: 'tel:+8613800000000' }, ['a[href]']) });
autoListeners.click({ target: element({ href: '/pricing' }, ['a[href]']) });
autoListeners.click({ target: element({ href: 'https://ignored.example', 'data-svgstat-ignore': '' }, ['[data-svgstat-ignore]', 'a[href]']) });
autoListeners.click({ target: element({
  href: 'https://docs.example/ignored-by-custom',
  'data-svgstat-event': 'upgrade_click',
  'data-svgstat-value': '99',
  'data-svgstat-currency': 'cny',
  'data-svgstat-property-plan': 'pro',
  'data-svgstat-property-email': 'private@example.com'
}, ['[data-svgstat-event]', 'a[href]']) });
autoListeners.submit({ target: element({ id: 'lead-form', method: 'POST', action: '/thank-you?email=private@example.com' }, []) });
autoWindowListeners.error({ error: { name: 'TypeError', message: 'private value' } });
autoWindowListeners.unhandledrejection({ reason: 'private rejection' });
autoWindowListeners.error({ target: { tagName: 'SCRIPT', currentSrc: 'https://cdn.example/app.js?token=secret', getAttribute: () => null } });
performanceObservers['largest-contentful-paint']({ getEntries: () => [{ startTime: 1800 }] });
performanceObservers.event({ getEntries: () => [{ interactionId: 1, duration: 240 }] });
performanceObservers['layout-shift']({ getEntries: () => [{ value: 0.06, hadRecentInput: false }, { value: 0.5, hadRecentInput: true }] });
autoContext.document.visibilityState = 'hidden';
autoListeners.visibilitychange();

const automaticEvents = autoRequests.slice(1).map(request => ({ ...request, properties: request.properties ? JSON.parse(request.properties) : {} }));
assert.deepEqual(automaticEvents.map(request => request.event), ['outbound_click', 'file_download', 'contact_click', 'contact_click', 'upgrade_click', 'form_submit', 'js_error', 'js_error', 'resource_error', 'web_vital_lcp', 'web_vital_inp', 'web_vital_cls']);
assert.deepEqual(automaticEvents[0].properties, { target_domain: 'docs.example', target_path: '/guide' });
assert.deepEqual(automaticEvents[1].properties, { file_extension: 'pdf', target_domain: 'site.example', target_path: '/files/report.PDF' });
assert.deepEqual(automaticEvents[2].properties, { contact_type: 'email' });
assert.deepEqual(automaticEvents[3].properties, { contact_type: 'phone' });
assert.deepEqual(automaticEvents[4].properties, { value: 99, currency: 'CNY', plan: 'pro', email: 'private@example.com' });
assert.deepEqual(automaticEvents[5].properties, { form_id: 'lead-form', method: 'post', target_domain: 'site.example', target_path: '/thank-you' });
assert.deepEqual(automaticEvents[6].properties, { error_type: 'type_error' });
assert.deepEqual(automaticEvents[7].properties, { error_type: 'unhandled_rejection' });
assert.deepEqual(automaticEvents[8].properties, { resource_type: 'script', target_domain: 'cdn.example', target_path: '/app.js' });
assert.deepEqual(automaticEvents[9].properties, { value: 1800, rating: 'good' });
assert.deepEqual(automaticEvents[10].properties, { value: 240, rating: 'needs_improvement' });
assert.deepEqual(automaticEvents[11].properties, { value: 0.06, rating: 'good' });
assert.equal(JSON.stringify(automaticEvents).includes('secret'), false);
assert.equal(JSON.stringify(automaticEvents).includes('+8613800000000'), false);

const disabledListeners = {};
const disabledContext = {
  ...autoContext,
  document: {
    currentScript: { src: 'https://stats.example/sdk.js', getAttribute: name => name === 'data-project' ? 'manual-project' : (name === 'data-auto-track' ? 'false' : null) },
    referrer: '',
    addEventListener: (name, callback) => { disabledListeners[name] = callback; }
  },
  navigator: { sendBeacon: () => true },
  sessionStorage: { getItem: () => null, setItem() {} },
  __svgstatProjects: {},
  svgstat: undefined
};
disabledContext.window = disabledContext;
vm.runInNewContext(fs.readFileSync(new URL('../web/static/js/sdk.js', import.meta.url), 'utf8'), disabledContext);
assert.deepEqual(disabledListeners, {});
