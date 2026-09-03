# Frontend Implementation Guide

## Overview

The FaultIQ frontend is a modern React 19 + Next.js 16 application with TypeScript, Tailwind CSS, and comprehensive authentication/RBAC support.

## Architecture

### Directory Structure

```
frontend/src/
├── app/                          # Next.js app router
│   ├── layout.tsx               # Root layout with Providers
│   ├── page.tsx                 # Home/landing page
│   ├── dashboard/page.tsx       # Dashboard (protected)
│   ├── projects/page.tsx        # Projects list (NEW)
│   ├── incidents/page.tsx       # Incidents list (NEW)
│   ├── reports/page.tsx         # Reports & analytics (NEW)
│   └── settings/page.tsx        # User settings (NEW)
├── components/
│   ├── auth/
│   │   └── Protected.tsx         # Protected route wrapper
│   ├── navigation/
│   │   ├── Header.tsx           # Top navigation
│   │   ├── Sidebar.tsx          # Side navigation (desktop)
│   │   ├── Footer.tsx           # Footer (NEW)
│   │   ├── MobileMenu.tsx       # Mobile hamburger menu (NEW)
│   │   └── ThemeToggle.tsx      # Dark mode toggle
│   ├── layouts/
│   │   └── Layout.tsx           # Main layout wrapper
│   ├── ui/                      # Reusable UI components
│   │   ├── button.tsx
│   │   ├── card.tsx
│   │   ├── Modal.tsx
│   │   └── Skeleton.tsx
│   ├── charts/                  # Chart components (placeholder)
│   ├── dashboard/               # Dashboard-specific components
│   └── forms/                   # Form components
├── config/
│   └── index.ts                # App configuration
├── contexts/
│   └── tenant-context.tsx      # Tenant context provider
├── hooks/                       # Custom hooks
├── middleware/                  # Next.js middleware
├── providers/
│   ├── auth-provider.tsx       # Authentication context
│   └── index.tsx               # Providers wrapper
├── services/
│   └── api-client.ts           # Axios client with auth interceptor
├── store/                       # State management (Zustand)
├── themes/
│   └── design-tokens.ts        # Color, spacing tokens
├── types/
│   └── index.ts                # TypeScript type definitions
└── utils/
    └── roles.ts                # Role checking utilities
```

## Key Features

### 1. Authentication Flow

**Keycloak Integration**
- Realm: `faultiq`
- Client ID: `faultiq-ui`
- Flow: OAuth 2.0 Authorization Code with PKCE

**Auth Provider** (`providers/auth-provider.tsx`)
```typescript
- Manages Keycloak initialization
- Handles silent SSO check
- Auto-refreshes tokens every 60 seconds
- Provides auth context to all pages
```

### 2. RBAC (Role-Based Access Control)

**Protected Component**
```tsx
<Protected roles={['user', 'admin']}>
  <ProtectedContent />
</Protected>
```

**Role Utility Functions**
```typescript
hasRoles(user, required)  // Check roles
isAdmin(user)             // Shortcut for admin check
```

**Supported Roles**
- `admin` - Full access
- `user` - Standard user
- `analyst` - Can analyze incidents
- `viewer` - Read-only
- `signal:publisher` - Can publish signals

### 3. Responsive Design

**Breakpoints** (Tailwind CSS)
- `sm`: 640px
- `md`: 768px
- `lg`: 1024px
- `xl`: 1280px

**Mobile-First Approach**
- Mobile: Single column, hamburger menu
- Tablet: Two columns, compact sidebar
- Desktop: Full layout, always-visible sidebar

**Mobile Menu** (`components/navigation/MobileMenu.tsx`)
- Hamburger button (md:hidden)
- Dropdown navigation
- Auto-closes on link click
- Smooth animations

### 4. Responsive Pages

