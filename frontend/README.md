# FaultIQ Frontend

A modern, responsive web application for incident detection, analysis, and management built with Next.js, React, and TypeScript.

## Features

- ✅ **Modern UI/UX** - Professional design with gradient backgrounds, animations, and responsive layout
- 🔐 **OAuth 2.0/SSO** - Secure authentication via Keycloak with JWT tokens
- 📊 **Service Graphs** - Visualize service dependencies using Mermaid diagrams
- 🎯 **Role-Based Access** - Fine-grained RBAC with admin, user, analyst, and viewer roles
- 🌙 **Dark Mode** - Full dark mode support with theme persistence
- 📱 **Responsive Design** - Mobile-first design that works on all devices
- ⚡ **Real-time Updates** - WebSocket support for live incident notifications
- 🔄 **Auto Token Refresh** - Seamless authentication with automatic JWT refresh
- 📈 **Analytics Dashboard** - Comprehensive dashboards with incident metrics

## Quick Start

### Prerequisites
- Node.js 20+
- npm or yarn

### Local Development

1. **Install dependencies:**
```bash
npm install
```

2. **Configure environment:**
```bash
# Copy the example file
cp .env.example .env.local

# .env.local is already provided with localhost defaults
```

3. **Start development server:**
```bash
npm run dev
```

4. **Open in browser:**
Visit http://localhost:4001

### Docker Setup

**Frontend only:**
```bash
docker-compose up
```

**Full stack (includes backend services):**
```bash
cd ..
docker-compose up
```

## Available Scripts

```bash
# Development server with hot reload
npm run dev

# Build for production
npm run build

# Start production build
npm start

# Type checking
npm run type-check

# Linting
npm run lint
npm run lint:fix

# Code formatting
npm run format

# Run tests
npm run test
npm run test:watch
```

## Project Structure

```
src/
├── app/                 # Next.js pages (file-based routing)
│   ├── login/          # OAuth/SSO login page
│   ├── dashboard/      # Main dashboard with service graphs
│   ├── projects/       # Projects list and detail pages
│   ├── incidents/      # Incident management
│   ├── reports/        # Analytics and reports
│   └── settings/       # User and admin settings
├── components/         # React components
│   ├── auth/           # Authentication components
│   ├── charts/         # Data visualization (ServiceGraph)
│   ├── dashboard/      # Dashboard components
│   ├── forms/          # Form components
│   ├── layouts/        # Layout components (Header, Sidebar)
│   ├── navigation/     # Navigation components
│   └── ui/             # Reusable UI components
├── providers/          # React context providers
│   ├── auth-provider   # Keycloak integration & JWT handling
│   └── index           # Provider wrapper
├── services/           # API client and services
│   └── api-client      # Axios instance with auth interceptor
├── contexts/           # React contexts
│   └── tenant-context  # Multi-tenant support
├── hooks/              # Custom React hooks
├── types/              # TypeScript type definitions
├── utils/              # Utility functions (roles, formatting, etc.)
├── config/             # Configuration (env vars, constants)
├── lib/                # Library utilities
└── themes/             # Design tokens, colors, typography
```

## Authentication Flow

1. User visits `/login`
2. Clicks "Sign In with SSO"
3. Redirected to Keycloak OAuth endpoint
4. User authenticates at Keycloak (email/password or external provider)
5. Keycloak redirects back with authorization code
6. Frontend exchanges code for JWT token
7. Token stored in session/memory
8. Auto-refresh every 60 seconds with 30-second buffer
9. All API requests include `Authorization: Bearer <token>` header
10. 401 response triggers automatic re-authentication

## Role-Based Access Control

Frontend roles are provided by Keycloak in JWT claims (`realm_access.roles`):

- **admin** - Full system access, user management, audit logs
- **user** - Create/edit projects, view all dashboards
- **analyst** - Analyze incidents, view reports
- **viewer** - Read-only access to dashboards and reports
- **signal:publisher** - Publish signals to detection engine

Role checks use the `hasRoles()` utility function:

