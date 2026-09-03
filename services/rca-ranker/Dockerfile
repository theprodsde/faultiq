# Multi-stage build for rca-ranker
FROM golang:1.23-alpine AS builder

WORKDIR /build

COPY services/rca-ranker/go.mod .
COPY services/rca-ranker/go.sum .
RUN go mod download

COPY services/rca-ranker/*.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o rca-ranker .

# Final stage - use Alpine for runtime
FROM alpine:latest

RUN apk add --no-cache ca-certificates curl

COPY --from=builder /build/rca-ranker /app/rca-ranker

WORKDIR /app

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

EXPOSE 8087

ENTRYPOINT ["/app/rca-ranker"]
