'use client'

import { ReactNode } from 'react'
import { ThemeProvider } from 'next-themes'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Provider as ReduxProvider } from 'react-redux'
import { AuthProvider } from './auth-provider'
import { TenantProvider } from '@/contexts/tenant-context'
import { store } from '@/store'
import ThemeInitializer from '@/components/ThemeInitializer'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60 * 5,
      gcTime:    1000 * 60 * 10,
      retry: 1,
    },
  },
})

export function Providers({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider attribute="class" defaultTheme="dark" enableSystem>
      <ReduxProvider store={store}>
        <QueryClientProvider client={queryClient}>
          <AuthProvider>
            <TenantProvider>
              {children}
              <ThemeInitializer />
            </TenantProvider>
          </AuthProvider>
        </QueryClientProvider>
      </ReduxProvider>
    </ThemeProvider>
  )
}
