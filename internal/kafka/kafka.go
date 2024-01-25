package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Shopify/sarama"
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

	caCert, err := os.ReadFile(caPath)
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
	config.Version = sarama.V0_10_2_0
	config.Admin.Timeout = 5 * time.Second

	var brokers []string
	for _, b := range strings.Split(brokersString, ",") {
		brokers = append(brokers, b)
	}

	return &Kafka{
		config:  config,
		brokers: brokers,
		topic:   topic,
	}, nil
}

func (k *Kafka) Handler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		// verify all brokers are working as expected
		for _, b := range k.brokers {
			broker := sarama.NewBroker(b)
			if err := broker.Open(k.config); err != nil {
				log.Errorf("opening connection to broker: %s: %s", broker.Addr(), err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			connected, err := broker.Connected()
			if err != nil || !connected {
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

		ts := fmt.Sprintf("%d", time.Now().Unix())
		// test produce to topic
		producer, err := sarama.NewSyncProducer(k.brokers, k.config)
		if err != nil {
			log.Errorf("could not create kafka producer: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		msg := &sarama.ProducerMessage{
			Topic: k.topic,
			Key:   sarama.StringEncoder(ts),
			Value: sarama.StringEncoder(ts),
		}
		p, o, err := producer.SendMessage(msg)
		log.Infof("produces message to kafka topic: %s (partition: %d, offset: %d)", "", p, o)

		consumer, err := sarama.NewConsumer(k.brokers, k.config)
		if err != nil {
			log.Errorf("could not create kafka consumer: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
		}

		c, err := consumer.ConsumePartition(k.topic, p, o)
		consumedMessage := <-c.Messages()

		consumedValue := string(consumedMessage.Value)
		if consumedValue != ts {
			log.Infof("consumed (%s) is not equal to what we produced (%s)", consumedValue, ts)
			w.WriteHeader(http.StatusOK)
			return
		}

		log.Infof("consumed same msg (%s) as we produced.", ts)
		w.WriteHeader(http.StatusOK)
	}
}
