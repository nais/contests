package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

func Handler(dbUser, dbPassword, dbHost string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn := fmt.Sprintf("user=%s password=%s dbname=contests host=%s sslmode=disable connect_timeout=5", dbUser, dbPassword, dbHost)
		db, err := sql.Open("postgres", conn)
		if err != nil {
			log.Errorf("Opening connection to database: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer db.Close()
		err = db.PingContext(ctx)
		if err != nil {
			log.Errorf("Pinging database: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Infof("Successfully pinged database")
		w.WriteHeader(http.StatusOK)
	}
}
