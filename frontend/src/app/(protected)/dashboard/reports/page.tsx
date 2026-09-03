"use client"

import { useState, useEffect, useMemo } from "react"
import { useListIncidentsQuery, useListProjectsQuery, useGetSignalHistoryBatchQuery } from "@/store/services"
import { useTenant } from "@/contexts/tenant-context"
import { Card } from "@/components/ui/card"
import { Loading, EmptyState, ErrorState } from "@/components/ui/states"
import { BarChart3, TrendingDown, TrendingUp, AlertCircle, CheckCircle2, Download, Bell, BellOff } from "lucide-react"
import { motion } from "framer-motion"
import { ErrorBoundary } from "@/components/ErrorBoundary"
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  Legend,
} from "recharts"
import type { IncidentListItem, IncidentStatus } from "@/types/api"

const STATUS_PALETTE: Record<IncidentStatus, string> = {
  OPEN:         "#ef4444",
  ACKNOWLEDGED: "#f59e0b",
  RESOLVED:     "#22c55e",
}

function groupByDate(incidents: IncidentListItem[]) {
  const map: Record<string, number> = {}
  for (const inc of incidents) {
    const day = new Date(typeof inc.detectedAt === "number" ? inc.detectedAt : inc.detectedAt).toLocaleDateString("en-US", { month: "short", day: "numeric" })
    map[day] = (map[day] ?? 0) + 1
  }
  return Object.entries(map)
    .map(([date, count]) => ({ date, count }))
    .slice(-14)
}

function groupByStatus(incidents: IncidentListItem[]) {
  const map: Partial<Record<IncidentStatus, number>> = {}
  for (const inc of incidents) {
    map[inc.status] = (map[inc.status] ?? 0) + 1
  }
  return Object.entries(map).map(([status, value]) => ({ status, value }))
}

function confidenceHistogram(incidents: IncidentListItem[]) {
  const buckets: Record<string, number> = {
    "0–20": 0,
    "21–40": 0,
    "41–60": 0,
    "61–80": 0,
    "81–100": 0,
  }
  for (const inc of incidents) {
    const pct = Math.round(inc.confidence * 100)
    if (pct <= 20) buckets["0–20"]++
    else if (pct <= 40) buckets["21–40"]++
    else if (pct <= 60) buckets["41–60"]++
    else if (pct <= 80) buckets["61–80"]++
    else buckets["81–100"]++
  }
  return Object.entries(buckets).map(([range, count]) => ({ range, count }))
}

function computeMttr(incidents: IncidentListItem[]) {
  const resolved = incidents.filter(
    (i) => i.status === "RESOLVED" && i.resolvedAt
  )
  const byService: Record<string, number[]> = {}
  for (const inc of resolved) {
    const ts = typeof inc.detectedAt === "number" ? inc.detectedAt : new Date(inc.detectedAt).getTime()
    const resolvedTs = new Date(inc.resolvedAt!).getTime()
    const mins = (resolvedTs - ts) / 60000
    if (!byService[inc.service]) byService[inc.service] = []
    byService[inc.service].push(mins)
  }
  return Object.entries(byService).map(([service, times]) => ({
    service,
    avgMttrMin: Math.round(times.reduce((s, v) => s + v, 0) / times.length),
  }))
}

function computeSLOStatus(
  incidents: IncidentListItem[],
  signalHistoryBatch?: Record<string, { sloStatus?: string; uptime?: number; sloTarget?: number }>,
) {
  const now = Date.now()
  const cutoff = now - 24 * 60 * 60 * 1000
  const recent = incidents.filter((i) => {
    const ts = typeof i.detectedAt === "number" ? i.detectedAt : new Date(i.detectedAt).getTime()
    return ts >= cutoff
  })
  const serviceNames = Array.from(new Set(recent.map((i) => i.service)))
  return serviceNames.map((svc) => {
    const svcIncidents = recent.filter((i) => i.service === svc)
    const openCount = svcIncidents.filter((i) => i.status === "OPEN").length
    const avgConf = svcIncidents.length
      ? svcIncidents.reduce((s, i) => s + i.confidence, 0) / svcIncidents.length
      : 0
    // Use real sloStatus from signal history if available; fall back to incident-count approximation
    const histEntry = signalHistoryBatch?.[svc]
    const status: "ok" | "warning" | "critical" = histEntry?.sloStatus
      ? (histEntry.sloStatus as "ok" | "warning" | "critical")
      : openCount > 2 ? "critical" : openCount > 0 ? "warning" : "ok"
    return { service: svc, openCount, total: svcIncidents.length, status, avgConf, uptime: histEntry?.uptime, sloTarget: histEntry?.sloTarget }
  })
}

