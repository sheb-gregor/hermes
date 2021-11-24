build_docker:
	docker build -t hermes --build-arg CONFIG=master .

generate:
	PATH=${HOME}/go/bin:${PATH};go generate ./...

test:
	PATH=${HOME}/go/bin:${PATH};go test ./...

lint:
	PATH=${HOME}/go/bin:${PATH};golangci-lint run --skip-dirs scripts

check: generate test lint
