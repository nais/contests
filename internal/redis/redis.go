package redis

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

func Handler(ctx context.Context, client *redis.Client) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		epoch := fmt.Sprintf("%d", time.Now().UnixNano())
		err := client.Set(ctx, "foo", epoch, 0).Err()
		if err != nil {
			http.Error(w, fmt.Sprintf("create value: %v", err), http.StatusInternalServerError)
			return
		}
		log.Info("Successfully created value in redis")

		val, err := client.Get(ctx, "foo").Result()
		if err != nil {
			http.Error(w, fmt.Sprintf("get value: %v", err), http.StatusInternalServerError)
			return
		}
		if val != epoch {
			http.Error(w, fmt.Sprintf("unexpected value: %v", val), http.StatusInternalServerError)
			return
		}
		log.Info("Successfully read value from redis")
	}
}
