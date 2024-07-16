package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

var cfg aws.Config

const (
	visibilityTimeout = 60 * 10
	waitingTimeout    = 20
)

type MsgType struct {
	Message string `json:"message"`
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	var err error

	awsProfile := os.Getenv("AWS_PROFILE")
	log.Printf("AWS_PROFILE: %s", awsProfile)

	if awsProfile != "" {
		log.Printf("Use AWS profile %s", awsProfile)
		cfg, err = config.LoadDefaultConfig(context.Background(),
			config.WithSharedConfigProfile(awsProfile),
		)
		if err != nil {
			log.Fatalf("error loading config %v", err)
		}

	} else {
		log.Println("Use container role")
		cfg, err = config.LoadDefaultConfig(context.Background())
		if err != nil {
			log.Fatalf("error loading config %v", err)
		}
	}
}

func getSecret(secretName string) (*secretsmanager.GetSecretValueOutput, error) {

	// Create a Secrets Manager client
	svc := secretsmanager.NewFromConfig(cfg)

	// Retrieve the secret value
	result, err := svc.GetSecretValue(context.TODO(), &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve secret %q: %v", secretName, err)
	}

	return result, nil
}

func main() {
	log.Println("Service is started")
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	tableName := os.Getenv("DDB_TABLE")
	log.Printf("DDB_TABLE: %s", tableName)

	// Create DDB service client
	ddbSvc := dynamodb.NewFromConfig(cfg)
	log.Println("ddbSvc: %+v", ddbSvc)

	nc, err := NatsConnect(ctx)
	if err != nil {
		log.Fatalf("error connecting to NATS %v", err)
	}

	// Create a JetStream context, which is needed for KV functionality
	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("error connecting to Jetstream %v", err)
	}

	if err = SeedDb(ctx, js, ddbSvc, tableName); err != nil {
		log.Fatalf("error seeding database %v", err)
	}

	if err = startService(ctx, nc, ddbSvc, tableName); err != nil {
		log.Fatalf("error starting service %v", err)
	}

	<-ctx.Done()
	log.Println("Service is stopped")
}

func NatsConnect(ctx context.Context) (*nats.Conn, error) {
	err := captureCredsToFile()
	if err != nil {
		return nil, err
	}
	url := "tls://connect.ngs.global"
	credsFile := "/tmp/nats.creds"
	return nats.Connect(url, nats.UserCredentials(credsFile))
}

func captureCredsToFile() error {
	base64encodedCreds := os.Getenv("NATS_CREDENTIALS")
	// Decode the base64 string
	decodedBytes, err := base64.StdEncoding.DecodeString(base64encodedCreds)
	if err != nil {
		log.Println("Error decoding base64 string:", err)
		return err
	}
	// Write the decoded bytes to a file
	// Create a new file
	file, err := os.Create("/tmp/nats.creds")
	if err != nil {
		log.Println("Error creating file:", err)
		return err
	}
	defer file.Close()

	// Write the decoded bytes to the file
	_, err = file.Write(decodedBytes)
	if err != nil {
		log.Println("Error writing to file:", err)
		return err
	}

	log.Println("Decoded bytes written to file successfully")
	return nil
}
