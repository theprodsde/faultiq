# Backend API Integration Guide

## Overview
This guide explains how to integrate the frontend with real backend APIs instead of mock data.

## Current State
- ✅ Authentication: Fully integrated with Keycloak
- ✅ UI/Components: Complete and responsive
- ⚠️ Data Fetching: Currently using mock data
- ⚠️ API Integration: Ready for implementation

## Integration Points

### 1. Projects & Environments
**File:** `src/app/(protected)/dashboard/projects/page.tsx`

**Current:** Mock data
```typescript
const projects: ProjectWithEnvs[] = [
  {
    id: "1",
    name: "E-Commerce Platform",
    // ... mock data
  },
]
```

**To Integrate:**
```typescript
import api from "@/services/api-client"

export default function ProjectsPage() {
  const [projects, setProjects] = useState<ProjectWithEnvs[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchProjects = async () => {
      try {
        const response = await api.get('/projects')
        setProjects(response.data)
      } catch (error) {
        console.error('Failed to fetch projects:', error)
      } finally {
        setLoading(false)
      }
    }

    fetchProjects()
  }, [])

  // ... rest of component
}
```

### 2. Dashboard Stats
**File:** `src/app/(protected)/dashboard/page.tsx`

**Current:** Hardcoded values
```typescript
const stats = [
  { label: "Active Projects", value: "8", ... },
  // ... more stats
]
```

**To Integrate:**
```typescript
import api from "@/services/api-client"
import { useQuery } from "@tanstack/react-query"

export default function DashboardPage() {
  const { user } = useAuth()
  
  // Fetch stats
  const { data: stats } = useQuery({
    queryKey: ['dashboard-stats'],
    queryFn: async () => {
      const response = await api.get('/dashboard/stats')
      return response.data
    },
  })

  // Fetch projects
  const { data: projects } = useQuery({
    queryKey: ['recent-projects'],
    queryFn: async () => {
      const response = await api.get('/projects?limit=3&sort=-createdAt')
      return response.data
    },
  })

  // ... rest of component
}
```

### 3. Services Health Status
**File:** `src/app/(public)/services-status/page.tsx`

**Current:** Mock service list
```typescript
const [services, setServices] = useState<Service[]>([
  { id: "api-gateway", name: "API Gateway", ... },
  // ... mock services
])
```

**To Integrate:**
```typescript
import { healthCheckService } from "@/services/health-check"

export default function ServicesStatus() {
  const [status, setStatus] = useState<HealthCheckResponse | null>(null)

  useEffect(() => {
    const fetchStatus = async () => {
      const response = await healthCheckService.getServiceHealthCached()
      setStatus(response)
    }

    fetchStatus()
    const interval = setInterval(fetchStatus, 30000) // Refresh every 30s
    return () => clearInterval(interval)
  }, [])

  if (!status) return <Loader />

  return (
    // ... render status.services
  )
}
```

## API Client Setup

The API client is already configured in `src/services/api-client.ts`:

```typescript
import axios from 'axios'
import { config } from '@/config'

const api = axios.create({
  baseURL: config.apiUrl,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Interceptor: Add auth token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('auth_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Interceptor: Handle 401 responses
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // Trigger re-authentication
      window.location.href = '/login'
    }
    return Promise.reject(error)
  },
)

export default api
```

## Recommended Integration Approach

### 1. React Query for Data Fetching
The project includes `@tanstack/react-query` for efficient data management:

```typescript
import { useQuery, useMutation } from '@tanstack/react-query'
import api from '@/services/api-client'

export function useProjects() {
  return useQuery({
    queryKey: ['projects'],
    queryFn: async () => {
      const response = await api.get('/projects')
      return response.data
    },
  })
}

export function useCreateProject() {
  return useMutation({
    mutationFn: async (data) => {
      const response = await api.post('/projects', data)
      return response.data
    },
  })
}
```

### 2. Create Custom Hooks
```typescript
// src/hooks/useProjects.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '@/services/api-client'

export function useProjects() {
  return useQuery({
    queryKey: ['projects'],
    queryFn: async () => {
      const response = await api.get('/projects')
      return response.data
    },
  })
}

export function useProject(id: string) {
  return useQuery({
    queryKey: ['project', id],
    queryFn: async () => {
      const response = await api.get(`/projects/${id}`)
      return response.data
    },
  })
}

export function useCreateProject() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: async (data) => {
      const response = await api.post('/projects', data)
      return response.data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
    },
  })
}
```

