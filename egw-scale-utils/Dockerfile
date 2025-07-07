ARG BASE_IMAGE=docker.io/library/alpine:3.22.0@sha256:8a1f59ffb675680d47db6337b49d22281a139e9d709335b492be023728e11715
ARG GOLANG_IMAGE=docker.io/library/golang:1.24.4@sha256:20a022e5112a144aa7b7aeb3f22ebf2cdaefcc4aac0d64e8deeee8cdc18b9c0f

FROM ${GOLANG_IMAGE} AS builder
ADD . /go/src/github.com/cilium/scaffolding/egw-scale-utils
WORKDIR /go/src/github.com/cilium/scaffolding/egw-scale-utils
RUN make local

FROM ${BASE_IMAGE}

COPY --from=builder /go/src/github.com/cilium/scaffolding/egw-scale-utils/egw-scale-utils /usr/bin/egw-scale-utils

ENTRYPOINT ["/usr/bin/egw-scale-utils"]
