FROM golang:1.24 AS builder
RUN apt-get update && apt-get install -y --reinstall ca-certificates && update-ca-certificates
WORKDIR /src
COPY . .
WORKDIR /src/services/code-indexer
RUN CGO_ENABLED=0 GOOS=linux go build -o /code-indexer .

FROM alpine:3.18
# git is required for clone/fetch operations
RUN apk add --no-cache ca-certificates git openssh-client curl
COPY --from=builder /code-indexer /code-indexer
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
EXPOSE 8092
ENTRYPOINT ["/code-indexer"]
