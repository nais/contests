package main

import (
	"fmt"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"github.com/nais/contests/internal/bucket"
	"github.com/nais/contests/internal/database"
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
	dbUser               string
	dbPassword           string
	dbHost               string
)

func init() {
	flag.StringVar(&bindAddr, "bind-address", ":8080", "ip:port where http requests are served")
	flag.StringVar(&bucketName, "bucket-name", os.Getenv("BUCKET_NAME"), "name of bucket")
	flag.StringVar(&kafkaBrokers, "kafka-brokers", os.Getenv("KAFKA_BROKERS"), "kafka brokers")
	flag.StringVar(&kafkaCertificatePath, "kafka-certificate-path", os.Getenv("KAFKA_CERTIFICATE_PATH"), "kafka certificate path")
	flag.StringVar(&kafkaPrivateKeyPath, "kafka-private-key-path", os.Getenv("KAFKA_PRIVATE_KEY_PATH"), "kafka private key path")
	flag.StringVar(&kafkaCAPath, "kafka-ca-path", os.Getenv("KAFKA_CA_PATH"), "kafka ca path")
	flag.StringVar(&dbUser, "db-username", os.Getenv("NAIS_DATABASE_CONTESTS_CONTESTS_USERNAME"), "database username")
	flag.StringVar(&dbPassword, "db-password", os.Getenv("NAIS_DATABASE_CONTESTS_CONTESTS_PASSWORD"), "database password")
	flag.StringVar(&dbHost, "db-host", os.Getenv("NAIS_DATABASE_CONTESTS_CONTESTS_HOST"), "database host")
	flag.Parse()
}

func main() {
	if bucketName != "" {
		log.Infof("Detected bucket configuration, setting up handler for %s", bucketName)
		http.HandleFunc("/bucket", bucket.Handler(bucketName))
	} else {
		log.Infof("No bucket configuration detected, skipping handler")
	}

	if kafkaBrokers != "" && kafkaCertificatePath != "" && kafkaPrivateKeyPath != "" && kafkaCAPath != "" {
		log.Info("Detected Kafka configuration, setting up handler")
		k, err := kafka.New(kafkaBrokers, kafkaCAPath, kafkaCertificatePath, kafkaPrivateKeyPath)
		if err != nil {
			log.Errorf("Initializing Kafka: %s", err)
		}
		http.HandleFunc("/kafka", k.Handler())
	} else {
		log.Infof("No kafka configuration detected, skipping handler")
	}

	if dbUser != "" && dbPassword != "" && dbHost != "" {
		log.Info("Detected database configuration, setting up handler")
		http.HandleFunc("/database", database.Handler(dbUser, dbPassword, dbHost))
	} else {
		log.Infof("No database configuration detected, skipping handler")
	}

	http.HandleFunc("/ping", func(r http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(r, "pong\n")
		r.WriteHeader(http.StatusOK)
	})

	log.Infof("running @ %s", bindAddr)

	if err := http.ListenAndServe(bindAddr, nil); err != nil {
		log.Fatal(err)
	}
}
