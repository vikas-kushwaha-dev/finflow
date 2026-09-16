FROM golang:1.26-alpine AS build

WORKDIR /src/services/ledger-service

COPY services/ledger-service/go.mod services/ledger-service/go.sum ./
RUN go mod download

COPY services/ledger-service ./

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/payment-consumer ./cmd/payment-consumer

FROM alpine:3.22

RUN addgroup -S -g 10001 finflow && adduser -S -D -H -u 10001 -G finflow finflow

WORKDIR /app

COPY --from=build /out/api /app/api
COPY --from=build /out/payment-consumer /app/payment-consumer

USER 10001:10001

CMD ["/app/api"]
