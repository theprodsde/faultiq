FROM golang:1.24 AS builder
RUN apt-get update && apt-get install -y --reinstall ca-certificates && update-ca-certificates
WORKDIR /src
COPY . .
WORKDIR /src/services/graph-manager
RUN CGO_ENABLED=0 GOOS=linux go build -o /graph-manager ./

FROM alpine:3.18
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /graph-manager /graph-manager
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
EXPOSE 8086
ENTRYPOINT ["/graph-manager"]
