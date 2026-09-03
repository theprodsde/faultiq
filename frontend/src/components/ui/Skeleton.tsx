'use client'

import React from 'react'

export function Skeleton({ className = '', style }: { className?: string; style?: React.CSSProperties }) {
  return <div className={`animate-pulse bg-slate-200 dark:bg-slate-700 rounded ${className}`} style={style} />
}
