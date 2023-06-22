.PHONY: install init build build_docker test deploy start-ls stop-ls

.EXPORT_ALL_VARIABLES:
GOPROXY = direct
NETWORK_NAME="localstack-shared-net"

install: 
	@which localstack || pip install localstack
	@which awslocal || pip install awscli-local

init:
	cd cdk;\
	npm i

build_docker:
	docker build -t go-fargate .

deploy: build_docker
	cd cdk;\
	cdklocal bootstrap;\
	cdklocal deploy ---require-approval never

start-ls:
	-docker network create $(NETWORK_NAME) 2> /dev/null;
	LAMBDA_DOCKER_NETWORK=$(NETWORK_NAME) DOCKER_FLAGS="--network $(NETWORK_NAME)" DEBUG=1 localstack start -d

stop-ls:
	localstack stop

run: start-ls init deploy
	./run.sh
	make stop-ls

test: install start-ls init deploy
	./run.sh;./test.sh;exit_code=`echo $$?`;\
	make stop-ls; exit $$exit_code
