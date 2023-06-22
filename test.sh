#!/bin/bash
set -e
expected_queue_url="http://localhost:4566/000000000000/sqs-fargate-queue"
expected_db="sqs-fargate-ddb-table"
source assert.sh/assert.sh

echo "Running tests for fargate-ddb-cdk-go sample..."
sqs_queue=$(awslocal sqs list-queues | jq -r .QueueUrls[0])
db_table=$(awslocal dynamodb list-tables | jq -r .TableNames[0])
ddb_item=$(awslocal dynamodb scan --table-name sqs-fargate-ddb-table | jq -r .Items[0].message.S)

assert_eq "$sqs_queue" "$expected_queue_url" "Queue urls do not match!"
assert_eq "$db_table" "$expected_db" "Database tables do not match!"
assert_eq "$ddb_item" "hello world" "Message body does not match!"
echo "Tests completed"