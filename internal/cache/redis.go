package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisClient struct {
	rdb *redis.Client
	ttl time.Duration
}

func newRedisClient(addr string, ttlSeconds int) (*redisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, err
	}

	return &redisClient{
		rdb: rdb,
		ttl: time.Duration(ttlSeconds) * time.Second,
	}, nil
}

func (r *redisClient) Set(ctx context.Context, key string, data []byte) error {
	return r.rdb.Set(ctx, key, data, r.ttl).Err()
}

func (r *redisClient) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := r.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

func (r *redisClient) Del(ctx context.Context, key string) error {
	return r.rdb.Del(ctx, key).Err()
}

func (r *redisClient) Close() error {
	return r.rdb.Close()
}
