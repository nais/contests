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
		indexCreateRequest := opensearchapi.CreateRequest{
			Index: indexName,
		}
		rs, err := indexCreateRequest.Do(ctx, client)
		if err != nil && rs.StatusCode == 400 {
			log.Info("Index already exists")
		} else {
			_, err = client.Indices.Create(indexName)
			if err != nil {
				http.Error(w, fmt.Sprintf("create index: %v", err), http.StatusInternalServerError)
				return
			}
			log.Info("Successfully created index")
		}
		// Create document
		epoch := fmt.Sprintf("%d", time.Now().UnixNano())
		document := strings.NewReader(`{ "Application": "contests" }`)
		documentCreateRequest := opensearchapi.CreateRequest{
			Index:      indexName,
			DocumentID: epoch,
			Body:       document,
		}
		rs, err = documentCreateRequest.Do(ctx, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("create document: %v", err), http.StatusInternalServerError)
			return
		}
		log.Info("Successfully wrote document to opensearch: %v", rs)

		// Retrieving same document
		getRequest := opensearchapi.GetRequest{
			Index:      indexName,
			DocumentID: epoch,
		}
		rs, err = getRequest.Do(ctx, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("get document: %v", err), http.StatusInternalServerError)
			return
		}

		log.Info("Successfully read document from opensearch: %v", rs)
		w.WriteHeader(http.StatusOK)
	}
}
