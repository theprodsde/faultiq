'use client'

import React from 'react'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useAuth } from '@/providers/auth-provider'

const NAV_ITEMS = [
  { href: '/dashboard', label: 'Dashboard', icon: '📊' },
  { href: '/dashboard/demo', label: 'Live Demo', icon: '🎯' },
  { href: '/dashboard/graph', label: 'Service Graph', icon: '🔗' },
  { href: '/dashboard/incidents', label: 'Incidents', icon: '🚨' },
  { href: '/dashboard/deployments', label: 'Deployments', icon: '🚀' },
  { href: '/dashboard/projects', label: 'Projects', icon: '📁' },
  { href: '/dashboard/reports', label: 'Reports', icon: '📈' },
]

const BOTTOM_ITEMS = [
  { href: '/dashboard/settings', label: 'Settings', icon: '⚙️' },
]

export default function Sidebar() {
  const { isAuthenticated } = useAuth()
  const pathname = usePathname()

  if (!isAuthenticated) return null

  const isActive = (href: string) =>
    href === '/dashboard' ? pathname === '/dashboard' : pathname.startsWith(href)

  return (
    <aside className="hidden md:block w-64 border-r border-slate-200 dark:border-slate-800/50 bg-slate-50 dark:bg-slate-950">
      <div className="h-full sticky top-0 flex flex-col p-4">
        <nav className="space-y-1 flex-1">
          {NAV_ITEMS.map(({ href, label, icon }) => (
            <Link
              key={href}
              href={href}
              className={`block px-3 py-2 rounded-md text-sm font-medium transition-colors ${
                isActive(href)
                  ? 'bg-blue-50 dark:bg-blue-950 text-blue-700 dark:text-blue-300'
                  : 'text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-900'
              }`}
            >
              {icon} {label}
            </Link>
          ))}
        </nav>
        <nav className="space-y-1 border-t border-slate-200 dark:border-slate-800 pt-2 mt-2">
          {BOTTOM_ITEMS.map(({ href, label, icon }) => (
            <Link
              key={href}
              href={href}
              className={`block px-3 py-2 rounded-md text-sm font-medium transition-colors ${
                isActive(href)
                  ? 'bg-blue-50 dark:bg-blue-950 text-blue-700 dark:text-blue-300'
                  : 'text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-900'
              }`}
            >
              {icon} {label}
            </Link>
          ))}
        </nav>
      </div>
    </aside>
  )
}
