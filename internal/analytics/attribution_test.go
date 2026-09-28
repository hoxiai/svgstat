package analytics

import "testing"

func TestWebsiteAttributionClassifiesReferrers(t *testing.T) {
	tests := []struct {
		path, referrer, siteHost string
		source, medium, term     string
	}{
		// Search engines collapse to one canonical source per engine.
		{path: "/", referrer: "https://www.google.com/", source: "google", medium: "organic"},
		{path: "/", referrer: "https://www.google.com.hk/", source: "google", medium: "organic"},
		{path: "/", referrer: "https://www.google.co.jp/url?q=x", source: "google", medium: "organic"},
		{path: "/", referrer: "android-app://com.google.android.googlequicksearchbox/", source: "google", medium: "organic"},
		{path: "/", referrer: "https://m.baidu.com/", source: "baidu", medium: "organic"},
		{path: "/", referrer: "https://www.baidu.com/s?wd=svg%20badge", source: "baidu", medium: "organic", term: "svg badge"},
		{path: "/", referrer: "https://cn.bing.com/search?q=svg+counter", source: "bing", medium: "organic", term: "svg counter"},
		{path: "/", referrer: "https://m.sm.cn/", source: "shenma", medium: "organic"},
		{path: "/", referrer: "https://so.toutiao.com/", source: "toutiao", medium: "organic"},
		{path: "/", referrer: "https://www.so.com/", source: "360", medium: "organic"},
		{path: "/", referrer: "https://www.sogou.com/", source: "sogou", medium: "organic"},
		{path: "/", referrer: "https://duckduckgo.com/", source: "duckduckgo", medium: "organic"},
		{path: "/", referrer: "https://yandex.ru/", source: "yandex", medium: "organic"},
		// Substring look-alikes are ordinary referrals.
		{path: "/", referrer: "https://www.also.com/", source: "also.com", medium: "referral"},
		{path: "/", referrer: "https://notgoogle.com/", source: "notgoogle.com", medium: "referral"},
		// Email clients are not search.
		{path: "/", referrer: "https://mail.google.com/", source: "gmail", medium: "email"},
		{path: "/", referrer: "https://mail.qq.com/", source: "qq-mail", medium: "email"},
		// AI assistants.
		{path: "/", referrer: "https://chatgpt.com/", source: "chatgpt", medium: "ai"},
		{path: "/", referrer: "https://www.perplexity.ai/", source: "perplexity", medium: "ai"},
		{path: "/", referrer: "https://gemini.google.com/", source: "gemini", medium: "ai"},
		// Social redirectors map to their network.
		{path: "/", referrer: "https://t.co/abc", source: "x", medium: "social"},
		{path: "/", referrer: "https://x.com/user/status/1", source: "x", medium: "social"},
		{path: "/", referrer: "https://l.facebook.com/", source: "facebook", medium: "social"},
		{path: "/", referrer: "https://link.zhihu.com/?target=x", source: "zhihu", medium: "social"},
		{path: "/", referrer: "https://www.zhihu.com/question/1", source: "zhihu", medium: "social"},
		{path: "/", referrer: "https://out.reddit.com/", source: "reddit", medium: "social"},
		{path: "/", referrer: "https://news.ycombinator.com/", source: "hackernews", medium: "social"},
		{path: "/", referrer: "https://mp.weixin.qq.com/s/abc", source: "wechat", medium: "social"},
		// Known referral sites get a canonical name but stay referral.
		{path: "/", referrer: "https://github.com/hoxiai/svgstat", source: "github", medium: "referral"},
		{path: "/", referrer: "https://www.example.org/post", source: "example.org", medium: "referral"},
		// Direct and self-referrals.
		{path: "/", referrer: "", source: "direct", medium: "none"},
		{path: "/", referrer: "https://mysite.com/other", siteHost: "mysite.com", source: "direct", medium: "none"},
		{path: "/", referrer: "https://www.mysite.com/other", siteHost: "mysite.com", source: "direct", medium: "none"},
		{path: "/", referrer: "https://blog.mysite.com/", siteHost: "mysite.com", source: "blog.mysite.com", medium: "referral"},
		// Paid click IDs beat the organic referrer.
		{path: "/?gclid=abc", referrer: "https://www.google.com/", source: "google", medium: "cpc"},
		{path: "/?msclkid=abc", referrer: "https://www.bing.com/", source: "bing", medium: "cpc"},
		{path: "/?bd_vid=abc", referrer: "https://www.baidu.com/", source: "baidu", medium: "cpc"},
		{path: "/?fbclid=abc", referrer: "", source: "facebook", medium: "social"},
		// Explicit UTM wins and is normalized.
		{path: "/?utm_source=Twitter", referrer: "", source: "x", medium: "social"},
		{path: "/?utm_source=my-partner", referrer: "", source: "my-partner", medium: "referral"},
		{path: "/?utm_source=www.google.com&utm_medium=PPC", referrer: "", source: "google", medium: "cpc"},
		{path: "/?utm_source=newsletter&utm_medium=e-mail", referrer: "", source: "newsletter", medium: "email"},
		{path: "/?utm_source=google&utm_medium=cpc&utm_term=SVG+Badge&gclid=x", referrer: "https://www.google.com/", source: "google", medium: "cpc", term: "svg badge"},
		{path: "/?utm_source=direct", referrer: "https://www.google.com/", source: "direct", medium: "none"},
	}
	for _, test := range tests {
		got := websiteAttribution(test.path, test.referrer, test.siteHost)
		if got.Source != test.source || got.Medium != test.medium || got.Term != test.term {
			t.Errorf("websiteAttribution(%q, %q, %q) = %q/%q term %q, want %q/%q term %q",
				test.path, test.referrer, test.siteHost, got.Source, got.Medium, got.Term, test.source, test.medium, test.term)
		}
	}
}

func TestWebsiteAttributionDropsSelfReferrer(t *testing.T) {
	got := websiteAttribution("/pricing", "https://mysite.com/docs?x=1", "mysite.com")
	if got.Referrer != "" {
		t.Fatalf("Referrer = %q, want empty for a self-referral", got.Referrer)
	}
}

func TestWebsiteAttributionKeepsReferrerPathWithoutQuery(t *testing.T) {
	got := websiteAttribution("/", "https://www.baidu.com/s?wd=secret", "")
	if got.Referrer != "https://www.baidu.com/s" {
		t.Fatalf("Referrer = %q, want query stripped", got.Referrer)
	}
}
