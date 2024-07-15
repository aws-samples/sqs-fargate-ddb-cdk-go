package main

import (
	"context"
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

	secretKey := os.Getenv("NATS_CREDENTIALS")
	log.Printf("NATS_CREDENTIALS key: %s", secretKey)
	secretValue, err := getSecret(secretKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

	}
	log.Printf("NATS_CREDENTIALS value: %v", secretValue)

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
	url := "tls://connect.ngs.global"
	creds := `-----BEGIN NATS USER JWT-----
eyJ0eXAiOiJKV1QiLCJhbGciOiJlZDI1NTE5LW5rZXkifQ.eyJqdGkiOiJJNEJWTVVGT1hPMjVEN0szWEcyWVpNNlFSSUxTSFdHTklOMklMWlozWEtSWEZPVjRBUUFBIiwiaWF0IjoxNzIwNzMzMTc0LCJpc3MiOiJBRFc2UU9ZU0ozREFRNUJLTUU3NlVEV0RUNVZFMk1EVk5VQlBYRVpKN1VSR0hTSkY3WFgzSVlMRiIsIm5hbWUiOiJzZXJ2aWNlIiwic3ViIjoiVUNLMlRZSENLS0xDRk1TNktIT1dJTUZZV1dYRkUyMjZYQUJDVVJIU0lSN0dINFlBSjVUSUtVQkYiLCJuYXRzIjp7InB1YiI6e30sInN1YiI6e30sInN1YnMiOi0xLCJkYXRhIjotMSwicGF5bG9hZCI6LTEsImlzc3Vlcl9hY2NvdW50IjoiQUNUM0ZPWFVCNjMzQkpRRlBQRkk1U1laUzVKM09aMlRDWjVMQVlETFlMWEdIUFgyTkJFQlRQREEiLCJ0eXBlIjoidXNlciIsInZlcnNpb24iOjJ9fQ.nDbN0Q2spRDnjitnr--3wewXFRsS2sbjNOfDh5HH77WpcyyJiruKCkV3jYwN4HgNHAM_3llsSIG18aIL6IDLBQ
------END NATS USER JWT------

************************* IMPORTANT *************************
NKEY Seed printed below can be used to sign and prove identity.
NKEYs are sensitive and should be treated as secrets.

-----BEGIN USER NKEY SEED-----
SUAAJUF34ILUVMRMXVPBDJYVYKOD7O4LXQYOKM45KPHWVRV4LDFGWPWMAI
------END USER NKEY SEED------

*************************************************************`

	// Write the credentials to a file
	credsFile := "/tmp/nats.creds"
	err := os.WriteFile(credsFile, []byte(creds), 0600)
	if err != nil {
		return nil, err
	}

	return nats.Connect(url, nats.UserCredentials(credsFile))
}
