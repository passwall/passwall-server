FROM golang:1.13-alpine AS builder
WORKDIR /app
RUN apk add gcc g++ --no-cache
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY ./cmd ./cmd
COPY ./internal ./internal
COPY ./model ./model
COPY ./public ./public

RUN mkdir store

RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-w -extldflags "-static"' ./cmd/passwall-server

FROM scratch

COPY --from=builder /app/passwall-server /app/passwall-server

COPY --from=builder /app/store /app/store

WORKDIR /app

ENTRYPOINT ["/app/passwall-server"]
