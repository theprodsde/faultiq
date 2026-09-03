'use client'

import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { motion } from 'framer-motion'

export default function NotFound() {
  const router = useRouter()

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-900 via-primary-900 to-accent-900 dark:from-slate-950 dark:via-primary-950 dark:to-accent-950 flex items-center justify-center px-4">
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
          🔍
        </motion.div>

        <div className="space-y-3">
          <h1 className="text-5xl font-bold text-white">404</h1>
          <p className="text-2xl font-semibold text-slate-300">Page Not Found</p>
          <p className="text-lg text-slate-400">
            The page you're looking for doesn't exist or has been moved.
          </p>
        </div>

        <div className="flex gap-4 justify-center pt-4">
          <Button
            onClick={() => router.back()}
            variant="secondary"
            className="border-white/30 text-white hover:bg-white/10"
          >
            Go Back
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
