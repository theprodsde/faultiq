'use client'

import React, { ButtonHTMLAttributes } from 'react'
import clsx from 'clsx'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'outline' | 'accent' | 'ghost' | 'danger'
  size?: 'sm' | 'md' | 'lg'
  isLoading?: boolean
  icon?: React.ReactNode
}

export function Button({
  variant = 'primary',
  size = 'md',
  isLoading = false,
  icon,
  children,
  className,
  ...props
}: ButtonProps) {
  const baseClasses = 'inline-flex items-center justify-center font-medium rounded-lg transition-all duration-200 focus-visible:ring-2 focus-visible:ring-offset-2'

  const variantClasses = {
    primary: 'bg-blue-600 text-white hover:bg-blue-700 active:scale-95 dark:bg-blue-700 dark:hover:bg-blue-600',
    secondary: 'bg-white text-slate-800 border border-slate-200 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-700 active:scale-95',
    outline: 'bg-transparent text-slate-800 dark:text-slate-200 border border-slate-200 dark:border-slate-700 hover:bg-slate-50 dark:hover:bg-slate-800 active:scale-95',
    accent: 'bg-emerald-600 text-white hover:bg-emerald-700 active:scale-95',
    ghost: 'bg-transparent text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800',
    danger: 'bg-red-600 text-white hover:bg-red-700 active:scale-95',
  }

  const sizeClasses = {
    sm: 'px-3 py-1.5 text-sm gap-1.5',
    md: 'px-4 py-2.5 text-base gap-2',
    lg: 'px-6 py-3 text-lg gap-2.5',
  }

  return (
    <button
      className={clsx(baseClasses, variantClasses[variant], sizeClasses[size], isLoading && 'opacity-50 cursor-not-allowed', className)}
      disabled={isLoading || props.disabled}
      {...props}
    >
      {isLoading && <span className="animate-spin mr-2">⏳</span>}
      {icon && !isLoading && icon}
      {children}
    </button>
  )
}
