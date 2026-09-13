FROM golang:1.26-alpine AS build

WORKDIR /src/services/ledger-service

COPY services/ledger-service/go.mod services/ledger-service/go.sum ./
RUN go mod download

COPY services/ledger-service ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/payment-consumer ./cmd/payment-consumer

FROM alpine:3.22

RUN addgroup -S finflow && adduser -S finflow -G finflow

WORKDIR /app

COPY --from=build /out/payment-consumer /app/payment-consumer

USER finflow

CMD ["/app/payment-consumer"]

