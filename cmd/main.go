package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	uuid "github.com/satori/go.uuid"
)

var cfg aws.Config
var ctx = context.Background()

const (
	visibilityTimeout = 60 * 10
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
		log.Println("Use AWS profile")
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithSharedConfigProfile(awsProfile),
		)
		if err != nil {
			log.Fatalf("Error loading profile %v", err)
		}

	} else {
		log.Println("Use container role")
		cfg, err = config.LoadDefaultConfig(ctx)
		if err != nil {
			log.Fatalf("Error loading profile %v", err)
		}
	}
}

func main() {
	log.Println("Service started")

	queueUrl := os.Getenv("SQS_URL")

	log.Printf("QUEUE_URL: %s", queueUrl)

	tableName := os.Getenv("DDB_TABLE")

	log.Printf("DDB_TABLE: %s", tableName)

	// Create S3 service client
	sqsSvc := sqs.NewFromConfig(cfg)

	// Create DDB service client
	ddbSvc := dynamodb.NewFromConfig(cfg)

	for {
		log.Printf("waiting for new message...")
		_, err := processSQS(sqsSvc, queueUrl, ddbSvc, tableName)
		if err != nil {
			log.Fatalf("Error processing message: %#v", err)
		}

	}
}

func processSQS(sqsSvc *sqs.Client, queueUrl string, ddbSvc *dynamodb.Client, tableName string) (bool, error) {
	input := &sqs.ReceiveMessageInput{
		QueueUrl:            &queueUrl,
		MaxNumberOfMessages: 1,
		VisibilityTimeout:   visibilityTimeout,
		WaitTimeSeconds:     20, //Long polling 20 sec
	}

	resp, err := sqsSvc.ReceiveMessage(ctx, input)
	if err != nil {
		log.Fatalf("Error receiving message %v", err)
	}

	log.Printf("received messages: %v", len(resp.Messages))
	if len(resp.Messages) == 0 {
		return false, nil
	}

	for _, msg := range resp.Messages {
		var newMsg MsgType

		err := json.Unmarshal([]byte(*msg.Body), &newMsg)
		if err != nil {
			return false, err
		}

		log.Printf("message received: %#v", newMsg.Message)

		err = putToDDB(ddbSvc, tableName, newMsg.Message)
		if err != nil {
			return false, err
		}
		log.Printf("message is saved in DDB")

		_, err = sqsSvc.DeleteMessage(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      &queueUrl,
			ReceiptHandle: msg.ReceiptHandle,
		})
		if err != nil {
			return false, err
		}
		log.Printf("message is deleted from queue")

	}
	return true, nil

}

func GetUTCTimestampNow() string {
	t := time.Now().UTC()
	return t.Format("2006-01-02T15:04:05.000Z")

}

func GetUUID() string {
	id := uuid.NewV4()
	return id.String()
}

func putToDDB(ddbSvc *dynamodb.Client, tableName string, message string) error {
	inputMap := make(map[string]types.AttributeValue)

	msgId := GetUUID()
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
