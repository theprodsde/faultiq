FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY services/k8s-connector/go.mod services/k8s-connector/go.sum* ./
RUN go mod download
COPY services/k8s-connector/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o k8s-connector .

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/k8s-connector .
USER appuser
EXPOSE 8093
CMD ["./k8s-connector"]
