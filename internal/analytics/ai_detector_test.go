package analytics

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hoxiai/svgstat/internal/cache"
)

func TestDetectAICrawler(t *testing.T) {
	cases := []struct {
		name     string
		ua       string
		wantAI   bool
		wantName string
	}{
		{
			name:     "GPTBot",
			ua:       "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; GPTBot/1.2; +https://openai.com/gptbot)",
			wantAI:   true,
			wantName: "OpenAI (GPTBot)",
		},
		{
			name:     "ChatGPT-User",
			ua:       "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; ChatGPT-User/1.0; +https://openai.com/bot",
			wantAI:   true,
			wantName: "OpenAI (ChatGPT-User)",
		},
		{
			name:     "OAI-SearchBot",
			ua:       "Mozilla/5.0 (compatible; OAI-SearchBot/1.0; +https://openai.com/searchbot)",
			wantAI:   true,
			wantName: "OpenAI (SearchBot)",
		},
		{
			name:     "ClaudeBot",
			ua:       "Mozilla/5.0 (compatible; ClaudeBot/1.0; +claudebot@anthropic.com)",
			wantAI:   true,
			wantName: "Anthropic (ClaudeBot)",
		},
		{
			name:     "Claude-Web",
			ua:       "Mozilla/5.0 (compatible; Claude-Web/1.0; +https://www.anthropic.com)",
			wantAI:   true,
			wantName: "Anthropic (ClaudeWeb)",
		},
		{
			name:     "Anthropic-AI",
			ua:       "anthropic-ai/1.0",
			wantAI:   true,
			wantName: "Anthropic (Claude)",
		},
		{
			name:     "PerplexityBot",
			ua:       "Mozilla/5.0 (compatible; PerplexityBot/1.0; +https://perplexity.ai/perplexitybot)",
			wantAI:   true,
			wantName: "Perplexity",
		},
		{
			name:     "ByteSpider",
			ua:       "Mozilla/5.0 (Linux; Android 5.0) AppleWebKit/537.36 (KHTML, like Gecko) Mobile Safari/537.36 (compatible; Bytespider; spider-feedback@bytedance.com)",
			wantAI:   true,
			wantName: "字节跳动 (豆包/Bytespider)",
		},
		{
			name:     "DeepSeekBot",
			ua:       "Mozilla/5.0 (compatible; DeepSeekBot/1.0; +https://www.deepseek.com)",
			wantAI:   true,
			wantName: "DeepSeek (深度求索)",
		},
		{
			name:     "Google-Extended",
			ua:       "Mozilla/5.0 (compatible; Google-Extended)",
			wantAI:   true,
			wantName: "Google (Gemini)",
		},
		{
			name:     "Applebot-Extended",
			ua:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15 (Applebot-Extended/0.1)",
			wantAI:   true,
			wantName: "Apple (Applebot)",
		},
		{
			name:     "Meta-ExternalAgent",
			ua:       "Mozilla/5.0 (compatible; Meta-ExternalAgent/1.0; +https://developers.facebook.com/docs/sharing/webmasters/crawler)",
			wantAI:   true,
			wantName: "Meta (Llama)",
		},
		{
			name:     "Cohere-AI",
			ua:       "cohere-ai/1.0",
			wantAI:   true,
			wantName: "Cohere",
		},
		{
			name:     "Moonshot",
			ua:       "Mozilla/5.0 (compatible; MoonshotBot/1.0; +https://moonshot.cn)",
			wantAI:   true,
			wantName: "Moonshot (Kimi)",
		},
		{
			name:     "KimiBot",
			ua:       "Mozilla/5.0 (compatible; KimiBot/1.0; +https://kimi.moonshot.cn)",
			wantAI:   true,
			wantName: "Moonshot (Kimi)",
		},
		{
			name:     "YouBot",
			ua:       "Mozilla/5.0 (compatible; YouBot/1.0; +https://you.com/youbot)",
			wantAI:   true,
			wantName: "You.com",
		},
		{
			name:     "Chrome desktop",
			ua:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
			wantAI:   false,
			wantName: "",
		},
		{
			name:     "cURL",
			ua:       "curl/7.68.0",
			wantAI:   false,
			wantName: "",
		},
		{
			name:     "Googlebot standard",
			ua:       "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			wantAI:   false,
			wantName: "",
		},
		{
			name:     "Baiduspider standard",
			ua:       "Baiduspider+(+http://www.baidu.com/search/spider.htm)",
			wantAI:   false,
			wantName: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isAI, name := DetectAICrawler(tc.ua)
			if isAI != tc.wantAI || name != tc.wantName {
				t.Errorf("DetectAICrawler(%q) = (%v, %q), want (%v, %q)", tc.ua, isAI, name, tc.wantAI, tc.wantName)
			}
		})
	}
}

func TestVisitStream_AICrawlerTracked(t *testing.T) {
	a, projectID := newTestAnalytics(t)
	ctx := context.Background()

	req := httptest.NewRequest("GET", "https://example.com/blog/ai-future", nil)
	req.RemoteAddr = "20.15.10.5:443"
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ClaudeBot/1.0; +claudebot@anthropic.com)")

	err := a.TrackRequest(ctx, req, projectID)
	if err != nil {
		t.Fatalf("TrackRequest failed: %v", err)
	}

	// Verify visit stream contains the AI crawler item
	items, err := a.GetVisitStream(ctx, projectID, 10)
	if err != nil {
		t.Fatalf("GetVisitStream failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 stream item, got %d", len(items))
	}

	item := items[0]
	if !item.IsAIAgent {
		t.Errorf("expected IsAIAgent=true, got %v", item.IsAIAgent)
	}
	if item.AIName != "Anthropic (ClaudeBot)" {
		t.Errorf("expected AIName='Anthropic (ClaudeBot)', got %q", item.AIName)
	}
	if item.SourceCategory != "ai" {
		t.Errorf("expected SourceCategory='ai', got %q", item.SourceCategory)
	}
	if item.SourceName != "Anthropic (ClaudeBot)" {
		t.Errorf("expected SourceName='Anthropic (ClaudeBot)', got %q", item.SourceName)
	}
	if item.DeviceType != "bot" {
		t.Errorf("expected DeviceType='bot', got %q", item.DeviceType)
	}
	if item.Browser != "Anthropic (ClaudeBot)" {
		t.Errorf("expected Browser='Anthropic (ClaudeBot)', got %q", item.Browser)
	}

	// Verify project:{id}:ai_visits:{date} key was incremented
	today := time.Now().UTC().Format("2006-01-02")
	aiKey := cache.BuildKey("project", projectID, "ai_visits", today)
	val, err := a.cache.Get(ctx, aiKey)
	if err != nil {
		t.Fatalf("failed to get ai_visits key %q: %v", aiKey, err)
	}
	if val != "1" {
		t.Errorf("expected ai_visits key to be '1', got %q", val)
	}

	// Verify GetTodayStats includes AIVisits
	todayStats, err := a.GetTodayStats(ctx, projectID)
	if err != nil {
		t.Fatalf("GetTodayStats failed: %v", err)
	}
	if todayStats.AIVisits != 1 {
		t.Errorf("expected todayStats.AIVisits=1, got %d", todayStats.AIVisits)
	}
}

