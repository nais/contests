package opensearch

import (
	"context"
	"fmt"
	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func Handler(ctx context.Context, client *opensearch.Client) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		// Create index contests
		indexName := "contests"
		_, err := client.Indices.Create(indexName)
		if err != nil {
			http.Error(w, fmt.Sprintf("create index: %v", err), http.StatusInternalServerError)
			return
		}

		// Create document
		epoch := fmt.Sprintf("%d", time.Now().UnixNano())
		document := strings.NewReader(`{ "Application": "contests" }`)
		_, err = client.Create(indexName, epoch, document)
		if err != nil {
			http.Error(w, fmt.Sprintf("create document: %v", err), http.StatusInternalServerError)
			return
		}
		log.Info("Successfully wrote to opensearch")

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

		log.Info("Successfully read from opensearch")
		w.WriteHeader(http.StatusOK)
	}
}
