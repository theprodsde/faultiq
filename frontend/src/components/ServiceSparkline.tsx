"use client"

import { memo } from "react"
import { LineChart, Line, ResponsiveContainer } from "recharts"
import { useGetSignalHistoryQuery } from "@/store/services"

interface ServiceSparklineProps {
  projectId: string
  serviceId: string
  height?: number
  preloadedData?: {
    errorRateTimeline: { time: string; errorRate: number }[]
    avgLatency: number
    faultCount: number
    uptime: number
    sloStatus?: string
  }
}

export const ServiceSparkline = memo(function ServiceSparkline({ projectId, serviceId, height = 50, preloadedData }: ServiceSparklineProps) {
  const skip = !projectId || !serviceId || preloadedData !== undefined
  const { data, isLoading } = useGetSignalHistoryQuery(
    { projectId, serviceId, hours: 24 },
    { skip, pollingInterval: 60000 }
  )

  const activeData = preloadedData ?? data
  const timeline = activeData?.errorRateTimeline ?? []
  const effectiveLoading = isLoading && preloadedData === undefined

  const isElevated =
    timeline.length > 0 &&
    timeline.slice(-3).some((p) => p.errorRate > 0.01)

  const sloStatus = preloadedData?.sloStatus
  const strokeColor =
    sloStatus === "critical" ? "#ef4444" :
    sloStatus === "warning"  ? "#eab308" :
    sloStatus === "ok"       ? "#22c55e" :
    isElevated               ? "#ef4444" : "#22c55e"

  if (effectiveLoading || timeline.length === 0) {
    return (
      <div
        style={{ height }}
        className="w-full bg-slate-100 dark:bg-slate-800 rounded animate-pulse opacity-40"
      />
    )
  }

  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart data={timeline} margin={{ top: 2, right: 2, bottom: 2, left: 2 }}>
        <Line
          type="monotone"
          dataKey="errorRate"
          stroke={strokeColor}
          strokeWidth={1.5}
          dot={false}
          isAnimationActive={false}
        />
      </LineChart>
    </ResponsiveContainer>
  )
}, (prevProps, nextProps) => {
  return (
    prevProps.projectId === nextProps.projectId &&
    prevProps.serviceId === nextProps.serviceId &&
    prevProps.height === nextProps.height &&
    prevProps.preloadedData === nextProps.preloadedData
  )
})
