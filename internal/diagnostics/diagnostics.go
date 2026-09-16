package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/hoxiai/svgstat/internal/cache"
	"github.com/redis/go-redis/v9"
)

const retention = 30 * time.Minute

type Entry struct {
	Timestamp    time.Time `json:"timestamp"`
	Status       string    `json:"status"`
	Reason       string    `json:"reason,omitempty"`
	Mode         string    `json:"mode"`
	Type         string    `json:"type"`
	EventName    string    `json:"eventName,omitempty"`
	Path         string    `json:"path,omitempty"`
	Origin       string    `json:"origin,omitempty"`
	Referrer     string    `json:"referrer,omitempty"`
	Source       string    `json:"source,omitempty"`
	Medium       string    `json:"medium,omitempty"`
	Campaign     string    `json:"campaign,omitempty"`
	PropertyKeys []string  `json:"propertyKeys"`
}

type Issue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type Report struct {
	ProjectID      string     `json:"projectId"`
	Status         string     `json:"status"`
	Accepted       int64      `json:"accepted"`
	Rejected       int64      `json:"rejected"`
	TestEvents     int64      `json:"testEvents"`
	LastAcceptedAt *time.Time `json:"lastAcceptedAt,omitempty"`
	LastRejectedAt *time.Time `json:"lastRejectedAt,omitempty"`
	Issues         []Issue    `json:"issues"`
	Entries        []Entry    `json:"entries"`
}

type Service struct{ cache *cache.Cache }

func New(cache *cache.Cache) *Service { return &Service{cache: cache} }

func (s *Service) Record(ctx context.Context, projectID string, entry Entry) error {
	entry.Timestamp = time.Now().UTC()
	entry.Path = truncate(cleanPath(entry.Path), 512)
	entry.Origin = cleanOrigin(entry.Origin)
	entry.Referrer = cleanReferrer(entry.Referrer)
	sort.Strings(entry.PropertyKeys)
	encoded, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal diagnostic entry: %w", err)
	}
	entriesKey := cache.BuildKey("project", projectID, "diagnostics", "entries")
	pipe := s.cache.Pipeline()
	pipe.LPush(ctx, entriesKey, encoded)
	pipe.LTrim(ctx, entriesKey, 0, 99)
	pipe.Expire(ctx, entriesKey, retention)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *Service) GetReport(ctx context.Context, projectID string, trackingEnabled bool, domains []string, now time.Time) (*Report, error) {
	entriesKey := cache.BuildKey("project", projectID, "diagnostics", "entries")
	entriesCmd := s.cache.GetClient().LRange(ctx, entriesKey, 0, 99)
	entries, err := entriesCmd.Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to read diagnostics: %w", err)
	}
	report := &Report{ProjectID: projectID, Status: "healthy", Issues: []Issue{}, Entries: []Entry{}}
	for _, raw := range entries {
		var entry Entry
		if json.Unmarshal([]byte(raw), &entry) != nil {
			continue
		}
		age := now.UTC().Sub(entry.Timestamp)
		if age < 0 || age > retention {
			continue
		}
		report.Entries = append(report.Entries, entry)
		if entry.Status == "accepted" {
			report.Accepted++
			if report.LastAcceptedAt == nil || entry.Timestamp.After(*report.LastAcceptedAt) {
				value := entry.Timestamp
				report.LastAcceptedAt = &value
			}
		}
		if entry.Status == "rejected" {
			report.Rejected++
			if report.LastRejectedAt == nil || entry.Timestamp.After(*report.LastRejectedAt) {
				value := entry.Timestamp
				report.LastRejectedAt = &value
			}
		}
		if entry.Mode == "test" {
			report.TestEvents++
		}
	}
	if !trackingEnabled {
		report.Issues = append(report.Issues, Issue{Code: "tracking_disabled", Severity: "error", Message: "Website tracking is disabled."})
	}
	if len(domains) == 0 {
		report.Issues = append(report.Issues, Issue{Code: "domains_unrestricted", Severity: "warning", Message: "Allowed domains are empty; any website can send events."})
	}
	if report.LastAcceptedAt == nil {
		report.Issues = append(report.Issues, Issue{Code: "no_recent_events", Severity: "warning", Message: "No accepted events were seen in the last 30 minutes."})
	}
	if report.Rejected > 0 {
		report.Issues = append(report.Issues, Issue{Code: "recent_rejections", Severity: "error", Message: "Recent events were rejected; inspect the debugger for the reason."})
	}
	if hasDuplicatePageviews(report.Entries) {
		report.Issues = append(report.Issues, Issue{Code: "duplicate_pageviews", Severity: "warning", Message: "Repeated page views for the same path were detected within a few seconds."})
	}
	if hasMissingUTMAttribution(report.Entries) {
		report.Issues = append(report.Issues, Issue{Code: "utm_attribution_missing", Severity: "error", Message: "A URL contains UTM parameters but attribution was not captured."})
	}
	for _, issue := range report.Issues {
		if issue.Severity == "error" {
			report.Status = "error"
			return report, nil
		}
	}
	if len(report.Issues) > 0 {
		report.Status = "warning"
	}
	return report, nil
}

func (s *Service) Clear(ctx context.Context, projectID string) error {
	return s.cache.Delete(ctx, cache.BuildKey("project", projectID, "diagnostics", "entries"))
}

func PropertyKeys(raw map[string]interface{}) []string {
	keys := make([]string, 0, len(raw))
	for key, value := range raw {
		keys = append(keys, key+":"+propertyType(value))
	}
	sort.Strings(keys)
	return keys
}

func hasDuplicatePageviews(entries []Entry) bool {
	timestamps := map[string][]time.Time{}
	for _, entry := range entries {
		if entry.Status != "accepted" || entry.Type != "pageview" {
			continue
		}
		key := entry.Mode + "|" + entry.Origin + "|" + entry.Path
		timestamps[key] = append(timestamps[key], entry.Timestamp)
	}
	for _, values := range timestamps {
		sort.Slice(values, func(left, right int) bool { return values[left].Before(values[right]) })
		for index := 1; index < len(values); index++ {
			if values[index].Sub(values[index-1]) <= 3*time.Second {
				return true
			}
		}
	}
	return false
}

func hasMissingUTMAttribution(entries []Entry) bool {
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Path), "utm_source=") && (entry.Source == "" || entry.Source == "direct") {
			return true
		}
	}
	return false
}

func propertyType(value interface{}) string {
	switch value.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case json.Number, float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "number"
	case nil:
		return "null"
	default:
		return "unsupported"
	}
}
func cleanOrigin(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}
func cleanReferrer(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}
func cleanPath(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || !strings.HasPrefix(parsed.Path, "/") {
		return ""
	}
	query := url.Values{}
	for key := range parsed.Query() {
		query.Set(key, "")
	}
	if encoded := query.Encode(); encoded != "" {
		return parsed.EscapedPath() + "?" + encoded
	}
	return parsed.EscapedPath()
}
func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}
