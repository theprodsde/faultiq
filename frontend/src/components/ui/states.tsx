"use client"

import { Loader2 } from "lucide-react"
import { cn } from "@/lib/utils"

interface LoadingProps {
  text?: string
  className?: string
  size?: "sm" | "md" | "lg"
}

export function Loading({ text = "Loading…", className, size = "md" }: LoadingProps) {
  const sizeMap = { sm: "w-4 h-4", md: "w-6 h-6", lg: "w-8 h-8" }
  return (
    <div className={cn("flex flex-col items-center justify-center gap-3 py-12 text-slate-500", className)}>
      <Loader2 className={cn("animate-spin", sizeMap[size])} />
      {text && <p className="text-sm">{text}</p>}
    </div>
  )
}

interface EmptyStateProps {
  icon?: React.ReactNode
  title: string
  description?: string
  action?: React.ReactNode
  className?: string
}

export function EmptyState({ icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div className={cn("flex flex-col items-center justify-center gap-4 py-16 text-center", className)}>
      {icon && (
        <div className="w-12 h-12 rounded-xl bg-slate-100 dark:bg-slate-800 flex items-center justify-center text-slate-400">
          {icon}
        </div>
      )}
      <div>
        <p className="font-semibold text-slate-900 dark:text-slate-100">{title}</p>
        {description && (
          <p className="text-sm text-slate-500 dark:text-slate-400 mt-1 max-w-xs mx-auto">{description}</p>
        )}
      </div>
      {action}
    </div>
  )
}

interface ErrorStateProps {
  title?: string
  message?: string
  retry?: () => void
  className?: string
}

export function ErrorState({ title = "Something went wrong", message, retry, className }: ErrorStateProps) {
  return (
    <div className={cn("flex flex-col items-center justify-center gap-4 py-16 text-center", className)}>
      <div className="w-12 h-12 rounded-xl bg-red-50 dark:bg-red-900/20 flex items-center justify-center text-red-500">
        <span className="text-xl">⚠</span>
      </div>
      <div>
        <p className="font-semibold text-slate-900 dark:text-slate-100">{title}</p>
        {message && <p className="text-sm text-slate-500 dark:text-slate-400 mt-1 max-w-xs mx-auto">{message}</p>}
      </div>
      {retry && (
        <button
          onClick={retry}
          className="text-sm text-blue-600 hover:underline dark:text-blue-400"
        >
          Try again
        </button>
      )}
    </div>
  )
}
