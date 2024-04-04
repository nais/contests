package main

import (
	"context"
	_ "crypto/tls"
	"fmt"
	"net/http"
	"os"

	"github.com/nais/contests/internal/opensearch"
	"github.com/nais/contests/internal/redis"

	bq "cloud.google.com/go/bigquery"
	_ "github.com/lib/pq"
	"github.com/nais/contests/internal/bigquery"
	"github.com/nais/contests/internal/bucket"
	"github.com/nais/contests/internal/database"
	"github.com/nais/contests/internal/kafka"
	osgo "github.com/opensearch-project/opensearch-go"
	redgo "github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

var (
	bindAddr             string
	bucketName           string
	kafkaBrokers         string
	kafkaCAPath          string
	kafkaCertificatePath string
	kafkaPrivateKeyPath  string
	kafkaTopic           string
	dbUser               string
	dbPassword           string
	dbHost               string
	bigQueryDatasetName  string
	bigQueryProjectID    string
	opensearchUri        string
	opensearchUser       string
	opensearchPassword   string
	redisUri             string
	redisUser            string
	redisPassword        string
	azureAppClientID     string
)

func init() {
	flag.StringVar(&bindAddr, "bind-address", ":8080", "ip:port where http requests are served")
	flag.StringVar(&bucketName, "bucket-name", os.Getenv("BUCKET_NAME"), "name of bucket")
	flag.StringVar(&kafkaBrokers, "kafka-brokers", os.Getenv("KAFKA_BROKERS"), "kafka brokers")
	flag.StringVar(&kafkaCAPath, "kafka-ca-path", os.Getenv("KAFKA_CA_PATH"), "kafka ca path")
	flag.StringVar(&kafkaCertificatePath, "kafka-certificate-path", os.Getenv("KAFKA_CERTIFICATE_PATH"), "kafka certificate path")
	flag.StringVar(&kafkaPrivateKeyPath, "kafka-private-key-path", os.Getenv("KAFKA_PRIVATE_KEY_PATH"), "kafka private key path")
	flag.StringVar(&kafkaTopic, "contests", os.Getenv("KAFKA_TOPIC"), "kafka topic")
	flag.StringVar(&dbUser, "db-username", os.Getenv("NAIS_DATABASE_CONTESTS_CONTESTS_USERNAME"), "database username")
	flag.StringVar(&dbPassword, "db-password", os.Getenv("NAIS_DATABASE_CONTESTS_CONTESTS_PASSWORD"), "database password")
	flag.StringVar(&dbHost, "db-host", os.Getenv("NAIS_DATABASE_CONTESTS_CONTESTS_HOST"), "database host")
	flag.StringVar(&bigQueryDatasetName, "bigquery-dataset-name", os.Getenv("BIGQUERY_DATASET_NAME"), "name of bigquery dataset")
	flag.StringVar(&bigQueryProjectID, "bigquery-project-id", os.Getenv("GCP_TEAM_PROJECT_ID"), "project id of bigquery dataset")
	flag.StringVar(&opensearchUri, "opensearch-uri", os.Getenv("OPEN_SEARCH_URI"), "opensearch uri")
	flag.StringVar(&opensearchUser, "opensearch-username", os.Getenv("OPEN_SEARCH_USERNAME"), "opensearch username")
	flag.StringVar(&opensearchPassword, "opensearch-password", os.Getenv("OPEN_SEARCH_PASSWORD"), "opensearch password")
	flag.StringVar(&redisUri, "redis-uri", os.Getenv("REDIS_URI_SESSIONS"), "redis uri")
	flag.StringVar(&redisUser, "redis-username", os.Getenv("REDIS_USERNAME_SESSIONS"), "redis username")
	flag.StringVar(&redisPassword, "redis-password", os.Getenv("REDIS_PASSWORD_SESSIONS"), "redis password")
	flag.StringVar(&azureAppClientID, "azure-app-client-id", os.Getenv("AZURE_APP_CLIENT_ID"), "azure app client id")
	flag.Parse()
}

func main() {
	ctx := context.Background()
	if bucketName != "" {
		log.Infof("Detected bucket configuration, setting up handler for %s", bucketName)
		http.HandleFunc("/bucket", bucket.Handler(bucketName))
	} else {
		log.Infof("No bucket configuration detected, skipping handler")
	}

	if kafkaBrokers != "" && kafkaCertificatePath != "" && kafkaPrivateKeyPath != "" && kafkaCAPath != "" {
		log.Info("Detected Kafka configuration, setting up handler")
		k, err := kafka.New(kafkaBrokers, kafkaCAPath, kafkaCertificatePath, kafkaPrivateKeyPath, kafkaTopic)
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

	if bigQueryDatasetName != "" && bigQueryProjectID != "" {
		bqClient, err := bq.NewClient(ctx, bigQueryProjectID)
		if err != nil {
			log.Errorf("Detected BigQuery configuration, but failed to set up client: %v", err)
		} else {
			log.Infof("Detected BigQuery configuration, setting up handler for %v in project %v", bigQueryDatasetName, bigQueryProjectID)
			dataset := bqClient.Dataset(bigQueryDatasetName)
			http.HandleFunc("/bigquery", bigquery.Handler(ctx, dataset))
		}
	} else {
		log.Info("No BigQuery configuration detected, skipping handler")
	}

	if opensearchUri != "" && opensearchUser != "" && opensearchPassword != "" {
		client, err := osgo.NewClient(osgo.Config{
			Addresses: []string{opensearchUri},
			Username:  opensearchUser,
			Password:  opensearchPassword,
		})
		if err != nil {
			log.Errorf("Detected opensearch configuration, but failed to set up client: %v", err)
		} else {
			log.Info("Detected opensearch configuration, setting up handler")
			http.HandleFunc("/opensearch", opensearch.Handler(ctx, client))
		}
	} else {
		log.Info("No opensearch configuration detected, skipping handler")
	}

	if redisUri != "" && redisUser != "" && redisPassword != "" {
		redisOpts, err := redgo.ParseURL(redisUri)
		if err != nil {
			log.Errorf("Detected redis configuration, but failed to parse URI: %v", err)
		}
		redisOpts.Username = redisUser
		redisOpts.Password = redisPassword

		client := redgo.NewClient(redisOpts)
		log.Info("Detected redis configuration, setting up handler")
		http.HandleFunc("/redis", redis.Handler(ctx, client))
	} else {
		log.Info("No redis configuration detected, skipping handler")
	}

	if azureAppClientID != "" {
		log.Info("Detected Azure app configuration, setting up handler")
		http.HandleFunc("/azure", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintf(w, "Azure app client id: %s", azureAppClientID)
			log.Info("Successfully returned Azure app client id")
		})
	}

	http.HandleFunc("/ping", func(r http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(r, "pong\n")
	})

	log.Infof("running @ %s", bindAddr)

	if err := http.ListenAndServe(bindAddr, nil); err != nil {
		log.Fatal(err)
	}
}
