build_docker:
	docker build -t registry.gitlab.inn4science.com/ctp/hermes/master --build-arg CONFIG=master .

push_docker:
	docker push registry.gitlab.inn4science.com/ctp/hermes/master:latest

generate:
	PATH=${HOME}/go/bin:${PATH};go generate ./...

test:
	PATH=${HOME}/go/bin:${PATH};go test ./...

lint:
	PATH=${HOME}/go/bin:${PATH};golangci-lint run --skip-dirs scripts

check: generate test lint
