// Package valkey checks Valkey by writing, reading and deleting a short-lived key.
package valkey

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/nais/contests/internal/uniqid"
	"github.com/redis/go-redis/v9"
)

// Check verifies Valkey access using the injected workload credentials.
func Check(ctx context.Context) (err error) {
	uri, err := url.Parse(os.Getenv("VALKEY_URI_CONTESTS"))
	if err != nil {
		return fmt.Errorf("parse Valkey URI: %w", err)
	}
	if uri.Scheme != "valkeys" {
		return errors.New("VALKEY_URI_CONTESTS must use TLS (valkeys://)")
	}
	uri.Scheme = "rediss"
	opts, err := redis.ParseURL(uri.String())
	if err != nil {
		return fmt.Errorf("configure Valkey: %w", err)
	}
	opts.Username = os.Getenv("VALKEY_USERNAME_CONTESTS")
	opts.Password = os.Getenv("VALKEY_PASSWORD_CONTESTS")
	if opts.Username == "" || opts.Password == "" {
		return errors.New("valkey username and password are required")
	}
	opts.ContextTimeoutEnabled = true
	opts.MaxRetries = -1
	opts.PoolSize = 1
	client := redis.NewClient(opts)
	defer func() { err = errors.Join(err, client.Close()) }()
	return check(ctx, client)
}

func check(ctx context.Context, client *redis.Client) (err error) {
	value, err := uniqid.Suffix()
	if err != nil {
		return err
	}
	key := "contests:" + value
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		if cleanupErr := client.Del(cleanupCtx, key).Err(); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("delete key: %w", cleanupErr))
		}
	}()
	if err := client.Set(ctx, key, value, time.Minute).Err(); err != nil {
		return fmt.Errorf("set key: %w", err)
	}
	got, err := client.Get(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("get key: %w", err)
	}
	if got != value {
		return errors.New("key content did not match")
	}
	return nil
}
