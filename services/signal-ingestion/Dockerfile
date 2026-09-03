FROM golang:1.24 AS builder
RUN apt-get update && apt-get install -y --reinstall ca-certificates && update-ca-certificates
WORKDIR /src
COPY . .
WORKDIR /src/services/signal-ingestion
RUN CGO_ENABLED=0 GOOS=linux go build -o /signal-ingestion ./

FROM alpine:3.18
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /signal-ingestion /signal-ingestion
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
EXPOSE 8085
ENTRYPOINT ["/signal-ingestion"]
