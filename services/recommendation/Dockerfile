FROM golang:1.24 AS builder
RUN apt-get update && apt-get install -y --reinstall ca-certificates && update-ca-certificates
WORKDIR /src
COPY . .
WORKDIR /src/services/recommendation
RUN CGO_ENABLED=0 GOOS=linux go build -o /recommendation .

FROM alpine:3.18
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /recommendation /recommendation
COPY --from=builder /src/services/recommendation/recommendations.json /recommendations.json
ENV RECOMMENDATIONS_PATH=/recommendations.json
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
EXPOSE 8090
ENTRYPOINT ["/recommendation"]
