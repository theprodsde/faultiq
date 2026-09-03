"use client"

import { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import Keycloak from 'keycloak-js'
import { config } from '@/config'
import api, { setAuthToken, onUnauthorized } from '@/services/api-client'
import { store } from '@/store'
import { baseApi } from '@/store/api'
import type { User } from '@/types'

interface AuthContextType {
  isInitialized: boolean
  isAuthenticated: boolean
  user: User | null
  keycloak: Keycloak | null
  login: () => void
  logout: () => void
  token: string | null
  authenticateWithToken?: (accessToken: string, refreshToken?: string) => void
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isInitialized, setIsInitialized] = useState(false)
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const [user, setUser] = useState<User | null>(null)
  const [keycloak, setKeycloak] = useState<Keycloak | null>(null)
  const [token, setToken] = useState<string | null>(null)

  useEffect(() => {
    let refreshHandle: ReturnType<typeof setInterval> | null = null

    const initKeycloak = async () => {
        console.debug('[Auth] initKeycloak starting')
      try {
        // Validate Keycloak endpoint via .well-known to avoid blocking public pages
        let wellKnown: string
        if (typeof window !== 'undefined') {
          // Use proxied server-side endpoint to avoid CORS issues from browser
          wellKnown = `/api/keycloak-well-known`
        } else {
          const base = config.authUrl.replace(/\/$/, '')
          wellKnown = `${base}/realms/${config.keycloakRealm}/.well-known/openid-configuration`
        }
        try {
          // Retry transient network failures a few times (Keycloak may still be starting)
          const fetchWithRetries = async (url: string, attempts = 5, delayMs = 500) => {
            for (let i = 0; i < attempts; i++) {
              try {
                const resp = await fetch(url, { method: 'GET' })
                if (resp.ok) return resp
                console.warn('Keycloak well-known endpoint returned', resp.status, url)
              } catch (e) {
                if (i === attempts - 1) throw e
              }
              await new Promise((res) => setTimeout(res, delayMs * (i + 1)))
            }
            throw new Error('fetchWithRetries exhausted')
          }

          let r
          try {
            r = await fetchWithRetries(wellKnown, 5, 500)
          } catch (e) {
            console.warn('Failed to fetch Keycloak well-known config at', wellKnown, e)
            setIsInitialized(true)
            return
          }
        } catch (e) {
          console.warn('Failed to fetch Keycloak well-known config at', wellKnown, e)
          setIsInitialized(true)
          return
        }

        const kc = new Keycloak({
          url: config.authUrl,
          realm: config.keycloakRealm,
          clientId: config.keycloakClientId,
        })
        console.debug('[Auth] Keycloak instance created', { url: config.authUrl, realm: config.keycloakRealm, clientId: config.keycloakClientId })
        setKeycloak(kc)

        // Always attempt silent check-sso so an existing Keycloak session keeps the user logged in across reloads
        let authenticated = false
        try {
          console.debug('[Auth] calling kc.init check-sso')
          authenticated = await kc.init({
            onLoad: 'check-sso',
            checkLoginIframe: false,
            flow: 'standard',
            pkceMethod: 'S256',
          })
          console.debug('[Auth] kc.init check-sso result', { authenticated })
        } catch (e) {
          console.warn('Keycloak init (check-sso) failed', e)
        }

        if (authenticated && kc.token) {
          setIsAuthenticated(true)
          setToken(kc.token || null)
          setAuthToken(kc.token || null)
          if (typeof window !== 'undefined' && kc.token) (window as any).__faultiqToken = kc.token
        } else {
          // No active Keycloak session — try to restore token from localStorage if present
          try {
            if (typeof window !== 'undefined') {
              const stored = localStorage.getItem('faultiq.token')
              if (stored) {
                const isExpired = (tokenStr: string) => {
                  try {
                    const parts = tokenStr.split('.')
                    if (parts.length < 2) return true
                    const payload = JSON.parse(decodeURIComponent(escape(atob(parts[1].replace(/-/g, '+').replace(/_/g, '/')))))
                    const exp = payload.exp || 0
                    const now = Math.floor(Date.now() / 1000)
                    return exp > 0 ? exp <= now : false
                  } catch (e) {
                    return true
                  }
                }
                if (!isExpired(stored)) {
                  authenticateWithToken(stored)
                } else {
                  localStorage.removeItem('faultiq.token')
                }
              }
            }
          } catch (e) {
            // ignore
          }
        }

        // If authenticated, populate user info from tokenParsed
        if (authenticated && kc.tokenParsed) {
          const parsed: any = kc.tokenParsed
          const userInfo: any = {
            id: parsed.sub || '',
            email: parsed.email || '',
            username: parsed.preferred_username || '',
            firstName: parsed.given_name || '',
            lastName: parsed.family_name || '',
            roles: parsed.realm_access?.roles || parsed.roles || [],
            permissions: [],
            // include optional tenant/projects claims so TenantProvider can derive tenantId
            tenant: parsed.tenant || parsed.attributes?.tenant || null,
            projects: parsed.projects || parsed.attributes?.projects || null,
          }
          setUser(userInfo)
        }

        // Setup token refresh (only active when kc.token exists)
        refreshHandle = setInterval(() => {
          if (kc.token) {
            kc.updateToken(30)
              .then((updated) => {
                if (updated) {
                  setToken(kc.token || null)
                  setAuthToken(kc.token || null)
                  if (typeof window !== 'undefined') (window as any).__faultiqToken = kc.token
                  // persist refreshed token
                  try {
                    if (typeof window !== 'undefined' && kc.token) localStorage.setItem('faultiq.token', kc.token)
                  } catch (e) {}
                }
              })
              .catch(() => {
                console.warn('Token refresh failed')
                setAuthToken(null)
              })
          }
        }, 60000)

        // Wire API client to attempt re-auth on 401
        onUnauthorized(() => {
          try {
            kc.updateToken(5).then((refreshed) => {
              if (refreshed) {
                setToken(kc.token || null)
                setAuthToken(kc.token || null)
              } else {
                kc.logout()
              }
            })
          } catch (e) {
            kc.logout()
          }
        })

        setIsInitialized(true)
      } catch (error) {
        console.error('Keycloak initialization failed:', error)
        setIsInitialized(true)
      }
    }

    initKeycloak()

    return () => {
      if (refreshHandle) clearInterval(refreshHandle)
    }
  }, [])

  const login = () => {
    try {
      console.debug('[Auth] login() called')
      if (keycloak) {
        console.debug('[Auth] calling keycloak.login() on existing instance')
        keycloak.login()
        return
      }
      console.debug('[Auth] creating Keycloak instance for login fallback')
      const kc = new Keycloak({
        url: config.authUrl,
        realm: config.keycloakRealm,
        clientId: config.keycloakClientId,
      })
      setKeycloak(kc)
      console.debug('[Auth] calling kc.init with login-required (fallback)')
      kc.init({ onLoad: 'login-required', checkLoginIframe: false, flow: 'standard', pkceMethod: 'S256' }).catch((e) => {
        console.warn('Keycloak init for login failed', e)
      })
    } catch (e) {
      console.warn('Login failed, Keycloak unavailable', e)
    }
  }

  const authenticateWithToken = (accessToken: string, refreshToken?: string) => {
    if (!accessToken) return
    setToken(accessToken)
    setAuthToken(accessToken)
    setIsAuthenticated(true)
    // expose for RTK Query
    if (typeof window !== 'undefined') {
      ;(window as any).__faultiqToken = accessToken
      try {
        localStorage.setItem('faultiq.token', accessToken)
        if (refreshToken) localStorage.setItem('faultiq.refresh', refreshToken)
      } catch (e) {}
    }

    // Decode minimal user info from JWT
    try {
      const parts = accessToken.split('.')
      if (parts.length >= 2) {
        const payload = JSON.parse(decodeURIComponent(escape(atob(parts[1].replace(/-/g, '+').replace(/_/g, '/')))))
        const userInfo: any = {
          id: payload.sub || '',
          email: payload.email || '',
          username: payload.preferred_username || payload.username || '',
          firstName: payload.given_name || '',
          lastName: payload.family_name || '',
          roles: payload.realm_access?.roles || payload.roles || [],
          permissions: [],
          tenant: payload.tenant || payload.attributes?.tenant || null,
          projects: payload.projects || payload.attributes?.projects || null,
        }
        setUser(userInfo)
        // apply theme preference if present in token
        try {
          const themePref = payload.theme || payload.preferred_theme || null
          if (themePref && typeof window !== 'undefined' && (window as any).setFaultIQTheme) {
            ;(window as any).setFaultIQTheme(themePref)
          } else if (themePref && typeof window !== 'undefined') {
            localStorage.setItem('theme', themePref)
            document.documentElement.setAttribute('data-theme', themePref)
          }
        } catch (e) {}
      }
    } catch (e) {
      // ignore decode errors
    }
  }

  const logout = () => {
    try {
      // Attempt Keycloak logout if initialized
      if (keycloak) {
        try {
          keycloak.logout()
          // If Keycloak performs a redirect for logout, stop further local handling
          return
        } catch (e) {
          console.warn('Keycloak logout error', e)
        }
      }
    } catch (e) {
      console.warn('Logout failed, Keycloak unavailable', e)
    }

    // Clear local auth state regardless
    setToken(null)
    setAuthToken(null)
    setIsAuthenticated(false)
    setUser(null)
    try {
      if (typeof window !== 'undefined') {
        try {
          // Clear any stored tokens / theme preferences in storage
          try {
            localStorage.removeItem('faultiq.token')
            localStorage.removeItem('faultiq.refresh')
          } catch (e) {}
          try {
            sessionStorage.removeItem('faultiq.session')
          } catch (e) {}

          // Attempt to remove same-origin cookies
          try {
            document.cookie.split(';').forEach((c) => {
              const idx = c.indexOf('=')
              const name = idx > -1 ? c.substr(0, idx).trim() : c.trim()
              document.cookie = `${name}=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/`
            })
          } catch (e) {}

          // remove global token used by RTK Query
          try {
            delete (window as any).__faultiqToken
          } catch (e) {}

          // reset RTK Query cached state to remove any cached auth data
          try {
            store.dispatch(baseApi.util.resetApiState())
          } catch (e) {}
        } catch (e) {}

        // redirect to home
        window.location.href = '/'
      }
    } catch (e) {}
  }

  return (
    <AuthContext.Provider value={{ isInitialized, isAuthenticated, user, keycloak, login, logout, token, authenticateWithToken }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  return context
}
