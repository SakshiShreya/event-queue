package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// NewClient builds a Redis client from a URL such as redis://localhost:6379
// and checks the connection with a ping. The caller must Close the client.
func NewClient(ctx context.Context, url string) (*goredis.Client, error) {
	opts, err := goredis.ParseURL(url)
	if err != nil {
		// do not include the URL itself: it may contain a password
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}

	client := goredis.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping redis at %s: %w", opts.Addr, err)
	}

	return client, nil
}
