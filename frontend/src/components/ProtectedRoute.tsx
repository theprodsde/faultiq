"use client"

import { useAuth } from "@/providers/auth-provider"
import { useRouter, usePathname } from "next/navigation"
import { useEffect } from "react"
import { Loader } from "lucide-react"

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isInitialized, isAuthenticated } = useAuth()
  const router = useRouter()

  const pathname = usePathname()

  useEffect(() => {
    if (isInitialized && !isAuthenticated) {
      const next = pathname || '/'
      router.push(`/login?next=${encodeURIComponent(next)}`)
    }
  }, [isInitialized, isAuthenticated, router, pathname])

  if (!isInitialized || !isAuthenticated) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-slate-50 dark:bg-slate-950">
        <Loader className="w-8 h-8 animate-spin text-blue-600" />
      </div>
    )
  }

  return <>{children}</>
}
