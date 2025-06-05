package main

import (
	"context"
	_ "crypto/tls"
	"fmt"
	"net/http"
	"os"

	"github.com/nais/contests/internal/opensearch"
	"github.com/nais/contests/internal/valkey"

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
	dbUrl                string
	bigQueryDatasetName  string
	bigQueryProjectID    string
	opensearchUri        string
	opensearchUser       string
	opensearchPassword   string
	valkeyUri            string
	valkeyUser           string
	valkeyPassword       string
	azureAppClientID     string
	postgresUrl          string
)

func init() {
	flag.StringVar(&bindAddr, "bind-address", ":8080", "ip:port where http requests are served")
	flag.StringVar(&bucketName, "bucket-name", os.Getenv("BUCKET_NAME"), "name of bucket")
	flag.StringVar(&kafkaBrokers, "kafka-brokers", os.Getenv("KAFKA_BROKERS"), "kafka brokers")
	flag.StringVar(&kafkaCAPath, "kafka-ca-path", os.Getenv("KAFKA_CA_PATH"), "kafka ca path")
	flag.StringVar(&kafkaCertificatePath, "kafka-certificate-path", os.Getenv("KAFKA_CERTIFICATE_PATH"), "kafka certificate path")
	flag.StringVar(&kafkaPrivateKeyPath, "kafka-private-key-path", os.Getenv("KAFKA_PRIVATE_KEY_PATH"), "kafka private key path")
	flag.StringVar(&kafkaTopic, "contests", os.Getenv("KAFKA_TOPIC"), "kafka topic")
	flag.StringVar(&dbUrl, "db-url", os.Getenv("NAIS_DATABASE_CONTESTS_CONTESTS_URL"), "database url")
	flag.StringVar(&bigQueryDatasetName, "bigquery-dataset-name", os.Getenv("BIGQUERY_DATASET_NAME"), "name of bigquery dataset")
	flag.StringVar(&bigQueryProjectID, "bigquery-project-id", os.Getenv("GCP_TEAM_PROJECT_ID"), "project id of bigquery dataset")
	flag.StringVar(&opensearchUri, "opensearch-uri", os.Getenv("OPEN_SEARCH_URI"), "opensearch uri")
	flag.StringVar(&opensearchUser, "opensearch-username", os.Getenv("OPEN_SEARCH_USERNAME"), "opensearch username")
	flag.StringVar(&opensearchPassword, "opensearch-password", os.Getenv("OPEN_SEARCH_PASSWORD"), "opensearch password")
	// Since we're still using the old redis client, we can't use VALKEY_URI_SESSIONS, as that uses `valkeys` as scheme, which the redis client won't accept
	flag.StringVar(&valkeyUri, "valkey-uri", os.Getenv("REDIS_URI_SESSIONS"), "valkey uri")
	flag.StringVar(&valkeyUser, "valkey-username", os.Getenv("VALKEY_USERNAME_SESSIONS"), "valkey username")
	flag.StringVar(&valkeyPassword, "valkey-password", os.Getenv("VALKEY_PASSWORD_SESSIONS"), "valkey password")
	flag.StringVar(&azureAppClientID, "azure-app-client-id", os.Getenv("AZURE_APP_CLIENT_ID"), "azure app client id")
	flag.StringVar(&postgresUrl, "postgres-url", os.Getenv("PGURL"), "postgres url")
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

	if dbUrl != "" {
		log.Info("Detected database configuration for sql instance, setting up handler")
		http.HandleFunc("/database", database.Handler(dbUrl))
	} else {
		log.Infof("No database configuration detected, skipping handler")
	}

	if postgresUrl != "" {
		log.Info("Detected database configuration for postgres operator, setting up handler")
		http.HandleFunc("/database", database.Handler(postgresUrl))
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

	if valkeyUri != "" && valkeyUser != "" && valkeyPassword != "" {
		valkeyOpts, err := redgo.ParseURL(valkeyUri)
		if err != nil {
			log.Errorf("Detected valkey configuration, but failed to parse URI: %v", err)
		}
		valkeyOpts.Username = valkeyUser
		valkeyOpts.Password = valkeyPassword

		client := redgo.NewClient(valkeyOpts)
		log.Info("Detected valkey configuration, setting up handler")
		http.HandleFunc("/valkey", valkey.Handler(ctx, client))
	} else {
		log.Info("No valkey configuration detected, skipping handler")
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
