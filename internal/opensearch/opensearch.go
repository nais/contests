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
		indexName := "contests"
		epoch := fmt.Sprintf("%d", time.Now().UnixNano())
		document := strings.NewReader(`{ "Application": "contests" }`)
		indexRequest := opensearchapi.IndexRequest{
			Index:      indexName,
			DocumentID: epoch,
			Body:       document,
		}

		rs, err := indexRequest.Do(ctx, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("create document: %v", err), http.StatusInternalServerError)
			return
		}
		defer rs.Body.Close()
		log.Info("Successfully created document in opensearch")

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
		log.Info("Successfully read document from opensearch")

		w.WriteHeader(http.StatusOK)
	}
}
