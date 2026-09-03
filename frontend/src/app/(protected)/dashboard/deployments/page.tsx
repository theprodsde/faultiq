"use client"

import { useState, useMemo } from "react"
import {
  useListDeploymentsQuery,
  useCreateDeploymentMutation,
  useListProjectsQuery,
  useListIncidentsQuery,
} from "@/store/services"
import { useTenant } from "@/contexts/tenant-context"
import { Card } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Loading, EmptyState, ErrorState } from "@/components/ui/states"
import { Rocket, Clock, User, Info, Plus, X } from "lucide-react"
import { motion } from "framer-motion"
import { useRouter } from "next/navigation"
import ClientDate from "@/components/ClientDate"
import type { DeploymentStatus, Deployment } from "@/types/api"

// ── Timeline helpers ───────────────────────────────────────────────────────

type TimelineEvent = {
  time: number        // unix timestamp ms
  type: "deployment" | "incident"
  label: string
  status: string
  id: string
}

function relativeTime(ms: number): string {
  const diffMs = Date.now() - ms
  const diffMins = Math.floor(diffMs / 60000)
  if (diffMins < 1) return "just now"
  if (diffMins < 60) return `${diffMins}m ago`
  const diffHours = Math.floor(diffMs / 3600000)
  if (diffHours < 24) return `${diffHours}h ago`
  const diffDays = Math.floor(diffMs / 86400000)
  if (diffDays === 1) return "yesterday"
  return `${diffDays}d ago`
}

const STATUS_COLORS: Record<DeploymentStatus, string> = {
  deploying:    "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300",
  healthy:      "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300",
  degraded:     "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300",
  rolled_back:  "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400",
}

const STATUS_VARIANTS: Record<DeploymentStatus, "default" | "secondary" | "destructive" | "warning" | "success"> = {
  deploying:   "default",
  healthy:     "success",
  degraded:    "destructive",
  rolled_back: "secondary",
}

function NotifyModal({
  projectId,
  onClose,
}: {
  projectId: string
  onClose: () => void
}) {
  const [serviceId, setServiceId] = useState("")
  const [version, setVersion] = useState("")
  const [commitHash, setCommitHash] = useState("")
  const [createDeployment, { isLoading }] = useCreateDeploymentMutation()
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      await createDeployment({
        serviceId,
        version,
        commitHash,
        projectId,
        environment: "prod",
      }).unwrap()
      setSuccess(true)
      setTimeout(onClose, 1500)
    } catch {
      setError("Failed to notify deployment. Check your inputs and try again.")
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        className="w-full max-w-md bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-2xl"
      >
        <div className="flex items-center justify-between p-5 border-b border-slate-200 dark:border-slate-700">
          <div className="flex items-center gap-2">
            <Rocket className="w-5 h-5 text-blue-500" />
            <h2 className="font-semibold text-slate-900 dark:text-white">Notify Deployment</h2>
          </div>
          <button onClick={onClose} className="p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-400 transition-colors">
            <X className="w-4 h-4" />
          </button>
        </div>
        <form onSubmit={handleSubmit} className="p-5 space-y-4">
          <div>
            <label className="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1">Service ID</label>
            <input
              required
              value={serviceId}
              onChange={(e) => setServiceId(e.target.value)}
              placeholder="e.g. payments-service"
              className="w-full text-sm px-3 py-2 rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-blue-500 text-slate-900 dark:text-white"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1">Version</label>
            <input
              required
              value={version}
              onChange={(e) => setVersion(e.target.value)}
              placeholder="e.g. v1.2.3 or 1.2.3"
              className="w-full text-sm px-3 py-2 rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-blue-500 text-slate-900 dark:text-white"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1">Commit Hash</label>
            <input
              required
              value={commitHash}
              onChange={(e) => setCommitHash(e.target.value)}
              placeholder="e.g. a1b2c3d4"
              className="w-full text-sm px-3 py-2 rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-blue-500 text-slate-900 dark:text-white font-mono"
            />
          </div>
          {error && (
            <p className="text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 px-3 py-2 rounded-lg">{error}</p>
          )}
          {success && (
            <p className="text-xs text-green-600 dark:text-green-400 bg-green-50 dark:bg-green-900/20 px-3 py-2 rounded-lg">
              Deployment notified! Health gate open for 5 minutes.
            </p>
          )}
          <div className="flex gap-2 pt-1">
            <Button type="submit" variant="primary" disabled={isLoading} className="flex-1">
              {isLoading ? "Notifying…" : "Notify Deploy"}
            </Button>
            <Button type="button" variant="outline" onClick={onClose}>Cancel</Button>
          </div>
        </form>
      </motion.div>
    </div>
  )
}

const STATUSES: Array<DeploymentStatus | ""> = ["", "deploying", "healthy", "degraded", "rolled_back"]

