package main

import (
	"context"
	"log"
	"math/rand"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/nats-io/nats.go/jetstream"
)

var seedKey = "seed"
var kvBucket = "config"

type Client struct {
	ID            string `dynamodbav:"id"`
	ClientBalance int    `dynamodbav:"client_balance"`
}

func SeedDb(ctx context.Context, js jetstream.JetStream, ddb *dynamodb.Client, table string) error {

	log.Printf("seed db, getting bucket %s ...", kvBucket)
	bucket, err := js.KeyValue(ctx, kvBucket)
	if err != nil {
		log.Printf("error getting kv bucket: %v", err)
		return err
	}
	_, err = bucket.Get(ctx, seedKey)
	if err != nil {
		if err == jetstream.ErrKeyNotFound {
			log.Printf("need to seed db ...")
		} else {
			log.Printf("error getting seed key: %v", err)
			return err
		}
	} else {
		log.Printf("db already seeded, finished.")
		return nil
	}

	log.Printf("seeding the db ...")

	// Mock data
	var rows = 1000
	for i := 1; i <= rows; i++ {
		client := Client{
			ID:            strconv.Itoa(i),
			ClientBalance: rand.Intn(9900) + 100, // generates a number in [100, 10000)
		}
		av, err := attributevalue.MarshalMap(client)
		if err != nil {
			return err
		}

		// Create item in table
		_, err = ddb.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(table), Item: av})
		if err != nil {
			return err
		}
	}

	_, err = bucket.Create(ctx, seedKey, []byte("done"))
	if err != nil {
		log.Printf("error creating seed key: %v", err)
		return err
	}

	log.Printf("successfully seeded database with %d rows", rows)
	return nil
}

type BalRequest struct {
	ID string `dynamodbav:"id"`
}

type BalResponse struct {
	ClientBalance int `dynamodbav:"client_balance"`
}

func GetBalance(ctx context.Context, ddb *dynamodb.Client, table string, clientID string) (int, error) {

	log.Println("get bal for client: ", clientID)
	req := BalRequest{ID: clientID}
	av, err := attributevalue.MarshalMap(req)
	if err != nil {
		return 0, err
	}

	// Get the item from the table
	result, err := ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:       aws.String(table),
		Key:             av,
		AttributesToGet: []string{"client_balance"},
	})
	if err != nil {
		return 0, err
	}
	log.Println("response: %+v", result)
	if result.Item == nil {
		log.Println("could not find item")
		return 0, nil
	}
	bal := BalResponse{}
	err = attributevalue.UnmarshalMap(result.Item, &bal)
	if err != nil {
		return 0, err
	}

	return bal.ClientBalance, nil
}
