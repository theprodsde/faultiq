# Multi-stage build for rca-ranker
FROM golang:1.24.13 AS builder
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /build

COPY services/rca-ranker/go.mod .
COPY services/rca-ranker/go.sum .
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
