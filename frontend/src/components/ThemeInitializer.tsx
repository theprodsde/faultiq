"use client"

import { useEffect } from "react"
import { useTheme } from "next-themes"

export default function ThemeInitializer() {
  const { theme, setTheme } = useTheme()

  useEffect(() => {
    try {
      const stored = typeof window !== "undefined" ? localStorage.getItem('theme') : null
      if (stored) {
        setTheme(stored)
      } else {
        // allow next-themes to resolve system preference
        // no-op here
      }

      // expose helper to change theme programmatically
      ;(window as any).setFaultIQTheme = (t: string | null) => {
        try {
          if (t) {
            localStorage.setItem('theme', t)
            setTheme(t)
          } else {
            localStorage.removeItem('theme')
            setTheme('system')
          }
        } catch (e) {}
      }
    } catch (e) {
      // ignore
    }
  }, [setTheme])

  return null
}
