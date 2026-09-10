"use client"

import { useState, useEffect, useMemo } from "react"
import { useListIncidentsQuery } from "@/store/services"
import { useTenant } from "@/contexts/tenant-context"
import { useListProjectsQuery } from "@/store/services"
import { Card } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Loading, EmptyState, ErrorState } from "@/components/ui/states"
import {
  AlertCircle,
  ChevronRight,
  Clock,
  Target,
  FolderOpen,
  Search,
  Wifi,
  WifiOff,
  SortAsc,
  SortDesc,
} from "lucide-react"
import Link from "next/link"
import { motion } from "framer-motion"
import { useSearchParams, useRouter } from "next/navigation"
import type { IncidentListItem, IncidentStatus } from "@/types/api"
import ClientDate from "@/components/ClientDate"
import { useIncidentEvents } from "@/hooks/useIncidentEvents"

const STATUS_COLORS: Record<IncidentStatus, "destructive" | "warning" | "success"> = {
  OPEN:         "destructive",
  ACKNOWLEDGED: "warning",
  RESOLVED:     "success",
}

const DATE_RANGES = [
  { label: "1h",  ms: 60 * 60 * 1000 },
  { label: "6h",  ms: 6 * 60 * 60 * 1000 },
  { label: "24h", ms: 24 * 60 * 60 * 1000 },
  { label: "7d",  ms: 7 * 24 * 60 * 60 * 1000 },
]

const ENVIRONMENTS = ["prod", "staging", "dev"]

