.PHONY: init build build_docker deploy

.EXPORT_ALL_VARIABLES:
AWS_PROFILE = training

bootstrap:
	cd cdk;\
	cdk bootstrap --profile ${AWS_PROFILE}

test:
	cd cdk;\
	go test

build_docker:
	docker build -t go-fargate .

deploy: build_docker
	cd cdk;\
	cdk deploy --profile ${AWS_PROFILE} --stack-timeout-in-minutes 5

diff: 
	cd cdk;\
	cdk diff --profile ${AWS_PROFILE} 

destroy:
	cd cdk;\
	cdk destroy --profile ${AWS_PROFILE} 

benchmark:
	go run benchmark.go

cp-list-djo:
	cd control-plane;	go run main.go --action list --systemId 2iM54Ea75kFpyBM37W7Ee5Yhn4A --initials DO

cp-list:
	cd control-plane;	go run main.go --action list --initials DO

cp-create:
	cd control-plane;	go run main.go --action create --initials DO

cp-delete:
	cd control-plane;	go run main.go --action delete --initials DO	