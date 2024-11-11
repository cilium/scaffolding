ARG BASE_IMAGE=docker.io/library/alpine:3.20.3@sha256:beefdbd8a1da6d2915566fde36db9db0b524eb737fc57cd1367effd16dc0d06d
ARG GOLANG_IMAGE=docker.io/library/golang:1.23.3@sha256:d56c3e08fe5b27729ee3834854ae8f7015af48fd651cd25d1e3bcf3c19830174
ARG ETCD_SERVER_IMAGE=gcr.io/etcd-development/etcd:v3.5.16@sha256:0d0a9fe2d8344722acfb6f456beb0c64328b58f51dc6dee6291976e62a7b5a3f

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
