import { App, Stack, StackProps, RemovalPolicy, SecretValue, Tags } from 'aws-cdk-lib';
import * as path from 'path';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as ecs from 'aws-cdk-lib/aws-ecs';
import * as sm from "aws-cdk-lib/aws-secretsmanager";
import * as fs from 'fs';
// import * as sqs from 'aws-cdk-lib/aws-sqs';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import * as cw from 'aws-cdk-lib/aws-cloudwatch';
import { LogGroup, RetentionDays } from 'aws-cdk-lib/aws-logs';
import { DockerImageAsset } from 'aws-cdk-lib/aws-ecr-assets';
import { ContainerImage, FargatePlatformVersion } from 'aws-cdk-lib/aws-ecs';
import { execSync } from 'child_process';

import { default as config } from '../config/config';

export class FargateServiceStack extends Stack {
  private uniqueSuffix: string;
  private currentUser: string;

  constructor(scope: App, id: string, props?: StackProps) {

    // Generate the unique suffix before calling super
    const uniqueSuffix = FargateServiceStack.getCurrentUser();
    
    // Modify the stack name to include the unique suffix
    super(scope, `${id}-${uniqueSuffix}`, props);

    this.uniqueSuffix = uniqueSuffix;
    this.currentUser = FargateServiceStack.getUserEmail();

    // Update resource names with the unique suffix
    const tableName = `${config.tableName}-${this.uniqueSuffix}`;
    const clusterName = `${config.clusterName}-${this.uniqueSuffix}`;
    const logGroupName = `${config.service.logGroup}-${this.uniqueSuffix}`;
    const serviceName = `${config.service.name}-${this.uniqueSuffix}`;
    const dashboardName = `${config.dashboard.name}-${this.uniqueSuffix}`;
    const vpcName = `FargateVPC-${this.uniqueSuffix}`;
    const secretName = `${this.uniqueSuffix}/NATSCredentials`;

    const ddbTable = new dynamodb.Table(this, "Table", {
      tableName: tableName,
      partitionKey: { name: 'id', type: dynamodb.AttributeType.STRING },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      removalPolicy: RemovalPolicy.DESTROY
    });

    // Read the content of the NGS-poc-service.creds file
    const credsFilePath = path.join(__dirname, '../config/NGS-poc-service.creds');
    const secretContent = fs.readFileSync(credsFilePath, 'utf8');


    // Create a new secret in AWS Secrets Manager with the content of NGS-poc-service.creds
    const secret = new sm.Secret(this, secretName, {
      secretName: 'NATSCredentials',
      description: 'NATs service credentials',
      secretStringValue: SecretValue.unsafePlainText(secretContent),
      removalPolicy: RemovalPolicy.DESTROY //change it if you want to keep the secret
    });


    const asset = new DockerImageAsset(this, "GoDockerImage", {
      directory: path.join(__dirname, "..", ".."),
    });

    const vpc = new ec2.Vpc(this, "EcsVpc", {
      maxAzs: 3,
      vpcName: vpcName,
    });

    const cluster = new ecs.Cluster(this, "EcsCluster", {
      vpc: vpc,
      clusterName: clusterName,
      containerInsights: false,      
    });

    const logGroup = new LogGroup(this, "FargateLogGroup", {
      logGroupName: logGroupName,
      retention: RetentionDays.ONE_DAY,
      removalPolicy: RemovalPolicy.DESTROY
    });

    const taskDef = new ecs.FargateTaskDefinition(this, "MyTask", {
      cpu: config.service.cpu,
      memoryLimitMiB: config.service.memory,
    });

    const container = new ecs.ContainerDefinition(this, "MyContainer", {
      image: ContainerImage.fromDockerImageAsset(asset),
      taskDefinition: taskDef,
      environment: {
        NATS_CREDENTIALS: secretName,
        DDB_TABLE: ddbTable.tableName
      },
      logging: new ecs.AwsLogDriver({
        logGroup: logGroup,
        streamPrefix: config.service.logStreamPrefix,
      })
    });

    const myService = new ecs.FargateService(this, "MyService", {
      taskDefinition: taskDef,
      cluster: cluster,
      platformVersion: FargatePlatformVersion.VERSION1_4,
      serviceName: serviceName,
      desiredCount: 1
    });

    // //grant service role permission to read from SQS
    // queue.grantConsumeMessages(taskDef.taskRole)

    // grant service role permission to read from secret and write to DynamoDB
    // container.addSecret("NATS_CREDENTIALS", secret)
    secret.grantRead(taskDef.taskRole)
    ddbTable.grantWriteData(taskDef.taskRole)

    const dashboardStart = "-P1D";
    const dashboard = new cw.Dashboard(this, "ServiceDashboard", {
      dashboardName: dashboardName,
      start: dashboardStart
    });

    dashboard.addWidgets(new cw.LogQueryWidget({
      logGroupNames: [logGroupName],
      view: cw.LogQueryVisualizationType.LINE,
      queryLines: [
        'filter @message like /is saved in DDB/',
        '| stats count(*) as messagesSavedInDynamoDBCount by bin(5m)',
        '| sort exceptionCount desc',
      ],
      title: config.dashboard.ddbWidgetTitle,
      width: 24
    }));

    dashboard.addWidgets(new cw.LogQueryWidget({
      logGroupNames: [logGroupName],
      view: cw.LogQueryVisualizationType.LINE,
      queryLines: [
        'filter @message like /is received from SQS/',
        '| stats count(*) as sqsMessageReceivedCount by bin(5m)',
        '| sort exceptionCount desc',
      ],
      title: config.dashboard.sqsWidgetTitle,
      width: 24
    }));

    // Add tags to all resources in the stack
    this.addTags();
  }

