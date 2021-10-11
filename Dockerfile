FROM golang:1.16-alpine AS builder

WORKDIR /app
RUN apk add gcc g++ ca-certificates --no-cache
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY ./cmd ./cmd
COPY ./internal ./internal
COPY ./model ./model
COPY ./pkg ./pkg
COPY ./public ./public

RUN mkdir store

RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-w -extldflags "-static"' ./cmd/passwall-server

RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-w -extldflags "-static"' ./cmd/passwall-cli

FROM alpine:latest

WORKDIR /app

# ENV PW_DIR=/app/store

ENTRYPOINT ["/app/passwall-server"]

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder /app/passwall-server /app/passwall-server

COPY --from=builder /app/passwall-cli /app/passwall-cli