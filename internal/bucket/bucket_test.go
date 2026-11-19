package bucket

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

func TestCheckIncludesCleanupInOutcome(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name         string
		readStatus   int
		deleteStatus int
		content      string
		wantErr      string
	}{
		{name: "success", content: payload},
		{name: "mismatched content", content: "wrong", wantErr: "content did not match"},
		{name: "read failure still deletes object", readStatus: http.StatusForbidden, wantErr: "open object"},
		{name: "delete failure fails check", content: payload, deleteStatus: http.StatusForbidden, wantErr: "delete object"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			deleted := false
			transport := roundTripper(func(req *http.Request) (*http.Response, error) {
				status := http.StatusOK
				body := ""
				header := make(http.Header)
				switch req.Method {
				case http.MethodPost:
					header.Set("Content-Type", "application/json")
					body = `{"bucket":"test","name":"probe","generation":"1","size":"7"}`
				case http.MethodGet:
					if tt.readStatus != 0 {
						status = tt.readStatus
					}
					header.Set("Content-Type", "application/octet-stream")
					body = tt.content
				case http.MethodDelete:
					deleted = true
					status = http.StatusNoContent
					if tt.deleteStatus != 0 {
						status = tt.deleteStatus
					}
				default:
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
					status = http.StatusBadRequest
				}
				header.Set("Content-Length", strconv.Itoa(len(body)))
				return &http.Response{
					StatusCode:    status,
					Header:        header,
					Body:          io.NopCloser(strings.NewReader(body)),
					ContentLength: int64(len(body)),
					Request:       req,
				}, nil
			})
			client, err := storage.NewClient(t.Context(), option.WithHTTPClient(&http.Client{Transport: transport}), option.WithoutAuthentication())
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := client.Close(); err != nil {
					t.Error(err)
				}
			}()
			err = check(t.Context(), client.Bucket("test"))
			if tt.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if !deleted {
				t.Fatal("probe object was not cleaned up")
			}
		})
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