function ConfidenceBar({ value }: { value: number }) {
  const pct = Math.round(value * 100)
  const color = pct >= 80 ? "bg-red-500" : pct >= 50 ? "bg-yellow-500" : "bg-blue-500"
  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 h-1.5 rounded-full bg-slate-200 dark:bg-slate-700">
        <div className={`h-1.5 rounded-full ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-xs font-mono text-slate-500">{pct}%</span>
    </div>
  )
}

export default function IncidentsPage() {
  const { tenantId } = useTenant()
  const router = useRouter()
  const searchParams = useSearchParams()
  const projectFromUrl = searchParams?.get("project")

  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(projectFromUrl || null)
  const [statusFilter, setStatusFilter] = useState<IncidentStatus | "">("")
  const [environment, setEnvironment] = useState<string>("prod")
  const [searchQuery, setSearchQuery] = useState("")
  const [dateRange, setDateRange] = useState<string | null>(null)
  const [minConfidence, setMinConfidence] = useState(0)
  const [sortOrder, setSortOrder] = useState<"newest" | "confidence">("newest")
  // Cursor-based pagination state
  const [cursor, setCursor] = useState<string | undefined>(undefined)
  const [prevItems, setPrevItems] = useState<IncidentListItem[]>([])

  const { data: projectsData } = useListProjectsQuery(tenantId ?? "", {
    skip: !tenantId,
  })

  // Update URL when project selection changes (avoid redundant navigation)
  const currentProjectParam = searchParams?.get("project")
  useEffect(() => {
    if (selectedProjectId && selectedProjectId !== currentProjectParam) {
      router.replace(`/dashboard/incidents?project=${selectedProjectId}`)
    }
  }, [selectedProjectId, currentProjectParam, router])

  // Reset pagination when any filter changes
  useEffect(() => {
    setCursor(undefined)
    setPrevItems([])
  }, [selectedProjectId, statusFilter, searchQuery, environment])

  const {
    data,
    isLoading,
    isFetching,
    isError,
    refetch,
  } = useListIncidentsQuery(
    {
      projectId: selectedProjectId || "",
      status: statusFilter || undefined,
      environment: environment || undefined,
      limit: 50,
      q: searchQuery || undefined,
      cursor,
    },
    { skip: !selectedProjectId, pollingInterval: !cursor ? 30000 : 0 }
  )

  // All visible incidents = prev pages + current page
  const pageIncidents = useMemo(() => {
    return [...prevItems, ...(data?.incidents ?? [])]
  }, [prevItems, data?.incidents])

  const handleLoadMore = () => {
    if (data?.hasMore && data?.nextCursor) {
      setPrevItems(pageIncidents)
      setCursor(data.nextCursor)
    }
  }

  // SSE real-time events
  const { connected: sseConnected } = useIncidentEvents({
    projectId: selectedProjectId ?? undefined,
    onEvent: () => refetch(),
  })

  // Client-side filtering and sorting (search is handled server-side via ?q= param)
  const filteredIncidents = useMemo(() => {
    let list: IncidentListItem[] = pageIncidents

    // Date range filter
    if (dateRange) {
      const rangeDef = DATE_RANGES.find((r) => r.label === dateRange)
      if (rangeDef) {
        const cutoff = Date.now() - rangeDef.ms
        list = list.filter((i) => {
          const ts = typeof i.detectedAt === "number" ? i.detectedAt : new Date(i.detectedAt).getTime()
          return ts >= cutoff
        })
      }
    }

    // Min confidence
    if (minConfidence > 0) {
      list = list.filter((i) => Math.round(i.confidence * 100) >= minConfidence)
    }

    // Sort
    if (sortOrder === "confidence") {
      list = [...list].sort((a, b) => b.confidence - a.confidence)
    } else {
      list = [...list].sort((a, b) => {
        const ta = typeof a.detectedAt === "number" ? a.detectedAt : new Date(a.detectedAt).getTime()
        const tb = typeof b.detectedAt === "number" ? b.detectedAt : new Date(b.detectedAt).getTime()
        return tb - ta
      })
    }

    return list
  }, [pageIncidents, dateRange, minConfidence, sortOrder])

  const statuses: (IncidentStatus | "")[] = ["", "OPEN", "ACKNOWLEDGED", "RESOLVED"]

  const selectedProject = useMemo(
    () => projectsData?.projects?.find(p => p.id === selectedProjectId),
    [projectsData, selectedProjectId]
  )

  return (
    <div>
      {/* Header */}
      <motion.div
        initial={{ opacity: 0, y: 16 }}
        animate={{ opacity: 1, y: 0 }}
        className="flex flex-wrap items-start justify-between gap-4 mb-8"
      >
        <div>
          <h1 className="text-3xl font-bold text-slate-900 dark:text-white">Incidents</h1>
          <p className="text-slate-500 dark:text-slate-400 mt-1">
            {selectedProject ? `${selectedProject.name} incidents - Root cause analysis results` : "Select a project to view incidents"}
          </p>
        </div>
        {/* SSE indicator */}
        {selectedProjectId && (
          <div className={`flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-full border ${
            sseConnected
              ? "text-green-700 dark:text-green-400 border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900/20"
              : "text-slate-500 dark:text-slate-400 border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800"
          }`}>
            {sseConnected ? (
              <><Wifi className="w-3 h-3" /> Live</>
            ) : (
              <><WifiOff className="w-3 h-3" /> Polling</>
            )}
          </div>
        )}
      </motion.div>

      {/* Filters row 1: project + environment + status */}
      <div className="flex flex-wrap gap-3 mb-3">
        {/* Project picker */}
        {projectsData?.projects && projectsData.projects.length > 0 && (
          <select
            value={selectedProjectId ?? ""}
            onChange={(e) => setSelectedProjectId(e.target.value || null)}
            className="text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-700 dark:text-slate-300"
          >
            <option value="">Select a project...</option>
            {projectsData?.projects?.map((p) => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
        )}

        {/* Environment picker */}
        {selectedProjectId && (
          <select
            value={environment}
            onChange={(e) => setEnvironment(e.target.value)}
            className="text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-700 dark:text-slate-300"
          >
            {ENVIRONMENTS.map((env) => (
              <option key={env} value={env}>{env}</option>
            ))}
          </select>
        )}

        {/* Status filter chips */}
        {selectedProjectId && (
          <div className="flex gap-2 flex-wrap">
            {statuses.map((s) => (
              <button
                key={s}
                onClick={() => setStatusFilter(s)}
                className={`px-3 py-1.5 text-xs font-semibold rounded-full border transition-colors ${
                  statusFilter === s
                    ? "bg-blue-600 text-white border-blue-600"
                    : "border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 hover:border-blue-400 hover:text-blue-600"
                }`}
              >
                {s || "All"}
              </button>
            ))}
          </div>
        )}

        {/* Refresh button */}
        {selectedProjectId && (
          <button
            onClick={() => refetch()}
            className="ml-auto px-3 py-1.5 text-xs font-semibold rounded-lg bg-blue-600 text-white hover:bg-blue-700 transition-colors"
          >
            Refresh
          </button>
        )}
      </div>

      {/* Filters row 2: search + date range + confidence + sort */}
      {selectedProjectId && (
        <div className="flex flex-wrap items-center gap-3 mb-6 p-3 rounded-xl bg-slate-50 dark:bg-slate-800/60 border border-slate-200 dark:border-slate-700">
          {/* Search input */}
          <div className="relative">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-slate-400" />
            <input
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search service…"
              className="pl-8 pr-3 py-1.5 text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-700 dark:text-slate-300 focus:outline-none focus:ring-2 focus:ring-blue-500 w-44"
            />
          </div>

          {/* Date range */}
          <div className="flex items-center gap-1">
            {DATE_RANGES.map((r) => (
              <button
                key={r.label}
                onClick={() => setDateRange(dateRange === r.label ? null : r.label)}
                className={`px-2.5 py-1 text-xs font-semibold rounded-md transition-colors ${
                  dateRange === r.label
                    ? "bg-blue-600 text-white"
                    : "bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 hover:border-blue-400"
                }`}
              >
                {r.label}
              </button>
            ))}
          </div>

          {/* Min confidence slider */}
          <div className="flex items-center gap-2">
            <span className="text-xs text-slate-500 whitespace-nowrap">Min conf:</span>
            <input
              type="range"
              min={0}
              max={100}
              step={5}
              value={minConfidence}
              onChange={(e) => setMinConfidence(Number(e.target.value))}
              className="w-24 accent-blue-600"
            />
            <span className="text-xs font-mono text-slate-600 dark:text-slate-400 w-8">{minConfidence}%</span>
          </div>

          {/* Sort toggle */}
          <button
            onClick={() => setSortOrder(sortOrder === "newest" ? "confidence" : "newest")}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-400 hover:border-blue-400 transition-colors"
          >
            {sortOrder === "newest" ? (
              <><SortDesc className="w-3.5 h-3.5" /> Newest</>
            ) : (
              <><SortAsc className="w-3.5 h-3.5" /> Confidence</>
            )}
          </button>

          <span className="text-xs text-slate-400 ml-auto">
            {filteredIncidents.length} result{filteredIncidents.length !== 1 ? "s" : ""}
          </span>
        </div>
      )}

      {/* Content */}
      {!selectedProjectId ? (
        <EmptyState
          icon={<FolderOpen className="w-12 h-12" />}
          title="Select a project to view incidents"
          description="Choose a project from the dropdown above to see its incidents and perform root cause analysis."
        />
      ) : isLoading ? (
        <Loading text="Loading incidents…" />
      ) : isError ? (
        <ErrorState message="Could not load incidents." retry={refetch} />
      ) : !filteredIncidents.length ? (
        <div>
          <EmptyState
            icon={<AlertCircle className="w-6 h-6" />}
            title="No incidents found"
            description={statusFilter ? `No ${statusFilter.toLowerCase()} incidents.` : "No incidents detected yet. Incidents appear when fault signals are analyzed."}
          />
          <div className="mt-4 p-3 text-xs text-slate-500 dark:text-slate-400 bg-slate-50 dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700">
            <p>Tip: Go to <strong>Service Graph</strong> and use "Simulate Service Failure" to trigger incidents.</p>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          {filteredIncidents.map((incident, idx) => (
            <motion.div
              key={incident.id}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.04 }}
            >
              <Link href={`/dashboard/incidents/${incident.id}?project=${selectedProjectId}`}>
                <Card className="p-5 hover:border-blue-400 dark:hover:border-blue-500 cursor-pointer transition-all group">
                  <div className="flex items-start gap-4">
                    <div className="flex-1 min-w-0">
                      <div className="flex flex-wrap items-center gap-2 mb-2">
                        <Badge variant={STATUS_COLORS[incident.status]} className="text-xs">
                          {incident.status}
                        </Badge>
                        {incident.environment && incident.environment !== "prod" && (
                          <span className="text-[10px] font-semibold px-1.5 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400">
                            {incident.environment}
                          </span>
                        )}
                        {incident.phase && incident.phase !== "RESOLVED" && (
                          <span className="text-[10px] font-semibold px-1.5 py-0.5 rounded bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400">
                            {incident.phase}
                          </span>
                        )}
                        <span className="text-xs text-slate-400 flex items-center gap-1">
                          <Clock className="w-3 h-3" />
                          <ClientDate iso={typeof incident.detectedAt === "number" ? new Date(incident.detectedAt).toISOString() : incident.detectedAt as string} />
                        </span>
                      </div>

                      <p className="font-semibold text-slate-900 dark:text-white flex items-center gap-2">
                        <Target className="w-4 h-4 text-slate-400 shrink-0" />
                        Service: <span className="text-blue-600 dark:text-blue-400">{incident.service}</span>
                      </p>

                      {incident.rootCauseCandidate && (
                        <p className="text-sm text-slate-600 dark:text-slate-400 mt-2">
                          Root cause: <span className="font-semibold text-orange-600 dark:text-orange-400">{incident.rootCauseCandidate}</span>
                        </p>
                      )}

                      <div className="mt-2">
                        <ConfidenceBar value={incident.confidence} />
                      </div>

                      <p className="text-xs text-slate-500 mt-1">
                        {incident.evidenceCount} evidence item{incident.evidenceCount !== 1 ? "s" : ""}
                      </p>
                    </div>

                    <ChevronRight className="w-4 h-4 text-slate-300 group-hover:text-blue-500 transition-colors shrink-0 mt-1" />
                  </div>
                </Card>
              </Link>
            </motion.div>
          ))}

          {/* Cursor pagination: load more */}
          {data?.hasMore && !isFetching && (
            <div className="mt-4 flex justify-center">
              <button
                onClick={handleLoadMore}
                className="px-4 py-2 text-sm font-medium rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-400 hover:border-blue-400 hover:text-blue-600 transition-colors"
              >
                Load more
              </button>
            </div>
          )}
          {isFetching && cursor && (
            <div className="mt-4 flex justify-center">
              <Loading text="Loading more…" />
            </div>
          )}
        </div>
      )}
    </div>
  )
}
