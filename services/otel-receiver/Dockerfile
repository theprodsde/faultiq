FROM golang:1.24.13 AS builder
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY services/otel-receiver/go.mod ./
ENV GOPROXY=direct
ENV GOSUMDB=off
ENV GOINSECURE=*
RUN git config --global http.sslVerify false
RUN go mod download
COPY services/otel-receiver/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o otel-receiver .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates && rm -rf /var/lib/apt/lists/*
RUN groupadd -r appgroup && useradd -r -g appgroup appuser
WORKDIR /app
COPY --from=builder /app/otel-receiver .
USER appuser
EXPOSE 4317
CMD ["./otel-receiver"]