  private static getCurrentUser(): string {
    try {
      // Use AWS CLI to get caller identity synchronously
      const result = execSync('aws sts get-caller-identity --query Arn --output text', { encoding: 'utf8' }).toString().trim();
      
      let username = '';
      if (result.includes(':user/')) {
        username = result.split(':user/').pop() || '';
      } else if (result.includes(':assumed-role/')) {
        const parts = result.split(':assumed-role/').pop()?.split('/') || [];
        username = parts[parts.length - 1] || '';
      } else {
        username = result.split('/').pop() || '';
      }

      // Extract the first two initials and convert to uppercase
      const initials = username
        .split(/[\s.-]+/) // Split by spaces, dots, and hyphens
        .map(part => part[0] || '') // Get the first character of each part
        .join('') // Join the initials
        .slice(0, 2) // Take only the first two characters
        .toUpperCase(); // Convert to uppercase

      // Ensure the result is alphanumeric
      const sanitized = initials.replace(/[^A-Z0-9]/g, '');

      // If we couldn't extract any valid initials, fall back to a timestamp
      if (sanitized.length === 0) {
        return `U${Date.now().toString(36).slice(0, 6)}`;
      }

      // Pad with 'X' if we only got one initial
      return sanitized.padEnd(2, 'X');
    } catch (error) {
      console.error('Error getting caller identity:', error);
      // Fallback to a timestamp-based suffix if we can't get the user identity
      return `U${Date.now().toString(36).slice(0, 6)}`;
    }
  }

  private static getUserEmail(): string {
    try {
      // Use AWS CLI to get caller identity synchronously
      const result = execSync('aws sts get-caller-identity --query Arn --output text', { encoding: 'utf8' }).toString().trim();

      let email = '';
      if (result.includes(':user/')) {
        email = result.split(':user/').pop() || '';
      } else if (result.includes(':assumed-role/')) {
        const parts = result.split(':assumed-role/').pop()?.split('/') || [];
        email = parts[parts.length - 1] || '';
      } else {
        email = result.split('/').pop() || '';
      }

      // If the email doesn't contain '@', it's probably not a valid email address
      if (!email.includes('@')) {
        // Try to get the user's email from their IAM user profile
        const userInfo = execSync('aws iam get-user --query User.UserName --output text', { encoding: 'utf8' }).toString().trim();
        const userEmail = execSync(`aws iam list-user-tags --user-name ${userInfo} --query "Tags[?Key=='email'].Value" --output text`, { encoding: 'utf8' }).toString().trim();
        
        if (userEmail) {
          email = userEmail;
        }
      }

      // If we still don't have a valid email, return a placeholder
      if (!email.includes('@')) {
        return 'unknown@example.com';
      }

      return email;
    } catch (error) {
      console.error('Error getting user email:', error);
      // Fallback to a placeholder email if we can't get the user's email
      return 'unknown@example.com';
    }
  }


  private addTags(): void {
    Tags.of(this).add("product", "digital_wallet");
    Tags.of(this).add("owner", this.currentUser);
    Tags.of(this).add("project", "nats-fargate-ddb-cdk-go");
    Tags.of(this).add("environment", "poc");
    Tags.of(this).add("developer_id", this.uniqueSuffix);
  }


}