export default function DeploymentsPage() {
  const { tenantId } = useTenant()
  const router = useRouter()
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)
  const [serviceFilter, setServiceFilter] = useState<string>("")
  const [statusFilter, setStatusFilter] = useState<DeploymentStatus | "">("")
  const [showModal, setShowModal] = useState(false)

  const { data: projectsData } = useListProjectsQuery(tenantId ?? "", { skip: !tenantId })

  const { data, isLoading, isError, refetch } = useListDeploymentsQuery(
    {
      projectId: selectedProjectId ?? "",
      serviceId: serviceFilter || undefined,
    },
    { skip: !selectedProjectId }
  )

  // Fetch incidents for the correlation timeline
  const { data: incidentsData } = useListIncidentsQuery(
    { projectId: selectedProjectId ?? "", limit: 100 },
    { skip: !selectedProjectId }
  )

  // Sort by deployedAt DESC and apply status filter
  const deployments: Deployment[] = [...(data?.deployments ?? [])]
    .sort((a, b) => new Date(b.deployedAt).getTime() - new Date(a.deployedAt).getTime())
    .filter((d) => !statusFilter || d.status === statusFilter)

  // Unique service names for filter dropdown
  const serviceNames = Array.from(new Set((data?.deployments ?? []).map((d) => d.serviceId)))

  // Build unified timeline (last 7 days)
  const sevenDaysAgo = Date.now() - 7 * 24 * 60 * 60 * 1000
  const timelineEvents: TimelineEvent[] = useMemo(() => {
    const deployEvents: TimelineEvent[] = (data?.deployments ?? []).map((d) => ({
      time: new Date(d.deployedAt).getTime(),
      type: "deployment" as const,
      label: `${d.serviceName || d.serviceId} ${d.version}`,
      status: d.status,
      id: d.id,
    }))
    const incidentEvents: TimelineEvent[] = (incidentsData?.incidents ?? []).map((i) => ({
      time: typeof i.detectedAt === "number" ? i.detectedAt : new Date(i.detectedAt as string).getTime(),
      type: "incident" as const,
      label: `${i.service} – ${i.status}`,
      status: i.status,
      id: i.id,
    }))
    return [...deployEvents, ...incidentEvents]
      .filter((e) => e.time >= sevenDaysAgo)
      .sort((a, b) => b.time - a.time)
  }, [data?.deployments, incidentsData?.incidents, sevenDaysAgo])

  return (
    <div>
      <motion.div
        initial={{ opacity: 0, y: 16 }}
        animate={{ opacity: 1, y: 0 }}
        className="flex flex-wrap items-start justify-between gap-4 mb-8"
      >
        <div>
          <h1 className="text-3xl font-bold text-slate-900 dark:text-white">Deployments</h1>
          <p className="text-slate-500 dark:text-slate-400 mt-1">
            Track service deployments and their health gate status
          </p>
        </div>
        {selectedProjectId && (
          <Button variant="primary" onClick={() => setShowModal(true)}>
            <Plus className="w-4 h-4 mr-1.5" /> Notify Deploy
          </Button>
        )}
      </motion.div>

      {/* Deployment × Incident correlation timeline */}
      {selectedProjectId && timelineEvents.length > 0 && (
        <div className="mb-8">
          <h2 className="text-base font-semibold text-slate-900 dark:text-white mb-3">
            Deployment &amp; Incident Timeline
            <span className="ml-2 text-xs font-normal text-slate-400">last 7 days</span>
          </h2>
          <div className="rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 divide-y divide-slate-100 dark:divide-slate-800 overflow-hidden">
            {timelineEvents.slice(0, 30).map((event, idx) => {
              // Detect possible correlation: incident within 30 min after a deploy
              const prevEvent = timelineEvents[idx - 1]
              const isPossibleCorrelation =
                event.type === "incident" &&
                prevEvent?.type === "deployment" &&
                event.time - prevEvent.time <= 30 * 60 * 1000 &&
                event.time >= prevEvent.time

              return (
                <div
                  key={`${event.type}-${event.id}`}
                  className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/60 transition-colors"
                  onClick={() =>
                    router.push(
                      event.type === "deployment"
                        ? `/dashboard/deployments`
                        : `/dashboard/incidents/${event.id}?project=${selectedProjectId}`
                    )
                  }
                >
                  <span className="text-base shrink-0" aria-hidden>
                    {event.type === "deployment" ? "🚀" : "🚨"}
                  </span>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-slate-900 dark:text-white truncate">
                      {event.label}
                    </p>
                    <p className="text-xs text-slate-400">{relativeTime(event.time)}</p>
                  </div>
                  {isPossibleCorrelation && (
                    <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300 shrink-0">
                      ⚠ Possible correlation
                    </span>
                  )}
                  <span
                    className={`text-xs px-2 py-0.5 rounded-full shrink-0 ${
                      event.status === "degraded" || event.status === "OPEN"
                        ? "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300"
                        : event.status === "healthy" || event.status === "RESOLVED"
                        ? "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300"
                        : "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400"
                    }`}
                  >
                    {event.status}
                  </span>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Health gate explanation */}
      <div className="mb-5 flex items-start gap-2 px-4 py-3 rounded-xl bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 text-sm text-blue-800 dark:text-blue-200">
        <Info className="w-4 h-4 mt-0.5 flex-shrink-0" />
        <span>
          <strong>Health gate:</strong> 5 min window after each deploy to detect faults. If error rate spikes during this window, the deployment is automatically marked as <em>degraded</em>.
        </span>
      </div>

      {/* Filters row */}
      <div className="flex flex-wrap gap-3 mb-6">
        {/* Project picker */}
        {(projectsData?.projects?.length ?? 0) > 0 && (
          <select
            value={selectedProjectId ?? ""}
            onChange={(e) => setSelectedProjectId(e.target.value || null)}
            className="text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-700 dark:text-slate-300"
          >
            <option value="">Select a project…</option>
            {projectsData?.projects?.map((p) => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
        )}

        {/* Service filter */}
        {selectedProjectId && serviceNames.length > 0 && (
          <select
            value={serviceFilter}
            onChange={(e) => setServiceFilter(e.target.value)}
            className="text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-700 dark:text-slate-300"
          >
            <option value="">All services</option>
            {serviceNames.map((svc) => (
              <option key={svc} value={svc}>{svc}</option>
            ))}
          </select>
        )}

        {/* Status chips */}
        {selectedProjectId && (
          <div className="flex gap-2 flex-wrap">
            {STATUSES.map((s) => (
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

        {selectedProjectId && (
          <button
            onClick={() => refetch()}
            className="ml-auto px-3 py-1.5 text-xs font-semibold rounded-lg bg-blue-600 text-white hover:bg-blue-700 transition-colors"
          >
            Refresh
          </button>
        )}
      </div>

      {/* Content */}
      {!selectedProjectId ? (
        <EmptyState
          icon={<Rocket className="w-12 h-12" />}
          title="Select a project to view deployments"
          description="Choose a project from the dropdown above to see its deployment history."
        />
      ) : isLoading ? (
        <Loading text="Loading deployments…" />
      ) : isError ? (
        <ErrorState message="Could not load deployments." retry={refetch} />
      ) : deployments.length === 0 ? (
        <EmptyState
          icon={<Rocket className="w-8 h-8" />}
          title="No deployments found"
          description={statusFilter ? `No ${statusFilter} deployments.` : "No deployments recorded yet. Use the 'Notify Deploy' button to register a deployment."}
        />
      ) : (
        <div className="space-y-3">
          {deployments.map((dep, idx) => (
            <motion.div
              key={dep.id}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.03 }}
            >
              <Card className="p-5">
                <div className="flex items-start justify-between gap-4">
                  <div className="flex-1 min-w-0">
                    <div className="flex flex-wrap items-center gap-2 mb-2">
                      <span className={`px-2.5 py-0.5 rounded-full text-xs font-semibold ${STATUS_COLORS[dep.status]}`}>
                        {dep.status}
                      </span>
                      <span className="text-xs text-slate-400 flex items-center gap-1">
                        <Clock className="w-3 h-3" />
                        <ClientDate iso={dep.deployedAt} />
                      </span>
                    </div>

                    <div className="flex flex-wrap items-center gap-3">
                      <p className="font-semibold text-slate-900 dark:text-white">
                        {dep.serviceName || dep.serviceId}
                      </p>
                      <span className="text-sm font-mono bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded text-slate-700 dark:text-slate-300">
                        {dep.version}
                      </span>
                      {dep.commitHash && (
                        <span className="text-xs font-mono text-slate-400">
                          {dep.commitHash.slice(0, 7)}
                        </span>
                      )}
                    </div>

                    {dep.deployedBy && (
                      <p className="text-xs text-slate-500 mt-1.5 flex items-center gap-1">
                        <User className="w-3 h-3" />
                        {dep.deployedBy}
                      </p>
                    )}
                    {dep.notes && (
                      <p className="text-xs text-slate-400 mt-1 italic">{dep.notes}</p>
                    )}
                  </div>
                </div>
              </Card>
            </motion.div>
          ))}
        </div>
      )}

      {showModal && selectedProjectId && (
        <NotifyModal
          projectId={selectedProjectId}
          onClose={() => {
            setShowModal(false)
            refetch()
          }}
        />
      )}
    </div>
  )
}
