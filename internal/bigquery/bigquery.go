package bigquery

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/bigquery"
	log "github.com/sirupsen/logrus"
)

const payload = "data"

type TestTableRow struct {
	InsertTime time.Time
}

// Creates a temporary table with name current timestamp, that lasts for 1 minute. After creation it inserts a row with current timestamp as value.
func Handler(ctx context.Context, dataset *bigquery.Dataset) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		now := time.Now()

		row := TestTableRow{InsertTime: now}
		schema, err := bigquery.InferSchema(row)
		if err != nil {
			http.Error(w, fmt.Sprintf("infer schema: %v", err), http.StatusInternalServerError)
			return
		}

		table := dataset.Table(fmt.Sprintf("%s", now))
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