function StatCard({
  label,
  value,
  icon,
  trend,
}: {
  label: string
  value: string | number
  icon: React.ReactNode
  trend?: "up" | "down" | "neutral"
}) {
  return (
    <Card className="p-5">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-xs text-slate-400 uppercase tracking-wide mb-2">{label}</p>
          <p className="text-3xl font-bold text-slate-900 dark:text-white">{value}</p>
        </div>
        <div className="w-10 h-10 rounded-xl bg-slate-100 dark:bg-slate-800 flex items-center justify-center text-slate-500">
          {icon}
        </div>
      </div>
      {trend && (
        <div className="mt-3 flex items-center gap-1 text-xs">
          {trend === "down" && <TrendingDown className="w-3.5 h-3.5 text-green-500" />}
          {trend === "up"   && <TrendingUp   className="w-3.5 h-3.5 text-red-500"   />}
          <span className={trend === "down" ? "text-green-600 dark:text-green-400" : trend === "up" ? "text-red-600 dark:text-red-400" : "text-slate-400"}>
            {trend === "down" ? "Trending down" : trend === "up" ? "Trending up" : "Stable"}
          </span>
        </div>
      )}
    </Card>
  )
}

const SLO_COLORS = {
  ok:       { bg: "bg-green-50 dark:bg-green-900/20",  border: "border-green-200 dark:border-green-800",  dot: "bg-green-500",  label: "text-green-700 dark:text-green-300" },
  warning:  { bg: "bg-yellow-50 dark:bg-yellow-900/20", border: "border-yellow-200 dark:border-yellow-800", dot: "bg-yellow-500", label: "text-yellow-700 dark:text-yellow-300" },
  critical: { bg: "bg-red-50 dark:bg-red-900/20",      border: "border-red-200 dark:border-red-800",       dot: "bg-red-500 animate-pulse",   label: "text-red-700 dark:text-red-300" },
}

