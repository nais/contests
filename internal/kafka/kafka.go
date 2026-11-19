package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/nais/contests/internal/uniqid"
)

// Kafka holds the connection settings for a single produce/consume check.
type Kafka struct {
	config  *sarama.Config
	brokers []string
	topic   string
}

// Check produces and consumes one message using the injected workload credentials.
func Check(ctx context.Context) error {
	k, err := New(os.Getenv("KAFKA_BROKERS"), os.Getenv("KAFKA_CA_PATH"), os.Getenv("KAFKA_CERTIFICATE_PATH"), os.Getenv("KAFKA_PRIVATE_KEY_PATH"), os.Getenv("KAFKA_TOPIC"))
	if err != nil {
		return err
	}
	return k.Check(ctx)
}

// New loads the current TLS credentials without connecting to Kafka.
func New(brokersString, caPath, certPath, keyPath, topic string) (*Kafka, error) {
	if brokersString == "" || caPath == "" || certPath == "" || keyPath == "" || topic == "" {
		return nil, errors.New("Kafka brokers, certificates and topic are required")
	}
	keypair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load Kafka client certificate: %w", err)
	}

	caCert, err := os.ReadFile(caPath) //nolint:gosec // path is supplied by the workload configuration
	if err != nil {
		return nil, fmt.Errorf("load Kafka CA: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("Kafka CA file contains no certificates")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{keypair},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS12,
	}

	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = tlsConfig
	config.Net.DialTimeout = time.Second
	config.Net.ReadTimeout = time.Second
	config.Net.WriteTimeout = time.Second
	config.Version = sarama.MaxVersion
	config.Admin.Timeout = time.Second
	config.Producer.Timeout = time.Second
	config.Producer.Retry.Max = 0
	config.Consumer.MaxWaitTime = time.Second
	config.Consumer.Return.Errors = true

	return &Kafka{
		config:  config,
		brokers: strings.Split(brokersString, ","),
		topic:   topic,
	}, nil
}

// Check verifies every broker, then produces and consumes the same message.
func (k *Kafka) Check(ctx context.Context) error {
	return runProbe(ctx, k.probe)
}

// runProbe returns when the check deadline expires.
// The worker may briefly outlive the check while an in-flight Sarama
// operation or cleanup finishes; Sarama timeouts are bounded by the same
// check deadline.
func runProbe(ctx context.Context, probe func(context.Context) error) error {
	if _, err := remainingTimeout(ctx); err != nil {
		return err
	}
	result := make(chan error, 1)
	go func() { result <- probe(ctx) }()
	select {
	case err := <-result:
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (k *Kafka) probe(ctx context.Context) (err error) {
	for _, b := range k.brokers {
		timeout, err := remainingTimeout(ctx)
		if err != nil {
			return err
		}

		brokerConfig := withOperationTimeout(k.config, timeout)
		broker := sarama.NewBroker(b)

		if err := broker.Open(brokerConfig); err != nil {
			_ = broker.Close()
			return fmt.Errorf("open broker %s: %w", broker.Addr(), err)
		}

		if _, err := remainingTimeout(ctx); err != nil {
			_ = broker.Close()
			return err
		}
		connected, err := broker.Connected()
		if err != nil || !connected {
			closeErr := broker.Close()

			if err == nil {
				err = fmt.Errorf("broker is not connected")
			}
			return errors.Join(fmt.Errorf("verify broker %s: %w", broker.Addr(), err), closeErr)
		}

		if err := broker.Close(); err != nil {
			return fmt.Errorf("close broker: %w", err)
		}
	}

	ts, err := uniqid.Suffix()
	if err != nil {
		return fmt.Errorf("generate message ID: %w", err)
	}

	producerTimeout, err := remainingTimeout(ctx)
	if err != nil {
		return err
	}

	producer, err := sarama.NewSyncProducer(k.brokers, withOperationTimeout(k.config, producerTimeout))
	if err != nil {
		return fmt.Errorf("create producer: %w", err)
	}
	defer func() {
		err = errors.Join(err, producer.Close())
	}()

	msg := &sarama.ProducerMessage{
		Topic: k.topic,
		Key:   sarama.StringEncoder(ts),
		Value: sarama.StringEncoder(ts),
	}

	if _, err := remainingTimeout(ctx); err != nil {
		return err
	}
	p, o, err := producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("produce message: %w", err)
	}

	consumerTimeout, err := remainingTimeout(ctx)
	if err != nil {
		return err
	}

	consumer, err := sarama.NewConsumer(k.brokers, withOperationTimeout(k.config, consumerTimeout))
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}
	defer func() {
		err = errors.Join(err, consumer.Close())
	}()

	if _, err := remainingTimeout(ctx); err != nil {
		return err
	}
	c, err := consumer.ConsumePartition(k.topic, p, o)
	if err != nil {
		return fmt.Errorf("consume partition: %w", err)
	}
	defer func() {
		err = errors.Join(err, c.Close())
	}()

	if _, err := remainingTimeout(ctx); err != nil {
		return err
	}
	consumedMessage, err := waitForMessage(ctx, c.Messages(), c.Errors())
	if err != nil {
		return fmt.Errorf("consume message: %w", err)
	}

	consumedValue := string(consumedMessage.Value)
	if consumedValue != ts {
		return fmt.Errorf("consumed message did not match produced message")
	}

	return nil
}

func remainingTimeout(ctx context.Context) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		return 0, fmt.Errorf("probe context has no deadline")
	}

	remaining := time.Until(deadline)
	if remaining < time.Millisecond {
		err := ctx.Err()
		if err == nil {
			return 0, context.DeadlineExceeded
		}
		return 0, err
	}

	return remaining, nil
}

func withOperationTimeout(base *sarama.Config, timeout time.Duration) *sarama.Config {
	config := *base
	operationTimeout := min(time.Second, timeout)
	config.Net.DialTimeout = operationTimeout
	config.Net.ReadTimeout = operationTimeout
	config.Net.WriteTimeout = operationTimeout
	config.Admin.Timeout = operationTimeout
	config.Producer.Timeout = operationTimeout
	config.Metadata.Timeout = operationTimeout
	config.Consumer.MaxWaitTime = min(base.Consumer.MaxWaitTime, operationTimeout)
	config.Metadata.Retry.Max = 0
	config.Metadata.Retry.Backoff = 0
	config.Producer.Retry.Max = 0
	config.Producer.Retry.Backoff = 0
	config.Consumer.Retry.Max = 1
	config.Consumer.Retry.Backoff = 0

	return &config
}

func waitForMessage(ctx context.Context, messages <-chan *sarama.ConsumerMessage, consumerErrors <-chan *sarama.ConsumerError) (*sarama.ConsumerMessage, error) {
	for {
		select {
		case message, ok := <-messages:
			if !ok {
				return nil, fmt.Errorf("messages channel closed")
			}
			if message == nil {
				return nil, fmt.Errorf("received nil message")
			}
			return message, nil
		case consumerErr, ok := <-consumerErrors:
			if !ok {
				consumerErrors = nil
				continue
			}
			if consumerErr == nil {
				continue
			}
			return nil, consumerErr.Err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
