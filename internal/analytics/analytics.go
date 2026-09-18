package analytics

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hoxiai/svgstat/internal/cache"
	"github.com/hoxiai/svgstat/internal/geoip"
	"github.com/hoxiai/svgstat/internal/project"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type Analytics struct {
	cache              *cache.Cache
	projectRepo        project.Repository
	geoIP              *geoip.GeoIP
	keyTTL             time.Duration
	ipSalt             string
	maxDailyVisitors   int
	maxDimensionValues int
	observer           interface{ ObserveCardinalityDrop(string) }
	clientIPResolver   interface{ ClientIP(*http.Request) string }
}

const cappedHashIncrementScript = `
if redis.call('HEXISTS', KEYS[1], ARGV[1]) == 1 or redis.call('HLEN', KEYS[1]) < tonumber(ARGV[3]) then
  redis.call('HINCRBY', KEYS[1], ARGV[1], ARGV[2])
  redis.call('EXPIRE', KEYS[1], ARGV[4])
  return 1
end
return 0
`

const boundedRegistrySetAddScript = `
if redis.call('HEXISTS', KEYS[1], ARGV[1]) == 1 or redis.call('HLEN', KEYS[1]) < tonumber(ARGV[3]) then
  redis.call('HSET', KEYS[1], ARGV[1], 1)
  redis.call('SADD', KEYS[2], ARGV[2])
  redis.call('EXPIRE', KEYS[1], ARGV[4])
  redis.call('EXPIRE', KEYS[2], ARGV[4])
  return 1
end
return 0
`

const boundedHashIncrementSetAddScript = `
if redis.call('HEXISTS', KEYS[1], ARGV[1]) == 1 or redis.call('HLEN', KEYS[1]) < tonumber(ARGV[4]) then
  redis.call('HINCRBY', KEYS[1], ARGV[1], ARGV[3])
  redis.call('SADD', KEYS[2], ARGV[2])
  redis.call('EXPIRE', KEYS[1], ARGV[5])
  redis.call('EXPIRE', KEYS[2], ARGV[5])
  return 1
end
return 0
`

type RequestData struct {
	ProjectID  string `json:"projectId"`
	IP         string `json:"ip"`
	UserAgent  string `json:"userAgent"`
	Referrer   string `json:"referrer"`
	Path       string `json:"path"`
	Country    string `json:"country"`
	Region     string `json:"region"`
	City       string `json:"city"`
	DeviceType string `json:"deviceType"`
	Browser    string `json:"browser"`
	IsBot      bool   `json:"isBot"`
	VisitorID  string `json:"-"`
	Source     string `json:"source"`
	Medium     string `json:"medium"`
	Campaign   string `json:"campaign"`
}

type DailyStats struct {
	ProjectID              string                                     `json:"projectId"`
	Date                   string                                     `json:"date"`
	PV                     int64                                      `json:"pv"`
	UV                     int64                                      `json:"uv"`
	Requests               int64                                      `json:"requests"`
	Bots                   int64                                      `json:"bots"`
	Referrers              map[string]int64                           `json:"referrers"`
	Countries              map[string]int64                           `json:"countries"`
	Regions                map[string]int64                           `json:"regions"`
	Cities                 map[string]int64                           `json:"cities"`
	Devices                map[string]int64                           `json:"devices"`
	Browsers               map[string]int64                           `json:"browsers"`
	Paths                  map[string]int64                           `json:"paths"`
	IPs                    map[string]int64                           `json:"ips"`
	Sources                map[string]int64                           `json:"sources"`
	Mediums                map[string]int64                           `json:"mediums"`
	Campaigns              map[string]int64                           `json:"campaigns"`
	Events                 map[string]int64                           `json:"events"`
	EventVisitors          map[string]int64                           `json:"eventVisitors"`
	EventSources           map[string]map[string]int64                `json:"eventSources"`
	EventMediums           map[string]map[string]int64                `json:"eventMediums"`
	EventCampaigns         map[string]map[string]int64                `json:"eventCampaigns"`
	EventValues            map[string]map[string]float64              `json:"eventValues"`
	FunnelSteps            map[string][]int64                         `json:"funnelSteps"`
	AudienceSegments       map[string]map[string]int64                `json:"audienceSegments"`
	EventSegments          map[string]map[string]map[string]int64     `json:"eventSegments"`
	FunnelSegments         map[string]map[string]map[string][]int64   `json:"funnelSegments"`
	Sessions               int64                                      `json:"sessions"`
	Bounces                int64                                      `json:"bounces"`
	SessionDurationSeconds int64                                      `json:"sessionDurationSeconds"`
	SessionPageviews       int64                                      `json:"sessionPageviews"`
	Entrances              map[string]int64                           `json:"entrances"`
	Exits                  map[string]int64                           `json:"exits"`
	PageFlows              map[string]int64                           `json:"pageFlows"`
	SessionSegments        map[string]map[string]SessionQualityCounts `json:"sessionSegments"`
}

type SessionQualityCounts struct {
	Sessions        int64 `json:"sessions"`
	Bounces         int64 `json:"bounces"`
	Pageviews       int64 `json:"pageviews"`
	DurationSeconds int64 `json:"durationSeconds"`
}

type EventData struct {
	Name         string
	Path         string
	Referrer     string
	VisitorID    string
	Value        *float64
	Currency     string
	PropertyKeys []string
	Dimensions   map[string]string
}

type VisitorDetail struct {
	VisitorID   string `json:"visitorId"`
	IP          string `json:"ip"`
	Path        string `json:"path"`
	Referrer    string `json:"referrer"`
	Country     string `json:"country"`
	Region      string `json:"region"`
	City        string `json:"city"`
	DeviceType  string `json:"deviceType"`
	Browser     string `json:"browser"`
	Requests    int64  `json:"requests"`
	FirstSeenAt string `json:"firstSeenAt"`
	LastSeenAt  string `json:"lastSeenAt"`
}

type VisitorList struct {
	ProjectID  string          `json:"projectId"`
	Date       string          `json:"date"`
	Page       int             `json:"page"`
	PageSize   int             `json:"pageSize"`
	Total      int             `json:"total"`
	TotalPages int             `json:"totalPages"`
	Items      []VisitorDetail `json:"items"`
}

type VisitorQuery struct {
	Page     int
	PageSize int
	Device   string
	Browser  string
	Path     string
	Sort     string
}

type InstallationTimes struct {
	FirstSeenAt *time.Time
	LastSeenAt  *time.Time
}

