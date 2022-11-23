package database

import (
	"database/sql"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
)

func Handler(dbUser, dbPassword, dbHost string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn := fmt.Sprintf("user=%s password=%s dbname=contests host=%s sslmode=disable",
			dbUser,
			dbPassword,
			dbHost)

		db, err := sql.Open("postgres", conn)
		if err != nil {
			log.Errorf("Opening connection to database: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer db.Close()
		err = db.Ping()
		if err != nil {
			log.Errorf("Pinging database: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Infof("Successfully pinged database")
		w.WriteHeader(http.StatusOK)
	}
}
