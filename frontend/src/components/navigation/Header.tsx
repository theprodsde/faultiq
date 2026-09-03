'use client'

import React from 'react'
import { Button } from '@/components/ui/button'
import ThemeToggle from './ThemeToggle'
import MobileMenu from './MobileMenu'
import Link from 'next/link'
import { useRouter, usePathname } from 'next/navigation'
import { useAuth } from '@/providers/auth-provider'
import { appPath } from '@/lib/routes'

export default function Header() {
  const { isAuthenticated, logout, user } = useAuth()
  const router = useRouter()
  const pathname = usePathname()

  // Dashboard has its own header — hide the global one
  if (pathname?.startsWith('/dashboard')) return null

  return (
    <header className="sticky top-0 z-50 w-full border-b border-slate-200/50 dark:border-slate-800/50 bg-white/80 dark:bg-slate-900/80 backdrop-blur-md">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-3 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <MobileMenu />
          <div className="text-xl font-bold bg-gradient-to-r from-primary-700 to-accent-700 bg-clip-text text-transparent">⚡ FaultIQ</div>
          <div className="hidden md:flex items-center gap-6 ml-6">
            <nav className="hidden md:flex items-center gap-6">
              <Link href={appPath('/')} className="text-sm text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-50">Home</Link>
              <Link href={appPath('/about')} className="text-sm text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-50">About</Link>
              <Link href={appPath('/services-status')} className="text-sm text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-50">Services Status</Link>
              <Link href={appPath('/contact')} className="text-sm text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-50">Contact</Link>
            </nav>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <ThemeToggle />
          {isAuthenticated ? (
            <>
              <div className="hidden sm:block text-sm text-slate-700 dark:text-slate-300">Signed in as {user?.firstName ? `${user.firstName} ${user.lastName}` : user?.username || user?.email}</div>
              <Button variant="ghost" onClick={logout} className="text-sm">Sign out</Button>
            </>
          ) : (
            <>
              <Button onClick={() => router.push(`${appPath('/login')}`)} className="text-sm">Sign in</Button>
            </>
          )}
        </div>
      </div>
    </header>
  )
}
