// Auth Types
export interface User {
  id: string
  email: string
  username: string
  firstName: string
  lastName: string
  avatar?: string
  roles: string[]
  permissions: string[]
}

export interface Tenant {
  id: string
  name: string
  description?: string
  logo?: string
  slug: string
  createdAt: string
  updatedAt: string
  members: number
  plan: 'free' | 'pro' | 'enterprise'
}

export interface Project {
  id: string
  tenantId: string
  name: string
  description?: string
  slug: string
  environment: 'dev' | 'staging' | 'prod'
  createdAt: string
  updatedAt: string
  status: 'active' | 'archived' | 'paused'
}

export interface AuthToken {
  access_token: string
  refresh_token: string
  id_token: string
  expires_in: number
  token_type: string
}

export interface APIResponse<T> {
  success: boolean
  data?: T
  error?: string
  message?: string
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  pageSize: number
  hasMore: boolean
}

export type UserRole = 'super_admin' | 'tenant_admin' | 'project_admin' | 'developer' | 'viewer' | 'auditor'

export interface RolePermission {
  role: UserRole
  permissions: string[]
}

export interface Incident {
  id: string
  projectId: string
  title: string
  description: string
  severity: 'critical' | 'high' | 'medium' | 'low'
  status: 'open' | 'acknowledged' | 'resolved'
  createdAt: string
  updatedAt: string
  resolvedAt?: string
}

export interface Alert {
  id: string
  projectId: string
  title: string
  condition: string
  threshold: number
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface Metric {
  timestamp: string
  value: number
  label?: string
}

export interface ServiceHealth {
  name: string
  status: 'healthy' | 'degraded' | 'down'
  uptime: number
  responseTime: number
  lastChecked: string
}
