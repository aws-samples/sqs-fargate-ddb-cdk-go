.PHONY: build build_docker deploy

.EXPORT_ALL_VARIABLES:
AWS_PROFILE = default
GOPROXY = direct

build_docker:
	docker build -t go-fargate .

deploy: build_docker
	cd cdk;\
	cdk deploy --profile ${AWS_PROFILE}

destroy:
	cd cdk;\
	cdk destroy --profile ${AWS_PROFILE}