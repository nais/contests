package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"github.com/nais/contests/internal/bucket"
	"github.com/nais/contests/internal/kafka"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

var (
	bindAddr             string
	bucketName           string
	dbUser               string
	dbPassword           string
	dbHost               string
	dbName               string
	kafkaBrokers         string
	kafkaCertificatePath string
	kafkaPrivateKeyPath  string
	kafkaCAPath          string
)

var (
	dbAppName         = strings.ToUpper(strings.Replace(getEnv("NAIS_APP_NAME", "contests"), "-", "_", -1))
	defaultDbPassword = os.Getenv(fmt.Sprintf("NAIS_DATABASE_%[1]s_%[1]s_PASSWORD", dbAppName))
	defaultDbUsername = os.Getenv(fmt.Sprintf("NAIS_DATABASE_%[1]s_%[1]s_USERNAME", dbAppName))
	defaultDbName     = os.Getenv(fmt.Sprintf("NAIS_DATABASE_%[1]s_%[1]s_DATABASE", dbAppName))
)

func init() {
	flag.StringVar(&bindAddr, "bind-address", ":8080", "ip:port where http requests are served")
	flag.StringVar(&bucketName, "bucket-name", os.Getenv("BUCKET_NAME"), "name of bucket")
	flag.StringVar(&dbName, "db-name", defaultDbName, "database name")
	flag.StringVar(&dbUser, "db-user", defaultDbUsername, "database username")
	flag.StringVar(&dbPassword, "db-password", defaultDbPassword, "database password")
	flag.StringVar(&dbHost, "db-hostname", "localhost", "database hostname")
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

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}
