package valkey

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/nais/contests/internal/uniqid"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

func Handler(client *redis.Client) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()

		epoch := uniqid.Suffix()
		key := "contests:" + epoch
		defer func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			defer cleanupCancel()

			if err := client.Del(cleanupCtx, key).Err(); err != nil {
				log.Errorf("Deleting valkey value: %s", err)
			}
		}()

		err := client.Set(ctx, key, epoch, time.Minute).Err()
		if err != nil {
			http.Error(w, fmt.Sprintf("create value: %v", err), http.StatusInternalServerError)
			return
		}
		log.Info("Successfully created value in valkey")

		val, err := client.Get(ctx, key).Result()
		if err != nil {
			http.Error(w, fmt.Sprintf("get value: %v", err), http.StatusInternalServerError)
			return
		}
		if val != epoch {
			http.Error(w, fmt.Sprintf("unexpected value: %v", val), http.StatusInternalServerError)
			return
		}
		log.Info("Successfully read value from valkey")
	}
}
