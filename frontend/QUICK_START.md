# Frontend Quick Start Guide

## 🚀 30-Second Setup

```bash
# 1. Install dependencies
npm install --legacy-peer-deps

# 2. Start Docker services (from project root)
docker-compose up keycloak -d

# 3. Run dev server
npm run dev

# 4. Open browser
# http://localhost:4001
```

## 📱 What You'll See

### Public Pages
- **Home** (`/`) - Landing page with features and CTA
- **About** (`/about`) - Company mission and values
- **Contact** (`/contact`) - Contact form
- **Status** (`/services-status`) - Backend services health

### Authentication
- **Login** (`/login`) - Keycloak login page
- Demo credentials: `admin` / `admin`

### Dashboard (Protected)
- **Dashboard** (`/dashboard`) - Stats and recent projects
- **Projects** (`/dashboard/projects`) - Project listing with environments
- **Settings** (`/dashboard/settings`) - User preferences

## 🛠️ Development Commands

```bash
# Start development server
npm run dev

# Type checking
npm run type-check

# Build for production
npm run build

# Run production build locally
npm start

# Linting
npm run lint

# Format code
npm run format

# Run tests
npm run test

# Watch mode for tests
npm run test:watch
```

## 🔍 File Structure Overview

```
src/
├── app/
│   ├── (public)/          # Public pages (home, about, etc.)
│   ├── (protected)/       # Protected dashboard pages
│   └── layout.tsx         # Root layout
├── components/
│   ├── ui/               # Reusable UI components
│   ├── layouts/          # Page layouts
│   └── ProtectedRoute.tsx # Auth guard
└── services/
    └── health-check.ts   # Backend health checking
```

## 🔐 Authentication

The app uses **Keycloak** for authentication:

1. Visit `/login` to authenticate
2. After login, you're redirected to `/dashboard`
3. Protected routes automatically redirect to login if not authenticated
4. Token is automatically refreshed every 60 seconds

## 🎨 Customization

### Change Colors
Edit `src/app/globals.css` - Tailwind configuration

### Update Branding
- Logo: `src/components/layouts/PublicHeader.tsx`
- Company name: `src/config/index.ts`

### Add New Pages
1. Create folder in `(public)` or `(protected)` group
2. Add `page.tsx` file
3. Import layout components

## 🐛 Troubleshooting

### "Cannot connect to Keycloak"
```bash
# Check if Keycloak is running
docker ps | grep keycloak

# Restart if needed
docker-compose up keycloak -d
```

### "Port 4001 already in use"
```bash
# Change port in next.config.ts or use:
npm run dev -- -p 3000
```

### Build errors
```bash
# Clean build cache
rm -rf .next
npm run build
```

## 📚 Key Technologies

- **Next.js 16** - React framework
- **TypeScript** - Type safety
- **Tailwind CSS** - Styling
- **Keycloak** - Authentication
- **Framer Motion** - Animations
- **Radix UI** - Accessible components

## 🎯 Important URLs

| Service | URL | Purpose |
|---------|-----|---------|
| Frontend | http://localhost:4001 | Main app |
| Keycloak | http://localhost:8081 | Authentication |
| Keycloak Admin | http://localhost:8081/admin | Realm management |
| API Gateway | http://localhost:8080 | Backend API |

## 📝 Environment Variables

Edit `.env.local`:
```env
# Keycloak
NEXT_PUBLIC_AUTH_URL=http://localhost:8081
NEXT_PUBLIC_KEYCLOAK_REALM=faultiq
NEXT_PUBLIC_KEYCLOAK_CLIENT_ID=faultiq-ui
NEXT_PUBLIC_KEYCLOAK_REDIRECT_URI=http://localhost:4001

# API
NEXT_PUBLIC_API_GATEWAY_URL=http://localhost:8080/api/v1
```

## 🧪 Testing the App

1. **Test public pages** - No login required
   - Visit `/`, `/about`, `/contact`, `/services-status`

2. **Test authentication** - Login required
   - Click "Sign In" on any public page
   - Use `admin` / `admin` credentials
   - Should redirect to `/dashboard`

3. **Test dashboard** - Protected routes
   - Verify you can view dashboard pages
   - Try logging out and accessing `/dashboard` directly
   - Should redirect to `/login`

## 📖 Additional Resources

- [Frontend Documentation](./FRONTEND_REDESIGN.md)
- [Next.js Docs](https://nextjs.org/docs)
- [Tailwind CSS](https://tailwindcss.com)
- [Keycloak Documentation](https://www.keycloak.org/documentation)

---

**Need help?** Check the console for error messages or review the component files directly.
