package analytics

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/hoxiai/svgstat/internal/cache"
	"github.com/redis/go-redis/v9"
)

const sessionTimeout = 30 * time.Minute

const sessionPageLimit = 500
const sessionFlowLimit = 1000

const updateSessionScript = `
local separator = ARGV[5]
local last_at = tonumber(redis.call('HGET', KEYS[1], 'last_at') or '0')
local stored_date = redis.call('HGET', KEYS[1], 'date')
local current_at = tonumber(ARGV[1])
if stored_date == ARGV[2] and last_at > 0 and current_at < last_at then return -1 end
local is_new = stored_date ~= ARGV[2] or last_at == 0 or current_at - last_at > tonumber(ARGV[4])

local function capped_increment(key, field, amount, limit)
  if redis.call('HEXISTS', key, field) == 1 or redis.call('HLEN', key) < limit then
    redis.call('HINCRBY', key, field, amount)
  end
end

local function increment_segments(metric, amount, use_stored)
  for index = 11, #ARGV, 2 do
    local dimension = ARGV[index]
    local value = ARGV[index + 1]
    if use_stored then value = redis.call('HGET', KEYS[1], 'segment:' .. dimension) or '' end
    if dimension and value and value ~= '' then
      capped_increment(KEYS[9], metric .. separator .. dimension .. separator .. value, amount, tonumber(ARGV[10]))
      if not use_stored then redis.call('HSET', KEYS[1], 'segment:' .. dimension, value) end
    end
  end
end

if is_new then
  for index = 11, #ARGV, 2 do redis.call('HDEL', KEYS[1], 'segment:' .. ARGV[index]) end
  redis.call('HSET', KEYS[1], 'date', ARGV[2], 'last_at', current_at, 'last_path', ARGV[3], 'pages', 1)
  redis.call('INCR', KEYS[2])
  redis.call('INCR', KEYS[3])
  redis.call('INCR', KEYS[5])
  capped_increment(KEYS[6], ARGV[3], 1, tonumber(ARGV[8]))
  capped_increment(KEYS[7], ARGV[3], 1, tonumber(ARGV[8]))
  increment_segments('sessions', 1, false)
  increment_segments('bounces', 1, false)
  increment_segments('pageviews', 1, false)
else
  local previous_path = redis.call('HGET', KEYS[1], 'last_path') or ''
  local pages = redis.call('HINCRBY', KEYS[1], 'pages', 1)
  local duration = math.floor((current_at - last_at) / 1000)
  if duration < 0 then duration = 0 end
  redis.call('INCRBY', KEYS[4], duration)
  redis.call('INCR', KEYS[5])
  if previous_path ~= '' then
    if redis.call('HEXISTS', KEYS[7], previous_path) == 1 then
      local remaining = redis.call('HINCRBY', KEYS[7], previous_path, -1)
      if remaining <= 0 then redis.call('HDEL', KEYS[7], previous_path) end
    end
    if previous_path ~= ARGV[3] then
      capped_increment(KEYS[8], previous_path .. separator .. ARGV[3], 1, tonumber(ARGV[9]))
    end
  end
  capped_increment(KEYS[7], ARGV[3], 1, tonumber(ARGV[8]))
  increment_segments('pageviews', 1, true)
  increment_segments('duration', duration, true)
  if pages == 2 then
    redis.call('DECR', KEYS[3])
    increment_segments('bounces', -1, true)
  end
  redis.call('HSET', KEYS[1], 'last_at', current_at, 'last_path', ARGV[3])
end

redis.call('EXPIRE', KEYS[1], tonumber(ARGV[7]))
for index = 2, #KEYS do redis.call('EXPIRE', KEYS[index], tonumber(ARGV[6])) end
return is_new and 1 or 0
`

func (a *Analytics) queueSessionUpdate(ctx context.Context, pipe redis.Pipeliner, data *RequestData, visitorHash, date string, now time.Time) {
	segments := analysisSegments(data)
	path := safeSessionDimension(data.Path)
	segmentFieldLimit := a.maxDimensionValues * 4
	if segmentFieldLimit <= 0 {
		segmentFieldLimit = 1000000
	}
	args := []interface{}{now.UnixMilli(), date, path, sessionTimeout.Milliseconds(), compositeSeparator, int64(a.keyTTL.Seconds()), int64(sessionTimeout.Seconds()), sessionPageLimit, sessionFlowLimit, segmentFieldLimit}
	for _, dimension := range []string{"source", "medium", "campaign", "device", "country"} {
		args = append(args, dimension, safeSessionDimension(segments[dimension]))
	}
	keys := []string{
		cache.BuildKey("project", data.ProjectID, "session", visitorHash),
		cache.BuildKey("project", data.ProjectID, "sessions", date),
		cache.BuildKey("project", data.ProjectID, "bounces", date),
		cache.BuildKey("project", data.ProjectID, "session_duration", date),
		cache.BuildKey("project", data.ProjectID, "session_pageviews", date),
		cache.BuildKey("project", data.ProjectID, "entrances", date),
		cache.BuildKey("project", data.ProjectID, "exits", date),
		cache.BuildKey("project", data.ProjectID, "page_flows", date),
		cache.BuildKey("project", data.ProjectID, "session_segments", date),
	}
	pipe.Eval(ctx, updateSessionScript, keys, args...)
}

func safeSessionDimension(value string) string {
	return strings.ReplaceAll(value, compositeSeparator, "")
}

func parseCounts(values map[string]string, target map[string]int64) {
	for field, raw := range values {
		target[field], _ = strconv.ParseInt(raw, 10, 64)
		if target[field] <= 0 {
			delete(target, field)
		}
	}
}

func parseSessionSegments(values map[string]string, target map[string]map[string]SessionQualityCounts) {
	for field, raw := range values {
		parts := splitCompositeField(field, 3)
		if len(parts) != 3 {
			continue
		}
		count, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			continue
		}
		metric, dimension, value := parts[0], parts[1], parts[2]
		if target[dimension] == nil {
			target[dimension] = map[string]SessionQualityCounts{}
		}
		quality := target[dimension][value]
		switch metric {
		case "sessions":
			quality.Sessions = count
		case "bounces":
			quality.Bounces = count
		case "pageviews":
			quality.Pageviews = count
		case "duration":
			quality.DurationSeconds = count
		}
		target[dimension][value] = quality
	}
}

func splitCompositeField(value string, parts int) []string {
	result := make([]string, 0, parts)
	for len(result) < parts-1 {
		index := -1
		for position := range value {
			if value[position] == compositeSeparator[0] {
				index = position
				break
			}
		}
		if index < 0 {
			return nil
		}
		result = append(result, value[:index])
		value = value[index+1:]
	}
	return append(result, value)
}
