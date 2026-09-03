'use client'

import React from 'react'
import clsx from 'clsx'

export function Modal({ open, onClose, children }: { open: boolean; onClose: () => void; children: React.ReactNode }) {
  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div className={clsx('relative z-10 bg-white dark:bg-slate-900 rounded shadow-lg w-full max-w-2xl p-6')}>{children}</div>
    </div>
  )
}