type RealtimeStats struct {
	ProjectID  string     `json:"projectId"`
	PV5        int64      `json:"pv5"`
	PV30       int64      `json:"pv30"`
	Visitors5  int64      `json:"visitors5"`
	Visitors30 int64      `json:"visitors30"`
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`
}

func New(cache *cache.Cache, projectRepo project.Repository, geoIP *geoip.GeoIP, keyTTL time.Duration, ipSalt string) *Analytics {
	if keyTTL <= 0 {
		keyTTL = 72 * time.Hour
	}
	return &Analytics{
		cache:       cache,
		projectRepo: projectRepo,
		geoIP:       geoIP,
		keyTTL:      keyTTL,
		ipSalt:      ipSalt,
	}
}

func (a *Analytics) Cache() *cache.Cache {
	return a.cache
}

func (a *Analytics) SetCardinalityLimits(maxDailyVisitors, maxDimensionValues int, observer interface{ ObserveCardinalityDrop(string) }) {
	a.maxDailyVisitors = maxDailyVisitors
	a.maxDimensionValues = maxDimensionValues
	a.observer = observer
}

func (a *Analytics) SetClientIPResolver(resolver interface{ ClientIP(*http.Request) string }) {
	a.clientIPResolver = resolver
}

// currentDate returns the UTC day bucket. Day buckets are UTC so runtime keys
// and the worker flush windows agree regardless of server timezone.
func currentDate() string {
	return time.Now().UTC().Format("2006-01-02")
}

func (a *Analytics) TrackRequest(ctx context.Context, req *http.Request, projectID string) error {
	data := a.extractRequestData(req, projectID)
	return a.trackRequestData(ctx, data, true, false)
}

func (a *Analytics) TrackPageview(ctx context.Context, req *http.Request, projectID, path, referrer, visitorID string) error {
	data := a.extractRequestData(req, projectID)
	data.Path, data.Referrer, data.Source, data.Medium, data.Campaign = websiteAttribution(path, referrer)
	data.VisitorID = visitorID
	return a.trackRequestData(ctx, data, false, true)
}

func (a *Analytics) TrackEvent(ctx context.Context, req *http.Request, projectID string, event EventData) error {
	data := a.extractRequestData(req, projectID)
	if data.IsBot {
		return nil
	}
	data.Path, data.Referrer, data.Source, data.Medium, data.Campaign = websiteAttribution(event.Path, event.Referrer)
	data.VisitorID = event.VisitorID
	event.Name = a.boundedEventName(ctx, projectID, currentDate(), event.Name)
	visitorHash := a.hashVisitor(data.IP, data.UserAgent+"|"+data.VisitorID)
	date := currentDate()
	exactVisitor := a.reserveExactVisitor(ctx, projectID, date, visitorHash)
	pipe := a.cache.Pipeline()

	eventsKey := cache.BuildKey("project", projectID, "events", date)
	visitorsKey := cache.BuildKey("project", projectID, "event_visitors", date, event.Name)
	pipe.HIncrBy(ctx, eventsKey, event.Name, 1)
	if exactVisitor {
		pipe.SAdd(ctx, visitorsKey, visitorHash)
	}
	keys := []string{eventsKey, visitorsKey}
	segments := analysisSegments(data)
	eventSegments := make(map[string]string, len(segments)+len(event.Dimensions))
	for key, value := range segments {
		eventSegments[key] = value
	}
	for key, value := range event.Dimensions {
		if value != "" {
			eventSegments["property_"+key] = value
		}
	}
	segmentRegistryKey := cache.BuildKey("project", projectID, "event_segment_values", date)
	if exactVisitor {
		for dimension, value := range eventSegments {
			field := compositeFields(event.Name, dimension, value)
			segmentVisitorKey := cache.BuildKey("project", projectID, "event_segment_visitors", date, event.Name, dimension, segmentKeyPart(value))
			a.queueBoundedSetAdd(ctx, pipe, segmentRegistryKey, segmentVisitorKey, field, visitorHash)
			keys = append(keys, segmentVisitorKey)
		}
	}
	keys = append(keys, segmentRegistryKey)
	for dimension, value := range map[string]string{"event_sources": data.Source, "event_mediums": data.Medium, "event_campaigns": data.Campaign} {
		if value == "" {
			continue
		}
		key := cache.BuildKey("project", projectID, dimension, date)
		field := compositeField(event.Name, value)
		visitorKey := cache.BuildKey("project", projectID, dimension+"_visitors", date, event.Name, segmentKeyPart(value))
		if exactVisitor {
			a.queueBoundedHashIncrementSetAdd(ctx, pipe, key, visitorKey, field, visitorHash, 1)
		} else {
			a.queueCappedHashIncrement(ctx, pipe, key, field, 1)
		}
		keys = append(keys, key, visitorKey)
	}
	if event.Value != nil {
		key := cache.BuildKey("project", projectID, "event_values", date)
		pipe.HIncrByFloat(ctx, key, compositeField(event.Name, event.Currency), *event.Value)
		keys = append(keys, key)
	}
	pipe.SetNX(ctx, cache.BuildKey("project", projectID, "installation", "first_seen"), time.Now().UTC().Format(time.RFC3339), 0)
	pipe.Set(ctx, cache.BuildKey("project", projectID, "installation", "last_seen"), time.Now().UTC().Format(time.RFC3339), 0)
	for _, key := range keys {
		pipe.Expire(ctx, key, a.keyTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to track custom event: %w", err)
	}
	if !exactVisitor {
		return nil
	}
	return a.advanceFunnels(ctx, projectID, date, visitorHash, event.Name, segments)
}

func (a *Analytics) boundedEventName(ctx context.Context, projectID, date, eventName string) string {
	if a.maxDimensionValues <= 0 {
		return eventName
	}
	key := cache.BuildKey("project", projectID, "event_names_guard", date)
	allowed, err := a.cache.ReserveMember(ctx, key, eventName, a.maxDimensionValues, a.keyTTL)
	if err != nil {
		log.Warn().Err(err).Str("project_id", projectID).Msg("Event cardinality guard unavailable")
		return "_other"
	}
	if !allowed {
		if a.observer != nil {
			a.observer.ObserveCardinalityDrop("event_name")
		}
		return "_other"
	}
	return eventName
}

func (a *Analytics) advanceFunnels(ctx context.Context, projectID, date, visitorHash, eventName string, segments map[string]string) error {
	definitions, err := a.cache.GetClient().HGetAll(ctx, cache.BuildKey("project", projectID, "funnel_definitions")).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to read funnel definitions: %w", err)
	}
	for funnelID, rawSteps := range definitions {
		var steps []string
		if json.Unmarshal([]byte(rawSteps), &steps) != nil || len(steps) < 2 {
			continue
		}
		stateKey := cache.BuildKey("project", projectID, "funnel_session", funnelID, visitorHash)
		segmentStateKey := cache.BuildKey("project", projectID, "funnel_session_segments", funnelID, visitorHash)
		stateValues, _ := a.cache.GetClient().MGet(ctx, stateKey, segmentStateKey).Result()
		state := parseStateValue(stateValues, 0)
		if state >= len(steps) {
			continue
		}
		funnelSegments := parseSegmentState(stateValues)
		if eventName != steps[state] {
			if state > 0 && eventName == steps[0] {
				state = 0
			} else {
				continue
			}
		}
		if state == 0 || len(funnelSegments) == 0 {
			funnelSegments = segments
		}
		segmentRegistryKey := cache.BuildKey("project", projectID, "funnel_segment_values", date)
		pipe := a.cache.Pipeline()
		for reachedStep := 0; reachedStep <= state; reachedStep++ {
			step := fmt.Sprintf("%d", reachedStep)
			stepKey := cache.BuildKey("project", projectID, "funnel_step", date, funnelID, step)
			pipe.SAdd(ctx, stepKey, visitorHash)
			pipe.Expire(ctx, stepKey, a.keyTTL)
			for dimension, value := range funnelSegments {
				field := compositeFields(funnelID, step, dimension, value)
				segmentVisitorKey := cache.BuildKey("project", projectID, "funnel_segment_visitors", date, funnelID, step, dimension, segmentKeyPart(value))
				a.queueBoundedSetAdd(ctx, pipe, segmentRegistryKey, segmentVisitorKey, field, visitorHash)
				pipe.Expire(ctx, segmentVisitorKey, a.keyTTL)
			}
		}
		pipe.Expire(ctx, segmentRegistryKey, a.keyTTL)
		pipe.Set(ctx, stateKey, state+1, 24*time.Hour)
		encodedSegments, _ := json.Marshal(funnelSegments)
		pipe.Set(ctx, segmentStateKey, encodedSegments, 24*time.Hour)
		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("failed to advance funnel: %w", err)
		}
	}
	return nil
}

func parseStateValue(values []interface{}, index int) int {
	if index >= len(values) {
		return 0
	}
	raw, ok := values[index].(string)
	if !ok {
		return 0
	}
	value, _ := strconv.Atoi(raw)
	return value
}

func parseSegmentState(values []interface{}) map[string]string {
	if len(values) < 2 {
		return nil
	}
	raw, ok := values[1].(string)
	if !ok || raw == "" {
		return nil
	}
	segments := map[string]string{}
	if json.Unmarshal([]byte(raw), &segments) != nil {
		return nil
	}
	return segments
}

const compositeSeparator = "\x1f"

func compositeField(eventName, dimension string) string {
	return eventName + compositeSeparator + dimension
}

func compositeFields(values ...string) string { return strings.Join(values, compositeSeparator) }

func segmentKeyPart(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:8])
}

func analysisSegments(data *RequestData) map[string]string {
	segments := map[string]string{
		"source": data.Source, "medium": data.Medium, "campaign": data.Campaign,
		"path": data.Path, "device": data.DeviceType, "country": data.Country,
	}
	for dimension, value := range segments {
		if value == "" {
			delete(segments, dimension)
		}
	}
	return segments
}

func (a *Analytics) trackRequestData(ctx context.Context, data *RequestData, countRequest, countPageview bool) error {
	projectID := data.ProjectID
	eventTime := time.Now().UTC()
	date := eventTime.Format("2006-01-02")
	now := eventTime.Format(time.RFC3339)
	pipe := a.cache.Pipeline()

	pvKey := cache.BuildKey("project", projectID, "pv", date)
	requestsKey := cache.BuildKey("project", projectID, "requests", date)
	botsKey := cache.BuildKey("project", projectID, "bots", date)
	uvSetKey := cache.BuildKey("project", projectID, "uvset", date)
	referrerKey := cache.BuildKey("project", projectID, "referrer", date)
	countryKey := cache.BuildKey("project", projectID, "country", date)
	regionKey := cache.BuildKey("project", projectID, "region", date)
	cityKey := cache.BuildKey("project", projectID, "city", date)
	deviceKey := cache.BuildKey("project", projectID, "device", date)
	browserKey := cache.BuildKey("project", projectID, "browser", date)
	pathKey := cache.BuildKey("project", projectID, "path", date)
	ipKey := cache.BuildKey("project", projectID, "network_id_v2", date)
	sourceKey := cache.BuildKey("project", projectID, "source", date)
	mediumKey := cache.BuildKey("project", projectID, "medium", date)
	campaignKey := cache.BuildKey("project", projectID, "campaign", date)
	visitorsKey := cache.BuildKey("project", projectID, "visitors_v2", date)
	installationFirstKey := cache.BuildKey("project", projectID, "installation", "first_seen")
	installationLastKey := cache.BuildKey("project", projectID, "installation", "last_seen")

	if countRequest {
		pipe.Incr(ctx, requestsKey)
	}
	pipe.SetNX(ctx, installationFirstKey, now, 0)
	pipe.Set(ctx, installationLastKey, now, 0)
	if data.IsBot {
		pipe.Incr(ctx, botsKey)
	} else if shouldCountPageview(data.IsBot, countPageview) {
		pipe.Incr(ctx, pvKey)
		visitorHash := a.hashVisitor(data.IP, data.UserAgent+"|"+data.VisitorID)
		exactVisitor := a.reserveExactVisitor(ctx, projectID, date, visitorHash)
		minute := eventTime.Format("200601021504")
		realtimePVKey := cache.BuildKey("project", projectID, "realtime", "pv", minute)
		realtimeVisitorsKey := cache.BuildKey("project", projectID, "realtime", "visitors", minute)
		realtimeLastSeenKey := cache.BuildKey("project", projectID, "realtime", "last_seen")
		visitorKey := cache.BuildKey("project", projectID, "visitor_v2", date, visitorHash)
		pipe.Incr(ctx, realtimePVKey)
		if exactVisitor {
			pipe.SAdd(ctx, realtimeVisitorsKey, visitorHash)
		}
		pipe.Expire(ctx, realtimePVKey, 2*time.Hour)
		pipe.Expire(ctx, realtimeVisitorsKey, 2*time.Hour)
		pipe.Set(ctx, realtimeLastSeenKey, now, 2*time.Hour)
		if exactVisitor {
			pipe.SAdd(ctx, uvSetKey, visitorHash)
			pipe.SAdd(ctx, visitorsKey, visitorHash)
			pipe.HSetNX(ctx, visitorKey, "visitor_id", visitorHash)
			pipe.HSetNX(ctx, visitorKey, "first_seen_at", now)
			pipe.HSet(ctx, visitorKey, map[string]interface{}{
				"last_seen_at": now,
				"ip":           data.IP,
				"path":         data.Path,
				"referrer":     data.Referrer,
				"country":      data.Country,
				"region":       data.Region,
				"city":         data.City,
				"device_type":  data.DeviceType,
				"browser":      data.Browser,
			})
			pipe.HIncrBy(ctx, visitorKey, "requests", 1)
			pipe.Expire(ctx, visitorKey, a.keyTTL)
			a.queueSessionUpdate(ctx, pipe, data, visitorHash, date, eventTime)
		}

		if data.Referrer != "" {
			a.queueCappedHashIncrement(ctx, pipe, referrerKey, data.Referrer, 1)
		}
		if data.Country != "" {
			a.queueCappedHashIncrement(ctx, pipe, countryKey, data.Country, 1)
		}
		if data.Region != "" {
			a.queueCappedHashIncrement(ctx, pipe, regionKey, data.Region, 1)
		}
		if data.City != "" {
			a.queueCappedHashIncrement(ctx, pipe, cityKey, data.City, 1)
		}
		if data.DeviceType != "" {
			a.queueCappedHashIncrement(ctx, pipe, deviceKey, data.DeviceType, 1)
		}
		if data.Browser != "" {
			a.queueCappedHashIncrement(ctx, pipe, browserKey, data.Browser, 1)
		}
		if data.Path != "" {
			a.queueCappedHashIncrement(ctx, pipe, pathKey, data.Path, 1)
		}
		if data.IP != "" {
			a.queueCappedHashIncrement(ctx, pipe, ipKey, data.IP, 1)
		}
		if data.Source != "" {
			a.queueCappedHashIncrement(ctx, pipe, sourceKey, data.Source, 1)
		}
		if data.Medium != "" {
			a.queueCappedHashIncrement(ctx, pipe, mediumKey, data.Medium, 1)
		}
		if data.Campaign != "" {
			a.queueCappedHashIncrement(ctx, pipe, campaignKey, data.Campaign, 1)
		}
		audienceRegistryKey := cache.BuildKey("project", projectID, "audience_segment_values", date)
		if exactVisitor {
			for dimension, value := range analysisSegments(data) {
				segmentVisitorKey := cache.BuildKey("project", projectID, "audience_segment_visitors", date, dimension, segmentKeyPart(value))
				a.queueBoundedSetAdd(ctx, pipe, audienceRegistryKey, segmentVisitorKey, compositeFields(dimension, value), visitorHash)
			}
		}
		pipe.Expire(ctx, audienceRegistryKey, a.keyTTL)
	}

	// Day-scoped keys must not live forever: the worker persists daily
	// aggregates to PostgreSQL, after which Redis copies are allowed to
	// expire. The TTL slides on every write, so keys survive until keyTTL
	// after the day's traffic stops.
	for _, key := range []string{
		pvKey, requestsKey, botsKey, uvSetKey, referrerKey, countryKey,
		regionKey, cityKey, deviceKey, browserKey, pathKey, ipKey, sourceKey,
		mediumKey, campaignKey, visitorsKey,
	} {
		pipe.Expire(ctx, key, a.keyTTL)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Error().Err(err).Str("project_id", projectID).Msg("Failed to track analytics")
		return fmt.Errorf("failed to track analytics: %w", err)
	}

	return nil
}

func (a *Analytics) reserveExactVisitor(ctx context.Context, projectID, date, visitorHash string) bool {
	if a.maxDailyVisitors <= 0 {
		return true
	}
	key := cache.BuildKey("project", projectID, "exact_visitors_guard", date)
	allowed, err := a.cache.ReserveMember(ctx, key, visitorHash, a.maxDailyVisitors, a.keyTTL)
	if err != nil {
		log.Warn().Err(err).Str("project_id", projectID).Msg("Cardinality guard unavailable; retaining aggregate counts only")
		return false
	}
	if !allowed && a.observer != nil {
		a.observer.ObserveCardinalityDrop("visitor")
	}
	return allowed
}

func (a *Analytics) queueCappedHashIncrement(ctx context.Context, pipe redis.Pipeliner, key, field string, amount int64) {
	if a.maxDimensionValues <= 0 {
		pipe.HIncrBy(ctx, key, field, amount)
		return
	}
	pipe.Eval(ctx, cappedHashIncrementScript, []string{key}, field, amount, a.maxDimensionValues, int64(a.keyTTL.Seconds()))
}

func (a *Analytics) queueBoundedSetAdd(ctx context.Context, pipe redis.Pipeliner, registryKey, setKey, field, member string) {
	if a.maxDimensionValues <= 0 {
		pipe.HSet(ctx, registryKey, field, 1)
		pipe.SAdd(ctx, setKey, member)
		return
	}
	pipe.Eval(ctx, boundedRegistrySetAddScript, []string{registryKey, setKey}, field, member, a.maxDimensionValues, int64(a.keyTTL.Seconds()))
}

func (a *Analytics) queueBoundedHashIncrementSetAdd(ctx context.Context, pipe redis.Pipeliner, hashKey, setKey, field, member string, amount int64) {
	if a.maxDimensionValues <= 0 {
		pipe.HIncrBy(ctx, hashKey, field, amount)
		pipe.SAdd(ctx, setKey, member)
		return
	}
	pipe.Eval(ctx, boundedHashIncrementSetAddScript, []string{hashKey, setKey}, field, member, amount, a.maxDimensionValues, int64(a.keyTTL.Seconds()))
}

func shouldCountPageview(isBot, pageviewEvent bool) bool {
	return pageviewEvent && !isBot
}

func (a *Analytics) GetInstallationTimes(ctx context.Context, projectID string) (*InstallationTimes, error) {
	values, err := a.cache.GetClient().MGet(ctx,
		cache.BuildKey("project", projectID, "installation", "first_seen"),
		cache.BuildKey("project", projectID, "installation", "last_seen"),
	).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get installation times: %w", err)
	}

	result := &InstallationTimes{}
	if len(values) > 0 {
		result.FirstSeenAt = parseOptionalTime(values[0])
	}
	if len(values) > 1 {
		result.LastSeenAt = parseOptionalTime(values[1])
	}
	return result, nil
}

func parseOptionalTime(value interface{}) *time.Time {
	raw, ok := value.(string)
	if !ok || raw == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func websiteAttribution(rawPath, rawReferrer string) (path, referrer, source, medium, campaign string) {
	path = "/"
	parsedPath, err := url.Parse(rawPath)
	if err == nil {
		if parsedPath.EscapedPath() != "" {
			path = parsedPath.EscapedPath()
		}
		if fragment := cleanPathFragment(parsedPath.Fragment); fragment != "" {
			path += "#" + fragment
		}
		query := parsedPath.Query()
		source = cleanDimension(query.Get("utm_source"))
		medium = cleanDimension(query.Get("utm_medium"))
		campaign = cleanDimension(query.Get("utm_campaign"))
	}
	referrer = cleanReferrer(rawReferrer)
	domain := referrerDomain(referrer)
	refSource, refMedium := classifyReferrer(domain)

	if source == "" {
		source = refSource
	}
	if medium == "" {
		if source == refSource {
			medium = refMedium
		} else if source == "direct" {
			medium = "none"
		} else {
			medium = "referral"
		}
	}
	return path, referrer, source, medium, campaign
}

func classifyReferrer(domain string) (source, medium string) {
	if domain == "" {
		return "direct", "none"
	}

	source = domain
	d := strings.TrimPrefix(strings.ToLower(domain), "www.")

	// Search engines -> organic
	if strings.Contains(d, "google.") ||
		strings.Contains(d, "bing.com") ||
		strings.Contains(d, "baidu.com") ||
		strings.Contains(d, "sogou.com") ||
		strings.Contains(d, "so.com") ||
		strings.Contains(d, "duckduckgo.com") ||
		strings.Contains(d, "yahoo.com") ||
		strings.Contains(d, "yandex.") ||
		strings.Contains(d, "ecosia.org") {
		return source, "organic"
	}

	// Social networks & communities -> social
	if d == "twitter.com" || d == "x.com" || d == "t.co" ||
		d == "facebook.com" || d == "instagram.com" ||
		d == "reddit.com" || d == "linkedin.com" ||
		d == "weibo.com" || d == "zhihu.com" ||
		d == "v2ex.com" || strings.Contains(d, "weixin.qq.com") {
		return source, "social"
	}

	return source, "referral"
}

func WebsiteAttribution(rawPath, rawReferrer string) (path, referrer, source, medium, campaign string) {
	return websiteAttribution(rawPath, rawReferrer)
}

func cleanDimension(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	return truncateRunes(value, 128)
}

func cleanPathFragment(value string) string {
	return truncateRunes(strings.TrimSpace(value), 256)
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

func cleanReferrer(value string) string {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		parsed.RawQuery = ""
		parsed.Fragment = ""
		parsed.RawPath = ""
		value = parsed.String()
	}
	return truncateRunes(value, 2048)
}

func referrerDomain(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

func (a *Analytics) GetRealtimeStats(ctx context.Context, projectID string, now time.Time) (*RealtimeStats, error) {
	minuteKeys := make([]string, 0, 30)
	visitorKeys := make([]string, 0, 30)
	for offset := 0; offset < 30; offset++ {
		minute := now.UTC().Add(-time.Duration(offset) * time.Minute).Format("200601021504")
		minuteKeys = append(minuteKeys, cache.BuildKey("project", projectID, "realtime", "pv", minute))
		visitorKeys = append(visitorKeys, cache.BuildKey("project", projectID, "realtime", "visitors", minute))
	}
	pipe := a.cache.Pipeline()
	pvCommands := make([]*redis.StringCmd, len(minuteKeys))
	for index, key := range minuteKeys {
		pvCommands[index] = pipe.Get(ctx, key)
	}
	visitors5Cmd := pipe.SUnion(ctx, visitorKeys[:5]...)
	visitors30Cmd := pipe.SUnion(ctx, visitorKeys...)
	lastSeenCmd := pipe.Get(ctx, cache.BuildKey("project", projectID, "realtime", "last_seen"))
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get realtime statistics: %w", err)
	}
	result := &RealtimeStats{ProjectID: projectID}
	for index, command := range pvCommands {
		value, _ := command.Int64()
		result.PV30 += value
		if index < 5 {
			result.PV5 += value
		}
	}
	result.Visitors5 = int64(len(visitors5Cmd.Val()))
	result.Visitors30 = int64(len(visitors30Cmd.Val()))
	if parsed := parseOptionalTime(lastSeenCmd.Val()); parsed != nil {
		result.LastSeenAt = parsed
	}
	return result, nil
}

func (a *Analytics) GetTodayStats(ctx context.Context, projectID string) (*DailyStats, error) {
	return a.GetStats(ctx, projectID, currentDate())
}

func (a *Analytics) GetStats(ctx context.Context, projectID, date string) (*DailyStats, error) {
	pipe := a.cache.Pipeline()

	pvKey := cache.BuildKey("project", projectID, "pv", date)
	requestsKey := cache.BuildKey("project", projectID, "requests", date)
	botsKey := cache.BuildKey("project", projectID, "bots", date)
	uvSetKey := cache.BuildKey("project", projectID, "uvset", date)
	referrerKey := cache.BuildKey("project", projectID, "referrer", date)
	countryKey := cache.BuildKey("project", projectID, "country", date)
	regionKey := cache.BuildKey("project", projectID, "region", date)
	cityKey := cache.BuildKey("project", projectID, "city", date)
	deviceKey := cache.BuildKey("project", projectID, "device", date)
	browserKey := cache.BuildKey("project", projectID, "browser", date)
	pathKey := cache.BuildKey("project", projectID, "path", date)
	ipKey := cache.BuildKey("project", projectID, "network_id_v2", date)
	sourceKey := cache.BuildKey("project", projectID, "source", date)
	mediumKey := cache.BuildKey("project", projectID, "medium", date)
	campaignKey := cache.BuildKey("project", projectID, "campaign", date)
	eventsKey := cache.BuildKey("project", projectID, "events", date)
	eventSourcesKey := cache.BuildKey("project", projectID, "event_sources", date)
	eventMediumsKey := cache.BuildKey("project", projectID, "event_mediums", date)
	eventCampaignsKey := cache.BuildKey("project", projectID, "event_campaigns", date)
	eventValuesKey := cache.BuildKey("project", projectID, "event_values", date)
	sessionsKey := cache.BuildKey("project", projectID, "sessions", date)
	bouncesKey := cache.BuildKey("project", projectID, "bounces", date)
	sessionDurationKey := cache.BuildKey("project", projectID, "session_duration", date)
	sessionPageviewsKey := cache.BuildKey("project", projectID, "session_pageviews", date)
	entrancesKey := cache.BuildKey("project", projectID, "entrances", date)
	exitsKey := cache.BuildKey("project", projectID, "exits", date)
	pageFlowsKey := cache.BuildKey("project", projectID, "page_flows", date)
	sessionSegmentsKey := cache.BuildKey("project", projectID, "session_segments", date)

	pvCmd := pipe.Get(ctx, pvKey)
	requestsCmd := pipe.Get(ctx, requestsKey)
	botsCmd := pipe.Get(ctx, botsKey)
	uvCmd := pipe.SCard(ctx, uvSetKey)
	referrersCmd := pipe.HGetAll(ctx, referrerKey)
	countriesCmd := pipe.HGetAll(ctx, countryKey)
	regionsCmd := pipe.HGetAll(ctx, regionKey)
	citiesCmd := pipe.HGetAll(ctx, cityKey)
	devicesCmd := pipe.HGetAll(ctx, deviceKey)
	browsersCmd := pipe.HGetAll(ctx, browserKey)
	pathsCmd := pipe.HGetAll(ctx, pathKey)
	ipsCmd := pipe.HGetAll(ctx, ipKey)
	sourcesCmd := pipe.HGetAll(ctx, sourceKey)
	mediumsCmd := pipe.HGetAll(ctx, mediumKey)
	campaignsCmd := pipe.HGetAll(ctx, campaignKey)
	eventsCmd := pipe.HGetAll(ctx, eventsKey)
	eventSourcesCmd := pipe.HGetAll(ctx, eventSourcesKey)
	eventMediumsCmd := pipe.HGetAll(ctx, eventMediumsKey)
	eventCampaignsCmd := pipe.HGetAll(ctx, eventCampaignsKey)
	eventValuesCmd := pipe.HGetAll(ctx, eventValuesKey)
	sessionsCmd := pipe.Get(ctx, sessionsKey)
	bouncesCmd := pipe.Get(ctx, bouncesKey)
	sessionDurationCmd := pipe.Get(ctx, sessionDurationKey)
	sessionPageviewsCmd := pipe.Get(ctx, sessionPageviewsKey)
	entrancesCmd := pipe.HGetAll(ctx, entrancesKey)
	exitsCmd := pipe.HGetAll(ctx, exitsKey)
	pageFlowsCmd := pipe.HGetAll(ctx, pageFlowsKey)
	sessionSegmentsCmd := pipe.HGetAll(ctx, sessionSegmentsKey)

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	stats := &DailyStats{
		ProjectID: projectID,
		Date:      date,
		Referrers: make(map[string]int64),
		Countries: make(map[string]int64),
		Regions:   make(map[string]int64),
		Cities:    make(map[string]int64),
		Devices:   make(map[string]int64),
		Browsers:  make(map[string]int64),
		Paths:     make(map[string]int64),
		IPs:       make(map[string]int64),
		Sources:   make(map[string]int64),
		Mediums:   make(map[string]int64),
		Campaigns: make(map[string]int64),
		Events:    make(map[string]int64), EventVisitors: make(map[string]int64),
		EventSources: make(map[string]map[string]int64), EventMediums: make(map[string]map[string]int64),
		EventCampaigns: make(map[string]map[string]int64), EventValues: make(map[string]map[string]float64),
		FunnelSteps:      make(map[string][]int64),
		AudienceSegments: make(map[string]map[string]int64),
		EventSegments:    make(map[string]map[string]map[string]int64),
		FunnelSegments:   make(map[string]map[string]map[string][]int64),
		Entrances:        make(map[string]int64),
		Exits:            make(map[string]int64),
		PageFlows:        make(map[string]int64),
		SessionSegments:  make(map[string]map[string]SessionQualityCounts),
	}

	stats.PV, _ = pvCmd.Int64()
	stats.Requests, _ = requestsCmd.Int64()
	stats.Bots, _ = botsCmd.Int64()
	stats.UV = uvCmd.Val()
	stats.Sessions, _ = sessionsCmd.Int64()
	stats.Bounces, _ = bouncesCmd.Int64()
	stats.SessionDurationSeconds, _ = sessionDurationCmd.Int64()
	stats.SessionPageviews, _ = sessionPageviewsCmd.Int64()

	for k, v := range referrersCmd.Val() {
		stats.Referrers[k], _ = parseToInt64(v)
	}
	for k, v := range countriesCmd.Val() {
		stats.Countries[k], _ = parseToInt64(v)
	}
	for k, v := range regionsCmd.Val() {
		stats.Regions[k], _ = parseToInt64(v)
	}
	for k, v := range citiesCmd.Val() {
		stats.Cities[k], _ = parseToInt64(v)
	}
	for k, v := range devicesCmd.Val() {
		stats.Devices[k], _ = parseToInt64(v)
	}
	for k, v := range browsersCmd.Val() {
		stats.Browsers[k], _ = parseToInt64(v)
	}
	for k, v := range pathsCmd.Val() {
		stats.Paths[k], _ = parseToInt64(v)
	}
	for k, v := range ipsCmd.Val() {
		stats.IPs[k], _ = parseToInt64(v)
	}
	for k, v := range sourcesCmd.Val() {
		stats.Sources[k], _ = parseToInt64(v)
	}
	for k, v := range mediumsCmd.Val() {
		stats.Mediums[k], _ = parseToInt64(v)
	}
	for k, v := range campaignsCmd.Val() {
		stats.Campaigns[k], _ = parseToInt64(v)
	}
	for k, v := range eventsCmd.Val() {
		stats.Events[k], _ = parseToInt64(v)
	}
	parseCounts(entrancesCmd.Val(), stats.Entrances)
	parseCounts(exitsCmd.Val(), stats.Exits)
	parseCounts(pageFlowsCmd.Val(), stats.PageFlows)
	parseSessionSegments(sessionSegmentsCmd.Val(), stats.SessionSegments)
	if len(stats.Events) > 0 {
		visitorPipe := a.cache.Pipeline()
		visitorCmds := make(map[string]*redis.IntCmd, len(stats.Events))
		for eventName := range stats.Events {
			visitorCmds[eventName] = visitorPipe.SCard(ctx, cache.BuildKey("project", projectID, "event_visitors", date, eventName))
		}
		_, _ = visitorPipe.Exec(ctx)
		for eventName, cmd := range visitorCmds {
			stats.EventVisitors[eventName], _ = cmd.Result()
		}
	}
	parseNestedCounts(eventSourcesCmd.Val(), stats.EventSources)
	parseNestedCounts(eventMediumsCmd.Val(), stats.EventMediums)
	parseNestedCounts(eventCampaignsCmd.Val(), stats.EventCampaigns)
	a.replaceNestedCountsWithVisitors(ctx, projectID, date, "event_sources", stats.EventSources)
	a.replaceNestedCountsWithVisitors(ctx, projectID, date, "event_mediums", stats.EventMediums)
	a.replaceNestedCountsWithVisitors(ctx, projectID, date, "event_campaigns", stats.EventCampaigns)
	parseNestedValues(eventValuesCmd.Val(), stats.EventValues)
	definitions, _ := a.cache.GetClient().HGetAll(ctx, cache.BuildKey("project", projectID, "funnel_definitions")).Result()
	for funnelID, rawSteps := range definitions {
		var steps []string
		if json.Unmarshal([]byte(rawSteps), &steps) != nil {
			continue
		}
		counts := make([]int64, len(steps))
		for index := range steps {
			counts[index], _ = a.cache.GetClient().SCard(ctx, cache.BuildKey("project", projectID, "funnel_step", date, funnelID, fmt.Sprintf("%d", index))).Result()
		}
		stats.FunnelSteps[funnelID] = counts
	}
	a.loadAudienceSegments(ctx, projectID, date, stats)
	a.loadEventSegments(ctx, projectID, date, stats)
	a.loadFunnelSegments(ctx, projectID, date, definitions, stats)

	return stats, nil
}

func (a *Analytics) loadAudienceSegments(ctx context.Context, projectID, date string, stats *DailyStats) {
	fields, _ := a.cache.GetClient().HKeys(ctx, cache.BuildKey("project", projectID, "audience_segment_values", date)).Result()
	for _, field := range fields {
		parts := strings.SplitN(field, compositeSeparator, 2)
		if len(parts) != 2 {
			continue
		}
		if stats.AudienceSegments[parts[0]] == nil {
			stats.AudienceSegments[parts[0]] = map[string]int64{}
		}
		stats.AudienceSegments[parts[0]][parts[1]], _ = a.cache.GetClient().SCard(ctx, cache.BuildKey("project", projectID, "audience_segment_visitors", date, parts[0], segmentKeyPart(parts[1]))).Result()
	}
}

func (a *Analytics) loadEventSegments(ctx context.Context, projectID, date string, stats *DailyStats) {
	fields, _ := a.cache.GetClient().HKeys(ctx, cache.BuildKey("project", projectID, "event_segment_values", date)).Result()
	for _, field := range fields {
		parts := strings.SplitN(field, compositeSeparator, 3)
		if len(parts) != 3 {
			continue
		}
		if stats.EventSegments[parts[0]] == nil {
			stats.EventSegments[parts[0]] = map[string]map[string]int64{}
		}
		if stats.EventSegments[parts[0]][parts[1]] == nil {
			stats.EventSegments[parts[0]][parts[1]] = map[string]int64{}
		}
		stats.EventSegments[parts[0]][parts[1]][parts[2]], _ = a.cache.GetClient().SCard(ctx, cache.BuildKey("project", projectID, "event_segment_visitors", date, parts[0], parts[1], segmentKeyPart(parts[2]))).Result()
	}
}

func (a *Analytics) loadFunnelSegments(ctx context.Context, projectID, date string, definitions map[string]string, stats *DailyStats) {
	fields, _ := a.cache.GetClient().HKeys(ctx, cache.BuildKey("project", projectID, "funnel_segment_values", date)).Result()
	for _, field := range fields {
		parts := strings.SplitN(field, compositeSeparator, 4)
		if len(parts) != 4 {
			continue
		}
		var steps []string
		if json.Unmarshal([]byte(definitions[parts[0]]), &steps) != nil {
			continue
		}
		step, err := strconv.Atoi(parts[1])
		if err != nil || step < 0 || step >= len(steps) {
			continue
		}
		if stats.FunnelSegments[parts[0]] == nil {
			stats.FunnelSegments[parts[0]] = map[string]map[string][]int64{}
		}
		if stats.FunnelSegments[parts[0]][parts[2]] == nil {
			stats.FunnelSegments[parts[0]][parts[2]] = map[string][]int64{}
		}
		counts := stats.FunnelSegments[parts[0]][parts[2]][parts[3]]
		if len(counts) != len(steps) {
			counts = make([]int64, len(steps))
		}
		counts[step], _ = a.cache.GetClient().SCard(ctx, cache.BuildKey("project", projectID, "funnel_segment_visitors", date, parts[0], parts[1], parts[2], segmentKeyPart(parts[3]))).Result()
		stats.FunnelSegments[parts[0]][parts[2]][parts[3]] = counts
	}
}

func (a *Analytics) replaceNestedCountsWithVisitors(ctx context.Context, projectID, date, dimension string, target map[string]map[string]int64) {
	for eventName, values := range target {
		for value := range values {
			count, err := a.cache.GetClient().SCard(ctx, cache.BuildKey("project", projectID, dimension+"_visitors", date, eventName, segmentKeyPart(value))).Result()
			if err == nil {
				target[eventName][value] = count
			}
		}
	}
}

func parseNestedCounts(values map[string]string, target map[string]map[string]int64) {
	for field, raw := range values {
		parts := strings.SplitN(field, compositeSeparator, 2)
		if len(parts) != 2 {
			continue
		}
		if target[parts[0]] == nil {
			target[parts[0]] = make(map[string]int64)
		}
		target[parts[0]][parts[1]], _ = parseToInt64(raw)
	}
}

func parseNestedValues(values map[string]string, target map[string]map[string]float64) {
	for field, raw := range values {
		parts := strings.SplitN(field, compositeSeparator, 2)
		if len(parts) != 2 {
			continue
		}
		if target[parts[0]] == nil {
			target[parts[0]] = make(map[string]float64)
		}
		var value float64
		_, _ = fmt.Sscanf(raw, "%f", &value)
		target[parts[0]][parts[1]] = value
	}
}

func (a *Analytics) GetTodayVisitors(ctx context.Context, projectID string, query VisitorQuery) (*VisitorList, error) {
	return a.GetVisitors(ctx, projectID, currentDate(), query)
}

func (a *Analytics) GetVisitors(ctx context.Context, projectID, date string, query VisitorQuery) (*VisitorList, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}

	visitorIDs, err := a.cache.GetClient().SMembers(ctx, cache.BuildKey("project", projectID, "visitors_v2", date)).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get visitors: %w", err)
	}

	details, err := a.getVisitorDetails(ctx, projectID, date, visitorIDs)
	if err != nil {
		return nil, err
	}

	details = filterVisitorDetails(details, query)
	sortVisitorDetails(details, query.Sort)

	total := len(details)
	totalPages := 0
	if total > 0 {
		totalPages = (total + query.PageSize - 1) / query.PageSize
	}
	if totalPages == 0 {
		query.Page = 1
	} else if query.Page > totalPages {
		query.Page = totalPages
	}

	start := (query.Page - 1) * query.PageSize
	end := start + query.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	items := make([]VisitorDetail, 0)
	if start < end {
		items = details[start:end]
	}

	return &VisitorList{
		ProjectID:  projectID,
		Date:       date,
		Page:       query.Page,
		PageSize:   query.PageSize,
		Total:      total,
		TotalPages: totalPages,
		Items:      items,
	}, nil
}

func (a *Analytics) extractRequestData(req *http.Request, projectID string) *RequestData {
	data := &RequestData{
		ProjectID: projectID,
		IP:        a.clientIP(req),
		UserAgent: req.UserAgent(),
		Referrer:  req.Referer(),
		Path:      req.URL.Path,
	}

	// GitHub's Camo image proxy strips the Referer, so embeds there can name
	// their source page explicitly, e.g. ?page_id=github.com/user/repo.
	// Camo's own User-Agent is still recognizable, so at least attribute
	// unlabelled fetches to github.com.
	if pageID := strings.TrimSpace(req.URL.Query().Get("page_id")); pageID != "" {
		data.Referrer = pageID
	} else if data.Referrer == "" && strings.Contains(strings.ToLower(data.UserAgent), "github-camo") {
		data.Referrer = "github.com"
	}

	data.IsBot = a.isBot(data.UserAgent)
	data.DeviceType = a.detectDeviceType(data.UserAgent)
	data.Browser = a.detectBrowser(data.UserAgent)

	if a.geoIP != nil {
		location, err := a.geoIP.Lookup(data.IP)
		if err == nil {
			data.Country = location.CountryISO
			data.Region = location.Region
			data.City = location.City
		}
	}
	data.IP = a.anonymizeIP(data.IP)

	return data
}

func (a *Analytics) clientIP(request *http.Request) string {
	if a.clientIPResolver != nil {
		return a.clientIPResolver.ClientIP(request)
	}
	host, _, _ := net.SplitHostPort(request.RemoteAddr)
	return host
}

func (a *Analytics) anonymizeIP(ip string) string {
	if ip == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(a.ipSalt + "|" + ip))
	return "anon_" + hex.EncodeToString(hash[:])[:12]
}

func (a *Analytics) hashVisitor(ip, userAgent string) string {
	hash := sha256.New()
	hash.Write([]byte(ip + "|" + userAgent))
	return hex.EncodeToString(hash.Sum(nil))[:16]
}

func (a *Analytics) isBot(userAgent string) bool {
	botKeywords := []string{
		"bot", "crawler", "spider", "scraper", "curl", "wget",
		"googlebot", "bingbot", "slurp", "duckduckbot", "baiduspider",
		"yandexbot", "sogou", "exabot", "facebot", "ia_archiver",
	}

	userAgentLower := strings.ToLower(userAgent)
	for _, keyword := range botKeywords {
		if strings.Contains(userAgentLower, keyword) {
			return true
		}
	}
	return false
}

func (a *Analytics) detectDeviceType(userAgent string) string {
	uaLower := strings.ToLower(userAgent)

	if strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "android") ||
		strings.Contains(uaLower, "mobile") {
		return "mobile"
	}
	if strings.Contains(uaLower, "ipad") || strings.Contains(uaLower, "tablet") {
		return "tablet"
	}
	return "desktop"
}

func (a *Analytics) detectBrowser(userAgent string) string {
	uaLower := strings.ToLower(userAgent)

	if strings.Contains(uaLower, "chrome") && !strings.Contains(uaLower, "edg") {
		return "chrome"
	}
	if strings.Contains(uaLower, "firefox") {
		return "firefox"
	}
	if strings.Contains(uaLower, "safari") && !strings.Contains(uaLower, "chrome") {
		return "safari"
	}
	if strings.Contains(uaLower, "edg") {
		return "edge"
	}
	if strings.Contains(uaLower, "opera") || strings.Contains(uaLower, "opr") {
		return "opera"
	}
	return "other"
}

func parseToInt64(s string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

func (a *Analytics) getVisitorDetails(ctx context.Context, projectID, date string, visitorIDs []string) ([]VisitorDetail, error) {
	if len(visitorIDs) == 0 {
		return []VisitorDetail{}, nil
	}

	visitorPipe := a.cache.Pipeline()
	visitorDetailCmds := make(map[string]*redis.MapStringStringCmd, len(visitorIDs))
	for _, visitorID := range visitorIDs {
		visitorKey := cache.BuildKey("project", projectID, "visitor_v2", date, visitorID)
		visitorDetailCmds[visitorID] = visitorPipe.HGetAll(ctx, visitorKey)
	}

	if _, err := visitorPipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get visitor details: %w", err)
	}

	details := make([]VisitorDetail, 0, len(visitorIDs))
	for _, visitorID := range visitorIDs {
		detail := buildVisitorDetail(visitorID, visitorDetailCmds[visitorID].Val())
		if detail.Requests == 0 {
			continue
		}
		details = append(details, detail)
	}

	sort.Slice(details, func(i, j int) bool {
		leftTime, _ := time.Parse(time.RFC3339, details[i].LastSeenAt)
		rightTime, _ := time.Parse(time.RFC3339, details[j].LastSeenAt)
		if leftTime.Equal(rightTime) {
			return details[i].Requests > details[j].Requests
		}
		return leftTime.After(rightTime)
	})

	return details, nil
}

func filterVisitorDetails(details []VisitorDetail, query VisitorQuery) []VisitorDetail {
	device := strings.TrimSpace(strings.ToLower(query.Device))
	browser := strings.TrimSpace(strings.ToLower(query.Browser))
	path := strings.TrimSpace(strings.ToLower(query.Path))

	if device == "" && browser == "" && path == "" {
		return details
	}

	filtered := make([]VisitorDetail, 0, len(details))
	for _, detail := range details {
		if device != "" && strings.ToLower(detail.DeviceType) != device {
			continue
		}
		if browser != "" && strings.ToLower(detail.Browser) != browser {
			continue
		}
		if path != "" && !strings.Contains(strings.ToLower(detail.Path), path) {
			continue
		}
		filtered = append(filtered, detail)
	}

	return filtered
}

func sortVisitorDetails(details []VisitorDetail, sortBy string) {
	switch sortBy {
	case "requests_asc":
		sort.Slice(details, func(i, j int) bool {
			if details[i].Requests == details[j].Requests {
				return details[i].LastSeenAt < details[j].LastSeenAt
			}
			return details[i].Requests < details[j].Requests
		})
	case "requests_desc":
		sort.Slice(details, func(i, j int) bool {
			if details[i].Requests == details[j].Requests {
				return details[i].LastSeenAt > details[j].LastSeenAt
			}
			return details[i].Requests > details[j].Requests
		})
	case "first_seen_asc":
		sort.Slice(details, func(i, j int) bool {
			return details[i].FirstSeenAt < details[j].FirstSeenAt
		})
	case "first_seen_desc":
		sort.Slice(details, func(i, j int) bool {
			return details[i].FirstSeenAt > details[j].FirstSeenAt
		})
	default:
		sort.Slice(details, func(i, j int) bool {
			if details[i].LastSeenAt == details[j].LastSeenAt {
				return details[i].Requests > details[j].Requests
			}
			return details[i].LastSeenAt > details[j].LastSeenAt
		})
	}
}

func buildVisitorDetail(visitorID string, data map[string]string) VisitorDetail {
	detail := VisitorDetail{
		VisitorID:   visitorID,
		IP:          data["ip"],
		Path:        data["path"],
		Referrer:    data["referrer"],
		Country:     data["country"],
		Region:      data["region"],
		City:        data["city"],
		DeviceType:  data["device_type"],
		Browser:     data["browser"],
		FirstSeenAt: data["first_seen_at"],
		LastSeenAt:  data["last_seen_at"],
	}
	detail.Requests, _ = parseToInt64(data["requests"])
	return detail
}

func (d VisitorDetail) MarshalJSON() ([]byte, error) {
	type Alias VisitorDetail
	return json.Marshal(Alias(d))
}
