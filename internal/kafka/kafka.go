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

		// verify all brokers are working as expected
		for _, b := range k.brokers {
			timeout, err := remainingTimeout(ctx)
			if err != nil {
				log.Errorf("probe deadline exceeded before opening broker connection: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			brokerConfig := withOperationTimeout(k.config, timeout)
			broker := sarama.NewBroker(b)

			if err := broker.Open(brokerConfig); err != nil {
				log.Errorf("opening connection to broker: %s: %s", broker.Addr(), err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			connected, err := broker.Connected()
			if err != nil || !connected {
				if closeErr := broker.Close(); closeErr != nil {
					log.Errorf("could not close connection to broker %s: %s", broker.Addr(), closeErr)
				}

				log.Errorf("verifying connection to broker: %s: %s", broker.Addr(), err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			log.Infof("Successfully connected to Kafka broker: %s", broker.Addr())

			if err := broker.Close(); err != nil {
				log.Errorf("could not close connection: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		ts, err := uniqid.Suffix()
		if err != nil {
			http.Error(w, fmt.Sprintf("generate message ID: %v", err), http.StatusInternalServerError)
			return
		}

		// test produce to topic
		producerTimeout, err := remainingTimeout(ctx)
		if err != nil {
			log.Errorf("probe deadline exceeded before creating producer: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		producer, err := sarama.NewSyncProducer(k.brokers, withOperationTimeout(k.config, producerTimeout))
		if err != nil {
			log.Errorf("could not create kafka producer: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
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

		p, o, err := producer.SendMessage(msg)
		if err != nil {
			log.Errorf("could not produce message to topic (%s): %s", k.topic, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
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
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		consumer, err := sarama.NewConsumer(k.brokers, withOperationTimeout(k.config, consumerTimeout))
		if err != nil {
			log.Errorf("could not create kafka consumer: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := consumer.Close(); err != nil {
				log.Errorf("could not close kafka consumer: %s", err)
			}
		}()

		c, err := consumer.ConsumePartition(k.topic, p, o)
		if err != nil {
			log.Errorf(
				"could not consume partition (%d) from topic (%s): %s",
				p,
				k.topic,
				err,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := c.Close(); err != nil {
				log.Errorf("could not close partition consumer: %s", err)
			}
		}()

		consumedMessage, err := waitForMessage(ctx, c.Messages(), c.Errors())
		if err != nil {
			log.Errorf("could not consume message: %s", err)
			http.Error(w, "consume message", http.StatusInternalServerError)
			return
		}

		consumedValue := string(consumedMessage.Value)
		if consumedValue != ts {
			log.Infof(
				"consumed (%s) is not equal to what we produced (%s)",
				consumedValue,
				ts,
			)
			http.Error(w, "consumed message did not match produced message", http.StatusInternalServerError)
			return
		}

		log.Infof("consumed same msg (%s) as we produced.", ts)
		w.WriteHeader(http.StatusOK)
	}
}

func remainingTimeout(ctx context.Context) (time.Duration, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return time.Second, nil
	}

	remaining := time.Until(deadline)
	if remaining <= 0 {
		err := ctx.Err()
		if err == nil {
			return 0, context.DeadlineExceeded
		}
		return 0, err
	}

	return remaining, nil
}

func withOperationTimeout(base *sarama.Config, timeout time.Duration) *sarama.Config {
	if timeout <= 0 {
		timeout = time.Millisecond
	}

	config := *base
	config.Net.DialTimeout = timeout
	config.Net.ReadTimeout = timeout
	config.Net.WriteTimeout = timeout
	config.Admin.Timeout = timeout
	config.Producer.Timeout = timeout
	config.Consumer.MaxWaitTime = timeout

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
