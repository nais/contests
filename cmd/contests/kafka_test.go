package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestKafkaEndpointChecksTLSConnectivity(t *testing.T) {
	server := httptest.NewUnstartedServer(http.NotFoundHandler())
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAnyClientCert}
	server.StartTLS()
	defer server.Close()

	cert := server.TLS.Certificates[0]
	key, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, file := range []struct {
		env, name, kind string
		data            []byte
	}{
		{"KAFKA_CA_PATH", "ca.crt", "CERTIFICATE", cert.Certificate[0]},
		{"KAFKA_CERTIFICATE_PATH", "client.crt", "CERTIFICATE", cert.Certificate[0]},
		{"KAFKA_PRIVATE_KEY_PATH", "client.key", "PRIVATE KEY", key},
	} {
		path := filepath.Join(dir, file.name)
		if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: file.kind, Bytes: file.data}), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(file.env, path)
	}

	stalled, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := stalled.Close(); err != nil {
			t.Error(err)
		}
	}()
	broker := server.Listener.Addr().String()
	_, port, err := net.SplitHostPort(broker)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, brokers string
		timeout       time.Duration
		want          int
	}{
		{"TLS connection with client certificate", broker, time.Second, http.StatusOK},
		{"every listed broker must be reachable", broker + "," + stalled.Addr().String(), 100 * time.Millisecond, http.StatusServiceUnavailable},
		{"broker certificate identity must match", net.JoinHostPort("localhost", port), time.Second, http.StatusServiceUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KAFKA_BROKERS", tt.brokers)
			ctx, cancel := context.WithTimeout(t.Context(), tt.timeout)
			defer cancel()
			request := httptest.NewRequest(http.MethodGet, "/kafka", nil).WithContext(ctx)
			recorder := httptest.NewRecorder()
			newHandler(newRegistry(), testLogger()).ServeHTTP(recorder, request)
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, tt.want, recorder.Body.String())
			}
		})
	}

	t.Run("credentials are loaded for each request", func(t *testing.T) {
		t.Setenv("KAFKA_BROKERS", broker)
		handler := newHandler(newRegistry(), testLogger())
		t.Setenv("KAFKA_PRIVATE_KEY_PATH", filepath.Join(dir, "missing.key"))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/kafka", nil))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
		}
		if strings.Contains(recorder.Body.String(), dir) {
			t.Error("response must not expose credential paths")
		}
	})
}