| Page | Path | Mobile | Tablet | Desktop | Protected |
|------|------|--------|--------|---------|-----------|
| Landing | `/` | ✅ | ✅ | ✅ | No |
| Dashboard | `/dashboard` | ✅ | ✅ | ✅ | Yes (user) |
| Projects | `/projects` | ✅ | ✅ | ✅ | Yes (user) |
| Incidents | `/incidents` | ✅ | ✅ | ✅ | Yes (user) |
| Reports | `/reports` | ✅ | ✅ | ✅ | Yes (user) |
| Settings | `/settings` | ✅ | ✅ | ✅ | Yes (user) |

### 5. Footer Component (NEW)

**Features**
- Responsive grid (1 col mobile, 4 cols desktop)
- Quick links sections (Product, Docs, Support)
- Copyright and legal links
- Dark mode support
- Sticky to bottom

**Location**: `components/navigation/Footer.tsx`

### 6. Layout Structure

```
┌─────────────────────────────────┐
│          Header                 │ (Top navigation, logo, user info)
├────────────┬────────────────────┤
│  Sidebar   │   Main Content     │ (Sidebar hidden on mobile)
│ (Desktop)  │                    │
│            │                    │
├────────────┴────────────────────┤
│          Footer                 │ (Bottom, always visible)
└─────────────────────────────────┘
```

## Component Usage

### Protected Page Example

```tsx
'use client'

import Protected from '@/components/auth/Protected'
import { useAuth } from '@/providers/auth-provider'
import { TenantProvider } from '@/contexts/tenant-context'

function PageContent() {
  const { user } = useAuth()
  return <div>Hello {user?.username}</div>
}

export default function Page() {
  return (
    <Protected roles={['user']}>
      <TenantProvider>
        <PageContent />
      </TenantProvider>
    </Protected>
  )
}
```

### Using the API Client

```tsx
import api from '@/services/api-client'

// GET request
const { data } = await api.get('/projects')

// POST request
await api.post('/projects', {
  name: 'My Project',
  slug: 'my-project'
})

// Errors automatically handled
// 401 triggers token refresh
```

### Using Role Checks

```tsx
import { useAuth } from '@/providers/auth-provider'
import { isAdmin, hasRoles } from '@/utils/roles'

function MyComponent() {
  const { user } = useAuth()

  return (
    <>
      {isAdmin(user) && <AdminPanel />}
      {hasRoles(user, ['analyst']) && <AnalysisTools />}
    </>
  )
}
```

## Environment Configuration

### Required Environment Variables

```bash
# .env.local
NEXT_PUBLIC_API_GATEWAY_URL=http://localhost:8080/api/v1
NEXT_PUBLIC_WEBSOCKET_URL=ws://localhost:8085
NEXT_PUBLIC_AUTH_URL=http://localhost:8180
NEXT_PUBLIC_KEYCLOAK_REALM=faultiq
NEXT_PUBLIC_KEYCLOAK_CLIENT_ID=faultiq-ui
NEXT_PUBLIC_KEYCLOAK_REDIRECT_URI=http://localhost:4001
NEXT_PUBLIC_APP_ENV=development
NEXT_PUBLIC_APP_NAME=FaultIQ
```

## Running Locally

### Development

```bash
cd frontend
npm install
npm run dev
# Opens http://localhost:4001
```

### Build & Production

```bash
npm run build
npm start
```

### Linting & Formatting

```bash
npm run lint           # Check
npm run lint:fix       # Fix
npm run format         # Format code
npm run type-check     # TypeScript check
```

## Styling

### Tailwind CSS

- **Config**: `tailwind.config.cjs`
- **CSS**: `app/globals.css`
- **Approach**: Utility-first

### Theme Support

- **Dark Mode**: `next-themes`
- **Toggle**: `components/navigation/ThemeToggle.tsx`
- **Persistence**: localStorage `theme` key

### Design Tokens

**File**: `themes/design-tokens.ts`

Colors:
- Primary: `#0F172A` (dark) to `#E0E7FF` (light)
- Accent: Orange/amber
- Neutral: Slate gray

Spacing: Standard Tailwind scale (4px base unit)

## Security

### Token Management

- Stored in: Keycloak session (not localStorage)
- Sent via: `Authorization: Bearer <token>` header
- Refresh: Every 60 seconds
- Fallback: Auto-logout on refresh failure

