FROM golang:1.24 AS builder
RUN apt-get update && apt-get install -y --reinstall ca-certificates && update-ca-certificates
WORKDIR /src
COPY . .
WORKDIR /src/services/demo-services
RUN CGO_ENABLED=0 GOOS=linux go build -o /demo-services .

FROM alpine:3.18
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /demo-services /demo-services
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
EXPOSE 8091
ENTRYPOINT ["/demo-services"]
