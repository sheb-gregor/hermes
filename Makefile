build_docker:
	docker build -t registry.gitlab.inn4science.com/ctp/hermes/master --build-arg CONFIG=master .

push_docker:
	docker push registry.gitlab.inn4science.com/ctp/hermes/master:latest
