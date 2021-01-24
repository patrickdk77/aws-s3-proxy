# AWS S3 Proxy
# docker run -d -p 8080:80 -e AWS_REGION -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_S3_BUCKET patrickdk77/s3-proxy

ARG BUILD_FROM_PREFIX

FROM ${BUILD_FROM_PREFIX}golang:1.15-alpine3.13 AS builder
ARG BUILD_ARCH
ARG QEMU_ARCH
COPY .gitignore qemu-${QEMU_ARCH}-static* /usr/bin/
RUN apk --no-cache add gcc musl-dev git
WORKDIR /go/src/
COPY . /go/src/
ARG BUILD_VERSION
ARG BUILD_DATE
ARG BUILD_REF
ARG BUILD_GOARCH
ARG BUILD_GOOS
RUN go mod download \
 && go mod verify \
 && CGO_ENABLED=0 GOOS=${BUILD_GOOS} GOARCH=${BUILD_GOARCH} go build \
    -ldflags '-s -w -X main.ver=${BUILD_VERSION} \
    -X main.commit=${BUILD_REF} -X main.date=${BUILD_DATE}' \
    -o /app

FROM alpine:3.13 AS libs
RUN apk --no-cache add ca-certificates

FROM scratch
COPY --from=libs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app /aws-s3-proxy
ENTRYPOINT ["/aws-s3-proxy"]

ARG BUILD_VERSION
ARG BUILD_DATE
ARG BUILD_REF
LABEL maintainer="Patrick Domack (patrickdk@patrickdk.com)" \
  Description="aws s3 proxy." \
  ForkedFrom="" \
  org.label-schema.schema-version="1.0" \
  org.label-schema.build-date="${BUILD_DATE}" \
  org.label-schema.name="aws-s3-proxy" \
  org.label-schema.description="AWS S3 proxy with indexing" \
  org.label-schema.url="https://github.com/patrickdk77/aws-s3-proxy" \
  org.label-schema.usage="https://github.com/patrickdk77/aws-s3-proxy/tree/master/README.md" \
  org.label-schema.vcs-url="https://github.com/patrickdk77/aws-s3-proxy" \
  org.label-schema.vcs-ref="${BUILD_REF}" \
  org.label-schema.version="${BUILD_VERSION}"

