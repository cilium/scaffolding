ARG BASE_IMAGE=docker.io/library/alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659
ARG GOLANG_IMAGE=docker.io/library/golang:1.26.1@sha256:e2ddb153f786ee6210bf8c40f7f35490b3ff7d38be70d1a0d358ba64225f6428

FROM ${GOLANG_IMAGE} AS builder
ADD . /go/src/github.com/cilium/scaffolding/egw-scale-utils
WORKDIR /go/src/github.com/cilium/scaffolding/egw-scale-utils
RUN make local

FROM ${BASE_IMAGE}

COPY --from=builder /go/src/github.com/cilium/scaffolding/egw-scale-utils/egw-scale-utils /usr/bin/egw-scale-utils

ENTRYPOINT ["/usr/bin/egw-scale-utils"]