### API Security

- All requests require valid JWT
- Tenant scoping enforced server-side
- Rate limiting: 300 req/min per tenant
- CORS: Configured for same-origin

### XSS Protection

- Next.js CSP headers
- Input sanitization on all forms
- No dangerouslySetInnerHTML except trusted content

## Performance

### Optimizations

- Code splitting: Automatic via Next.js
- Image optimization: `next/image`
- Font optimization: `next/font`
- CSS: Tailwind purges unused styles

### Build Metrics

```
npm run build
# Shows bundle size breakdown
```

## Testing

### Unit Tests

```bash
npm run test              # Run all tests
npm run test:watch       # Watch mode
npm run test:coverage    # Coverage report
```

### E2E Tests (Future)

```bash
# Setup: cypress or playwright
```

## Deployment

### Docker

**Dockerfile**: `frontend/Dockerfile`

```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:20-alpine
WORKDIR /app
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/package*.json ./
RUN npm ci --production
EXPOSE 4001
CMD ["npm", "start"]
```

**Build & Run**
```bash
docker build -t faultiq-frontend:latest .
docker run -p 4001:4001 faultiq-frontend:latest
```

### Environment Variables in Production

Set these in your deployment platform:
```
NEXT_PUBLIC_API_GATEWAY_URL=https://api.faultiq.example.com
NEXT_PUBLIC_AUTH_URL=https://auth.faultiq.example.com
NEXT_PUBLIC_KEYCLOAK_REALM=faultiq
NEXT_PUBLIC_KEYCLOAK_CLIENT_ID=faultiq-ui
NEXT_PUBLIC_KEYCLOAK_REDIRECT_URI=https://app.faultiq.example.com
```

## Troubleshooting

### Login Not Working

1. Check Keycloak is running: `docker-compose ps`
2. Verify JWKS endpoint: `curl http://localhost:8180/realms/faultiq/protocol/openid-connect/certs`
3. Check browser console for errors
4. Ensure `silent-check-sso.html` exists in public folder

### API Calls Failing with 401

1. Check token expiry: Token may have expired
2. Verify API Gateway is running: `curl http://localhost:8080/health`
3. Check CORS headers in response
4. Verify JWT_SECRET or JWKS_URL in api-gateway environment

### Mobile Menu Not Showing

1. Check viewport width (should be < 768px)
2. Browser cache: Clear and reload
3. Verify MobileMenu component is in Header

### Dark Mode Not Working

1. Check `ThemeToggle` component is rendered
2. Verify `next-themes` provider in `providers/index.tsx`
3. Check browser localStorage for `theme` key

## Contributing

### Adding a New Page

1. Create file: `src/app/new-page/page.tsx`
2. Wrap with `<Protected>` if authentication required
3. Wrap content with `<Layout>` or similar
4. Add navigation link in Sidebar and MobileMenu
5. Test on mobile, tablet, desktop

### Adding a New Component

1. Create file: `src/components/<category>/<Component>.tsx`
2. Make it responsive with Tailwind breakpoints
3. Support dark mode
4. Add TypeScript types
5. Add to Storybook (future)

### Styling Guidelines

- Use Tailwind utilities (no custom CSS unless necessary)
- Support dark mode: `dark:` prefix
- Responsive: Mobile-first, then `sm:`, `md:`, `lg:`
- Spacing: Use Tailwind scale (p-2, m-4, gap-6)
- Colors: Use design tokens from `themes/design-tokens.ts`

---

## Checklist for New Features

- [ ] Create component/page
- [ ] Add TypeScript types
- [ ] Implement responsive design (mobile, tablet, desktop)
- [ ] Add dark mode support
- [ ] Add to navigation (Sidebar + MobileMenu)
- [ ] Add RBAC protection if needed
- [ ] Test authentication flow
- [ ] Test on actual mobile device
- [ ] Update documentation
- [ ] Submit for code review

---

**Last Updated**: May 19, 2026  
**Status**: ✅ Complete
