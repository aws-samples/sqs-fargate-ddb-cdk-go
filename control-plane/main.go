package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"

	"github.com/synadia-io/control-plane-sdk-go/syncp"
)

func main() {
	opt := ParseOptions()
	client := syncp.NewAPIClient(syncp.NewConfiguration())
	ctx := context.WithValue(context.Background(), syncp.ContextServerVariables, map[string]string{
		"baseUrl": opt.BaseURL,
	})
	ctx = context.WithValue(ctx, syncp.ContextAccessToken, personalAccessToken())

	switch opt.Action {
	case "list":
		list(ctx, client, opt.SystemID)
	case "create":
		create(ctx, client, opt.Initials, opt.SystemID)
	case "delete":
		delete(ctx, client, opt.Initials)
	}
}

func list(ctx context.Context, client *syncp.APIClient, systemID string) {
	log.Printf("Listing accounts in system %s\n", systemID)
	accountList, _, err := client.SystemAPI.ListAccounts(ctx, systemID).Execute()
	if err != nil {
		handleApiError(err)
	}
	var accountNames []string
	for _, account := range accountList.Items {
		accountNames = append(accountNames, account.Name)
		log.Println("")
		log.Printf("---- %s ----", account.Name)
		log.Printf("id             : %s", account.Id)
		log.Printf("name           : %s", account.Name)
		log.Printf("isSystemAccount: %t", account.IsSystemAccount)
	}
}

func create(ctx context.Context, client *syncp.APIClient, initials, systemID string) {

	name := "POC-" + initials
	defaultPayload := int64(1048576)
	defaultSubs := int64(50)
	defaultData := int64(-1)
	defaultConn := int64(5)
	// defaultLeaf := int64(1)
	defaultExports := int64(1)
	defaultImports := int64(5)
	defaultWildcards := true
	// defaultConsumer := int64(10)
	// defaultDiskMaxStreamBytes := int64(1048576)
	// defaultDiskStorage := int64(2684354560)
	// defaultMaxBytesRequired := true
	// defaultSubscriptions := int64(10)
	// defaultData := int64(-1)
	defaultDescription := "default account"

	log.Printf("Creating account %s, for systemID: %s\n", name, systemID)
	resp, _, err := client.SystemAPI.CreateAccount(ctx, systemID).AccountCreateRequest(syncp.AccountCreateRequest{
		JwtSettings: &syncp.AccountJWTSettings{
			Info: syncp.Info{
				Description: &defaultDescription,
			},
			Authorization: nil,
			Limits: &syncp.OperatorLimits{
				NatsLimits: syncp.NatsLimits{
					Data:    &defaultData,
					Payload: &defaultPayload,
					Subs:    &defaultSubs,
				},
				AccountLimits: syncp.AccountLimits{
					Conn:    &defaultConn,
					Exports: &defaultExports,
					Imports: &defaultImports,
					// Leaf:      &defaultLeaf,
					Wildcards: &defaultWildcards,
				},
				JetStreamLimits: syncp.JetStreamLimits{
					// 	Consumer:           &defaultConsumer,
					// 	DiskMaxStreamBytes: &defaultDiskMaxStreamBytes,
					// 	DiskStorage:        &defaultDiskStorage,
					// 	MaxBytesRequired:   &defaultMaxBytesRequired,
				},
			},
		},
		Name:                 name,
		UserJwtExpiresInSecs: nil}).Execute()
	if err != nil {
		handleApiError(err)
	}
	log.Printf("Account created: %s\n", resp.Name)
}

func delete(ctx context.Context, client *syncp.APIClient, initials string) {
	panic("TODO")
}

type Options struct {
	Action   string
	Initials string
	SystemID string
	BaseURL  string
}

func ParseOptions() *Options {
	// Define flags
	action := flag.String("action", "list", "Specifies the action to perform. Required, must be list , create or delete.")
	initials := flag.String("initials", "", "Specifies your initials, must be 2 or 3 chars. Required.")
	systemId := flag.String("systemId", "2jJ2e6WvEiKdR7z1svJwxAfGJyg", "Specifies the systemId to use. Required.")
	baseUrl := flag.String("baseUrl", "https://cloud.synadia.com", "Specifies the base URL to use.")
	// Parse the flags
	flag.Parse()

	// Check if action is one of the allowed values
	allowed := []string{"list", "create", "delete"}
	if found, _ := findStringInSlice(allowed, *action); !found {
		log.Fatalf("Invalid action: '%s', only allowed %s", *action, allowed)
	}

	// Check if initials is 2 or 3 chars
	if len(*initials) != 2 && len(*initials) != 3 {
		log.Fatalf("Invalid initials: '%s', must be 2 or 3 chars long", *initials)
	}

	return &Options{
		Action:   *action,
		Initials: *initials,
		SystemID: *systemId,
		BaseURL:  *baseUrl,
	}
}

func personalAccessToken() string {
	// Read accessToken from file
	file := "../cdk/config/control-plane-agent.token"
	accessToken, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	return string(accessToken)
}

// Connect to Synadia Cloud
func NatsConnect() (context.Context, error) {
	// Read accessToken from file
	file := "../cdk/config/control-plane-agent.token"
	accessToken, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	ctx := context.WithValue(context.Background(), syncp.ContextServerVariables, map[string]string{
		"baseUrl": "https://cloud.synadia.com",
	})
	// log.Printf("Access token: %s", accessToken)
	return context.WithValue(ctx, syncp.ContextAccessToken, accessToken), nil
}

func handleApiError(err error) {
	// error with body
	apiErr := &syncp.GenericOpenAPIError{}
	if errors.As(err, &apiErr) {
		log.Fatal(apiErr.Error(), string(apiErr.Body()))
	}
	log.Fatal(err)
}

// findStringInSlice searches for a string in a slice of strings.
// Returns true and the index if found, false and -1 otherwise.
func findStringInSlice(slice []string, val string) (bool, int) {
	for i, item := range slice {
		if item == val {
			return true, i
		}
	}
	return false, -1
}
