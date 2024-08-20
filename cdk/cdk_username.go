package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func getCurrentUser(ctx context.Context) string {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
		return fallbackUsername()
	}

	svc := sts.NewFromConfig(cfg)

	input := &sts.GetCallerIdentityInput{}
	result, err := svc.GetCallerIdentity(ctx, input)
	if err != nil {
		log.Printf("Error getting caller identity: %v", err)
		return fallbackUsername()
	}

	arn := aws.ToString(result.Arn)
	username := extractUsernameFromArn(arn)

	initials := extractInitials(username)
	sanitized := sanitizeInitials(initials)

	if len(sanitized) == 0 {
		return fallbackUsername()
	}

	return padInitials(sanitized)
}

func extractUsernameFromArn(arn string) string {
	if strings.Contains(arn, ":user/") {
		return strings.Split(arn, ":user/")[1]
	} else if strings.Contains(arn, ":assumed-role/") {
		parts := strings.Split(strings.Split(arn, ":assumed-role/")[1], "/")
		return parts[len(parts)-1]
	} else {
		parts := strings.Split(arn, "/")
		return parts[len(parts)-1]
	}
}

func extractInitials(username string) string {
	parts := regexp.MustCompile(`[\s.-]+`).Split(username, -1)
	initials := ""
	for _, part := range parts {
		if len(part) > 0 {
			initials += string(part[0])
		}
	}
	return strings.ToUpper(initials)[:2]
}

func sanitizeInitials(initials string) string {
	return regexp.MustCompile(`[^A-Z0-9]`).ReplaceAllString(initials, "")
}

func padInitials(initials string) string {
	if len(initials) < 2 {
		return initials + "X"
	}
	return initials
}

func fallbackUsername() string {
	return fmt.Sprintf("U%s", time.Now().Format("150405"))
}
