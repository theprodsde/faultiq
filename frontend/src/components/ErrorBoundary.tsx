"use client"
import React from "react"
import { Button } from "@/components/ui/button"

interface Props {
  children: React.ReactNode
  fallback?: React.ReactNode
  context?: string   // e.g. "Graph Visualization" to show in the error message
}

interface State {
  hasError: boolean
  error?: Error
}

export class ErrorBoundary extends React.Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error(`[ErrorBoundary${this.props.context ? ` (${this.props.context})` : ""}]`, error, info)
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback
      return (
        <div className="flex flex-col items-center justify-center p-8 rounded-xl border border-red-200 dark:border-red-800 bg-red-50/50 dark:bg-red-900/10 text-center">
          <p className="text-sm font-semibold text-red-700 dark:text-red-300 mb-1">
            {this.props.context ? `${this.props.context} failed to load` : "Something went wrong"}
          </p>
          <p className="text-xs text-red-500 dark:text-red-400 mb-3 font-mono max-w-md truncate">
            {this.state.error?.message}
          </p>
          <Button
            size="sm"
            variant="outline"
            onClick={() => this.setState({ hasError: false, error: undefined })}
          >
            Try again
          </Button>
        </div>
      )
    }
    return this.props.children
  }
}
