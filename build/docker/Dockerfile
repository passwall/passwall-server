FROM golang:1.24-alpine AS builder

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

FROM alpine:latest

WORKDIR /app

# Create placeholder files BEFORE volume mounts
# This prevents Docker from creating them as directories
RUN touch /app/config.yml && \
    touch /app/passwall-server.log

ENTRYPOINT ["/app/passwall-server"]

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder /app/passwall-server /app/passwall-server
