# LocalStack Instructions
This sample is based on the the aws sample of the same name and modified to work on LocalStack instead.
It creates an sqs queue, a dynamodb table, and a fargate ecs container that writes messages from the queue to the database.
Here you can find the necessary steps you need to take.
1. Navigate into the cloned repo
2. Make sure the following environment variable is exported:
```
export LOCALSTACK_API_KEY=<your_api_key>
```
3. Run the execution script
```
./run-against-ls

For the original repo, check [the original sample](https://github.com/aws-samples/sqs-fargate-ddb-cdk-go).