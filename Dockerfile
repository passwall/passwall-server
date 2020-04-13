FROM golang:1.13-alpine AS builder
WORKDIR /app
RUN apk add gcc g++ --no-cache
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY main.go .
COPY ./controller ./controller
COPY ./model ./model
COPY ./pkg ./pkg

RUN CGO_ENABLED=1 GOOS=linux go build -a --ldflags="-s" -o passwall-api

FROM alpine:3.11

COPY --from=builder /app/passwall-api /app/passwall-api

WORKDIR /app
ENTRYPOINT ["/app/passwall-api"]
