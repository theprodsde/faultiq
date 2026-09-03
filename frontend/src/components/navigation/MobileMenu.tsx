'use client'

import React, { useState } from 'react'
import Link from 'next/link'
import { useAuth } from '@/providers/auth-provider'

export default function MobileMenu() {
  const [isOpen, setIsOpen] = useState(false)
  const { isAuthenticated } = useAuth()

  return (
    <>
      {/* Hamburger Button */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="md:hidden inline-flex items-center justify-center p-2 rounded-md text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100"
        aria-expanded={isOpen}
      >
        <svg
          className={`h-6 w-6 transition-transform ${isOpen ? 'rotate-90' : ''}`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M4 6h16M4 12h16M4 18h16"
          />
        </svg>
      </button>

      {/* Mobile Menu */}
      {isOpen && (
        <div className="md:hidden absolute top-16 left-0 right-0 bg-white dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800 shadow-lg z-40">
          <nav className="px-2 pt-2 pb-3 space-y-1">
            <Link href="/" onClick={() => setIsOpen(false)} className="block px-3 py-2 rounded-md text-base font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800">Home</Link>
            <Link href="/about" onClick={() => setIsOpen(false)} className="block px-3 py-2 rounded-md text-base font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800">About</Link>
            <Link href="/services-status" onClick={() => setIsOpen(false)} className="block px-3 py-2 rounded-md text-base font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800">Services Status</Link>
            <Link href="/contact" onClick={() => setIsOpen(false)} className="block px-3 py-2 rounded-md text-base font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800">Contact</Link>
            <hr className="my-2 border-slate-200 dark:border-slate-800" />
            {isAuthenticated && (
              <>
                <Link href="/dashboard" onClick={() => setIsOpen(false)} className="block px-3 py-2 rounded-md text-base font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800">📊 Dashboard</Link>
                <Link href="/projects" onClick={() => setIsOpen(false)} className="block px-3 py-2 rounded-md text-base font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800">📁 Projects</Link>
                <Link href="/incidents" onClick={() => setIsOpen(false)} className="block px-3 py-2 rounded-md text-base font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800">🚨 Incidents</Link>
                <Link href="/reports" onClick={() => setIsOpen(false)} className="block px-3 py-2 rounded-md text-base font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800">📈 Reports</Link>
                <hr className="my-2 border-slate-200 dark:border-slate-800" />
                <Link href="/settings" onClick={() => setIsOpen(false)} className="block px-3 py-2 rounded-md text-base font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800">⚙️ Settings</Link>
              </>
            )}
          </nav>
        </div>
      )}
    </>
  )
}
