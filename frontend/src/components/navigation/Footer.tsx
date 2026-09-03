'use client'

import React from 'react'
import { useRouter, usePathname } from 'next/navigation'
import { useAuth } from '@/providers/auth-provider'

export default function Footer() {
  const currentYear = new Date().getFullYear()
  const { isAuthenticated } = useAuth()
  const router = useRouter()
  const pathname = usePathname()

  // Dashboard has its own layout — hide the global footer
  if (pathname?.startsWith('/dashboard')) return null

  function navOrLogin(href: string) {
    if (!isAuthenticated && (href.startsWith('/dashboard') || href.startsWith('/projects') || href.startsWith('/incidents'))) {
      router.push(`/login`)
    } else {
      router.push(href)
    }
  }

  return (
    <footer className="border-t border-slate-200/50 dark:border-slate-800/50 bg-slate-50/50 dark:bg-slate-950/50 mt-auto">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-8">
          {/* Brand Section */}
          <div className="flex flex-col">
            <div className="text-lg font-bold bg-gradient-to-r from-primary-700 to-accent-700 bg-clip-text text-transparent mb-2">
              ⚡ FaultIQ
            </div>
            <p className="text-xs text-slate-600 dark:text-slate-400">
              Enterprise fault detection and RCA platform
            </p>
          </div>

          {/* Product Links */}
          <div className="flex flex-col">
            <h4 className="font-semibold text-sm text-slate-900 dark:text-slate-100 mb-3">Product</h4>
            <ul className="space-y-2 text-sm">
              <li>
                <button onClick={() => navOrLogin('/dashboard')} className="text-left text-slate-600 dark:text-slate-400 hover:text-primary-700">Dashboard</button>
              </li>
              <li>
                <button onClick={() => navOrLogin('/projects')} className="text-left text-slate-600 dark:text-slate-400 hover:text-primary-700">Projects</button>
              </li>
              <li>
                <button onClick={() => navOrLogin('/incidents')} className="text-left text-slate-600 dark:text-slate-400 hover:text-primary-700">Incidents</button>
              </li>
            </ul>
          </div>

          {/* Documentation Links */}
          <div className="flex flex-col">
            <h4 className="font-semibold text-sm text-slate-900 dark:text-slate-100 mb-3">Documentation</h4>
            <ul className="space-y-2 text-sm">
              <li>
                <a href="docs" className="text-slate-600 dark:text-slate-400 hover:text-primary-700">API Reference</a>
              </li>
              <li>
                <a href="architecture" className="text-slate-600 dark:text-slate-400 hover:text-primary-700">Architecture</a>
              </li>
              <li>
                <a href="guides" className="text-slate-600 dark:text-slate-400 hover:text-primary-700">User Guides</a>
              </li>
            </ul>
          </div>

          {/* Support Links */}
          <div className="flex flex-col">
            <h4 className="font-semibold text-sm text-slate-900 dark:text-slate-100 mb-3">Support</h4>
            <ul className="space-y-2 text-sm">
              <li>
                <button onClick={() => router.push('/services-status')} className="text-left text-slate-600 dark:text-slate-400 hover:text-primary-700">Status Page</button>
              </li>
              <li>
                <button onClick={() => router.push('/contact')} className="text-left text-slate-600 dark:text-slate-400 hover:text-primary-700">Contact Us</button>
              </li>
              <li>
                <button onClick={() => router.push('/privacy')} className="text-left text-slate-600 dark:text-slate-400 hover:text-primary-700">Privacy</button>
              </li>
            </ul>
          </div>
        </div>

        {/* Bottom Section */}
        <div className="border-t border-slate-200/50 dark:border-slate-800/50 mt-8 pt-8">
          <div className="flex flex-col md:flex-row items-center justify-between gap-4">
            <p className="text-xs text-slate-600 dark:text-slate-400">© {currentYear} FaultIQ. All rights reserved. Patent pending.</p>
            <div className="flex gap-6">
              <a href="#terms" className="text-xs text-slate-600 dark:text-slate-400 hover:text-primary-700">Terms</a>
              <a href="#privacy" className="text-xs text-slate-600 dark:text-slate-400 hover:text-primary-700">Privacy</a>
              <a href="#cookies" className="text-xs text-slate-600 dark:text-slate-400 hover:text-primary-700">Cookies</a>
            </div>
          </div>
        </div>
      </div>
    </footer>
  )
}
