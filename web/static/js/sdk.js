(function () {
    'use strict';

    var script = document.currentScript;
    if (!script) return;
    var project = (script.getAttribute('data-project') || '').trim().toLowerCase();
	var mode = (script.getAttribute('data-mode') || 'live').trim().toLowerCase();
	var autoTrack = script.getAttribute('data-auto-track') !== 'false';
    if (!project || script.getAttribute('data-track') === 'false') return;
	window.__svgstatProjects = window.__svgstatProjects || {};
	if (window.__svgstatProjects[project]) return;
	window.__svgstatProjects[project] = true;

    var endpoint;
    try {
        endpoint = new URL('/api/v1/collect', script.src).toString();
    } catch (_) {
        return;
    }

    var visitorKey = 'svgstat:visitor:' + project;
	var attributionKey = 'svgstat:attribution:' + project;
	var referrerKey = 'svgstat:referrer:' + project;
    var visitor = '';
	var attribution = '';
	var initialReferrer = '';
    try {
        visitor = sessionStorage.getItem(visitorKey) || '';
    } catch (_) {}
	if (!visitor) {
		visitor = window.crypto && typeof window.crypto.randomUUID === 'function'
			? window.crypto.randomUUID()
			: Date.now().toString(36) + Math.random().toString(36).slice(2);
		try { sessionStorage.setItem(visitorKey, visitor); } catch (_) {}
	}
	try { attribution = sessionStorage.getItem(attributionKey) || ''; } catch (_) {}
	if (!attribution) {
		attribution = location.pathname + location.search + location.hash;
		try { sessionStorage.setItem(attributionKey, attribution); } catch (_) {}
	}
	try { initialReferrer = sessionStorage.getItem(referrerKey) || ''; } catch (_) {}
	if (!initialReferrer) {
		initialReferrer = document.referrer || '';
		try { sessionStorage.setItem(referrerKey, initialReferrer); } catch (_) {}
	}

    var lastPath = '';
    function currentPath() {
		return location.pathname + location.search + location.hash;
	}

    function attributedPath(path) {
		var nextPath = path || currentPath();
		if (!nextPath || nextPath.charAt(0) !== '/') return '';
		var result = nextPath;
		if (attribution) {
			try {
				var landing = new URL(attribution, location.origin);
				var current = new URL(nextPath, location.origin);
				['utm_source', 'utm_medium', 'utm_campaign'].forEach(function (key) {
					var value = landing.searchParams.get(key);
					if (value && !current.searchParams.has(key)) current.searchParams.set(key, value);
				});
				result = current.pathname + current.search + current.hash;
			} catch (_) {}
		}
		return result.slice(0, 2048);
	}

	function send(body) {
		if (navigator.sendBeacon && navigator.sendBeacon(endpoint, body)) return;
		fetch(endpoint, {
			method: 'POST',
			body: body,
			mode: 'cors',
			credentials: 'omit',
			keepalive: true
		}).catch(function () {});
	}

	function baseBody(path) {
        var body = new URLSearchParams();
		body.set('project', project);
		body.set('mode', mode);
		body.set('path', attributedPath(path) || '/');
		body.set('referrer', initialReferrer.slice(0, 2048));
        if (visitor) body.set('visitor', visitor.slice(0, 64));
		return body;
	}

    function track(path) {
		var nextPath = path || currentPath();
        if (!nextPath || nextPath.charAt(0) !== '/' || nextPath === lastPath) return;
        lastPath = nextPath;
		var body = baseBody(nextPath);
		body.set('type', 'pageview');
		send(body);
	}

	function cleanProperties(properties) {
		if (!properties || Object.prototype.toString.call(properties) !== '[object Object]') return {};
		var result = {};
		Object.keys(properties).slice(0, 10).forEach(function (key) {
			var value = properties[key];
			if (!/^[a-zA-Z][a-zA-Z0-9_.-]{0,63}$/.test(key)) return;
			if (typeof value === 'string') result[key] = value.slice(0, 256);
			else if (typeof value === 'number' && Number.isFinite(value)) result[key] = value;
			else if (typeof value === 'boolean') result[key] = value;
		});
		return result;
	}

	function event(name, properties) {
		name = String(name || '').trim().toLowerCase();
		if (!/^[a-z][a-z0-9_.-]{0,63}$/.test(name)) return;
		var body = baseBody(currentPath());
		body.set('type', 'event');
		body.set('event', name);
		var cleaned = cleanProperties(properties);
		if (Object.keys(cleaned).length) body.set('properties', JSON.stringify(cleaned));
		send(body);
    }

	var downloadExtensions = /\.(7z|apk|avi|csv|doc|docx|dmg|epub|exe|gz|iso|json|mov|mp3|mp4|msi|ods|odt|pdf|pkg|ppt|pptx|rar|tar|tgz|txt|wav|webm|xls|xlsx|xml|zip)$/i;

	function closest(element, selector) {
		if (!element || typeof element.closest !== 'function') return null;
		try { return element.closest(selector); } catch (_) { return null; }
	}

	function safeTarget(raw) {
		try {
			var parsed = new URL(raw, location.origin);
			if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null;
			return {
				domain: parsed.hostname.toLowerCase().slice(0, 253),
				path: (parsed.pathname || '/').slice(0, 256),
				external: parsed.hostname.toLowerCase() !== String(location.hostname || '').toLowerCase()
			};
		} catch (_) { return null; }
	}

	function customProperties(element) {
		var properties = {};
		var value = element.getAttribute('data-svgstat-value');
		if (value !== null && value !== '') {
			var number = Number(value);
			if (Number.isFinite(number) && number >= 0 && number <= 1e12) properties.value = number;
		}
		var currency = String(element.getAttribute('data-svgstat-currency') || '').trim().toUpperCase();
		if (/^[A-Z]{3}$/.test(currency)) properties.currency = currency;
		Array.prototype.slice.call(element.attributes || []).forEach(function (attribute) {
			var prefix = 'data-svgstat-property-';
			if (!attribute || attribute.name.indexOf(prefix) !== 0) return;
			var key = attribute.name.slice(prefix.length);
			if (!/^[a-zA-Z][a-zA-Z0-9_.-]{0,63}$/.test(key) || key === 'value' || key === 'currency') return;
			properties[key] = String(attribute.value || '').slice(0, 256);
		});
		return cleanProperties(properties);
	}

	function trackCustomElement(element) {
		var name = element.getAttribute('data-svgstat-event');
		if (!name) return false;
		event(name, customProperties(element));
		return true;
	}

	function trackLink(link) {
		var href = String(link.getAttribute('href') || '').trim();
		if (!href || href.charAt(0) === '#') return;
		var protocol = href.split(':')[0].toLowerCase();
		if (protocol === 'mailto' || protocol === 'tel') {
			event('contact_click', { contact_type: protocol === 'mailto' ? 'email' : 'phone' });
			return;
		}
		var target = safeTarget(href);
		if (!target) return;
		var extensionMatch = target.path.match(/\.([a-z0-9]+)$/i);
		if (link.hasAttribute('download') || downloadExtensions.test(target.path)) {
			event('file_download', {
				file_extension: extensionMatch ? extensionMatch[1].toLowerCase().slice(0, 16) : 'unknown',
				target_domain: target.domain,
				target_path: target.path
			});
			return;
		}
		if (target.external) event('outbound_click', { target_domain: target.domain, target_path: target.path });
	}

	function trackForm(form) {
		if (trackCustomElement(form)) return;
		var properties = {};
		var formID = String(form.getAttribute('id') || '').trim();
		if (/^[a-zA-Z0-9_.:-]{1,64}$/.test(formID)) properties.form_id = formID;
		var method = String(form.getAttribute('method') || 'get').trim().toLowerCase();
		if (/^(get|post|put|patch|delete)$/.test(method)) properties.method = method;
		var target = safeTarget(form.getAttribute('action') || currentPath());
		if (target) {
			properties.target_domain = target.domain;
			properties.target_path = target.path;
		}
		event('form_submit', properties);
	}

	function handleClick(clickEvent) {
		var target = clickEvent && clickEvent.target;
		if (!target || closest(target, '[data-svgstat-ignore]')) return;
		var custom = closest(target, '[data-svgstat-event]');
		if (custom && trackCustomElement(custom)) return;
		var link = closest(target, 'a[href]');
		if (link) trackLink(link);
	}

	function handleSubmit(submitEvent) {
		var form = submitEvent && submitEvent.target;
		if (!form || closest(form, '[data-svgstat-ignore]')) return;
		trackForm(form);
	}

	var errorBudget = 20;
	function useErrorBudget() {
		if (errorBudget <= 0) return false;
		errorBudget -= 1;
		return true;
	}

	function runtimeErrorType(error) {
		var name = String(error && error.name || '').toLowerCase();
		if (name === 'typeerror') return 'type_error';
		if (name === 'referenceerror') return 'reference_error';
		if (name === 'syntaxerror') return 'syntax_error';
		if (name === 'rangeerror') return 'range_error';
		return 'unknown';
	}

	function resourceType(element) {
		var tag = String(element && element.tagName || '').toLowerCase();
		if (tag === 'img' || tag === 'picture') return 'image';
		if (tag === 'link') return 'stylesheet';
		if (tag === 'audio' || tag === 'video' || tag === 'source') return 'media';
		if (tag === 'script' || tag === 'iframe') return tag;
		return 'other';
	}

	function handleRuntimeError(errorEvent) {
		var target = errorEvent && errorEvent.target;
		if (target && target !== window && typeof target.getAttribute === 'function') {
			var raw = target.currentSrc || target.getAttribute('src') || target.getAttribute('href');
			var resource = safeTarget(raw);
			if (!resource || !useErrorBudget()) return;
			event('resource_error', { resource_type: resourceType(target), target_domain: resource.domain, target_path: resource.path });
			return;
		}
		if (useErrorBudget()) event('js_error', { error_type: runtimeErrorType(errorEvent && errorEvent.error) });
	}

	function handleUnhandledRejection() {
		if (useErrorBudget()) event('js_error', { error_type: 'unhandled_rejection' });
	}

	function vitalRating(name, value) {
		var limits = name === 'lcp' ? [2500, 4000] : (name === 'inp' ? [200, 500] : [0.1, 0.25]);
		return value <= limits[0] ? 'good' : (value <= limits[1] ? 'needs_improvement' : 'poor');
	}

	function observeWebVitals() {
		if (typeof PerformanceObserver !== 'function') return;
		var values = { lcp: 0, inp: 0, cls: 0 };
		var seen = { lcp: false, inp: false, cls: false };
		var sent = {};
		function observe(options, callback) {
			try {
				var observer = new PerformanceObserver(function (list) { callback(list.getEntries() || []); });
				observer.observe(options);
				return true;
			} catch (_) { return false; }
		}
		observe({ type: 'largest-contentful-paint', buffered: true }, function (entries) {
			var latest = entries[entries.length - 1];
			if (latest && Number.isFinite(latest.startTime)) { values.lcp = latest.startTime; seen.lcp = true; }
		});
		observe({ type: 'event', buffered: true, durationThreshold: 40 }, function (entries) {
			entries.forEach(function (entry) {
				if (entry && entry.interactionId && Number.isFinite(entry.duration)) { values.inp = Math.max(values.inp, entry.duration); seen.inp = true; }
			});
		});
		seen.cls = observe({ type: 'layout-shift', buffered: true }, function (entries) {
			entries.forEach(function (entry) { if (entry && !entry.hadRecentInput && Number.isFinite(entry.value)) values.cls += entry.value; });
		});
		function flushVitals() {
			['lcp', 'inp', 'cls'].forEach(function (name) {
				if (!seen[name] || sent[name]) return;
				sent[name] = true;
				var value = name === 'cls' ? Math.round(values[name] * 10000) / 10000 : Math.round(values[name]);
				event('web_vital_' + name, { value: value, rating: vitalRating(name, value) });
			});
		}
		document.addEventListener('visibilitychange', function () { if (document.visibilityState === 'hidden') flushVitals(); }, true);
		addEventListener('pagehide', flushVitals, true);
	}

    function scheduleTrack() {
        setTimeout(function () { track(currentPath()); }, 0);
    }

    ['pushState', 'replaceState'].forEach(function (method) {
        var original = history[method];
        if (typeof original !== 'function') return;
        history[method] = function () {
            var result = original.apply(this, arguments);
            scheduleTrack();
            return result;
        };
    });
    addEventListener('popstate', scheduleTrack);
    addEventListener('hashchange', scheduleTrack);
    window.svgstatTrack = function (path) { track(path || currentPath()); };
	var queued = Array.isArray(window.svgstat && window.svgstat.q) ? window.svgstat.q.slice() : [];
	window.svgstat = function (command, name, properties) {
		if (command === 'event') event(name, properties);
		else if (command === 'pageview') track(name || currentPath());
	};
	queued.forEach(function (args) { if (args && typeof args.length === 'number') window.svgstat.apply(null, args); });
	if (autoTrack && document.addEventListener) {
		document.addEventListener('click', handleClick, true);
		document.addEventListener('submit', handleSubmit, true);
		addEventListener('error', handleRuntimeError, true);
		addEventListener('unhandledrejection', handleUnhandledRejection, true);
		observeWebVitals();
	}
	track(currentPath());
})();
