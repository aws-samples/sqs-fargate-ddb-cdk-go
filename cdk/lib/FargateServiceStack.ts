import { App, Duration, Stack, StackProps, RemovalPolicy } from 'aws-cdk-lib';
import * as path from 'path';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as ecs from 'aws-cdk-lib/aws-ecs';
import * as sqs from 'aws-cdk-lib/aws-sqs';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import { LogGroup } from 'aws-cdk-lib/aws-logs';
import { DockerImageAsset } from 'aws-cdk-lib/aws-ecr-assets';
import { ContainerImage, FargatePlatformVersion } from 'aws-cdk-lib/aws-ecs';

import { default as config } from '../config/config';


export class FargateServiceStack extends Stack {
  constructor(scope: App, id: string, props?: StackProps) {
    super(scope, id, props);

    const ddbTable = new dynamodb.Table(this, "Table", {
      tableName: config.tableName,
      partitionKey: { name: 'id', type: dynamodb.AttributeType.STRING },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      removalPolicy: RemovalPolicy.DESTROY //change it if you want to keep the table
    });


    const queue = new sqs.Queue(this, "SqsQueue", {
      queueName: config.queueName,
      encryption: sqs.QueueEncryption.KMS_MANAGED,
      visibilityTimeout: Duration.minutes(15),
    })

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
        SQS_URL: queue.queueUrl,
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

    //grant service role permission to read from SQS
    queue.grantConsumeMessages(taskDef.taskRole)

    ddbTable.grantWriteData(taskDef.taskRole)

  }
}
