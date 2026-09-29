package analytics

import "strings"

type aiRule struct {
	keyword string
	name    string
}

var aiRules = []aiRule{
	{keyword: "gptbot", name: "OpenAI (GPTBot)"},
	{keyword: "chatgpt-user", name: "OpenAI (ChatGPT-User)"},
	{keyword: "oai-searchbot", name: "OpenAI (SearchBot)"},
	{keyword: "claudebot", name: "Anthropic (ClaudeBot)"},
	{keyword: "claude-web", name: "Anthropic (ClaudeWeb)"},
	{keyword: "anthropic-ai", name: "Anthropic (Claude)"},
	{keyword: "perplexitybot", name: "Perplexity"},
	{keyword: "bytespider", name: "字节跳动 (豆包/Bytespider)"},
	{keyword: "deepseekbot", name: "DeepSeek (深度求索)"},
	{keyword: "google-extended", name: "Google (Gemini)"},
	{keyword: "applebot-extended", name: "Apple (Applebot)"},
	{keyword: "meta-externalagent", name: "Meta (Llama)"},
	{keyword: "cohere-ai", name: "Cohere"},
	{keyword: "moonshot", name: "Moonshot (Kimi)"},
	{keyword: "kimibot", name: "Moonshot (Kimi)"},
	{keyword: "youbot", name: "You.com"},
}

// DetectAICrawler checks if a User-Agent belongs to an AI search agent or crawler.
func DetectAICrawler(userAgent string) (isAI bool, aiName string) {
	lower := strings.ToLower(userAgent)
	for _, rule := range aiRules {
		if strings.Contains(lower, rule.keyword) {
			return true, rule.name
		}
	}
	return false, ""
}
