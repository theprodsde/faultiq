'use client'

import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { motion } from 'framer-motion'

interface ErrorProps {
  error: Error & { digest?: string }
  reset: () => void
}

export default function Error({ error, reset }: ErrorProps) {
  const router = useRouter()

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-900 via-red-900 to-accent-900 dark:from-slate-950 dark:via-red-950 dark:to-accent-950 flex items-center justify-center px-4">
      <motion.div
        initial={{ opacity: 0, scale: 0.9 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ duration: 0.6 }}
        className="text-center space-y-8 max-w-lg"
      >
        <motion.div
          initial={{ rotate: 0 }}
          animate={{ rotate: [0, -5, 5, -5, 0] }}
          transition={{ duration: 2, repeat: Infinity }}
          className="text-9xl"
        >
          ⚠️
        </motion.div>

        <div className="space-y-3">
          <h1 className="text-5xl font-bold text-white">Something went wrong</h1>
          <p className="text-lg text-slate-300">
            We encountered an error while processing your request.
          </p>
          {process.env.NODE_ENV === 'development' && error && (
            <div className="mt-6 p-4 bg-red-500/10 border border-red-500/30 rounded-lg text-left">
              <p className="text-xs text-red-100 font-mono break-words">
                {error.message}
              </p>
            </div>
          )}
        </div>

        <div className="flex gap-4 justify-center pt-4">
          <Button
            onClick={reset}
            variant="secondary"
            className="border-white/30 text-white hover:bg-white/10"
          >
            Try Again
          </Button>
          <Button
            onClick={() => router.push('/dashboard')}
            variant="primary"
          >
            Go to Dashboard
          </Button>
        </div>
      </motion.div>
    </div>
  )
}
