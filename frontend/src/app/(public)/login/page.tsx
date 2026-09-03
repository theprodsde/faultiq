"use client"

import { useAuth } from "@/providers/auth-provider"
import { useRouter } from "next/navigation"
import { useEffect, useState } from "react"
import { motion } from "framer-motion"
import { Button } from "@/components/ui/button"
import Link from "next/link"
import { Loader } from "lucide-react"

export default function LoginPage() {
  const { isInitialized, isAuthenticated, authenticateWithToken, login } = useAuth()
  const router = useRouter()
  const [isLoading, setIsLoading] = useState(false)

  function getNextParam() {
    try {
      if (typeof window === 'undefined') return null
      const sp = new URLSearchParams(window.location.search)
      return sp.get('next')
    } catch (e) {
      return null
    }
  }

  useEffect(() => {
    if (isInitialized && isAuthenticated) {
      const next = getNextParam()
      if (next) router.push(next)
      else router.push('/dashboard')
    }
  }, [isInitialized, isAuthenticated, router])

  // Use Authorization Code Flow + PKCE via Keycloak redirect (recommended)
  const [error, setError] = useState<string | null>(null)

  const handleSSOLogin = (e?: React.FormEvent) => {
    if (e) e.preventDefault()
    setIsLoading(true)
    try {
      login()
    } catch (e) {
      setError('SSO login failed')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-900 via-blue-900 to-slate-900 dark:from-slate-950 dark:via-blue-950 dark:to-slate-950">
      {/* Background Grid (non-interactive) */}
      <div className="absolute inset-0 bg-grid-white/5 pointer-events-none" />

      <div className="relative min-h-screen flex items-center justify-center px-4 sm:px-6 lg:px-8">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8 }}
          className="w-full max-w-md"
        >
          {/* Card */}
          <div className="bg-white dark:bg-slate-900 rounded-2xl shadow-2xl border border-slate-200 dark:border-slate-800 p-8 md:p-12">
            {/* Logo */}
            <div className="text-center mb-8">
              <div className="text-5xl mb-4">⚡</div>
              <h1 className="text-3xl font-bold bg-gradient-to-r from-blue-600 to-cyan-600 bg-clip-text text-transparent mb-2">
                FaultIQ
              </h1>
              <p className="text-slate-600 dark:text-slate-400">
                Enterprise API Intelligence Platform
              </p>
            </div>

            {/* Authentication Status */}
            {!isInitialized ? (
              <motion.div
                animate={{ opacity: [0.5, 1, 0.5] }}
                transition={{ duration: 2, repeat: Infinity }}
                className="text-center py-8"
              >
                <Loader className="w-8 h-8 animate-spin text-blue-600 mx-auto mb-3" />
                <p className="text-slate-600 dark:text-slate-400">
                  Initializing authentication...
                </p>
              </motion.div>
            ) : isAuthenticated ? (
              <motion.div
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                className="text-center py-8"
              >
                <div className="text-4xl mb-3">✓</div>
                <p className="text-slate-600 dark:text-slate-400 mb-4">
                  Redirecting to dashboard...
                </p>
                <Loader className="w-6 h-6 animate-spin text-blue-600 mx-auto" />
              </motion.div>
            ) : (
              <>
                {/* Login Description */}
                <p className="text-center text-slate-600 dark:text-slate-400 mb-8">
                  Sign in with your Account or contact your administrator for access.
                </p>

                <div className="space-y-4 mb-6">
                  {error && <div className="text-sm text-red-600">{error}</div>}
                  <Button onClick={handleSSOLogin} size="lg" className="w-full" disabled={isLoading}>
                    {isLoading ? <><Loader className="w-4 h-4 animate-spin mr-2"/>Signing in...</> : 'Sign in with Single Sign-On'}
                  </Button>
                </div>

                {/* Demo Credentials - For Testing */}
                <div className="mt-8 pt-6 border-t border-slate-200 dark:border-slate-800">
                  <p className="text-xs font-semibold text-slate-500 dark:text-slate-400 mb-3 uppercase tracking-wider">
                    Demo Credentials
                  </p>
                  <div className="space-y-2 text-xs">
                    <div className="bg-slate-50 dark:bg-slate-800 p-3 rounded-lg">
                      <p className="font-medium text-slate-900 dark:text-slate-100 mb-1">🏢 acme-corp Admin</p>
                      <p className="text-slate-600 dark:text-slate-300">User: <code className="text-blue-600 dark:text-blue-400">org-admin</code></p>
                      <p className="text-slate-600 dark:text-slate-300">Pass: <code className="text-blue-600 dark:text-blue-400">orgadminpass</code></p>
                    </div>
                    <div className="bg-slate-50 dark:bg-slate-800 p-3 rounded-lg">
                      <p className="font-medium text-slate-900 dark:text-slate-100 mb-1">🏢 zen-inc Admin</p>
                      <p className="text-slate-600 dark:text-slate-300">User: <code className="text-blue-600 dark:text-blue-400">zen-admin</code></p>
                      <p className="text-slate-600 dark:text-slate-300">Pass: <code className="text-blue-600 dark:text-blue-400">zenadminpass</code></p>
                    </div>
                    <div className="bg-slate-50 dark:bg-slate-800 p-3 rounded-lg">
                      <p className="font-medium text-slate-900 dark:text-slate-100 mb-1">👤 Super Admin</p>
                      <p className="text-slate-600 dark:text-slate-300">User: <code className="text-blue-600 dark:text-blue-400">super</code></p>
                      <p className="text-slate-600 dark:text-slate-300">Pass: <code className="text-blue-600 dark:text-blue-400">superpass</code></p>
                    </div>
                  </div>
                </div>

                {/* Back to Home */}
                <div className="text-center mt-8 pt-6 border-t border-slate-200 dark:border-slate-800">
                  <p className="text-slate-600 dark:text-slate-400 text-sm">
                    Don't have an account?{" "}
                    <Link href="/" className="text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 font-medium">
                      Learn more
                    </Link>
                  </p>
                  <Link
                    href="/"
                    className="text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-50 text-sm mt-2 inline-block"
                  >
                    ← Back to Home
                  </Link>
                </div>
              </>
            )}
          </div>

          {/* Footer */}
          <div className="text-center mt-8 text-slate-400 dark:text-slate-500 text-sm">
            <p>
              <Link href="/about" className="hover:text-slate-300">
                About
              </Link>
              {" "} • {" "}
              <Link href="/contact" className="hover:text-slate-300">
                Contact
              </Link>
              {" "} • {" "}
              <Link href="/services-status" className="hover:text-slate-300">
                Status
              </Link>
            </p>
          </div>
        </motion.div>
      </div>
    </div>
  )
}
