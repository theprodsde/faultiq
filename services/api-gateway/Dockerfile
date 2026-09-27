FROM golang:1.24.13 AS gobuilder
RUN apt-get update && apt-get install -y --reinstall ca-certificates && update-ca-certificates
ENV GOPROXY=direct
ENV GOSUMDB=off
ENV GOINSECURE=*
RUN git config --global http.sslVerify false
WORKDIR /src
# Copy full repo into image for Go build
COPY . /src

# Build the binary from the service module
WORKDIR /src/services/api-gateway
RUN CGO_ENABLED=0 GOOS=linux go build -o /api-gateway ./

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=gobuilder /api-gateway /api-gateway
COPY --from=gobuilder /src/docs /docs
COPY /services/api-gateway/docker-entrypoint.sh /docker-entrypoint.sh
RUN sed -i 's/\r$//' /docker-entrypoint.sh && chmod +x /docker-entrypoint.sh
RUN groupadd -r appgroup && useradd -r -g appgroup appuser && \
    mkdir -p /usr/share/ui && chown appuser:appgroup /usr/share/ui
USER appuser
EXPOSE 8080
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["/api-gateway"]
