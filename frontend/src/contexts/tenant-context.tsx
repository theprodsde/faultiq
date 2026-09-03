'use client'

import React, { createContext, useContext, useEffect, useState } from 'react'
import { useAuth } from '@/providers/auth-provider'

interface TenantContextValue {
  tenantId: string | null
  setTenantId: (t: string | null) => void
}

const TenantContext = createContext<TenantContextValue | undefined>(undefined)

export function TenantProvider({ children }: { children: React.ReactNode }) {
  const { user, keycloak } = useAuth()
  const [tenantId, setTenantId] = useState<string | null>(null)

  useEffect(() => {
    if (user) {
      // Primary: read from user.tenant (set by auth-provider from token claim)
      const tenant = (user as any).tenant
      if (tenant && typeof tenant === 'string') {
        setTenantId(tenant)
        return
      }
      // Secondary: try to read directly from Keycloak tokenParsed
      if (keycloak?.tokenParsed) {
        const parsed = keycloak.tokenParsed as any
        if (parsed.tenant) {
          setTenantId(parsed.tenant)
          return
        }
      }
      // Last resort: leave null — do NOT fall back to username
      setTenantId(null)
    }
  }, [user, keycloak])

  return <TenantContext.Provider value={{ tenantId, setTenantId }}>{children}</TenantContext.Provider>
}

export function useTenant() {
  const ctx = useContext(TenantContext)
  if (!ctx) throw new Error('useTenant must be used within TenantProvider')
  return ctx
}
