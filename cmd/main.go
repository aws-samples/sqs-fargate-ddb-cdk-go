package main

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/aws/aws-sdk-go-v2/service/sqs"
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

func main() {
	log.Println("Service is started")
	ctx, cancel := context.WithCancel(context.Background())

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	queueUrl := os.Getenv("SQS_URL")
	log.Printf("QUEUE_URL: %s", queueUrl)

	tableName := os.Getenv("DDB_TABLE")
	log.Printf("DDB_TABLE: %s", tableName)

	// Create S3 service client
	sqsSvc := sqs.NewFromConfig(cfg)

	// Create DDB service client
	ddbSvc := dynamodb.NewFromConfig(cfg)

	defer func() {
		signal.Stop(signalChan)
		cancel()
	}()

loop:
	for {
		select {
		case <-signalChan: //if get SIGTERM
			log.Println("Got SIGTERM signal, cancelling the context")
			cancel() //cancel context

		default:
			_, err := processSQS(ctx, sqsSvc, queueUrl, ddbSvc, tableName)

			if err != nil {
				if errors.Is(err, context.Canceled) {
					log.Printf("stop processing, context is cancelled %v", err)
					break loop
				}

				log.Fatalf("error processing SQS %v", err)
			}
		}
	}
	log.Println("service is safely stopped")

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
