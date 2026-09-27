FROM golang:1.24.13 AS builder
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY services/k8s-connector/go.mod services/k8s-connector/go.sum* ./
ENV GOPROXY=direct
ENV GOSUMDB=off
ENV GOINSECURE=*
RUN git config --global http.sslVerify false
RUN go mod download
COPY services/k8s-connector/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o k8s-connector .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates && rm -rf /var/lib/apt/lists/*
RUN groupadd -r appgroup && useradd -r -g appgroup appuser
WORKDIR /app
COPY --from=builder /app/k8s-connector .
USER appuser
EXPOSE 8093
CMD ["./k8s-connector"]
