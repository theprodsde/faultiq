'use client'

import React from 'react'
import { useAuth } from '@/providers/auth-provider'
import { Skeleton } from '@/components/ui/Skeleton'
import { hasRoles } from '@/utils/roles'
import { useRouter } from 'next/navigation'

export default function Protected({ children, roles }: { children: React.ReactNode; roles?: string[] }) {
  const { isInitialized, isAuthenticated, user } = useAuth()
  const router = useRouter()

  if (!isInitialized) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Skeleton className="w-64 h-8 mb-4" />
      </div>
    )
  }

  if (!isAuthenticated) {
    // Redirect to login page when unauthenticated
    router.push('/login')
    return null
  }

  if (roles && !hasRoles(user, roles)) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <h2 className="text-xl font-semibold">Access Denied</h2>
          <p className="text-sm text-slate-600">You don’t have permission to view this page.</p>
        </div>
      </div>
    )
  }

  return <>{children}</>
}
