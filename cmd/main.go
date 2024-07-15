package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/nats-io/nats.go"
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
	m := nats.Msg{
		Subject: "test",
		Data:    []byte("hello from fargate"),
	}
	nc.PublishMsg(&m)

	// loop:
	// 	for {
	// 		select {
	// 		case <-signalChan: //if get SIGTERM
	// 			log.Println("Got SIGTERM signal, cancelling the context")
	// 			cancel() //cancel context

	// 		default:
	// 			_, err := processSQS(ctx, sqsSvc, natsSecret, ddbSvc, tableName)

	// 			if err != nil {
	// 				if errors.Is(err, context.Canceled) {
	// 					log.Printf("stop processing, context is cancelled %v", err)
	// 					break loop
	// 				}

	//				log.Fatalf("error processing SQS %v", err)
	//			}
	//		}
	//	}
	//
	// log.Println("service is safely stopped")
	<-ctx.Done()

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

func processSQS(ctx context.Context, sqsSvc *sqs.Client, queueUrl string, ddbSvc *dynamodb.Client, tableName string) (bool, error) {
	input := &sqs.ReceiveMessageInput{
		QueueUrl:            &queueUrl,
		MaxNumberOfMessages: 1,
		VisibilityTimeout:   visibilityTimeout,
		WaitTimeSeconds:     waitingTimeout, // use long polling
	}

	resp, err := sqsSvc.ReceiveMessage(ctx, input)

	if err != nil {
		return false, fmt.Errorf("error receiving message %w", err)
	}

	log.Printf("received messages: %v", len(resp.Messages))
	if len(resp.Messages) == 0 {
		return false, nil
	}

	for _, msg := range resp.Messages {
		var newMsg MsgType
		id := *msg.MessageId

		err := json.Unmarshal([]byte(*msg.Body), &newMsg)
		if err != nil {
			return false, fmt.Errorf("error unmarshalling %w", err)
		}

		log.Printf("message id %s is received from SQS: %#v", id, newMsg.Message)

		err = putToDDB(ctx, ddbSvc, tableName, id, newMsg.Message)
		if err != nil {
			return false, fmt.Errorf("error putting message to DDB %w", err)
		}
		log.Printf("message id %s is saved in DDB", id)

		_, err = sqsSvc.DeleteMessage(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      &queueUrl,
			ReceiptHandle: msg.ReceiptHandle,
		})

		if err != nil {
			return false, fmt.Errorf("error deleting message from SQS %w", err)
		}
		log.Printf("message id %s is deleted from queue", id)

	}
	return true, nil

}

func GetUTCTimestampNow() string {
	t := time.Now().UTC()
	return t.Format("2006-01-02T15:04:05.000Z")
}

func putToDDB(ctx context.Context, ddbSvc *dynamodb.Client, tableName string, msgId string, message string) error {
	inputMap := make(map[string]types.AttributeValue)

	utcTimeISO := GetUTCTimestampNow()

	inputMap["id"] = &types.AttributeValueMemberS{Value: msgId}
	inputMap["timestamp_utc"] = &types.AttributeValueMemberS{Value: utcTimeISO}
	inputMap["message"] = &types.AttributeValueMemberS{Value: message}

	input := &dynamodb.PutItemInput{
		Item:      inputMap,
		TableName: aws.String(tableName),
	}

	_, err := ddbSvc.PutItem(ctx, input)
	if err != nil {
		return err
	}
	return nil
}
