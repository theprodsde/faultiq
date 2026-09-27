FROM golang:1.24.13 AS builder
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
RUN apt-get update && apt-get install -y --reinstall ca-certificates && update-ca-certificates
ENV GOPROXY=direct
ENV GOSUMDB=off
ENV GOINSECURE=*
RUN git config --global http.sslVerify false
WORKDIR /src
# Copy only the onboarding-worker module so go.mod is in the build working dir
COPY ./services/onboarding-worker .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/onboarding-worker .

FROM debian:bookworm-slim
COPY --from=builder /bin/onboarding-worker /bin/onboarding-worker
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
RUN groupadd -r appgroup && useradd -r -g appgroup appuser
USER appuser
ENTRYPOINT ["/bin/onboarding-worker"]
