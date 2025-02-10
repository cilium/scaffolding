ARG BASE_IMAGE=docker.io/library/alpine:3.21.2@sha256:56fa17d2a7e7f168a043a2712e63aed1f8543aeafdcee47c58dcffe38ed51099
ARG GOLANG_IMAGE=docker.io/library/golang:1.23.6@sha256:927112936d6b496ed95f55f362cc09da6e3e624ef868814c56d55bd7323e0959
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
