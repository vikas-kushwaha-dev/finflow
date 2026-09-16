FROM golang:1.26-alpine AS build

WORKDIR /src/services/gateway-service

COPY services/gateway-service/go.mod services/gateway-service/go.sum ./
RUN go mod download

COPY services/gateway-service ./

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.22

RUN addgroup -S -g 10001 finflow && adduser -S -D -H -u 10001 -G finflow finflow

WORKDIR /app
COPY --from=build /out/api /app/api

USER 10001:10001

CMD ["/app/api"]
