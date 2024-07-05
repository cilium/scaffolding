ARG BASE_IMAGE=docker.io/library/alpine:3.20.1@sha256:b89d9c93e9ed3597455c90a0b88a8bbb5cb7188438f70953fede212a0c4394e0
ARG GOLANG_IMAGE=docker.io/library/golang:1.22.5@sha256:fcae9e0e7313c6467a7c6632ebb5e5fab99bd39bd5eb6ee34a211353e647827a
ARG ETCD_SERVER_IMAGE=gcr.io/etcd-development/etcd:v3.5.14@sha256:e690e8c2a36f7ededaa76e2178e7492954aef33616fa30e323bc00852fdb65c2

FROM ${GOLANG_IMAGE} AS builder
ADD . /go/src/github.com/cilium/scaffolding/cmapisrv-mock
WORKDIR /go/src/github.com/cilium/scaffolding/cmapisrv-mock
RUN make cmapisrv-mock
RUN strip cmapisrv-mock

FROM ${ETCD_SERVER_IMAGE} AS etcd

FROM ${BASE_IMAGE}

COPY --from=etcd /usr/local/bin/etcd /usr/bin/etcd
COPY --from=builder /go/src/github.com/cilium/scaffolding/cmapisrv-mock/etcd-config.yaml /var/lib/cilium/etcd-config.yaml
COPY --from=builder /go/src/github.com/cilium/scaffolding/cmapisrv-mock/cmapisrv-mock /usr/bin/cmapisrv-mock

ENTRYPOINT ["/usr/bin/cmapisrv-mock"]
