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

// seedDb seeds the database with mock data
// Before it seeds the database, it checks if the database has already been seeded, by looking for
// the 'seed' value in the 'confiog' kv bucket. If the key is found, it means the database has already been seeded,
// and it returns.
func seedDb(ctx context.Context, js jetstream.JetStream, ddb *dynamodb.Client, table string) error {

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

// getBalance gets the balance for a client by querying the database
func getBalance(ctx context.Context, ddb *dynamodb.Client, table string, clientID string) (int, error) {

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
	if result.Item == nil {
		log.Printf("get bal for client: %s , client not found, returning 0 balance", clientID)
		return 0, nil
	}
	bal := BalResponse{}
	err = attributevalue.UnmarshalMap(result.Item, &bal)
	if err != nil {
		return 0, err
	}

	log.Printf("get bal for client: %s, balance %d", clientID, bal.ClientBalance)
	return bal.ClientBalance, nil
}
