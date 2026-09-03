"use client"

import { useEffect, useState } from 'react'
import { config } from '@/config'

type Health = {
  status: string
  details?: Record<string, any>
} | null

export default function ServiceHealth() {
  const [health, setHealth] = useState<Health>(null)
  const [loading, setLoading] = useState(true)

  const getHealthUrl = () => {
    const api = config.apiUrl || ''
    return api.replace(/\/api\/v1\/?$/, '') + '/health'
  }

  useEffect(() => {
    let mounted = true
    const fetchHealth = async () => {
      setLoading(true)
      try {
        const r = await fetch(getHealthUrl(), { method: 'GET' })
        if (!mounted) return
        if (!r.ok) {
          setHealth({ status: 'down' })
        } else {
          // try to parse JSON body, otherwise treat as OK text
          try {
            const json = await r.json()
            setHealth({ status: 'ok', details: json })
          } catch (_) {
            setHealth({ status: 'ok' })
          }
        }
      } catch (e) {
        if (!mounted) return
        setHealth({ status: 'down' })
      } finally {
        if (mounted) setLoading(false)
      }
    }

    fetchHealth()
    const t = setInterval(fetchHealth, 30000)
    return () => {
      mounted = false
      clearInterval(t)
    }
  }, [])

  const status = loading ? 'checking' : health?.status || 'unknown'

  return (
    <div className="hidden sm:flex items-center gap-3">
      <div className="text-sm text-slate-300">Services</div>
      <div
        className={`px-3 py-1 rounded-full text-sm font-medium ${
          status === 'ok' ? 'bg-emerald-600 text-white' : status === 'down' ? 'bg-rose-600 text-white' : 'bg-slate-600 text-white'
        }`}
      >
        {status === 'checking' ? 'Checking…' : status === 'ok' ? 'OK' : status === 'down' ? 'Down' : 'Unknown'}
      </div>
    </div>
  )
}
