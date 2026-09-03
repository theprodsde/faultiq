"use client"

import { useAuth } from "@/providers/auth-provider"
import Link from "next/link"
import { Button } from "@/components/ui/button"
import { Menu, X } from "lucide-react"
import { useState } from "react"

export function PublicHeader() {
  const { isAuthenticated, login, logout } = useAuth()
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)

  return (
    <header className="sticky top-0 z-50 border-b border-slate-200 bg-white/80 backdrop-blur dark:border-slate-800 dark:bg-slate-900/80">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between py-4">
          {/* Logo */}
          <Link href="/" className="flex items-center gap-2">
            <div className="text-2xl font-bold bg-gradient-to-r from-blue-600 to-cyan-600 bg-clip-text text-transparent">
              ⚡ FaultIQ
            </div>
          </Link>

          {/* Desktop Navigation */}
          <nav className="hidden md:flex items-center gap-8">
            <Link
              href="/"
              className="text-sm font-medium text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-50 transition-colors"
            >
              Home
            </Link>
            <Link
              href="/about"
              className="text-sm font-medium text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-50 transition-colors"
            >
              About
            </Link>
            <Link
              href="/services-status"
              className="text-sm font-medium text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-50 transition-colors"
            >
              Services Status
            </Link>
            <Link
              href="/contact"
              className="text-sm font-medium text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-50 transition-colors"
            >
              Contact
            </Link>
          </nav>

          {/* Auth Buttons */}
          <div className="hidden md:flex items-center gap-4">
            {isAuthenticated ? (
              <>
                <Link href="/dashboard">
                  <Button variant="outline" size="sm">
                    Dashboard
                  </Button>
                </Link>
                <Button onClick={logout} size="sm">
                  Sign Out
                </Button>
              </>
            ) : (
              <Button onClick={login} size="sm">
                Sign In
              </Button>
            )}
          </div>

          {/* Mobile Menu Button */}
          <button
            className="md:hidden"
            onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
          >
            {mobileMenuOpen ? (
              <X className="w-6 h-6" />
            ) : (
              <Menu className="w-6 h-6" />
            )}
          </button>
        </div>

        {/* Mobile Navigation */}
        {mobileMenuOpen && (
          <nav className="md:hidden pb-4 border-t border-slate-200 dark:border-slate-800">
            <Link
              href="/"
              className="block py-2 text-sm font-medium text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-50"
              onClick={() => setMobileMenuOpen(false)}
            >
              Home
            </Link>
            <Link
              href="/about"
              className="block py-2 text-sm font-medium text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-50"
              onClick={() => setMobileMenuOpen(false)}
            >
              About
            </Link>
            <Link
              href="/services-status"
              className="block py-2 text-sm font-medium text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-50"
              onClick={() => setMobileMenuOpen(false)}
            >
              Services Status
            </Link>
            <Link
              href="/contact"
              className="block py-2 text-sm font-medium text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-50"
              onClick={() => setMobileMenuOpen(false)}
            >
              Contact
            </Link>
            <div className="flex gap-2 mt-4 pt-4 border-t border-slate-200 dark:border-slate-800">
              {isAuthenticated ? (
                <>
                  <Link href="/dashboard" className="flex-1">
                    <Button variant="outline" size="sm" className="w-full">
                      Dashboard
                    </Button>
                  </Link>
                  <Button
                    onClick={() => {
                      logout()
                      setMobileMenuOpen(false)
                    }}
                    size="sm"
                    className="flex-1"
                  >
                    Sign Out
                  </Button>
                </>
              ) : (
                <Button onClick={login} size="sm" className="w-full">
                  Sign In
                </Button>
              )}
            </div>
          </nav>
        )}
      </div>
    </header>
  )
}
