FROM golang:1.13-alpine AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY main.go .
COPY ./controller ./controller
COPY ./model ./model
COPY ./pkg ./pkg

RUN CGO_ENABLED=0 go build -a --installsuffix cgo --ldflags="-s" -o gpass


FROM scratch

COPY --from=builder /app .

ENTRYPOINT ["/gpass"]
