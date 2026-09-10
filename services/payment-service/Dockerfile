FROM golang:1.26-alpine AS build

WORKDIR /src/services/payment-service

COPY services/payment-service/go.mod services/payment-service/go.sum ./
RUN go mod download

COPY services/payment-service ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM alpine:3.22

RUN addgroup -S finflow && adduser -S finflow -G finflow

WORKDIR /app

COPY --from=build /out/api /app/api
COPY --from=build /out/migrate /app/migrate
COPY migrations /app/migrations

USER finflow

EXPOSE 8080

CMD ["/app/api"]

