package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"
)

// Options configures the behavior of a cache operation
type Options struct {
	UseLocal bool          // Use in-memory cache
	UseRedis bool          // Use Redis cache
	TTL      time.Duration // Expiration time
}

// Manager defines the unified cache interface
type Manager interface {
	Get(ctx context.Context, key string, opts Options) (interface{}, bool, error)
	Set(ctx context.Context, key string, value interface{}, opts Options) error
	Delete(ctx context.Context, key string, opts Options) error
	
	// Increment is Redis-only
	Increment(ctx context.Context, key string, opts Options) (int64, error)
	
	// Close connections
	Close() error
}

type cacheManager struct {
	local *cache.Cache
	redis *redis.Client
}

// NewManager creates a new unified cache manager
func NewManager(redisURL string) (Manager, error) {
	// Initialize Local Cache (Default cleanup 10 mins)
	localCache := cache.New(10*time.Minute, 20*time.Minute)

	// Initialize Redis
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url: %w", err)
	}
	redisClient := redis.NewClient(opt)

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &cacheManager{
		local: localCache,
		redis: redisClient,
	}, nil
}

func (m *cacheManager) Get(ctx context.Context, key string, opts Options) (interface{}, bool, error) {
	// 1. Check Local
	if opts.UseLocal {
		if val, found := m.local.Get(key); found {
			return val, true, nil
		}
	}

	// 2. Check Redis
	if opts.UseRedis {
		val, err := m.redis.Get(ctx, key).Bytes()
		if err == redis.Nil {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}

		// Hit in Redis!
		// Backfill Local if enabled
		if opts.UseLocal {
			m.local.Set(key, val, opts.TTL)
		}

		// Sliding TTL in Redis (Async)
		if opts.TTL > 0 {
			go m.redis.Expire(ctx, key, opts.TTL)
		}

		return val, true, nil
	}

	return nil, false, nil
}

func (m *cacheManager) Set(ctx context.Context, key string, value interface{}, opts Options) error {
	// 1. Set Local
	if opts.UseLocal {
		// go-cache handles expiration automatically
		m.local.Set(key, value, opts.TTL)
	}

	// 2. Set Redis
	if opts.UseRedis {
		// Redis requires bytes or string usually, or Marshaler.
		// For simple usage, we assume value is bytes/string/int
		return m.redis.Set(ctx, key, value, opts.TTL).Err()
	}

	return nil
}

func (m *cacheManager) Increment(ctx context.Context, key string, opts Options) (int64, error) {
	if !opts.UseRedis {
		return 0, fmt.Errorf("increment requires UseRedis=true")
	}
	
	val, err := m.redis.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	
	// Set TTL if this is a fresh key (val=1) or just extend it
	if opts.TTL > 0 {
		m.redis.Expire(ctx, key, opts.TTL)
	}
	
	return val, nil
}

func (m *cacheManager) Delete(ctx context.Context, key string, opts Options) error {
	if opts.UseLocal {
		m.local.Delete(key)
	}
	if opts.UseRedis {
		return m.redis.Del(ctx, key).Err()
	}
	return nil
}

func (m *cacheManager) Close() error {
	return m.redis.Close()
}


