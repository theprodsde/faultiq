'use client'

import React, { HTMLAttributes } from 'react'
import { cn } from '@/lib/utils'
import clsx from 'clsx'

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'elevated' | 'glass'
}

export function Card({ variant = 'default', className, children, ...props }: CardProps) {
  const variantClasses = {
    default: 'border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900',
    elevated: 'border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-lg',
    glass: 'border border-white/20 bg-white/10 backdrop-blur-md',
  }

  return (
    <div className={cn('rounded-lg', variantClasses[variant], className)} {...props}>
      {children}
    </div>
  )
}

export function CardHeader({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('border-b border-slate-200 dark:border-slate-800 px-6 py-4', className)} {...props} />
}

export function CardContent({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('p-6', className)} {...props} />
}

export function CardFooter({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={clsx('border-t border-slate-200 dark:border-slate-800 px-6 py-4 flex gap-4 justify-end', className)} {...props} />
}
