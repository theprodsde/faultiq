# Multi-stage build for rca-ranker
FROM golang:1.24.13 AS builder
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /build

COPY services/rca-ranker/go.mod .
COPY services/rca-ranker/go.sum .
ENV GOPROXY=direct
ENV GOSUMDB=off
ENV GOINSECURE=*
RUN git config --global http.sslVerify false
RUN go mod download

COPY services/rca-ranker/*.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o rca-ranker .

# Final stage - use Alpine for runtime
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /build/rca-ranker /app/rca-ranker

WORKDIR /app

RUN groupadd -r appgroup && useradd -r -g appgroup appuser
USER appuser

EXPOSE 8087

ENTRYPOINT ["/app/rca-ranker"]
