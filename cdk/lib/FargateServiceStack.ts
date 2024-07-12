import { App, Duration, Stack, StackProps, RemovalPolicy, SecretValue } from 'aws-cdk-lib';
import * as path from 'path';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as ecs from 'aws-cdk-lib/aws-ecs';
import * as sm from "aws-cdk-lib/aws-secretsmanager";
import * as fs from 'fs';
// import * as sqs from 'aws-cdk-lib/aws-sqs';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import * as cw from 'aws-cdk-lib/aws-cloudwatch';
import { LogGroup } from 'aws-cdk-lib/aws-logs';
import { DockerImageAsset } from 'aws-cdk-lib/aws-ecr-assets';
import { ContainerImage, FargatePlatformVersion } from 'aws-cdk-lib/aws-ecs';

import { default as config } from '../config/config';


export class FargateServiceStack extends Stack {
  constructor(scope: App, id: string, props?: StackProps) {
    super(scope, id, props);

    // Read the content of the NGS-poc-service.creds file
    const credsFilePath = path.join(__dirname, '../config/NGS-poc-service.creds');
    const secretContent = fs.readFileSync(credsFilePath, 'utf8');

    const ddbTable = new dynamodb.Table(this, "Table", {
      tableName: config.tableName,
      partitionKey: { name: 'id', type: dynamodb.AttributeType.STRING },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      removalPolicy: RemovalPolicy.DESTROY //change it if you want to keep the table
    });


    // Create a new secret in AWS Secrets Manager with the content of NGS-poc-service.creds
    const serviceCredentialsSecretId = 'serviceCreds';
    const secret = new sm.Secret(this, serviceCredentialsSecretId, {
      secretName: 'serviceCreds',
      description: 'NATs service credentials',
      secretStringValue: SecretValue.unsafePlainText(secretContent),
      removalPolicy: RemovalPolicy.DESTROY //change it if you want to keep the secret
    });


    // const queue = new sqs.Queue(this, "SqsQueue", {
    //   queueName: config.queueName,
    //   encryption: sqs.QueueEncryption.KMS_MANAGED,
    //   visibilityTimeout: Duration.minutes(15),
    // })

    const asset = new DockerImageAsset(this, "GoDockerImage", {
      directory: path.join(__dirname, "..", ".."),
    });

    const vpc = new ec2.Vpc(this, "EcsVpc", {
      maxAzs: 3 // Default is all AZs in the region
    });

    const cluster = new ecs.Cluster(this, "EcsCluster", {
      vpc: vpc,
      clusterName: config.clusterName,
      containerInsights: false
    });

    const logGroup = new LogGroup(this, "FargateLogGroup", {
      logGroupName: config.service.logGroup
    })

    const taskDef = new ecs.FargateTaskDefinition(this, "MyTask", {
      cpu: config.service.cpu,
      memoryLimitMiB: config.service.memory,
    })

    const container = new ecs.ContainerDefinition(this, "MyContainer", {
      image: ContainerImage.fromDockerImageAsset(asset),
      taskDefinition: taskDef,
      environment: {
        SECRET_NATS: serviceCredentialsSecretId,
        DDB_TABLE: ddbTable.tableName
      },
      logging: new ecs.AwsLogDriver({
        logGroup: logGroup,
        streamPrefix: config.service.logStreamPrefix,
      })
    }
    )

    const myService = new ecs.FargateService(this, "MyService", {
      taskDefinition: taskDef,
      cluster: cluster,
      platformVersion: FargatePlatformVersion.VERSION1_4,
      serviceName: config.service.name,
      desiredCount: 1
    })

    // //grant service role permission to read from SQS
    // queue.grantConsumeMessages(taskDef.taskRole)

    // grant service role permission to read from secret and write to DynamoDB
    secret.grantRead(taskDef.taskRole)
    ddbTable.grantWriteData(taskDef.taskRole)

    //Add CloudWatch dashboard
    const dashboardStart = "-P1D" // Start from 7 days in the past
    const dashboard = new cw.Dashboard(this,"ServiceDashboard",{
      dashboardName:config.dashboard.name,
      start:dashboardStart
    });

    dashboard.addWidgets(new cw.LogQueryWidget({
      logGroupNames: [config.service.logGroup],
      view: cw.LogQueryVisualizationType.LINE,
      queryLines: [
        'filter @message like /is saved in DDB/',
        '| stats count(*) as messagesSavedInDynamoDBCount by bin(5m)',
        '| sort exceptionCount desc',
      ],
      title:config.dashboard.ddbWidgetTitle,
      width: 24
    }));

    dashboard.addWidgets(new cw.LogQueryWidget({
      logGroupNames: [config.service.logGroup],
      view: cw.LogQueryVisualizationType.LINE,
      queryLines: [
        'filter @message like /is received from SQS/',
        '| stats count(*) as sqsMessageReceivedCount by bin(5m)',
        '| sort exceptionCount desc',
      ],
      title:config.dashboard.sqsWidgetTitle,
      width: 24
    }));

  }
}
