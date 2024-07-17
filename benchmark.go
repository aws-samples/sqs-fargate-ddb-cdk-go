package main

import (
	"encoding/json"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type Request struct {
	ClientID string `json:"client_id"`
}

func main() {
	// Connect to NATS server
	credsFile := "cdk/config/NGS-poc-service.creds"
	natsURl := "tls://connect.ngs.global"
	nc, err := nats.Connect(natsURl, nats.UserCredentials(credsFile))
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	var wg sync.WaitGroup

	log.Println("Sending requests to the service...")
	start := time.Now()
	apiCalls := 1000
	requestTimeout := 5 * time.Second

	// Create a buffered channel to limit concurrent goroutines
	maxConcurrency := 1000
	semaphore := make(chan struct{}, maxConcurrency)

	for id := 1; id <= apiCalls; id++ {
		wg.Add(1)
		semaphore <- struct{}{} // Block if there are already 20 goroutines running

		go func(clientID int) {

			// Ensure client id must be between 1 & 1000
			clientID = clientID % 1000
			if clientID == 0 {
				clientID = 1000
			}
			defer wg.Done()
			defer func() { <-semaphore }() // Release a spot in the semaphore

			// Create the request payload
			req := Request{ClientID: strconv.Itoa(clientID)}
			reqData, err := json.Marshal(req)
			if err != nil {
				log.Fatalf("Error marshalling request: %v", err)
				return
			}

			// Send request and wait for reply
			msg, err := nc.Request("customer.balance", reqData, requestTimeout)
			if err != nil {
				log.Fatalf("Error sending request for client %d: %v", clientID, err)
				return
			}

			// Check the reply is valid JSON
			balResp := map[string]interface{}{}
			if err = json.Unmarshal(msg.Data, &balResp); err != nil {
				log.Fatalf("Error unmarshalling response for client %d: %v", clientID, err)
				return
			}
			// log.Printf("Reply for client %d: %+v\n", clientID, balResp)
		}(id)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	dur := time.Since(start)
	averageMillis := float64(dur.Milliseconds()) / float64(apiCalls)
	log.Printf("Total of %d requests, with max concurrency %d, and request timeout %s, completed in %v. Average call took %.2f milliseconds", apiCalls, maxConcurrency, requestTimeout, dur, averageMillis)
}
