build_linux:
	@cd ../scripts/auth_provider && env GOOS=linux GOARCH=amd64 go build -o ../../tmp/auth_provider .
	@cd ../scripts/client && env GOOS=linux GOARCH=amd64 go build -o ../../tmp/client .
	@cd ../scripts/rabbit_emitter && env GOOS=linux GOARCH=amd64 go build -o ../../tmp/rabbit_emitter .
	@cd ../hermes && env GOOS=linux GOARCH=amd64 go build  -o tmp/hermes .


build:
	@cd ../scripts/auth_provider && go build -o ../../tmp/auth_provider .
	@cd ../scripts/client && go build -o ../../tmp/client .
	@cd ../scripts/rabbit_emitter && go build -o ../../tmp/rabbit_emitter .
	@cd ../hermes && go build -o tmp/hermes .


update_remote: build_linux
	@rsync -arv ./ teamo.lake:hermes_test/


fetch_reports:
	@scp teamo.lake:hermes_test/emitter_metrics_report.json ./
	@scp teamo.lake:hermes_test/client_metrics_report.json ./
