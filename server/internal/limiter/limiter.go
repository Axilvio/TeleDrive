package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const ttl = 60 * time.Second

type Limiter struct {
	client *redis.Client
}

func New(redisAddr string) *Limiter {
	cli := redis.NewClient(&redis.Options{Addr: redisAddr})
	return &Limiter{client: cli}
}

func (l *Limiter) Close() error {
	return l.client.Close()
}

func (l *Limiter) Ping(ctx context.Context) error {
	return l.client.Ping(ctx).Err()
}

func (l *Limiter) AllowConnection(ctx context.Context, userID int64, maxDevices int) (bool, int64, error) {
	key := l.key(userID)

	current, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		return false, 0, fmt.Errorf("redis incr: %w", err)
	}

	if current == 1 {
		if err := l.client.Expire(ctx, key, ttl).Err(); err != nil {
			return false, 0, fmt.Errorf("redis expire: %w", err)
		}
	}

	if int(current) > maxDevices {
		if _, derr := l.client.Decr(ctx, key).Result(); derr != nil {
			return false, current, fmt.Errorf("limit exceeded and decr failed: %w", derr)
		}
		return false, current - 1, nil
	}

	if err := l.client.Expire(ctx, key, ttl).Err(); err != nil {
		return false, current, fmt.Errorf("refresh ttl: %w", err)
	}

	return true, current, nil
}

func (l *Limiter) ReleaseConnection(ctx context.Context, userID int64) error {
	key := l.key(userID)
	val, err := l.client.Decr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("redis decr: %w", err)
	}
	if val <= 0 {
		if err := l.client.Del(ctx, key).Err(); err != nil {
			return fmt.Errorf("redis del: %w", err)
		}
		return nil
	}
	if err := l.client.Expire(ctx, key, ttl).Err(); err != nil {
		return fmt.Errorf("refresh ttl: %w", err)
	}
	return nil
}

func (l *Limiter) key(userID int64) string {
	return fmt.Sprintf("active_sessions:%d", userID)
}
