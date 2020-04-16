FROM golang:1.13-alpine AS builder
WORKDIR /app
RUN apk add gcc g++ --no-cache
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY main.go .
COPY ./login ./login
COPY ./pkg ./pkg
COPY ./store ./store

RUN CGO_ENABLED=1 GOOS=linux go build -a --ldflags="-s" -o passwall-api

FROM alpine:3.11

COPY --from=builder /app/passwall-api /app/passwall-api

WORKDIR /app

RUN mkdir store

ENTRYPOINT ["/app/passwall-api"]
