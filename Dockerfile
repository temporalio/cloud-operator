ARG BUILDER_IMAGE=golang
ARG BUILDER_IMAGE_TAG=1.26.8
ARG RUNTIME_IMAGE=scratch

FROM $BUILDER_IMAGE:$BUILDER_IMAGE_TAG AS builder

ARG TARGET_ARCH=amd64
ARG GIT_VERSION
ARG GIT_COMMIT
ARG BUILD_DATE
ARG VERSION_PKG=github.com/temporalio/cloud-operator/pkg/version

ENV GOPROXY=https://proxy.golang.org|direct
ENV GO111MODULE=on
ENV GOARCH=$TARGET_ARCH
ENV GOOS=linux
ENV CGO_ENABLED=0

WORKDIR /

COPY LICENSE go.mod go.sum ./
RUN go mod download

COPY api api
COPY cmd cmd
COPY controller controller
COPY pkg pkg

RUN go build -ldflags="-X ${VERSION_PKG}.GitVersion=${GIT_VERSION} -X ${VERSION_PKG}.GitCommit=${GIT_COMMIT} -X ${VERSION_PKG}.BuildDate=${BUILD_DATE}" \
    -a -o bin/cloud-operator cmd/cloud-operator/main.go

FROM $RUNTIME_IMAGE

ARG RUNTIME_IMAGE=scratch
ARG GIT_VERSION
ARG GIT_COMMIT
ARG BUILD_DATE

LABEL org.opencontainers.image.base.name=$RUNTIME_IMAGE
LABEL org.opencontainers.image.title="Temporal Cloud Operator"
LABEL org.opencontainers.image.description="A Kubernetes Operator that manages the lifecycle of Temporal Cloud resources"
LABEL org.opencontainers.image.source=https://github.com/temporalio/cloud-operator
LABEL org.opencontainers.image.version=$GIT_VERSION
LABEL org.opencontainers.image.revision=$GIT_COMMIT
LABEL org.opencontainers.image.vendor="Temporal Technologies, Inc."
LABEL org.opencontainers.image.created=$BUILD_DATE
LABEL org.opencontainers.image.licenses=Apache-2.0

WORKDIR /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /LICENSE /LICENSE
COPY --from=builder /bin/cloud-operator /bin/cloud-operator
USER 65532:65532

ENTRYPOINT ["/bin/cloud-operator"]
