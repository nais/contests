package opensearch

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"

	log "github.com/sirupsen/logrus"
)

func Handler(ctx context.Context, client *opensearch.Client) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		// Creating document
		indexName := "contests"
		epoch := fmt.Sprintf("%d", time.Now().UnixNano())
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
		log.Info("Successfully created document in opensearch")

		defer res.Body.Close()

		// Retrieving same document
		getRequest := opensearchapi.GetRequest{
			Index:      indexName,
			DocumentID: epoch,
		}

		_, err = getRequest.Do(ctx, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("get document: %v", err), http.StatusInternalServerError)
			return
		}
		log.Info("Successfully read document from opensearch")

		// Deleting same document
		deleteRequest := opensearchapi.DeleteRequest{
			Index:      indexName,
			DocumentID: epoch,
		}

		_, err = deleteRequest.Do(ctx, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("delete document: %v", err), http.StatusInternalServerError)
			return
		}
		log.Info("Successfully deleted document from opensearch")

		w.WriteHeader(http.StatusOK)
	}
}
