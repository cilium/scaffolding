ARG BASE_IMAGE=docker.io/library/alpine:3.23.2@sha256:865b95f46d98cf867a156fe4a135ad3fe50d2056aa3f25ed31662dff6da4eb62
ARG GOLANG_IMAGE=docker.io/library/golang:1.25.6@sha256:ce63a16e0f7063787ebb4eb28e72d477b00b4726f79874b3205a965ffd797ab2

FROM ${GOLANG_IMAGE} AS builder
ADD . /go/src/github.com/cilium/scaffolding/egw-scale-utils
WORKDIR /go/src/github.com/cilium/scaffolding/egw-scale-utils
RUN make local

FROM ${BASE_IMAGE}

COPY --from=builder /go/src/github.com/cilium/scaffolding/egw-scale-utils/egw-scale-utils /usr/bin/egw-scale-utils

ENTRYPOINT ["/usr/bin/egw-scale-utils"]
