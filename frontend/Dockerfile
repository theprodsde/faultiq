FROM node:20-slim AS builder
WORKDIR /app

# Optional build args (can be provided by docker-compose or build CLI)
ARG NEXT_PUBLIC_API_GATEWAY_URL
ARG NEXT_PUBLIC_WEBSOCKET_URL
ARG NEXT_PUBLIC_AUTH_URL
ARG NEXT_PUBLIC_KEYCLOAK_REALM
ARG NEXT_PUBLIC_KEYCLOAK_CLIENT_ID
ARG NEXT_PUBLIC_HOST_URL

# If provided, set build-time env vars so Next can bake them into the build
ENV NEXT_PUBLIC_API_GATEWAY_URL=$NEXT_PUBLIC_API_GATEWAY_URL
ENV NEXT_PUBLIC_WEBSOCKET_URL=$NEXT_PUBLIC_WEBSOCKET_URL
ENV NEXT_PUBLIC_AUTH_URL=$NEXT_PUBLIC_AUTH_URL
ENV NEXT_PUBLIC_KEYCLOAK_REALM=$NEXT_PUBLIC_KEYCLOAK_REALM
ENV NEXT_PUBLIC_KEYCLOAK_CLIENT_ID=$NEXT_PUBLIC_KEYCLOAK_CLIENT_ID
ENV NEXT_PUBLIC_HOST_URL=$NEXT_PUBLIC_HOST_URL

# Install dependencies and build
COPY package*.json ./
RUN npm install --legacy-peer-deps
COPY . .
RUN npm run build

FROM node:20-slim AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=4001

# Ensure basic network tooling (healthchecks) is available
RUN apt-get update && apt-get install -y --no-install-recommends \
	curl \
 && rm -rf /var/lib/apt/lists/*

# Copy built app
COPY --from=builder /app/package*.json ./
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public

USER node
EXPOSE 4001
# Use shell form so `$PORT` is expanded at runtime
CMD ["sh","-c","npx next start -p $PORT -H 0.0.0.0"]
