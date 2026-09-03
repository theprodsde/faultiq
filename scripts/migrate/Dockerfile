FROM golang:1.24 AS builder
RUN apt-get update && apt-get install -y --reinstall ca-certificates && update-ca-certificates
WORKDIR /src
# copy only the migrate module so go.mod is at /src/go.mod
COPY scripts/migrate /src
RUN CGO_ENABLED=0 GOOS=linux go build -o /db-migrate ./

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=builder /db-migrate /db-migrate
COPY scripts/migrations /migrations
ENTRYPOINT ["/db-migrate"]
