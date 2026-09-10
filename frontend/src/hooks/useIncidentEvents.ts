"use client"

import { useEffect, useRef, useCallback, useState } from "react"
import { config } from "@/config"

export interface IncidentEvent {
  type: "incident_created" | "incident_resolved" | "signal"
  incidentId?: string
  service?: string
  projectId?: string
  tenantId?: string
  rootCause?: string
  confidence?: number
  status?: string
}

/**
 * useIncidentEvents connects to the SSE endpoint and provides real-time incident events.
 * Falls back gracefully if the SSE endpoint is unavailable.
 */
export function useIncidentEvents(options?: {
  projectId?: string
  tenantId?: string
  onEvent?: (event: IncidentEvent) => void
}) {
  const [connected, setConnected] = useState(false)
  const [lastEvent, setLastEvent] = useState<IncidentEvent | null>(null)
  const eventSourceRef = useRef<EventSource | null>(null)
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Keep a ref to the latest onEvent callback so the connect closure never goes stale
  const onEventRef = useRef(options?.onEvent)
  useEffect(() => { onEventRef.current = options?.onEvent }, [options?.onEvent])

  const connect = useCallback(() => {
    // Use the WebSocket URL config (which points to signal-ingestion :8085)
    const baseUrl = config.websocketUrl?.replace("ws://", "http://").replace("wss://", "https://") || "http://localhost:8085"
    const url = `${baseUrl}/api/v1/events`

    try {
      const es = new EventSource(url)
      eventSourceRef.current = es

      es.addEventListener("connected", () => {
        setConnected(true)
      })

      es.addEventListener("incident", (e) => {
        try {
          const data = JSON.parse(e.data) as IncidentEvent
          // Filter by project/tenant if specified
          if (options?.projectId && data.projectId && data.projectId !== options.projectId) return
          if (options?.tenantId && data.tenantId && data.tenantId !== options.tenantId) return
          
          setLastEvent(data)
          onEventRef.current?.(data)
        } catch {}
      })

      es.addEventListener("signal", (e) => {
        try {
          const data = JSON.parse(e.data)
          const event: IncidentEvent = { type: "signal", ...data }
          if (options?.projectId && data.projectId && data.projectId !== options.projectId) return
          setLastEvent(event)
          onEventRef.current?.(event)
        } catch {}
      })

      es.onerror = () => {
        setConnected(false)
        es.close()
        eventSourceRef.current = null
        // Reconnect after 5 seconds
        reconnectTimeoutRef.current = setTimeout(connect, 5000)
      }
    } catch {
      // SSE not available, fall back to polling (handled by RTK Query)
      setConnected(false)
    }
  }, [options?.projectId, options?.tenantId])

  useEffect(() => {
    connect()
    return () => {
      eventSourceRef.current?.close()
      eventSourceRef.current = null
      if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current)
    }
  }, [connect])

  return { connected, lastEvent }
}
