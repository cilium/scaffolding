ARG BASE_IMAGE=docker.io/library/alpine:3.22.1@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1
ARG GOLANG_IMAGE=docker.io/library/golang:1.25.1@sha256:d7098379b7da665ab25b99795465ec320b1ca9d4addb9f77409c4827dc904211

FROM ${GOLANG_IMAGE} AS builder
ADD . /go/src/github.com/cilium/scaffolding/egw-scale-utils
WORKDIR /go/src/github.com/cilium/scaffolding/egw-scale-utils
RUN make local

FROM ${BASE_IMAGE}

COPY --from=builder /go/src/github.com/cilium/scaffolding/egw-scale-utils/egw-scale-utils /usr/bin/egw-scale-utils

ENTRYPOINT ["/usr/bin/egw-scale-utils"]
