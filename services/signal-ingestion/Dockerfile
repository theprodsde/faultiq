FROM golang:1.24.13 AS builder
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
RUN apt-get update && apt-get install -y --reinstall ca-certificates && update-ca-certificates
ENV GOPROXY=direct
ENV GOSUMDB=off
ENV GOINSECURE=*
RUN git config --global http.sslVerify false
WORKDIR /src
COPY . .
WORKDIR /src/services/signal-ingestion
RUN CGO_ENABLED=0 GOOS=linux go build -o /signal-ingestion ./

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /signal-ingestion /signal-ingestion
RUN groupadd -r appgroup && useradd -r -g appgroup appuser
USER appuser
EXPOSE 8085
ENTRYPOINT ["/signal-ingestion"]
