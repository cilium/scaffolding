DOCKER_IMAGE ?= quay.io/cilium/cmapisrv-mock:latest

.PHONY: all docker-image docker-image-load docker-image-push tests cmapisrv-mock local clean

all: local

docker-image:
	docker buildx build --platform $(shell uname -m) -t $(DOCKER_IMAGE) . -f Dockerfile -o type=docker

CLUSTER ?= kind
docker-image-load: docker-image
	kind load docker-image $(DOCKER_IMAGE) --name $(CLUSTER)

docker-image-push: docker-image
	docker push $(DOCKER_IMAGE)

tests:
	go test -mod=vendor ./...

cmapisrv-mock: tests
	CGO_ENABLED=0 go build -mod=vendor -o $@ *.go

local: cmapisrv-mock
	strip cmapisrv-mock

clean:
	rm -fr cmapisrv-mock
