package main

import (
	"net/http"
	"os"

	"github.com/nais/contests/internal/bucket"
	"github.com/nais/contests/internal/kafka"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

var (
	bindAddr             string
	bucketName           string
	kafkaBrokers         string
	kafkaCertificatePath string
	kafkaPrivateKeyPath  string
	kafkaCAPath          string
)

func init() {
	flag.StringVar(&bindAddr, "bind-address", ":8080", "ip:port where http requests are served")
	flag.StringVar(&bucketName, "bucket-name", os.Getenv("BUCKET_NAME"), "name of bucket")
	flag.StringVar(&kafkaBrokers, "kafka-brokers", os.Getenv("KAFKA_BROKERS"), "kafka brokers")
	flag.StringVar(&kafkaCertificatePath, "kafka-certificate-path", os.Getenv("KAFKA_CERTIFICATE_PATH"), "kafka certificate path")
	flag.StringVar(&kafkaPrivateKeyPath, "kafka-private-key-path", os.Getenv("KAFKA_PRIVATE_KEY_PATH"), "kafka private key path")
	flag.StringVar(&kafkaCAPath, "kafka-ca-path", os.Getenv("KAFKA_CA_PATH"), "kafka ca path")
	flag.Parse()
}

func main() {
	if bucketName != "" {
		log.Infof("Detected bucket configuration, setting up handler for %s", bucketName)
		http.HandleFunc("/bucket", bucket.Handler(bucketName))
	} else {
		log.Infof("No bucket configuration detected (env BUCKET_NAME, or --bucket-name), skipping handler")
	}

	if kafkaBrokers != "" && kafkaCertificatePath != "" && kafkaPrivateKeyPath != "" && kafkaCAPath != "" {
		log.Info("Detected Kafka configuration, setting up handler")
		k, err := kafka.New(kafkaBrokers, kafkaCAPath, kafkaCertificatePath, kafkaPrivateKeyPath)
		if err != nil {
			log.Errorf("Initializing Kafka: %s", err)
		}
		http.HandleFunc("/kafka", k.Handler())
	} else {
		log.Infof("No kafka configuration detected skipping handler")
	}

	log.Infof("running @ %s", bindAddr)

	if err := http.ListenAndServe(bindAddr, nil); err != nil {
		log.Fatal(err)
	}
}
