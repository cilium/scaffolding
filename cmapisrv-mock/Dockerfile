ARG BASE_IMAGE=docker.io/library/alpine:3.21.3@sha256:a8560b36e8b8210634f77d9f7f9efd7ffa463e380b75e2e74aff4511df3ef88c
ARG GOLANG_IMAGE=docker.io/library/golang:1.24.0@sha256:3f7444391c51a11a039bf0359ee81cc64e663c17d787ad0e637a4de1a3f62a71
ARG ETCD_SERVER_IMAGE=gcr.io/etcd-development/etcd:v3.5.18@sha256:9079538aa6e9ff9a57b4d737ca1f03f978e7b9e8fe45bb91b19bd933deb98912

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
