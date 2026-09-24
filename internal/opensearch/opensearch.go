package opensearch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"

	"github.com/nais/contests/internal/uniqid"
	log "github.com/sirupsen/logrus"
)

func Handler(client *opensearch.Client) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()

		// Creating document
		indexName := "contests"
		epoch := uniqid.Suffix()
		indexRequest := opensearchapi.IndexRequest{
			Index:      indexName,
			DocumentID: epoch,
			Body:       strings.NewReader(`{ "Application": "contests" }`),
		}

		res, err := indexRequest.Do(ctx, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("create document: %v", err), http.StatusInternalServerError)
			return
		}
		if err := closeResponse(res); err != nil {
			http.Error(w, fmt.Sprintf("close create document response: %v", err), http.StatusInternalServerError)
			return
		}
		if res.IsError() {
			http.Error(w, fmt.Sprintf("create document: %s", res.Status()), http.StatusInternalServerError)
			return
		}
		log.Info("Successfully created document in opensearch")

		// Retrieving same document
		getRequest := opensearchapi.GetRequest{
			Index:      indexName,
			DocumentID: epoch,
		}

		getRes, err := getRequest.Do(ctx, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("get document: %v", err), http.StatusInternalServerError)
			return
		}
		if err := closeResponse(getRes); err != nil {
			http.Error(w, fmt.Sprintf("close get document response: %v", err), http.StatusInternalServerError)
			return
		}
		if getRes.IsError() {
			http.Error(w, fmt.Sprintf("get document: %s", getRes.Status()), http.StatusInternalServerError)
			return
		}
		log.Info("Successfully read document from opensearch")

		// Deleting same document
		deleteRequest := opensearchapi.DeleteRequest{
			Index:      indexName,
			DocumentID: epoch,
		}

		deleteRes, err := deleteRequest.Do(ctx, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("delete document: %v", err), http.StatusInternalServerError)
			return
		}
		if err := closeResponse(deleteRes); err != nil {
			http.Error(w, fmt.Sprintf("close delete document response: %v", err), http.StatusInternalServerError)
			return
		}
		if deleteRes.IsError() {
			http.Error(w, fmt.Sprintf("delete document: %s", deleteRes.Status()), http.StatusInternalServerError)
			return
		}
		log.Info("Successfully deleted document from opensearch")

		w.WriteHeader(http.StatusOK)
	}
}

func closeResponse(response *opensearchapi.Response) error {
	var drainErr error
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		drainErr = fmt.Errorf("drain response body: %w", err)
	}

	var closeErr error
	if err := response.Body.Close(); err != nil {
		closeErr = fmt.Errorf("close response body: %w", err)
	}

	if drainErr != nil || closeErr != nil {
		return errors.Join(drainErr, closeErr)
	}

	return nil
}