export default function ReportsPage() {
  const { tenantId } = useTenant()
  const [selectedProjectId, setSelectedProjectId] = useState<string>("")
  // sloAlerts: serviceId -> boolean (stored in localStorage)
  const [sloAlerts, setSloAlerts] = useState<Record<string, boolean>>({})

  const { data: projectsData, isLoading: projectsLoading } = useListProjectsQuery(tenantId ?? "", { skip: !tenantId })
  const activeProjectId = selectedProjectId || projectsData?.projects?.[0]?.id || ""

  const { data, isLoading, isError, refetch } = useListIncidentsQuery(
    { projectId: activeProjectId, limit: 200 },
    { skip: !activeProjectId }
  )

  // Derive unique services from incidents to fetch batch signal history
  const incidentServices = useMemo(() => {
    const incidents = data?.incidents ?? []
    return Array.from(new Set(incidents.map((i) => i.service).filter(Boolean)))
  }, [data])

  const { data: signalHistoryBatch } = useGetSignalHistoryBatchQuery(
    { projectId: activeProjectId, services: incidentServices, hours: 24 },
    { skip: !activeProjectId || incidentServices.length === 0 }
  )

  // Load sloAlerts from localStorage on mount
  useEffect(() => {
    try {
      const stored = localStorage.getItem("sloAlerts")
      if (stored) setSloAlerts(JSON.parse(stored))
    } catch {
      // ignore
    }
  }, [])

  const toggleSloAlert = (service: string, critical: boolean) => {
    setSloAlerts((prev) => {
      const next = { ...prev, [service]: !prev[service] }
      try {
        localStorage.setItem("sloAlerts", JSON.stringify(next))
      } catch {
        // ignore
      }
      // Request browser notification permission
      if (next[service] && critical && typeof window !== "undefined" && "Notification" in window) {
        if (Notification.permission === "default") {
          Notification.requestPermission()
        } else if (Notification.permission === "granted") {
          new Notification(`SLO Alert: ${service}`, {
            body: "This service has active SLO violations.",
          })
        }
      }
      return next
    })
  }

  const incidents = data?.incidents ?? []
  const open        = incidents.filter((i) => i.status === "OPEN").length
  const resolved    = incidents.filter((i) => i.status === "RESOLVED").length
  const avgConfidence = incidents.length
    ? Math.round((incidents.reduce((s, i) => s + i.confidence, 0) / incidents.length) * 100)
    : 0

  const dailyData   = groupByDate(incidents)
  const statusData  = groupByStatus(incidents)
  const confData    = confidenceHistogram(incidents)
  const mttrData    = computeMttr(incidents)
  const sloData     = computeSLOStatus(incidents, signalHistoryBatch)

  // SLO summary counts
  const sloHealthy   = sloData.filter((s) => s.status === "ok").length
  const sloDegraded  = sloData.filter((s) => s.status === "warning").length
  const sloCritical  = sloData.filter((s) => s.status === "critical").length

  // Active SLO violations with alerts toggled on
  const violatingAlerted = sloData.filter(
    (s) => s.status === "critical" && sloAlerts[s.service]
  )

  const exportSloCSV = () => {
    const now = new Date().toISOString()
    const rows = [
      ["Service", "Status", "OpenIncidents", "AvgConfidence", "LastIncidentAt"],
      ...sloData.map((s) => [
        s.service,
        s.status,
        String(s.openCount),
        String(Math.round(s.avgConf * 100) / 100),
        now,
      ]),
    ]
    const csv = rows.map((r) => r.join(",")).join("\n")
    const blob = new Blob([csv], { type: "text/csv" })
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = "slo-report.csv"
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div>
      <motion.div initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }}>
        {/* Header */}
        <div className="flex flex-wrap items-start justify-between gap-4 mb-8">
          <div>
            <h1 className="text-3xl font-bold text-slate-900 dark:text-white">Analytics</h1>
            <p className="text-slate-500 dark:text-slate-400 mt-1">
              Incident trends and detection performance
            </p>
          </div>
          {(projectsData?.projects?.length ?? 0) > 0 && (
            <select
              value={activeProjectId}
              onChange={(e) => setSelectedProjectId(e.target.value)}
              className="text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-700 dark:text-slate-300"
            >
              {projectsData?.projects?.map((p) => (
                <option key={p.id} value={p.id}>{p.name}</option>
              ))}
            </select>
          )}
        </div>

        {!activeProjectId && projectsLoading ? (
          <Loading text="Loading projects…" />
        ) : !activeProjectId ? (
          <EmptyState icon={<BarChart3 className="w-6 h-6" />} title="No project selected" description="Create a project to see analytics." />
        ) : isLoading ? (
          <Loading text="Loading analytics…" />
        ) : isError ? (
          <ErrorState message="Could not load incidents." retry={refetch} />
        ) : (
          <>
            {/* SLO Summary card — very top */}
            {sloData.length > 0 && (
              <div className="mb-6">
                <Card className="p-5">
                  <div className="flex items-center justify-between flex-wrap gap-3">
                    <div>
                      <p className="text-sm font-semibold text-slate-900 dark:text-white mb-2">SLO Summary</p>
                      <div className="flex items-center gap-4 text-sm">
                        <span className="flex items-center gap-1.5 text-green-700 dark:text-green-300 font-medium">
                          <span className="w-2 h-2 rounded-full bg-green-500 inline-block" />
                          {sloHealthy} healthy
                        </span>
                        <span className="flex items-center gap-1.5 text-yellow-700 dark:text-yellow-300 font-medium">
                          <span className="w-2 h-2 rounded-full bg-yellow-500 inline-block" />
                          {sloDegraded} degraded
                        </span>
                        <span className="flex items-center gap-1.5 text-red-700 dark:text-red-300 font-medium">
                          <span className="w-2 h-2 rounded-full bg-red-500 inline-block" />
                          {sloCritical} critical
                        </span>
                      </div>
                    </div>
                    <button
                      onClick={exportSloCSV}
                      className="flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 hover:border-blue-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
                    >
                      <Download className="w-3.5 h-3.5" />
                      Export SLO Report
                    </button>
                  </div>
                </Card>
              </div>
            )}

            {/* SLO violation banner */}
            {violatingAlerted.length > 0 && (
              <div className="mb-6 px-4 py-3 rounded-xl border border-amber-300 dark:border-amber-700 bg-amber-50 dark:bg-amber-900/20 text-amber-800 dark:text-amber-200 text-sm flex items-center gap-2">
                <AlertCircle className="w-4 h-4 flex-shrink-0" />
                <span>
                  SLO violated:{" "}
                  {violatingAlerted.map((s) => (
                    <span key={s.service} className="font-semibold">
                      {s.service} has {s.openCount} open incident{s.openCount !== 1 ? "s" : ""}
                    </span>
                  )).reduce<React.ReactNode[]>((acc, el, i) => (i === 0 ? [el] : [...acc, ", ", el]), [])}
                </span>
              </div>
            )}

            {/* SLO Status — top section */}
            {sloData.length > 0 && (
              <div className="mb-8">
                <h2 className="text-base font-semibold text-slate-900 dark:text-white mb-1">SLO Status</h2>
                <p className="text-xs text-slate-500 dark:text-slate-400 mb-4">
                  SLO health computed from signal history (24h). Green = within target, Yellow = near threshold, Red = budget exceeded.
                </p>
                <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
                  {sloData.map((svc) => {
                    const colors = SLO_COLORS[svc.status]
                    const alertOn = !!sloAlerts[svc.service]
                    return (
                      <div
                        key={svc.service}
                        className={`p-4 rounded-xl border ${colors.bg} ${colors.border}`}
                      >
                        <div className="flex items-center gap-2 mb-2">
                          <span className={`w-2 h-2 rounded-full flex-shrink-0 ${colors.dot}`} />
                          <p className="text-xs font-semibold text-slate-800 dark:text-slate-100 truncate flex-1">
                            {svc.service}
                          </p>
                          <button
                            onClick={() => toggleSloAlert(svc.service, svc.status === "critical")}
                            title={alertOn ? "Disable SLO alert" : "Enable SLO alert"}
                            className={`flex-shrink-0 transition-colors ${alertOn ? "text-amber-600 dark:text-amber-400" : "text-slate-300 dark:text-slate-600 hover:text-amber-500"}`}
                          >
                            {alertOn ? <Bell className="w-3 h-3" /> : <BellOff className="w-3 h-3" />}
                          </button>
                        </div>
                        <p className={`text-xs font-medium ${colors.label}`}>
                          {svc.status === "ok" ? "Within budget" : svc.status === "warning" ? "Elevated" : "Budget exceeded"}
                        </p>
                        <p className="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
                          {svc.uptime !== undefined
                            ? `${svc.uptime.toFixed(1)}% uptime${svc.sloTarget ? ` (target ${svc.sloTarget.toFixed(1)}%)` : ""}`
                            : `${svc.openCount} open / ${svc.total} total incidents`}
                        </p>
                        <p className="text-[11px] text-slate-400 mt-0.5">
                          Avg conf: {Math.round(svc.avgConf * 100)}%
                        </p>
                      </div>
                    )
                  })}
                </div>
              </div>
            )}

            {/* KPI row */}
            <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
              <StatCard label="Total Incidents" value={incidents.length} icon={<AlertCircle className="w-5 h-5" />} />
              <StatCard label="Open" value={open} icon={<AlertCircle className="w-5 h-5 text-red-500" />} trend={open > 5 ? "up" : "down"} />
              <StatCard label="Resolved" value={resolved} icon={<CheckCircle2 className="w-5 h-5 text-green-500" />} trend={resolved > 0 ? "neutral" : "neutral"} />
              <StatCard label="Avg Confidence" value={`${avgConfidence}%`} icon={<BarChart3 className="w-5 h-5" />} />
            </div>

            {incidents.length === 0 ? (
              <EmptyState
                icon={<BarChart3 className="w-6 h-6" />}
                title="No incidents to analyze"
                description="Analytics will appear once incidents are detected for this project."
              />
            ) : (
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                {/* Daily incidents */}
                <ErrorBoundary context="Analytics Chart">
                  <Card className="p-5">
                    <p className="font-semibold text-slate-900 dark:text-white mb-4">Daily Incidents (last 14 days)</p>
                    <ResponsiveContainer width="100%" height={200}>
                      <BarChart data={dailyData} margin={{ left: -20 }}>
                        <XAxis dataKey="date" tick={{ fontSize: 11 }} />
                        <YAxis tick={{ fontSize: 11 }} allowDecimals={false} />
                        <Tooltip />
                        <Bar dataKey="count" name="Incidents" fill="#3b82f6" radius={[3, 3, 0, 0]} />
                      </BarChart>
                    </ResponsiveContainer>
                  </Card>
                </ErrorBoundary>

                {/* Status distribution */}
                <ErrorBoundary context="Analytics Chart">
                  <Card className="p-5">
                    <p className="font-semibold text-slate-900 dark:text-white mb-4">Status Distribution</p>
                    <ResponsiveContainer width="100%" height={200}>
                      <PieChart>
                        <Pie
                          data={statusData}
                          dataKey="value"
                          nameKey="status"
                          cx="50%"
                          cy="50%"
                          outerRadius={70}
                          label={({ name, percent }) => `${name} ${Math.round((percent ?? 0) * 100)}%`}
                          labelLine={false}
                        >
                          {statusData.map((entry) => (
                            <Cell
                              key={entry.status}
                              fill={STATUS_PALETTE[entry.status as IncidentStatus] ?? "#94a3b8"}
                            />
                          ))}
                        </Pie>
                        <Legend />
                        <Tooltip />
                      </PieChart>
                    </ResponsiveContainer>
                  </Card>
                </ErrorBoundary>

                {/* Confidence histogram */}
                <ErrorBoundary context="Analytics Chart">
                  <Card className="p-5 lg:col-span-2">
                    <p className="font-semibold text-slate-900 dark:text-white mb-4">RCA Confidence Distribution</p>
                    <ResponsiveContainer width="100%" height={180}>
                      <BarChart data={confData} margin={{ left: -20 }}>
                        <XAxis dataKey="range" tick={{ fontSize: 11 }} />
                        <YAxis tick={{ fontSize: 11 }} allowDecimals={false} />
                        <Tooltip />
                        <Bar dataKey="count" name="Incidents" fill="#8b5cf6" radius={[3, 3, 0, 0]} />
                      </BarChart>
                    </ResponsiveContainer>
                  </Card>
                </ErrorBoundary>

                {/* MTTR per service */}
                {mttrData.length > 0 && (
                  <ErrorBoundary context="Analytics Chart">
                    <Card className="p-5 lg:col-span-2">
                      <div className="mb-4">
                        <p className="font-semibold text-slate-900 dark:text-white">Mean Time to Resolve (MTTR) per Service</p>
                        <p className="text-xs text-slate-400 mt-0.5">Average minutes from detection to resolution, for resolved incidents</p>
                      </div>
                      <ResponsiveContainer width="100%" height={200}>
                        <BarChart data={mttrData} margin={{ left: -10 }}>
                          <XAxis dataKey="service" tick={{ fontSize: 11 }} />
                          <YAxis tick={{ fontSize: 11 }} allowDecimals={false} label={{ value: "min", angle: -90, position: "insideLeft", style: { fontSize: 10, fill: "#94a3b8" } }} />
                          <Tooltip formatter={(value) => [`${value} min`, "Avg MTTR"]} />
                          <Bar dataKey="avgMttrMin" name="Avg MTTR (min)" fill="#f59e0b" radius={[3, 3, 0, 0]} />
                        </BarChart>
                      </ResponsiveContainer>
                    </Card>
                  </ErrorBoundary>
                )}

                {/* Phase dwell time */}
                {(() => {
                  const phaseCounts: Record<string, number> = {
                    DETECTING: 0,
                    NARROWING: 0,
                    CONFIRMED: 0,
                    TRIAGING: 0,
                    FIXING: 0,
                    VERIFYING: 0,
                    RESOLVED: 0,
                  }
                  for (const inc of incidents) {
                    if (inc.phase) {
                      phaseCounts[inc.phase] = (phaseCounts[inc.phase] ?? 0) + 1
                    }
                  }
                  const phaseData = Object.entries(phaseCounts)
                    .filter(([, v]) => v > 0)
                    .map(([phase, count]) => ({ phase, count }))

                  if (phaseData.length === 0) return null

                  return (
                    <ErrorBoundary context="Analytics Chart">
                      <Card className="p-5 lg:col-span-2">
                        <div className="mb-4">
                          <p className="font-semibold text-slate-900 dark:text-white">Incident Phase Distribution</p>
                          <p className="text-xs text-slate-400 mt-0.5">Number of incidents currently in each SOP phase</p>
                        </div>
                        <ResponsiveContainer width="100%" height={180}>
                          <BarChart data={phaseData} layout="vertical" margin={{ left: 20 }}>
                            <XAxis type="number" tick={{ fontSize: 11 }} allowDecimals={false} />
                            <YAxis dataKey="phase" type="category" tick={{ fontSize: 11 }} width={80} />
                            <Tooltip />
                            <Bar dataKey="count" name="Incidents" fill="#06b6d4" radius={[0, 3, 3, 0]} />
                          </BarChart>
                        </ResponsiveContainer>
                      </Card>
                    </ErrorBoundary>
                  )
                })()}
              </div>
            )}
          </>
        )}
      </motion.div>
    </div>
  )
}
