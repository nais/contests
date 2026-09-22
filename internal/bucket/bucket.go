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
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()

		client, err := storage.NewClient(ctx)
		if err != nil {
			log.Errorf("Creating bucket client: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := client.Close(); err != nil {
				log.Errorf("Closing bucket client: %s", err)
			}
		}()
		bkt := client.Bucket(bucketName)
		objectName := fmt.Sprintf("contests-%d", time.Now().UnixNano())
		obj := bkt.Object(objectName)
		defer func() {
			if err := obj.Delete(ctx); err != nil {
				log.Errorf("Deleting bucket object: %s", err)
			}
		}()

		writer := obj.NewWriter(ctx)
		if _, err := fmt.Fprint(writer, payload); err != nil {
			log.Errorf("Writing data to bucket: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := writer.Close(); err != nil {
			log.Errorf("Closing bucket writer: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Info("Successfully wrote data to bucket")

		reader, err := obj.NewReader(ctx)
		if err != nil {
			log.Errorf("Creating bucket reader: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := reader.Close(); err != nil {
				log.Errorf("Closing bucket reader: %s", err)
			}
		}()
		b, err := io.ReadAll(reader)
		if err != nil {
			log.Errorf("Reading data from bucket: %s", err)
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
