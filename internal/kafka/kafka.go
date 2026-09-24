package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/nais/contests/internal/uniqid"
	log "github.com/sirupsen/logrus"
)

type Kafka struct {
	config  *sarama.Config
	brokers []string
	topic   string
}

func New(brokersString, caPath, certPath, keyPath, topic string) (*Kafka, error) {
	keypair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	caCert, err := os.ReadFile(caPath) //nolint:gosec // caPath is provided via CLI flag/env, not user input
	if err != nil {
		log.Println(err)
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{keypair},
		RootCAs:      caCertPool,
	}

	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = tlsConfig
	config.Net.DialTimeout = time.Second
	config.Net.ReadTimeout = time.Second
	config.Net.WriteTimeout = time.Second
	config.Version = sarama.V0_10_2_0
	config.Admin.Timeout = time.Second
	config.Producer.Timeout = time.Second
	config.Producer.Retry.Max = 0
	config.Consumer.MaxWaitTime = time.Second
	config.Consumer.Return.Errors = true

	var brokers []string
	brokers = append(brokers, strings.Split(brokersString, ",")...)

	return &Kafka{
		config:  config,
		brokers: brokers,
		topic:   topic,
	}, nil
}

func (k *Kafka) Handler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()

		if _, err := remainingTimeout(ctx); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := runProbe(ctx, k.probe); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

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

func (k *Kafka) probe(ctx context.Context) error {
	// verify all brokers are working as expected
	for _, b := range k.brokers {
		timeout, err := remainingTimeout(ctx)
		if err != nil {
			log.Errorf("probe deadline exceeded before opening broker connection: %s", err)
			return err
		}

		brokerConfig := withOperationTimeout(k.config, timeout)
		broker := sarama.NewBroker(b)

		if err := broker.Open(brokerConfig); err != nil {
			_ = broker.Close()
			log.Errorf("opening connection to broker: %s: %s", broker.Addr(), err)
			return err
		}

		if _, err := remainingTimeout(ctx); err != nil {
			_ = broker.Close()
			return err
		}
		connected, err := broker.Connected()
		if err != nil || !connected {
			if closeErr := broker.Close(); closeErr != nil {
				log.Errorf("could not close connection to broker %s: %s", broker.Addr(), closeErr)
			}

			log.Errorf("verifying connection to broker: %s: %s", broker.Addr(), err)
			return fmt.Errorf("verifying connection to broker %s: %v", broker.Addr(), err)
		}

		log.Infof("Successfully connected to Kafka broker: %s", broker.Addr())

		if err := broker.Close(); err != nil {
			log.Errorf("could not close connection: %s", err)
			return err
		}
	}

	ts, err := uniqid.Suffix()
	if err != nil {
		return fmt.Errorf("generate message ID: %w", err)
	}

	// test produce to topic
	producerTimeout, err := remainingTimeout(ctx)
	if err != nil {
		log.Errorf("probe deadline exceeded before creating producer: %s", err)
		return err
	}

	producer, err := sarama.NewSyncProducer(k.brokers, withOperationTimeout(k.config, producerTimeout))
	if err != nil {
		log.Errorf("could not create kafka producer: %s", err)
		return err
	}
	defer func() {
		if err := producer.Close(); err != nil {
			log.Errorf("could not close kafka producer: %s", err)
		}
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
		log.Errorf("could not produce message to topic (%s): %s", k.topic, err)
		return err
	}

	log.Infof(
		"produced message to kafka topic: %s (partition: %d, offset: %d)",
		k.topic,
		p,
		o,
	)

	consumerTimeout, err := remainingTimeout(ctx)
	if err != nil {
		log.Errorf("probe deadline exceeded before creating consumer: %s", err)
		return err
	}

	consumer, err := sarama.NewConsumer(k.brokers, withOperationTimeout(k.config, consumerTimeout))
	if err != nil {
		log.Errorf("could not create kafka consumer: %s", err)
		return err
	}
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Errorf("could not close kafka consumer: %s", err)
		}
	}()

	if _, err := remainingTimeout(ctx); err != nil {
		return err
	}
	c, err := consumer.ConsumePartition(k.topic, p, o)
	if err != nil {
		log.Errorf(
			"could not consume partition (%d) from topic (%s): %s",
			p,
			k.topic,
			err,
		)
		return err
	}
	defer func() {
		if err := c.Close(); err != nil {
			log.Errorf("could not close partition consumer: %s", err)
		}
	}()

	if _, err := remainingTimeout(ctx); err != nil {
		return err
	}
	consumedMessage, err := waitForMessage(ctx, c.Messages(), c.Errors())
	if err != nil {
		log.Errorf("could not consume message: %s", err)
		return err
	}

	consumedValue := string(consumedMessage.Value)
	if consumedValue != ts {
		log.Infof(
			"consumed (%s) is not equal to what we produced (%s)",
			consumedValue,
			ts,
		)
		return fmt.Errorf("consumed message did not match produced message")
	}

	log.Infof("consumed same msg (%s) as we produced.", ts)
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
	config.Net.DialTimeout = timeout
	config.Net.ReadTimeout = timeout
	config.Net.WriteTimeout = timeout
	config.Admin.Timeout = timeout
	config.Producer.Timeout = timeout
	config.Metadata.Timeout = timeout
	config.Consumer.MaxWaitTime = timeout
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
