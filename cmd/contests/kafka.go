package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
)

func checkKafka(ctx context.Context) error {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		return errors.New("KAFKA_BROKERS is required")
	}
	certificate, err := tls.LoadX509KeyPair(os.Getenv("KAFKA_CERTIFICATE_PATH"), os.Getenv("KAFKA_PRIVATE_KEY_PATH"))
	if err != nil {
		return fmt.Errorf("load Kafka client certificate: %w", err)
	}
	ca, err := os.ReadFile(os.Getenv("KAFKA_CA_PATH")) //nolint:gosec // path is supplied by the workload configuration
	if err != nil {
		return fmt.Errorf("load Kafka CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return errors.New("no certificates in Kafka CA file")
	}
	dialer := tls.Dialer{
		Config: &tls.Config{
			Certificates: []tls.Certificate{certificate},
			RootCAs:      roots,
			MinVersion:   tls.VersionTLS12,
		},
	}
	for broker := range strings.SplitSeq(brokers, ",") {
		conn, err := dialer.DialContext(ctx, "tcp", strings.TrimSpace(broker))
		if err != nil {
			return fmt.Errorf("connect to Kafka broker %q: %w", broker, err)
		}
		if err := conn.Close(); err != nil {
			return fmt.Errorf("close Kafka connection: %w", err)
		}
	}
	return nil
}
