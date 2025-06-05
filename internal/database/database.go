package database

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

func Handler(url string, logger log.FieldLogger) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logger.Info("Opening connection to database")
		db, err := sql.Open("postgres", url)
		if err != nil {
			logger.Errorf("Opening connection to database: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer db.Close()
		logger.Info("Pinging database")
		err = db.PingContext(ctx)
		if err != nil {
			logger.Errorf("Pinging database: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		logger.Infof("Successfully pinged database")
		w.WriteHeader(http.StatusOK)
	}
}
