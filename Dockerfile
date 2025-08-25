# AWS S3 Proxy
# docker run -d -p 8080:80 -e AWS_REGION -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_S3_BUCKET patrickdk/s3-proxy

ARG BUILD_FROM_PREFIX

FROM ${BUILD_FROM_PREFIX}golang:alpine AS builder
COPY .gitignore /usr/bin/
RUN apk --no-cache add gcc musl-dev git
WORKDIR /go/src/
COPY . /go/src/
ARG BUILD_VERSION
ARG BUILD_DATE
ARG BUILD_REF
ARG BUILD_GOARCH
ARG BUILD_GOOS
RUN export GOPROXY=direct \
 && go mod download \
 && go mod verify \
 && CGO_ENABLED=0 go build \
    -ldflags '-s -w -X main.ver=${BUILD_VERSION} \
    -X main.commit=${BUILD_REF} -X main.date=${BUILD_DATE}' -o /health ./healthcheck \
 && CGO_ENABLED=0 go build \
    -ldflags '-s -w -X main.ver=${BUILD_VERSION} \
    -X main.commit=${BUILD_REF} -X main.date=${BUILD_DATE}' -o /app

FROM alpine AS libs
RUN apk --no-cache add ca-certificates

FROM scratch
COPY --from=libs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /health /health
COPY --from=builder /app /aws-s3-proxy
ENTRYPOINT ["/aws-s3-proxy"]
HEALTHCHECK --interval=5s --timeout=1s --start-period=5s --retries=3 CMD [ "/health" ]

ARG BUILD_VERSION
ARG BUILD_DATE
ARG BUILD_REF
LABEL maintainer="Patrick Domack (patrickdk@patrickdk.com)" \
  Description="aws s3 proxy." \
  org.label-schema.schema-version="1.0" \
  org.label-schema.build-date="${BUILD_DATE}" \
  org.label-schema.name="aws-s3-proxy" \
  org.label-schema.description="AWS S3 proxy with indexing" \
  org.label-schema.url="https://github.com/patrickdk77/aws-s3-proxy" \
  org.label-schema.usage="https://github.com/patrickdk77/aws-s3-proxy/tree/master/README.md" \
  org.label-schema.vcs-url="https://github.com/patrickdk77/aws-s3-proxy" \
  org.label-schema.vcs-ref="${BUILD_REF}" \
  org.label-schema.version="${BUILD_VERSION}" \
  org.opencontainers.url="https://github.com/patrickdk77/aws-s3-proxy" \
  org.opencontainers.documentation="https://github.com/patrickdk77/aws-s3-proxy/tree/master/README.md" \
  org.opencontainers.source="https://github.com/patrickdk77/aws-s3-proxy" \
  org.opencontainers.revision="${BUILD_REF}" \
  org.opencontainers.image.authors="Patrick Domack (patrickdk@patrickdk.com)" \
  org.opencontainers.image.created="${BUILD_DATE}" \
  org.opencontainers.image.title="aws-s3-proxy" \
  org.opencontainers.image.description="AWS S3 Proxy with indexing" \
  org.opencontainers.image.version="${BUILD_VERSION}" \
  org.opencontainers.image.licenses="MIT" \
  org.opencontainers.image.ref.name="aws-s3-proxy" \
  version="${BUILD_VERSION}"
