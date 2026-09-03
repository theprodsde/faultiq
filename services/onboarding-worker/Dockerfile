FROM golang:1.24 AS builder
RUN apt-get update && apt-get install -y --reinstall ca-certificates && update-ca-certificates
WORKDIR /src
# Copy only the onboarding-worker module so go.mod is in the build working dir
COPY ./services/onboarding-worker .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/onboarding-worker .

FROM alpine:3.18
COPY --from=builder /bin/onboarding-worker /bin/onboarding-worker
RUN apk add --no-cache ca-certificates
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
ENTRYPOINT ["/bin/onboarding-worker"]
