package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecrassets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

func main() {
	defer jsii.Close()

	// Load the configuration
	config, err := loadConfig("cdk/config/project-config.json")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	app := awscdk.NewApp(nil)

	NewTestCdkGoStack(app, "TestCdkGoStack", config, &TestCdkGoStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

type TestCdkGoStackProps struct {
	awscdk.StackProps
}

func NewTestCdkGoStack(scope constructs.Construct, id string, config *ProjectConfig, props *TestCdkGoStackProps) awscdk.Stack {

	ctx := context.Background()
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}

	// Generate a unique suffix to be used for everything in the stack
	uniqueSuffix := getCurrentUser(ctx)
	id = fmt.Sprintf("%s-%s", id, uniqueSuffix)

	stack := awscdk.NewStack(scope, &id, &sprops)

	// Create DynamoDB table
	tableId := "Table"
	table := awsdynamodb.NewTableV2(
		stack,
		&tableId,
		&awsdynamodb.TablePropsV2{
			TableName: jsii.String(fmt.Sprintf("%s-%s", config.TableName, uniqueSuffix)),
			PartitionKey: &awsdynamodb.Attribute{
				Name: jsii.String("id"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
			Billing:       awsdynamodb.Billing_OnDemand(),
		},
	)

	// Read the NATS credentials from a file and encode them as a base64 string
	// So that we can inject them into the container as an environment variable
	secretValue, err := os.ReadFile(config.NatsCredentialsFile)
	if err != nil {
		log.Fatalf("Error reading NATS credentials file from path '%s', error: %v", config.NatsCredentialsFile, err)
	}
	base64encodedCreds := base64.StdEncoding.EncodeToString(secretValue)

	// awssecretsmanager.NewSecret(
	// 	stack,
	// 	jsii.String(secretName),
	// 	&awssecretsmanager.SecretProps{
	// 		Description:       jsii.String("NATS credentials"),
	// 		SecretName:        jsii.String(secretName),
	// 		SecretStringValue: awscdk.SecretValue_UnsafePlainText(jsii.String(string(secretValue))),
	// 		RemovalPolicy:     awscdk.RemovalPolicy_DESTROY,
	// 	},
	// )

	// Create a Docker image asset
	imageAsset := awsecrassets.NewDockerImageAsset(
		stack,
		jsii.String("DockerImageAsset"),
		&awsecrassets.DockerImageAssetProps{
			Directory: jsii.String(config.ProjectBaseDirectory),
		},
	)

	// Create a ECS VPC
	vpc := awsec2.NewVpc(
		stack,
		jsii.String("EcsVpc"),
		&awsec2.VpcProps{
			MaxAzs:  jsii.Number(3),
			VpcName: jsii.String(fmt.Sprintf("FargateVPC-%s", uniqueSuffix)),
		},
	)

	// Create an ECS CLuster
	cluster := awsecs.NewCluster(
		stack,
		jsii.String("EcsCluster"),
		&awsecs.ClusterProps{
			ClusterName:       jsii.String(fmt.Sprintf("%s-%s", config.ClusterName, uniqueSuffix)),
			Vpc:               vpc,
			ContainerInsights: jsii.Bool(true),
		},
	)

	// Create a Log Group
	logGroup := awslogs.NewLogGroup(
		stack,
		jsii.String("FargateLogGroup"),
		&awslogs.LogGroupProps{
			LogGroupName:  jsii.String(fmt.Sprintf("%s-%s", config.Service.LogGroup, uniqueSuffix)),
			Retention:     awslogs.RetentionDays_ONE_DAY,
			RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
		},
	)

	// Create a Fargate Task
	task := awsecs.NewTaskDefinition(
		stack,
		jsii.String("MyTask"),
		&awsecs.TaskDefinitionProps{
			Cpu:           jsii.String(fmt.Sprintf("%d", config.Service.CPU)),
			MemoryMiB:     jsii.String(fmt.Sprintf("%d", config.Service.Memory)),
			Compatibility: awsecs.Compatibility_FARGATE,
		},
	)

	// Create a Fargate Container
	awsecs.NewContainerDefinition(
		stack,
		jsii.String("MyContainer"),
		&awsecs.ContainerDefinitionProps{
			Image:          awsecs.ContainerImage_FromDockerImageAsset(imageAsset),
			TaskDefinition: task,
			Environment: &map[string]*string{
				"NATS_CREDENTIALS": &base64encodedCreds,
				"DDB_TABLE":        table.TableName(),
			},
			Logging: awsecs.NewAwsLogDriver(
				&awsecs.AwsLogDriverProps{
					LogGroup:     logGroup,
					StreamPrefix: jsii.String(config.Service.LogStreamPrefix),
				}),
		},
	)

	// Create a Fargate Container
	desiredCount := 3.0
	awsecs.NewFargateService(
		stack,
		jsii.String("MyService"),
		&awsecs.FargateServiceProps{
			TaskDefinition:  task,
			Cluster:         cluster,
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
			ServiceName:     jsii.String(fmt.Sprintf("%s-%s", config.Service.Name, uniqueSuffix)),
			DesiredCount:    &desiredCount,
		},
	)

	// Grant permissions
	table.GrantReadWriteData(task.TaskRole())

	// queue := awssqs.NewQueue(stack, jsii.String("TestCdkGoQueue"), &awssqs.QueueProps{
	// 	VisibilityTimeout: awscdk.Duration_Seconds(jsii.Number(300)),
	// })

	// topic := awssns.NewTopic(stack, jsii.String("TestCdkGoTopic"), &awssns.TopicProps{})
	// topic.AddSubscription(awssnssubscriptions.NewSqsSubscription(queue, &awssnssubscriptions.SqsSubscriptionProps{}))

	// Add Tags to all resources created by the stack
	awscdk.Tags_Of(stack).Add(jsii.String("product"), jsii.String("digital_wallet"), nil)
	awscdk.Tags_Of(stack).Add(jsii.String("owner"), jsii.String(getUserEmail(ctx)), nil)
	awscdk.Tags_Of(stack).Add(jsii.String("project"), jsii.String("nats-fargate-ddb-cdk-go"), nil)
	awscdk.Tags_Of(stack).Add(jsii.String("environment"), jsii.String("poc"), nil)
	awscdk.Tags_Of(stack).Add(jsii.String("developer_id"), &uniqueSuffix, nil)

	return stack
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	// If unspecified, this stack will be "environment-agnostic".
	// Account/Region-dependent features and context lookups will not work, but a
	// single synthesized template can be deployed anywhere.
	//---------------------------------------------------------------------------
	return nil

	// Uncomment if you know exactly what account and region you want to deploy
	// the stack to. This is the recommendation for production stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String("123456789012"),
	//  Region:  jsii.String("us-east-1"),
	// }

	// Uncomment to specialize this stack for the AWS Account and Region that are
	// implied by the current CLI configuration. This is recommended for dev
	// stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
	//  Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	// }
}
