import { App, Duration, Stack, StackProps, RemovalPolicy } from 'aws-cdk-lib';
import * as path from 'path';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as ecs from 'aws-cdk-lib/aws-ecs';
import * as sqs from 'aws-cdk-lib/aws-sqs';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import * as cw from 'aws-cdk-lib/aws-cloudwatch';
import { LogGroup, RetentionDays } from 'aws-cdk-lib/aws-logs';
import { DockerImageAsset } from 'aws-cdk-lib/aws-ecr-assets';
import { ContainerImage, FargatePlatformVersion } from 'aws-cdk-lib/aws-ecs';
import { execSync } from 'child_process';

import { default as config } from '../config/config';

export class FargateServiceStack extends Stack {
  private uniqueSuffix: string;

  constructor(scope: App, id: string, props?: StackProps) {

    // Generate the unique suffix before calling super
    const uniqueSuffix = FargateServiceStack.getCurrentUser();
    
    // Modify the stack name to include the unique suffix
    super(scope, `${id}-${uniqueSuffix}`, props);

    this.uniqueSuffix = uniqueSuffix;

    // Update resource names with the unique suffix
    const tableName = `${config.tableName}-${this.uniqueSuffix}`;
    const queueName = `${config.queueName}-${this.uniqueSuffix}`;
    const clusterName = `${config.clusterName}-${this.uniqueSuffix}`;
    const logGroupName = `${config.service.logGroup}-${this.uniqueSuffix}`;
    const serviceName = `${config.service.name}-${this.uniqueSuffix}`;
    const dashboardName = `${config.dashboard.name}-${this.uniqueSuffix}`;
    const vpcName = `FargateVPC-${this.uniqueSuffix}`;

    const ddbTable = new dynamodb.Table(this, "Table", {
      tableName: tableName,
      partitionKey: { name: 'id', type: dynamodb.AttributeType.STRING },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      removalPolicy: RemovalPolicy.DESTROY
    });

    const queue = new sqs.Queue(this, "SqsQueue", {
      queueName: queueName,
      encryption: sqs.QueueEncryption.KMS_MANAGED,
      visibilityTimeout: Duration.minutes(15),
    });

    const asset = new DockerImageAsset(this, "GoDockerImage", {
      directory: path.join(__dirname, "..", ".."),
    });

    const vpc = new ec2.Vpc(this, "EcsVpc", {
      maxAzs: 3,
      vpcName: vpcName
    });

    const cluster = new ecs.Cluster(this, "EcsCluster", {
      vpc: vpc,
      clusterName: clusterName,
      containerInsights: false
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
        SQS_URL: queue.queueUrl,
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

    queue.grantConsumeMessages(taskDef.taskRole);
    ddbTable.grantWriteData(taskDef.taskRole);

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
}