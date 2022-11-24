package bucket

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"cloud.google.com/go/storage"
	log "github.com/sirupsen/logrus"
)

const payload = "data"

func Handler(bucketName string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client, err := storage.NewClient(ctx)
		if err != nil {
			log.Errorf("Creating bucket client: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		bkt := client.Bucket(bucketName)
		obj := bkt.Object(payload)

		writer := obj.NewWriter(ctx)
		if _, err := fmt.Fprintf(writer, payload); err != nil {
			log.Errorf("Writing data: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		log.Info("Successfully wrote data to bucket")
		if err := writer.Close(); err != nil {
			log.Errorf("Closing writer: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		reader, err := obj.NewReader(ctx)
		if err != nil {
			log.Errorf("Creating reader: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer reader.Close()
		b, err := io.ReadAll(reader)
		if err != nil {
			log.Errorf("Reading data: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if string(b) != payload {
			log.Errorf("Wut, read wrong data")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		log.Info("Successfully read same data as we wrote from bucket")

		w.WriteHeader(http.StatusOK)
	}
}
