DOCKER_IMAGE ?= quay.io/cilium/egw-scale-utils:latest

.PHONY: all docker-image docker-image-load docker-image-push egw-scale-utils local clean

all: local

docker-image:
	docker buildx build --platform $(shell uname -m) -t $(DOCKER_IMAGE) . -f Dockerfile --load

CLUSTER ?= kind
docker-image-load: docker-image
	kind load docker-image $(DOCKER_IMAGE) --name $(CLUSTER)

docker-image-push: docker-image
	docker push $(DOCKER_IMAGE)

egw-scale-utils:
	CGO_ENABLED=0 go build -o $@ *.go

local: egw-scale-utils
	strip egw-scale-utils

clean:
	rm -fr egw-scale-utils
