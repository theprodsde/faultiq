# FaultIQ Frontend - Local Development Setup

## Prerequisites

- Node.js 20+ ([download](https://nodejs.org/))
- Docker & Docker Compose (optional, for full stack)
- Git

## Quick Start (Local Development)

### 1. Install Dependencies

```bash
cd frontend
npm install
```

### 2. Configure Environment Variables

Copy the example file and configure:

```bash
cp .env.example .env.local
```

Edit `.env.local` with your local values:

```env
# Development defaults (usually work as-is)
NEXT_PUBLIC_API_GATEWAY_URL=http://localhost:8080/api/v1
NEXT_PUBLIC_WEBSOCKET_URL=ws://localhost:8085
NEXT_PUBLIC_AUTH_URL=http://localhost:8180
NEXT_PUBLIC_KEYCLOAK_REALM=faultiq
NEXT_PUBLIC_KEYCLOAK_CLIENT_ID=faultiq-ui
NEXT_PUBLIC_KEYCLOAK_REDIRECT_URI=http://localhost:4001
```

### 3. Start the Development Server

```bash
npm run dev
```

The app will be available at `http://localhost:4001`

## Full Stack Setup with Docker Compose

If you want to run the entire FaultIQ stack:

### 1. From Project Root

```bash
# Start all services including frontend
docker-compose up -d
```

### 2. Access the Application

- **Frontend**: http://localhost:4001
- **API Gateway**: http://localhost:8080
- **Keycloak Admin**: http://localhost:8180
- **Neo4j Browser**: http://localhost:7474
- **Redpanda Console**: http://localhost:8090

### 3. Demo Credentials

- **Username**: admin@acme.com
- **Password**: admin

## Frontend-Only Docker Setup

To run just the frontend in Docker:

```bash
cd frontend

# Build the image
docker build -t faultiq-frontend:dev -f Dockerfile.dev .

# Run with docker-compose
docker-compose up
```

### Frontend Dockerfile for Development

Create `frontend/Dockerfile.dev`:

```dockerfile
FROM node:20-alpine
WORKDIR /app
ENV NODE_ENV=development
COPY package*.json ./
RUN npm install
COPY . .
EXPOSE 4001
CMD ["npm", "run", "dev"]
```

## Development Scripts

```bash
# Development server (with hot reload)
npm run dev

# Build for production
npm run build

# Start production build locally
npm start

# Type checking
npm run type-check

# Linting
npm run lint
npm run lint:fix

# Format code
npm run format

# Run tests
npm run test
npm run test:watch
npm run test:coverage
```

## Troubleshooting

### Issue: "Cannot connect to API Gateway"

**Solution**: Make sure the API Gateway is running:
```bash
# In another terminal from project root
docker-compose up api-gateway
```

### Issue: "Keycloak not responding"

**Solution**: Wait for Keycloak to fully start (30-60 seconds):
```bash
# Check logs
docker logs keycloak
```

### Issue: "Port 4001 already in use"

**Solution**: Use a different port:
```bash
npm run dev -- -p 3000
# App will be at http://localhost:3000
```

### Issue: "Module not found" errors

**Solution**: Clear cache and reinstall:
```bash
rm -rf node_modules package-lock.json .next
npm install
npm run dev
```

### Issue: Login redirects to blank page

**Solution**: Check Keycloak JWKS endpoint:
```bash
curl http://localhost:8180/realms/faultiq/protocol/openid-connect/certs
```

If this fails, Keycloak is not ready. Check logs:
```bash
docker logs keycloak
```

### Issue: Dark mode not working

**Solution**: Clear localStorage and browser cache:
```javascript
// In browser console
localStorage.clear()
location.reload()
```

## Project Structure

```
frontend/
├── src/
│   ├── app/              # Next.js pages and layouts
│   ├── components/       # React components
│   ├── services/         # API client & services
│   ├── contexts/         # React contexts (auth, tenant)
│   ├── providers/        # Context providers
│   ├── utils/           # Utility functions
│   ├── types/           # TypeScript types
│   ├── hooks/           # Custom hooks
│   ├── config/          # Configuration
│   └── themes/          # Design tokens
├── public/              # Static files
├── .env.example         # Environment template
├── next.config.ts       # Next.js configuration
├── tsconfig.json        # TypeScript configuration
├── tailwind.config.cjs  # Tailwind CSS configuration
└── package.json         # Dependencies
```

## Environment Variables Reference

### Required for Authentication

- `NEXT_PUBLIC_AUTH_URL` - Keycloak server URL
- `NEXT_PUBLIC_KEYCLOAK_REALM` - Keycloak realm name
- `NEXT_PUBLIC_KEYCLOAK_CLIENT_ID` - Keycloak client ID
- `NEXT_PUBLIC_KEYCLOAK_REDIRECT_URI` - Browser-accessible redirect URL

### API Configuration

- `NEXT_PUBLIC_API_GATEWAY_URL` - API Gateway base URL
- `NEXT_PUBLIC_WEBSOCKET_URL` - WebSocket server URL (for real-time features)

### Feature Flags

- `NEXT_PUBLIC_ENABLE_ANALYTICS` - Enable analytics (true/false)
- `NEXT_PUBLIC_ENABLE_DARK_MODE` - Enable dark mode toggle (true/false)
- `NEXT_PUBLIC_ENABLE_API_EXPLORER` - Show API explorer (true/false)
- `NEXT_PUBLIC_ENABLE_DOCUMENTATION` - Show docs link (true/false)
- `NEXT_PUBLIC_ENABLE_MONITORING` - Enable monitoring (true/false)

### App Configuration

- `NEXT_PUBLIC_APP_ENV` - Environment (development/staging/production)
- `NEXT_PUBLIC_APP_NAME` - Application name

## Common Tasks

### Add a New Page

1. Create file: `src/app/new-feature/page.tsx`
2. Wrap with `<Protected>` if authentication needed
3. Add link in Sidebar/MobileMenu
4. Test responsive design

### Add a New Component

1. Create file: `src/components/<category>/<Component>.tsx`
2. Export from component index if needed
3. Use Tailwind CSS for styling
4. Support dark mode with `dark:` prefix

### Run Production Build Locally

```bash
npm run build
npm start
# Visit http://localhost:3000
```

### Deploy to Production

```bash
# Build Docker image
docker build -t faultiq-frontend:latest .

# Push to registry
docker tag faultiq-frontend:latest my-registry/faultiq-frontend:latest
docker push my-registry/faultiq-frontend:latest

# Deploy via Docker/Kubernetes
```

## Performance Tips

- Use Next.js Image component for images
- Code-split with dynamic imports
- Monitor bundle size: `npm run build` shows breakdown
- Use React DevTools to check for unnecessary re-renders

## Testing

```bash
# Run all tests
npm run test

# Watch mode
npm run test:watch

# Coverage report
npm run test:coverage
```

## Debugging

### Enable Debug Logging

In browser console:
```javascript
localStorage.debug = '*'
location.reload()
```

### Check Authentication

```javascript
// In browser console after login
const token = sessionStorage.getItem('auth-token')
console.log(token)
```

### View Network Requests

Use Chrome DevTools Network tab to inspect API calls and check for:
- 401 errors → authentication issue
- 403 errors → authorization issue
- 500 errors → server issue

## Resources

- [Next.js Documentation](https://nextjs.org/docs)
- [React Documentation](https://react.dev)
- [Tailwind CSS](https://tailwindcss.com)
- [Keycloak Documentation](https://www.keycloak.org/documentation)

---

**Need Help?**

Check the main [README.md](../docs/README.md) or [FRONTEND_GUIDE.md](../docs/FRONTEND_GUIDE.md) for more information.
