ARG BASE_IMAGE=docker.io/library/alpine:3.22.1@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1
ARG GOLANG_IMAGE=docker.io/library/golang:1.25.1@sha256:a5e935dbd8bc3a5ea24388e376388c9a69b40628b6788a81658a801abbec8f2e
ARG ETCD_SERVER_IMAGE=gcr.io/etcd-development/etcd:v3.6.4@sha256:5d10878e4fd4ebfdf82bc142fb044542a3ca514c0ee169277643a84d6816892a

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
