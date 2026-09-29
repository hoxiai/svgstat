package analytics

import (
	"net"
	"net/url"
	"strings"
)

// Attribution is where a website visit came from, derived from the landing
// URL (UTM tags, ad click IDs) and the document referrer.
type Attribution struct {
	Path     string
	Referrer string
	Source   string
	Medium   string
	Campaign string
	Term     string
}

// referrerRule maps referrer hosts to a canonical source and medium. Hosts
// match exactly or as a dot-suffix (unless exact); brand matches brand.<tld>
// with an optional www./m. prefix, e.g. google.com.hk.
type referrerRule struct {
	source string
	medium string
	hosts  []string
	exact  bool
	brand  string
}

// Rules are checked in order: email and AI hosts live under search brands
// (mail.google.com, gemini.google.com), so they come first.
var referrerRules = []referrerRule{
	{source: "gmail", medium: "email", hosts: []string{"mail.google.com", "com.google.android.gm"}},
	{source: "outlook", medium: "email", hosts: []string{"outlook.live.com", "outlook.office.com", "outlook.office365.com"}},
	{source: "yahoo-mail", medium: "email", hosts: []string{"mail.yahoo.com"}},
	{source: "qq-mail", medium: "email", hosts: []string{"mail.qq.com", "exmail.qq.com"}},
	{source: "netease-mail", medium: "email", hosts: []string{"mail.163.com", "mail.126.com", "mail.yeah.net"}},
	{source: "proton-mail", medium: "email", hosts: []string{"mail.proton.me"}},

	{source: "chatgpt", medium: "ai", hosts: []string{"chatgpt.com", "chat.openai.com"}},
	{source: "perplexity", medium: "ai", hosts: []string{"perplexity.ai"}},
	{source: "gemini", medium: "ai", hosts: []string{"gemini.google.com", "bard.google.com"}},
	{source: "copilot", medium: "ai", hosts: []string{"copilot.microsoft.com"}},
	{source: "claude", medium: "ai", hosts: []string{"claude.ai"}},
	{source: "deepseek", medium: "ai", hosts: []string{"chat.deepseek.com"}},
	{source: "kimi", medium: "ai", hosts: []string{"kimi.com", "kimi.moonshot.cn"}},
	{source: "doubao", medium: "ai", hosts: []string{"doubao.com"}},
	{source: "yuanbao", medium: "ai", hosts: []string{"yuanbao.tencent.com"}},
	{source: "tongyi", medium: "ai", hosts: []string{"tongyi.aliyun.com", "tongyi.com"}},
	{source: "metaso", medium: "ai", hosts: []string{"metaso.cn"}},
	{source: "poe", medium: "ai", hosts: []string{"poe.com"}},

	{source: "google", medium: "organic", brand: "google", hosts: []string{"com.google.android.googlequicksearchbox"}},
	{source: "bing", medium: "organic", hosts: []string{"bing.com"}},
	{source: "baidu", medium: "organic", exact: true, hosts: []string{"baidu.com", "www.baidu.com", "m.baidu.com", "wap.baidu.com"}},
	{source: "yahoo", medium: "organic", hosts: []string{"search.yahoo.com", "search.yahoo.co.jp"}},
	{source: "duckduckgo", medium: "organic", hosts: []string{"duckduckgo.com"}},
	{source: "yandex", medium: "organic", brand: "yandex", hosts: []string{"ya.ru"}},
	{source: "sogou", medium: "organic", exact: true, hosts: []string{"sogou.com", "www.sogou.com", "m.sogou.com", "wap.sogou.com"}},
	{source: "360", medium: "organic", hosts: []string{"so.com"}},
	{source: "shenma", medium: "organic", hosts: []string{"sm.cn"}},
	{source: "toutiao", medium: "organic", hosts: []string{"so.toutiao.com"}},
	{source: "ecosia", medium: "organic", hosts: []string{"ecosia.org"}},
	{source: "brave", medium: "organic", hosts: []string{"search.brave.com"}},
	{source: "naver", medium: "organic", hosts: []string{"search.naver.com"}},
	{source: "seznam", medium: "organic", hosts: []string{"seznam.cz"}},
	{source: "qwant", medium: "organic", hosts: []string{"qwant.com"}},
	{source: "startpage", medium: "organic", hosts: []string{"startpage.com"}},

	{source: "x", medium: "social", hosts: []string{"x.com", "twitter.com", "t.co"}},
	{source: "facebook", medium: "social", hosts: []string{"facebook.com", "fb.com", "fb.me"}},
	{source: "instagram", medium: "social", hosts: []string{"instagram.com"}},
	{source: "linkedin", medium: "social", hosts: []string{"linkedin.com", "lnkd.in"}},
	{source: "reddit", medium: "social", hosts: []string{"reddit.com", "redd.it"}},
	{source: "youtube", medium: "social", hosts: []string{"youtube.com", "youtu.be"}},
	{source: "hackernews", medium: "social", hosts: []string{"news.ycombinator.com"}},
	{source: "producthunt", medium: "social", hosts: []string{"producthunt.com"}},
	{source: "weibo", medium: "social", hosts: []string{"weibo.com", "weibo.cn", "t.cn"}},
	{source: "zhihu", medium: "social", hosts: []string{"zhihu.com"}},
	{source: "wechat", medium: "social", hosts: []string{"weixin.qq.com"}},
	{source: "bilibili", medium: "social", hosts: []string{"bilibili.com", "b23.tv"}},
	{source: "xiaohongshu", medium: "social", hosts: []string{"xiaohongshu.com", "xhslink.com"}},
	{source: "douyin", medium: "social", hosts: []string{"douyin.com"}},
	{source: "tiktok", medium: "social", hosts: []string{"tiktok.com"}},
	{source: "tieba", medium: "social", hosts: []string{"tieba.baidu.com"}},
	{source: "douban", medium: "social", hosts: []string{"douban.com"}},
	{source: "v2ex", medium: "social", hosts: []string{"v2ex.com"}},
	{source: "juejin", medium: "social", hosts: []string{"juejin.cn"}},
	{source: "telegram", medium: "social", hosts: []string{"t.me", "telegram.org"}},
	{source: "discord", medium: "social", hosts: []string{"discord.com", "discord.gg"}},
	{source: "threads", medium: "social", hosts: []string{"threads.net"}},
	{source: "bluesky", medium: "social", hosts: []string{"bsky.app"}},
	{source: "pinterest", medium: "social", hosts: []string{"pinterest.com"}},
	{source: "medium", medium: "social", hosts: []string{"medium.com"}},
	{source: "devto", medium: "social", hosts: []string{"dev.to"}},

	{source: "github", medium: "referral", hosts: []string{"github.com"}},
	{source: "gitee", medium: "referral", hosts: []string{"gitee.com"}},
	{source: "stackoverflow", medium: "referral", hosts: []string{"stackoverflow.com"}},
	{source: "csdn", medium: "referral", hosts: []string{"csdn.net"}},
}