### 3. Use Hooks in Components
```typescript
import { useProjects } from '@/hooks/useProjects'

export function ProjectsList() {
  const { data: projects, isLoading, error } = useProjects()

  if (isLoading) return <Loader />
  if (error) return <Alert variant="destructive">{error.message}</Alert>

  return (
    <div>
      {projects?.map((project) => (
        <ProjectCard key={project.id} project={project} />
      ))}
    </div>
  )
}
```

## Expected API Endpoints

### Projects
```
GET  /projects                 # List all projects
GET  /projects/{id}            # Get single project
POST /projects                 # Create project
PUT  /projects/{id}            # Update project
DELETE /projects/{id}          # Delete project
```

### Environments
```
GET  /projects/{id}/environments           # List environments
POST /projects/{id}/environments           # Create environment
PUT  /projects/{id}/environments/{envId}   # Update environment
DELETE /projects/{id}/environments/{envId} # Delete environment
```

### Dashboard
```
GET  /dashboard/stats          # Dashboard statistics
GET  /dashboard/incidents      # Recent incidents
```

### Health
```
GET  /health                   # Service health check (public)
```

## Error Handling

```typescript
import { Alert, AlertTitle, AlertDescription } from '@/components/ui/alert'

export function useProjects() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['projects'],
    queryFn: async () => {
      try {
        const response = await api.get('/projects')
        return response.data
      } catch (error) {
        if (error.response?.status === 401) {
          throw new Error('Unauthorized')
        }
        if (error.response?.status === 403) {
          throw new Error('You do not have permission to view projects')
        }
        throw error
      }
    },
  })

  return { data, isLoading, error }
}

// In component:
if (error) {
  return (
    <Alert variant="destructive">
      <AlertTitle>Error</AlertTitle>
      <AlertDescription>{error.message}</AlertDescription>
    </Alert>
  )
}
```

## Loading States

```typescript
import { Skeleton } from '@/components/ui/Skeleton'

export function ProjectsList() {
  const { data: projects, isLoading } = useProjects()

  if (isLoading) {
    return (
      <div className="grid gap-4">
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-24" />
        ))}
      </div>
    )
  }

  return (
    // ... render projects
  )
}
```

## Pagination

```typescript
const [page, setPage] = useState(1)

const { data } = useQuery({
  queryKey: ['projects', page],
  queryFn: async () => {
    const response = await api.get('/projects', {
      params: { page, limit: 10 },
    })
    return response.data
  },
})
```

## Real-time Updates (WebSocket)

For real-time service status updates:

```typescript
import { useEffect, useState } from 'react'

export function useRealtimeStatus() {
  const [status, setStatus] = useState(null)

  useEffect(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${protocol}//localhost:8085/ws/status`)

    ws.onmessage = (event) => {
      setStatus(JSON.parse(event.data))
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
    }

    return () => ws.close()
  }, [])

  return status
}
```

## Testing Integration

```bash
# Mock API responses in development
npm run dev

# Use browser DevTools to inspect API calls
# Network tab shows all requests/responses

# Check authentication token
localStorage.getItem('auth_token')
```

## Deployment Considerations

1. **Environment Variables**
   ```env
   NEXT_PUBLIC_API_GATEWAY_URL=https://api.production.com
   ```

2. **CORS** - Ensure backend allows frontend origin

3. **Security**
   - Use HTTPS in production
   - Implement CSRF protection
   - Validate all user input

4. **Performance**
   - Cache API responses with React Query
   - Implement pagination for large datasets
   - Use request debouncing for search

## Next Steps

1. ✅ Keycloak authentication (done)
2. ⬜ Integrate projects API
3. ⬜ Integrate environments API
4. ⬜ Implement real-time WebSocket updates
5. ⬜ Add error handling and loading states
6. ⬜ Implement search and filtering
7. ⬜ Add pagination
8. ⬜ Create incident management pages
9. ⬜ Add notifications/alerts
10. ⬜ Performance optimization

---

**Ready to integrate?** Start with the Projects API and use the patterns above to add other endpoints.
