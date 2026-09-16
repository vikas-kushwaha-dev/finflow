FROM golang:1.26-alpine AS build

WORKDIR /src/services/payment-service

COPY services/payment-service/go.mod services/payment-service/go.sum ./
RUN go mod download

COPY services/payment-service ./

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/outbox-publisher ./cmd/outbox-publisher

FROM alpine:3.22

RUN addgroup -S -g 10001 finflow && adduser -S -D -H -u 10001 -G finflow finflow

WORKDIR /app

COPY --from=build /out/api /app/api
COPY --from=build /out/migrate /app/migrate
COPY --from=build /out/outbox-publisher /app/outbox-publisher
COPY migrations /app/migrations

USER 10001:10001

EXPOSE 8080

CMD ["/app/api"]
