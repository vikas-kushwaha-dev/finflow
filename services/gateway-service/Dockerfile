FROM golang:1.26-alpine AS build

WORKDIR /src/services/gateway-service

COPY services/gateway-service/go.mod services/gateway-service/go.sum ./
RUN go mod download

COPY services/gateway-service ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.22

RUN addgroup -S finflow && adduser -S finflow -G finflow

WORKDIR /app
COPY --from=build /out/api /app/api

USER finflow

CMD ["/app/api"]
