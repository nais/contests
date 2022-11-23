package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"strings"

	"github.com/Shopify/sarama"
	log "github.com/sirupsen/logrus"
)

type Kafka struct {
	name    string
	config  *sarama.Config
	brokers []*sarama.Broker
}

func New(brokersString, caPath, certPath, keyPath string) (*Kafka, error) {
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

	brokers := make([]*sarama.Broker, 0)
	for _, b := range strings.Split(brokersString, ",") {
		brokers = append(brokers, sarama.NewBroker(b))
	}

	return &Kafka{
		config:  config,
		brokers: brokers,
	}, nil
}

func (k *Kafka) Handler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, b := range k.brokers {
			if err := b.Open(k.config); err != nil {
				log.Errorf("opening connection to broker: %s: %s", b.Addr(), err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			connected, err := b.Connected()
			if err != nil || !connected {
				log.Errorf("verifying connection to broker: %s: %w", b.Addr(), err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if err := b.Close(); err != nil {
				log.Errorf("could not close connection: %w", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
		log.Infof("Successfully connected to all Kafka brokers")
		w.WriteHeader(http.StatusOK)
	}
}
