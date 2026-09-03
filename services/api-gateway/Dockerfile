FROM golang:1.24 AS gobuilder
RUN apt-get update && apt-get install -y --reinstall ca-certificates && update-ca-certificates
WORKDIR /src
# Copy full repo into image for Go build
COPY . /src

# Build the binary from the service module
WORKDIR /src/services/api-gateway
RUN CGO_ENABLED=0 GOOS=linux go build -o /api-gateway ./

FROM alpine:3.18
RUN apk add --no-cache ca-certificates curl
COPY --from=gobuilder /api-gateway /api-gateway
COPY --from=gobuilder /src/docs /docs
COPY /services/api-gateway/docker-entrypoint.sh /docker-entrypoint.sh
RUN sed -i 's/\r$//' /docker-entrypoint.sh && chmod +x /docker-entrypoint.sh
RUN addgroup -S appgroup && adduser -S appuser -G appgroup && \
    mkdir -p /usr/share/ui && chown appuser:appgroup /usr/share/ui
USER appuser
EXPOSE 8080
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["/api-gateway"]
