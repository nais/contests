package opensearch

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	osgo "github.com/opensearch-project/opensearch-go"
)

func TestHandlerFailsOnOpenSearchErrorStatus(t *testing.T) {
	t.Parallel()

	body := &trackingBody{Reader: bytes.NewReader(nil)}
	client := newTestClient(t, roundTripper(func(*http.Request) (*http.Response, error) {
		return response(http.StatusInternalServerError, body), nil
	}))

	recorder := httptest.NewRecorder()
	http.HandlerFunc(Handler(client)).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/opensearch", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
}

func TestHandlerClosesEveryOpenSearchResponseBody(t *testing.T) {
	t.Parallel()

	var bodies []*trackingBody
	client := newTestClient(t, roundTripper(func(*http.Request) (*http.Response, error) {
		body := &trackingBody{Reader: bytes.NewReader(nil)}
		bodies = append(bodies, body)
		return response(http.StatusOK, body), nil
	}))

	recorder := httptest.NewRecorder()
	http.HandlerFunc(Handler(client)).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/opensearch", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if len(bodies) < 3 {
		t.Fatalf("response count = %d, want at least 3", len(bodies))
	}
	for index, body := range bodies {
		if !body.closed {
			t.Errorf("response body %d was not closed", index)
		}
	}
}

func TestHandlerCleansUpDocumentAfterReadFailure(t *testing.T) {
	t.Parallel()

	requests := 0
	client := newTestClient(t, roundTripper(func(request *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			return response(http.StatusOK, &trackingBody{Reader: bytes.NewReader(nil)}), nil
		case 2:
			return response(http.StatusOK, &trackingBody{Reader: bytes.NewReader(nil)}), nil
		case 3:
			return response(http.StatusInternalServerError, &trackingBody{Reader: bytes.NewReader(nil)}), nil
		case 4:
			return response(http.StatusOK, &trackingBody{Reader: bytes.NewReader(nil)}), nil
		default:
			t.Fatalf("unexpected request %d: %s", requests, request.URL.Path)
			return nil, nil
		}
	}))

	recorder := httptest.NewRecorder()
	http.HandlerFunc(Handler(client)).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/opensearch", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if requests != 4 {
		t.Fatalf("request count = %d, want 4", requests)
	}
}

func newTestClient(t *testing.T, transport http.RoundTripper) *osgo.Client {
	t.Helper()

	client, err := osgo.NewClient(osgo.Config{
		Addresses: []string{"http://opensearch"},
		Transport: transport,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func response(statusCode int, body io.ReadCloser) *http.Response {
	return &http.Response{
		Body:       body,
		Header:     make(http.Header),
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (roundTrip roundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

type trackingBody struct {
	io.Reader
	closed bool
}

func (body *trackingBody) Close() error {
	body.closed = true
	return nil
}
