package analytics

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/hoxiai/svgstat/internal/cache"
	"github.com/redis/go-redis/v9"
)

const (
	visitStreamCapacity = 500
	visitStreamTTL      = 48 * time.Hour // 172800 seconds
)

// VisitStreamItem represents a real-time access log record.
type VisitStreamItem struct {
	ID             string `json:"id"`
	Timestamp      string `json:"timestamp"`
	TimeStr        string `json:"timeStr"`
	MaskedIP       string `json:"maskedIp"`
	Country        string `json:"country"`
	Region         string `json:"region"`
	City           string `json:"city"`
	SourceCategory string `json:"sourceCategory"`
	SourceName     string `json:"sourceName"`
	SourceURL      string `json:"sourceUrl"`
	SearchKeyword  string `json:"searchKeyword"`
	Path           string `json:"path"`
	DeviceType     string `json:"deviceType"`
	Browser        string `json:"browser"`
	IsAIAgent      bool   `json:"isAiAgent"`
	AIName         string `json:"aiName,omitempty"`
}

func newStreamID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// maskIP masks IPv4 addresses to a.b.*.* and IPv6 to prefix:*::*.
func maskIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")

	ip := net.ParseIP(raw)
	if ip != nil {
		if v4 := ip.To4(); v4 != nil {
			parts := strings.Split(v4.String(), ".")
			if len(parts) == 4 {
				return parts[0] + "." + parts[1] + ".*.*"
			}
		}
		if len(ip) == 16 {
			h1 := (uint16(ip[0]) << 8) | uint16(ip[1])
			h2 := (uint16(ip[2]) << 8) | uint16(ip[3])
			return fmt.Sprintf("%x:%x:*::*", h1, h2)
		}
		return "prefix:*"
	}
	if strings.HasPrefix(raw, "anon_") {
		return "anon_*"
	}
	return "unknown"
}

// classifySourceCategory maps traffic source/medium into search, referral, direct, ai, social, email.
func classifySourceCategory(source, medium string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	medium = strings.ToLower(strings.TrimSpace(medium))

	switch medium {
	case "organic", "search":
		return "search"
	case "ai":
		return "ai"
	case "social":
		return "social"
	case "email":
		return "email"
	case "referral":
		return "referral"
	case "cpc":
		switch source {
		case "google", "bing", "baidu", "sogou", "360", "yandex", "duckduckgo", "yahoo":
			return "search"
		case "facebook", "x", "twitter", "instagram", "linkedin", "tiktok", "weibo":
			return "social"
		default:
			return "referral"
		}
	}

	switch source {
	case "direct", "none", "":
		return "direct"
	case "google", "bing", "baidu", "sogou", "360", "yandex", "duckduckgo", "yahoo":
		return "search"
	case "chatgpt", "perplexity", "gemini", "copilot", "claude", "deepseek", "kimi", "doubao", "yuanbao", "tongyi", "metaso", "poe":
		return "ai"
	case "x", "twitter", "facebook", "instagram", "linkedin", "reddit", "youtube", "weibo", "zhihu", "wechat", "bilibili", "xiaohongshu", "douyin", "tiktok", "tieba", "douban", "v2ex", "juejin", "telegram", "discord", "threads", "bluesky":
		return "social"
	}

	return "referral"
}

func classifySourceName(source, referrer string) string {
	source = strings.TrimSpace(source)
	if source != "" && source != "none" {
		return source
	}
	referrer = strings.TrimSpace(referrer)
	if referrer != "" {
		if parsed, err := url.Parse(referrer); err == nil && parsed.Hostname() != "" {
			return strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
		}
	}
	return "direct"
}

func (a *Analytics) buildVisitStreamItem(data *RequestData, eventTime time.Time) VisitStreamItem {
	rawIP := data.RawIP
	if rawIP == "" {
		rawIP = data.IP
	}
	item := VisitStreamItem{
		ID:             newStreamID(),
		Timestamp:      eventTime.Format(time.RFC3339),
		TimeStr:        eventTime.Format("15:04:05"),
		MaskedIP:       maskIP(rawIP),
		Country:        data.Country,
		Region:         data.Region,
		City:           data.City,
		SourceCategory: classifySourceCategory(data.Source, data.Medium),
		SourceName:     classifySourceName(data.Source, data.Referrer),
		SourceURL:      data.Referrer,
		SearchKeyword:  data.Term,
		Path:           data.Path,
		DeviceType:     data.DeviceType,
		Browser:        data.Browser,
	}
	if isAI, aiName := DetectAICrawler(data.UserAgent); isAI {
		item.IsAIAgent = true
		item.AIName = aiName
		item.SourceCategory = "ai"
		item.SourceName = aiName
		item.DeviceType = "bot"
		item.Browser = aiName
	}
	return item
}

func (a *Analytics) pushVisitStream(ctx context.Context, pipe redis.Pipeliner, data *RequestData, eventTime time.Time) error {
	item := a.buildVisitStreamItem(data, eventTime)
	encoded, err := json.Marshal(item)
	if err != nil {
		return err
	}
	streamKey := cache.BuildKey("project", data.ProjectID, "visit_stream")
	if pipe != nil {
		pipe.LPush(ctx, streamKey, string(encoded))
		pipe.LTrim(ctx, streamKey, 0, visitStreamCapacity-1)
		pipe.Expire(ctx, streamKey, visitStreamTTL)
		return nil
	}
	p := a.cache.Pipeline()
	p.LPush(ctx, streamKey, string(encoded))
	p.LTrim(ctx, streamKey, 0, visitStreamCapacity-1)
	p.Expire(ctx, streamKey, visitStreamTTL)
	_, err = p.Exec(ctx)
	return err
}

// GetVisitStream retrieves recent visit items from the real-time access log stream.
func (a *Analytics) GetVisitStream(ctx context.Context, projectID string, limit int) ([]VisitStreamItem, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	streamKey := cache.BuildKey("project", projectID, "visit_stream")
	rawItems, err := a.cache.GetClient().LRange(ctx, streamKey, 0, int64(limit-1)).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get visit stream: %w", err)
	}
	items := make([]VisitStreamItem, 0, len(rawItems))
	for _, raw := range rawItems {
		var item VisitStreamItem
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}
