FROM golang:1.24.13 AS builder
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY services/k8s-connector/go.mod services/k8s-connector/go.sum* ./
ENV GOPROXY=https://proxy.golang.org,direct
ENV GOFLAGS=-mod=mod
# GOSUMDB checks a public transparency log (sum.golang.org). Behind a
# TLS-intercepting proxy that endpoint is often dropped, so skip that
# supplementary lookup. Module downloads still go over verified TLS via
# GOPROXY and are checked against committed go.sum where present.
ENV GOSUMDB=off
ENV GOTOOLCHAIN=local
# Trust optional corporate CA certs (for TLS-intercepting proxies like Zscaler).
# Empty/absent on normal machines -> no effect, standard TLS verification applies.
# Generate with ./scripts/generate-corporate-ca.sh (see scripts/certs/README.md).
COPY scripts/certs/ /usr/local/share/ca-certificates/faultiq-corp/
RUN if ls /usr/local/share/ca-certificates/faultiq-corp/*.crt >/dev/null 2>&1; then update-ca-certificates; fi
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