// clickIDs tag ad clicks on the landing URL; they identify paid traffic
// even when the referrer only says "google.com".
var clickIDs = []struct{ param, source, medium string }{
	{"gclid", "google", "cpc"}, {"gbraid", "google", "cpc"}, {"wbraid", "google", "cpc"}, {"dclid", "google", "cpc"},
	{"msclkid", "bing", "cpc"},
	{"bd_vid", "baidu", "cpc"},
	{"gdt_vid", "tencent", "cpc"}, {"qz_gdt", "tencent", "cpc"},
	{"ttclid", "tiktok", "cpc"},
	{"twclid", "x", "cpc"},
	{"li_fat_id", "linkedin", "cpc"},
	{"yclid", "yandex", "cpc"},
	{"epik", "pinterest", "cpc"},
	{"fbclid", "facebook", "social"},
}

var sourceAliases = map[string]string{
	"twitter": "x", "fb": "facebook", "ig": "instagram", "yt": "youtube",
	"weixin": "wechat", "hn": "hackernews",
}

var mediumAliases = map[string]string{
	"ppc": "cpc", "paid": "cpc", "paidsearch": "cpc", "paid_search": "cpc", "paid-search": "cpc", "sem": "cpc",
	"e-mail": "email", "newsletter": "email", "edm": "email",
	"social-media": "social", "social_media": "social", "sns": "social",
}

// searchQueryParams are the query parameters search engines use for the
// search term; most engines strip them, but some referrers still carry one.
var searchQueryParams = []string{"q", "wd", "word", "query", "p", "text", "keyword"}

var knownSourceMediums = func() map[string]string {
	result := map[string]string{"direct": "none"}
	for _, rule := range referrerRules {
		if _, ok := result[rule.source]; !ok {
			result[rule.source] = rule.medium
		}
	}
	return result
}()

