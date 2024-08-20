package main

import (
	"context"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func getUserEmail(ctx context.Context) string {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	stsSvc := sts.NewFromConfig(cfg)
	iamSvc := iam.NewFromConfig(cfg)

	// Get caller identity
	input := &sts.GetCallerIdentityInput{}
	result, err := stsSvc.GetCallerIdentity(ctx, input)
	if err != nil {
		log.Printf("Error getting caller identity: %v", err)
		return "unknown@example.com"
	}

	arn := aws.ToString(result.Arn)
	email := extractEmailFromArn(arn)

	// If the email doesn't contain '@', it's probably not a valid email address
	if !strings.Contains(email, "@") {
		// Try to get the user's email from their IAM user profile
		userInfo, err := iamSvc.GetUser(ctx, &iam.GetUserInput{})
		if err != nil {
			log.Printf("Error getting IAM user info: %v", err)
			return "unknown@example.com"
		}

		userTags, err := iamSvc.ListUserTags(ctx, &iam.ListUserTagsInput{
			UserName: userInfo.User.UserName,
		})
		if err != nil {
			log.Printf("Error listing IAM user tags: %v", err)
			return "unknown@example.com"
		}

		for _, tag := range userTags.Tags {
			if aws.ToString(tag.Key) == "email" {
				email = aws.ToString(tag.Value)
				break
			}
		}
	}

	// If we still don't have a valid email, return a placeholder
	if !strings.Contains(email, "@") {
		return "unknown@example.com"
	}

	return email
}

func extractEmailFromArn(arn string) string {
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
