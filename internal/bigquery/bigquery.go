package bigquery

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/nais/contests/internal/uniqid"
	log "github.com/sirupsen/logrus"
)

type TestTableRow struct {
	InsertTime time.Time
}

// Handler creates a temporary table with name current timestamp, that lasts for 1 minute. After creation it inserts a row with current timestamp as value.
func Handler(dataset *bigquery.Dataset) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()

		now := time.Now()

		row := TestTableRow{InsertTime: now}
		schema, err := bigquery.InferSchema(row)
		if err != nil {
			http.Error(w, fmt.Sprintf("infer schema: %v", err), http.StatusInternalServerError)
			return
		}

		suffix, err := uniqid.Suffix()
		if err != nil {
			http.Error(w, fmt.Sprintf("generate table ID: %v", err), http.StatusInternalServerError)
			return
		}
		table := dataset.Table("contests_" + suffix)
		md := &bigquery.TableMetadata{
			ExpirationTime: time.Now().Add(time.Minute),
			Schema:         schema,
		}

		err = table.Create(ctx, md)
		if err != nil {
			http.Error(w, fmt.Sprintf("create table: %v", err), http.StatusInternalServerError)
			return
		}

		err = table.Inserter().Put(ctx, row)
		if err != nil {
			http.Error(w, fmt.Sprintf("insert row: %v", err), http.StatusInternalServerError)
			return
		}

		log.Info("Successfully wrote to bigquery")
		w.WriteHeader(http.StatusOK)
	}
}
