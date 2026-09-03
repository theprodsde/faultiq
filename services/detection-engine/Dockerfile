FROM golang:1.24 AS builder
RUN apt-get update && apt-get install -y --reinstall ca-certificates && update-ca-certificates
WORKDIR /src
COPY . .
WORKDIR /src/services/detection-engine
RUN CGO_ENABLED=0 GOOS=linux go build -o /detection-engine ./

FROM alpine:3.18
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /detection-engine /detection-engine
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
EXPOSE 8084
ENTRYPOINT ["/detection-engine"]
