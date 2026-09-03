"use client"

import { useAuth } from "@/providers/auth-provider"
import { useState } from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import {
  LayoutGrid,
  FolderOpen,
  AlertCircle,
  GitBranch,
  BarChart3,
  Settings,
  LogOut,
  Menu,
  X,
  Bell,
  Moon,
  Sun,
  ChevronRight,
  Zap,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { useTheme } from "next-themes"
import { cn } from "@/lib/utils"
import { ContextBar } from "@/components/ContextBar"

interface NavItem {
  href: string
  label: string
  icon: React.ReactNode
  badge?: string
}

const navItems: NavItem[] = [
  { href: "/dashboard",            label: "Overview",    icon: <LayoutGrid className="w-4 h-4" /> },
  { href: "/dashboard/projects",   label: "Projects",    icon: <FolderOpen  className="w-4 h-4" /> },
  { href: "/dashboard/onboarding", label: "New Project", icon: <Zap        className="w-4 h-4" /> },
  { href: "/dashboard/incidents",  label: "Incidents",   icon: <AlertCircle className="w-4 h-4" />, badge: "live" },
  { href: "/dashboard/graph",      label: "Service Graph", icon: <GitBranch className="w-4 h-4" /> },
  { href: "/dashboard/reports",    label: "Analytics",   icon: <BarChart3   className="w-4 h-4" /> },
  { href: "/dashboard/settings",   label: "Settings",    icon: <Settings    className="w-4 h-4" /> },
]

function NavLink({ item, onClick }: { item: NavItem; onClick?: () => void }) {
  const pathname = usePathname()
  const active =
    item.href === "/dashboard"
      ? pathname === "/dashboard"
      : pathname.startsWith(item.href)

  return (
    <Link
      href={item.href}
      onClick={onClick}
      className={cn(
        "flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-all",
        active
          ? "bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300"
          : "text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-100"
      )}
    >
      <span className={active ? "text-blue-600 dark:text-blue-400" : ""}>{item.icon}</span>
      <span className="flex-1">{item.label}</span>
      {item.badge && (
        <span className="text-[10px] font-bold uppercase px-1.5 py-0.5 rounded-full bg-red-100 text-red-600 dark:bg-red-900/40 dark:text-red-300">
          {item.badge}
        </span>
      )}
    </Link>
  )
}

export function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth()
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const { theme, setTheme } = useTheme()

  const initials = user?.firstName && user?.lastName
    ? `${user.firstName[0]}${user.lastName[0]}`.toUpperCase()
    : user?.email?.[0]?.toUpperCase() ?? "U"

  const pathname = usePathname()

  return (
    <div className="flex h-screen overflow-hidden bg-slate-50 dark:bg-slate-950">
      {/* ─── Sidebar (always static on desktop, slide-in on mobile) ──── */}
      <aside
        className={cn(
          "z-50 w-64 flex flex-col bg-white dark:bg-slate-900 border-r border-slate-200 dark:border-slate-800 transition-transform duration-200 shrink-0",
          // Mobile: fixed overlay
          "fixed inset-y-0 left-0",
          sidebarOpen ? "translate-x-0" : "-translate-x-full",
          // Desktop: static in flow, always visible
          "md:static md:translate-x-0 md:h-screen md:sticky md:top-0"
        )}
      >
        {/* Logo */}
        <div className="flex items-center justify-between h-16 px-5 border-b border-slate-200 dark:border-slate-800 shrink-0">
          <Link href="/dashboard" className="flex items-center gap-2">
            <Zap className="w-5 h-5 text-blue-600" />
            <span className="text-lg font-bold bg-gradient-to-r from-blue-600 to-cyan-500 bg-clip-text text-transparent">
              FaultIQ
            </span>
          </Link>
          <button
            className="md:hidden p-1 rounded text-slate-400 hover:text-slate-700 dark:hover:text-slate-200"
            onClick={() => setSidebarOpen(false)}
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Navigation */}
        <nav className="flex-1 overflow-y-auto px-3 py-5 space-y-1">
          {navItems.map((item) => (
            <NavLink key={item.href} item={item} onClick={() => setSidebarOpen(false)} />
          ))}
        </nav>

        {/* User footer */}
        <div className="border-t border-slate-200 dark:border-slate-800 p-3 shrink-0">
          <div className="flex items-center gap-3 px-2 py-2 rounded-lg hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors">
            <Avatar className="w-8 h-8">
              <AvatarImage src="" />
              <AvatarFallback className="text-xs font-semibold bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
                {initials}
              </AvatarFallback>
            </Avatar>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-slate-900 dark:text-slate-100 truncate">
                {user?.firstName ? `${user.firstName} ${user.lastName}` : user?.email}
              </p>
              <p className="text-xs text-slate-500 dark:text-slate-400 truncate">{user?.email}</p>
            </div>
            <button
              onClick={logout}
              title="Sign out"
              className="p-1.5 rounded text-slate-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
            >
              <LogOut className="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </aside>

      {/* Mobile overlay */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/40 md:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* ─── Main area ────────────────────────────────────────────── */}
      <div className="flex flex-col flex-1 min-w-0 h-screen overflow-hidden">
        {/* Sticky top bar */}
        <header className="h-14 shrink-0 flex items-center gap-3 px-4 bg-white dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800 sticky top-0 z-30">
          {/* Mobile hamburger */}
          <button
            className="md:hidden p-1.5 rounded text-slate-500 hover:text-slate-800 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
            onClick={() => setSidebarOpen(true)}
          >
            <Menu className="w-5 h-5" />
          </button>

          {/* Breadcrumb / page context */}
          <div className="flex items-center gap-1.5 text-sm text-slate-500 dark:text-slate-400 flex-1 min-w-0">
            <span className="font-medium text-slate-700 dark:text-slate-200 truncate">
              {navItems.find(n => n.href === "/dashboard" ? pathname === "/dashboard" : pathname.startsWith(n.href))?.label ?? "Dashboard"}
            </span>
          </div>

          {/* Right side controls */}
          <div className="flex items-center gap-2">
            <button
              onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
              className="p-2 rounded-lg text-slate-500 hover:text-slate-800 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
              title="Toggle theme"
            >
              {theme === "dark" ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
            </button>
            <button className="p-2 rounded-lg text-slate-500 hover:text-slate-800 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors" title="Notifications">
              <Bell className="w-4 h-4" />
            </button>
          </div>
        </header>

        {/* Scrollable page content */}
        <main className="flex-1 overflow-y-auto min-h-0">
          <div className="max-w-7xl mx-auto p-4 lg:p-6">
            <ContextBar />
            {children}
          </div>
        </main>
      </div>
    </div>
  )
}
