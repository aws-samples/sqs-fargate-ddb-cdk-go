#!/bin/bash
SQS_QUEUE="sqs-fargate-queue"
NETWORK_NAME="localstack-shared-net"

docker network create $NETWORK_NAME 2> /dev/null
LAMBDA_DOCKER_NETWORK=$NETWORK_NAME DOCKER_FLAGS="--network $NETWORK_NAME" DEBUG=1 localstack start -d
# Instructions as via original README
docker build -t go-fargate .
cd cdk
npm i

cdklocal bootstrap
cdklocal deploy --require-approval never
echo "List queues to check if sqs queue was created"
awslocal sqs list-queues
echo "List tables to check if dynmodb table was created"
awslocal dynamodb list-tables

echo "Send message to the sqs queue that should be stored in the db"
awslocal sqs send-message --queue $SQS_QUEUE --message-body '{"message": "hello world"}'

echo "Giving the container time to write the message, then check if the fargate container successfully wrote the sqs message into the dynamodb table"
sleep 3
awslocal dynamodb scan --table-name sqs-fargate-ddb-table

localstack stop
