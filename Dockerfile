FROM golang:1.13-alpine AS builder
WORKDIR /app
RUN apk add gcc g++ --no-cache
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY main.go .
COPY ./helper ./helper
COPY ./login ./login
COPY ./pkg ./pkg
COPY ./util ./util

RUN CGO_ENABLED=1 GOOS=linux go build -a --ldflags="-s" -o passwall-server

FROM alpine:3.11

COPY --from=builder /app/passwall-server /app/passwall-server

WORKDIR /app

RUN mkdir store

ENTRYPOINT ["/app/passwall-server"]
