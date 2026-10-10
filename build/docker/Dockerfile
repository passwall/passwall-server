FROM golang:1.26-alpine@sha256:3082400e369fa24d5fc60bca20edab3f6d604e0c5a690ec66b295eff4dd87ade AS builder

WORKDIR /app
RUN apk add gcc g++ ca-certificates --no-cache

ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG COMMIT_ID=unknown
ARG BUILD_ID=unknown

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY ./cmd ./cmd
COPY ./internal ./internal
COPY ./pkg ./pkg
COPY ./public ./public

RUN mkdir store

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -a \
    -ldflags "-s -w -extldflags '-static' \
    -X github.com/passwall/passwall-server/pkg/buildvars.Version=${VERSION} \
    -X github.com/passwall/passwall-server/pkg/buildvars.BuildTime=${BUILD_TIME} \
    -X github.com/passwall/passwall-server/pkg/buildvars.CommitID=${COMMIT_ID} \
    -X github.com/passwall/passwall-server/pkg/buildvars.BuildID=${BUILD_ID}" \
    ./cmd/passwall-server

FROM alpine:3.22@sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8

WORKDIR /var/lib/passwall

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app/passwall-server /usr/local/bin/passwall-server

RUN addgroup -S -g 10001 passwall && \
    adduser -S -D -H -G passwall -u 10001 passwall && \
    mkdir -p /var/lib/passwall /var/log/passwall && \
    chown passwall:passwall /var/lib/passwall /var/log/passwall && \
    chmod 0755 /usr/local/bin/passwall-server

ENV PW_LOG_PATH=/var/log/passwall/passwall-server.log \
    PW_HTTP_LOG_PATH=/var/log/passwall/passwall-http.log

USER passwall:passwall

ENTRYPOINT ["/usr/local/bin/passwall-server"]
