ARG BASE_IMAGE=docker.io/library/alpine:3.23.4@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
ARG GOLANG_IMAGE=docker.io/library/golang:1.26.2@sha256:5f3787b7f902c07c7ec4f3aa91a301a3eda8133aa32661a3b3a3a86ab3a68a36
ARG ETCD_SERVER_IMAGE=gcr.io/etcd-development/etcd:v3.6.10@sha256:f65c61039e7b7fd6e651f7ec2459b880589892cb13cf79c2f71c92aa08fc5144

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