```tsx
import { hasRoles, isAdmin } from '@/utils/roles'

const canEdit = hasRoles(user.roles, ['admin', 'user'])
const isAdminUser = isAdmin(user)
```

Protected pages use the `<Protected>` component:

```tsx
<Protected roles={['admin']}>
  <AdminPanel />
</Protected>
```

## Configuration

### Environment Variables

See `.env.example` for all available configuration options:

- `NEXT_PUBLIC_API_GATEWAY_URL` - API backend URL
- `NEXT_PUBLIC_AUTH_URL` - Keycloak server URL
- `NEXT_PUBLIC_KEYCLOAK_*` - Keycloak realm/client configuration
- `NEXT_PUBLIC_APP_ENV` - Environment (development/staging/production)

### Themes

Tailwind CSS is configured with custom design tokens in `themes/design-tokens.ts`:

- Primary colors for buttons and highlights
- Accent colors for secondary elements
- Responsive breakpoints (mobile-first)
- Dark mode support with automatic detection

## API Integration

The frontend communicates with the FaultIQ backend services:

- **API Gateway** (port 8080) - RESTful API endpoint
- **Keycloak** (port 8180) - OAuth 2.0/OIDC provider
- **Signal Ingestion** (port 8085) - WebSocket for real-time updates

All API requests use the `api-client` service with automatic JWT injection:

```tsx
import { apiClient } from '@/services/api-client'

// Automatically includes Authorization header
const projects = await apiClient.get('/projects')
```

## Responsive Design

The frontend uses Tailwind CSS with mobile-first breakpoints:

- **Mobile** (< 640px) - Single column, hamburger menu
- **Tablet** (640px - 1024px) - Two columns, responsive grid
- **Desktop** (> 1024px) - Full layout, sidebar always visible

All pages test with:
```bash
# Chrome DevTools: Ctrl+Shift+M (Windows/Linux) or Cmd+Shift+M (Mac)
# Or resize browser window
```

## Dark Mode

Dark mode is available on all pages:

- Automatic detection of system preference
- Manual toggle in header
- Persistent selection in localStorage
- All components styled with `dark:` Tailwind prefix

## Troubleshooting

### Port 4001 Already in Use
```bash
npm run dev -- -p 3000  # Use port 3000 instead
```

### Cannot Connect to Backend
1. Check API Gateway is running: `docker-compose up api-gateway`
2. Verify `NEXT_PUBLIC_API_GATEWAY_URL` in `.env.local`
3. Check browser console for CORS errors

### Login Not Working
1. Verify Keycloak is running: `docker ps | grep keycloak`
2. Check Keycloak realm exists: http://localhost:8180
3. Review Keycloak logs: `docker logs keycloak`

### Module Not Found Errors
```bash
rm -rf node_modules package-lock.json
npm install
```

## Performance

- **Code Splitting** - Automatic with Next.js dynamic imports
- **Image Optimization** - Using Next.js Image component
- **Caching** - Browser caching + API response caching
- **Lazy Loading** - Mermaid diagrams load on-demand

Check bundle size:
```bash
npm run build
# Output shows component sizes
```

## Documentation

- [SETUP.md](./SETUP.md) - Detailed setup and development guide
- [../docs/FRONTEND_GUIDE.md](../docs/FRONTEND_GUIDE.md) - Frontend architecture and patterns
- [../docs/api.md](../docs/api.md) - API endpoint documentation
- [../docs/architecture.md](../docs/architecture.md) - System architecture overview

## Technology Stack

- **Framework** - Next.js 16.2.6
- **Runtime** - React 19.2.4
- **Language** - TypeScript 5.x
- **Styling** - Tailwind CSS 4.x
- **Animations** - Framer Motion 12.x
- **Auth** - Keycloak JS 26.2.4
- **HTTP Client** - Axios
- **Visualization** - Mermaid
- **Theme** - next-themes

## Contributing

1. Follow the existing code structure and naming conventions
2. Use TypeScript for type safety
3. Style components with Tailwind CSS
4. Support dark mode with `dark:` prefix
5. Test responsive design on multiple breakpoints
6. Include proper TypeScript types in all components

## License

Proprietary - FaultIQ Project
