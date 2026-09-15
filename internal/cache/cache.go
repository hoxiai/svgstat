package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/svgstat/svgstat/internal/config"
)

const rateLimitScript = `
local current = redis.call('INCR', KEYS[1])
if current == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
local ttl = redis.call('PTTL', KEYS[1])
return {current, ttl}
`

const reserveMemberScript = `
if redis.call('SISMEMBER', KEYS[1], ARGV[1]) == 1 then return 1 end
if redis.call('SCARD', KEYS[1]) >= tonumber(ARGV[2]) then return 0 end
redis.call('SADD', KEYS[1], ARGV[1])
redis.call('EXPIRE', KEYS[1], ARGV[3])
return 1
`

type Cache struct {
	client *redis.Client
}

func New(cfg *config.Config) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	log.Info().Msg("Connected to Redis successfully")
	return &Cache{client: client}, nil
}

func (c *Cache) Close() error {
	return c.client.Close()
}

func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *Cache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

func (c *Cache) Publish(ctx context.Context, channel, message string) error {
	return c.client.Publish(ctx, channel, message).Err()
}

func (c *Cache) Subscribe(ctx context.Context, channel string, handler func(string)) error {
	subscription := c.client.Subscribe(ctx, channel)
	defer subscription.Close()
	if _, err := subscription.Receive(ctx); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case message, ok := <-subscription.Channel():
			if !ok {
				return nil
			}
			handler(message.Payload)
		}
	}
}

func (c *Cache) AllowRate(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	result, err := c.client.Eval(ctx, rateLimitScript, []string{key}, window.Milliseconds()).Int64Slice()
	if err != nil {
		return false, 0, err
	}
	if len(result) != 2 {
		return false, 0, fmt.Errorf("unexpected rate limit response")
	}
	return result[0] <= int64(limit), time.Duration(result[1]) * time.Millisecond, nil
}

func (c *Cache) ReserveMember(ctx context.Context, key, member string, limit int, ttl time.Duration) (bool, error) {
	if limit <= 0 {
		return true, nil
	}
	result, err := c.client.Eval(ctx, reserveMemberScript, []string{key}, member, limit, int64(ttl.Seconds())).Int()
	return result == 1, err
}

func (c *Cache) Increment(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

func (c *Cache) IncrementBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.client.IncrBy(ctx, key, value).Result()
}

func (c *Cache) HashIncrement(ctx context.Context, key, field string) (int64, error) {
	return c.client.HIncrBy(ctx, key, field, 1).Result()
}

func (c *Cache) SetAdd(ctx context.Context, key, member string) (int64, error) {
	return c.client.SAdd(ctx, key, member).Result()
}

func (c *Cache) SetCard(ctx context.Context, key string) (int64, error) {
	return c.client.SCard(ctx, key).Result()
}

func (c *Cache) HashGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, key).Result()
}

func (c *Cache) Pipeline() redis.Pipeliner {
	return c.client.Pipeline()
}

func (c *Cache) GetClient() *redis.Client {
	return c.client
}

func BuildKey(parts ...string) string {
	return "svgstat:" + join(parts, ":")
}

func join(parts []string, sep string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += sep
		}
		result += part
	}
	return result
}
