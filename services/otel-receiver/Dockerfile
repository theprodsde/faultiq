FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY services/otel-receiver/go.mod ./
RUN go mod download
COPY services/otel-receiver/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o otel-receiver .

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/otel-receiver .
USER appuser
EXPOSE 4317
CMD ["./otel-receiver"]
