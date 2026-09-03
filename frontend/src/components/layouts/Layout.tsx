'use client'

import React, { ReactNode } from 'react'
import Header from '../navigation/Header'
import Sidebar from '../navigation/Sidebar'
import Footer from '../navigation/Footer'

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen flex flex-col bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-slate-100">
      <Header />
      <div className="flex flex-1">
        <Sidebar />
        <div className="flex-1 flex flex-col">
          <main className="flex-1">
            {children}
          </main>
          <Footer />
        </div>
      </div>
    </div>
  )
}