func websiteAttribution(rawPath, rawReferrer, siteHost string) Attribution {
	result := Attribution{Path: "/"}
	var query url.Values
	if parsedPath, err := url.Parse(rawPath); err == nil {
		if parsedPath.EscapedPath() != "" {
			result.Path = parsedPath.EscapedPath()
		}
		if fragment := cleanPathFragment(parsedPath.Fragment); fragment != "" {
			result.Path += "#" + fragment
		}
		query = parsedPath.Query()
	}
	source := normalizeSource(query.Get("utm_source"))
	medium := normalizeMedium(query.Get("utm_medium"))
	result.Campaign = cleanDimension(query.Get("utm_campaign"))
	result.Term = cleanDimension(query.Get("utm_term"))

	refSource, refMedium, refTerm := "direct", "none", ""
	refToParse := strings.TrimSpace(rawReferrer)
	if refToParse != "" && !strings.Contains(refToParse, "://") {
		refToParse = "https://" + refToParse
	}
	if parsed, err := url.Parse(refToParse); err == nil && parsed.Hostname() != "" {
		host := strings.ToLower(parsed.Hostname())
		if !sameSite(host, siteHost) {
			result.Referrer = cleanReferrer(refToParse)
			refSource, refMedium = classifyReferrerHost(host)
			// Google's /url redirector carries the destination in q, not a search.
			if refMedium == "organic" && parsed.Path != "/url" {
				refTerm = searchTerm(parsed.Query())
			}
		}
	}

	clickSource, clickMedium := "", ""
	for _, id := range clickIDs {
		if query.Get(id.param) != "" {
			clickSource, clickMedium = id.source, id.medium
			break
		}
	}

	if source == "" {
		if clickSource != "" {
			source = clickSource
		} else {
			source = refSource
		}
	}
	if medium == "" {
		switch {
		case clickSource != "" && source == clickSource:
			medium = clickMedium
		case source == refSource:
			medium = refMedium
		case knownSourceMediums[source] != "":
			medium = knownSourceMediums[source]
		default:
			medium = "referral"
		}
	}
	if result.Term == "" && source == refSource {
		result.Term = refTerm
	}
	result.Source, result.Medium = source, medium
	return result
}

// WebsiteAttribution attributes a website hit; siteHost is the tracked site's
// host so internal navigation is not counted as a referral.
func WebsiteAttribution(rawPath, rawReferrer, siteHost string) Attribution {
	return websiteAttribution(rawPath, rawReferrer, siteHost)
}

// classifyReferrerHost returns the canonical source and medium for a
// referrer host; unknown hosts are referrals named by their host.
func classifyReferrerHost(host string) (source, medium string) {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, rule := range referrerRules {
		if rule.matches(host) {
			return rule.source, rule.medium
		}
	}
	return strings.TrimPrefix(host, "www."), "referral"
}

func (rule referrerRule) matches(host string) bool {
	for _, candidate := range rule.hosts {
		if host == candidate || (!rule.exact && strings.HasSuffix(host, "."+candidate)) {
			return true
		}
	}
	return rule.brand != "" && isBrandHost(host, rule.brand)
}

// isBrandHost reports whether host is brand.<tld> (tld of one or two short
// labels, e.g. com, co.jp, com.hk), optionally prefixed with www. or m.
func isBrandHost(host, brand string) bool {
	host = strings.TrimPrefix(strings.TrimPrefix(host, "www."), "m.")
	rest, ok := strings.CutPrefix(host, brand+".")
	if !ok {
		return false
	}
	labels := strings.Split(rest, ".")
	if len(labels) > 2 {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > 3 {
			return false
		}
	}
	return true
}

func sameSite(host, siteHost string) bool {
	siteHost = strings.ToLower(strings.TrimSpace(siteHost))
	if h, _, err := net.SplitHostPort(siteHost); err == nil {
		siteHost = h
	}
	if siteHost == "" {
		return false
	}
	return strings.TrimPrefix(host, "www.") == strings.TrimPrefix(siteHost, "www.")
}

func normalizeSource(value string) string {
	value = cleanDimension(value)
	if alias, ok := sourceAliases[value]; ok {
		return alias
	}
	if strings.Contains(value, ".") && !strings.ContainsAny(value, " /") {
		source, _ := classifyReferrerHost(value)
		return source
	}
	return value
}

func normalizeMedium(value string) string {
	value = cleanDimension(value)
	if alias, ok := mediumAliases[value]; ok {
		return alias
	}
	return value
}

func searchTerm(query url.Values) string {
	for _, param := range searchQueryParams {
		if term := cleanDimension(query.Get(param)); term != "" {
			return term
		}
	}
	return ""
}
