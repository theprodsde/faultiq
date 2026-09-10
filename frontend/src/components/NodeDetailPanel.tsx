"use client"

import { useState, useMemo } from "react"
import { X, AlertCircle, Activity, Clock, TrendingUp, GitBranch, ExternalLink, Wrench, CheckCircle2, Zap, Code, ArrowRight, ArrowLeft, Database, Layers, Server, Globe, Rocket, ChevronRight, ShieldOff } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { CodeContextPanel } from "@/components/CodeContextPanel"
import { useGetRequestPathQuery, useListServiceDeploymentsQuery, useUpdateDeploymentMutation, useCreateMaintenanceMutation } from "@/store/services"
import type { ServiceNode, ServiceEdge, IncidentListItem, IncidentPhase } from "@/types/api"
import Link from "next/link"

const SOP_ACTIVE_PHASES: IncidentPhase[] = ["FIXING", "VERIFYING", "TRIAGING"]

interface NodeDetailPanelProps {
  node: ServiceNode | null
  incidents: IncidentListItem[]
  onClose: () => void
  projectId?: string
  // Pass the full graph so we can show dependencies and callers
  allNodes?: Record<string, ServiceNode>
  edges?: ServiceEdge[]
}

const TYPE_LABELS: Record<string, string> = {
  SERVICE: "Microservice",
  DATABASE: "Database",
  QUEUE:    "Message Queue",
  GATEWAY:  "API Gateway",
  EXTERNAL: "External System",
}

const TYPE_ICON: Record<string, React.ReactNode> = {
  SERVICE:  <Server className="w-3 h-3" />,
  DATABASE: <Database className="w-3 h-3" />,
  QUEUE:    <Layers className="w-3 h-3" />,
  GATEWAY:  <Activity className="w-3 h-3" />,
  EXTERNAL: <Globe className="w-3 h-3" />,
}

const STATUS_COLORS: Record<string, string> = {
  "2xx": "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300",
  "5xx": "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300",
  "timeout": "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300",
  "connection_error": "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300",
  "down": "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300",
  "503": "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300",
  "504": "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300",
  "degraded": "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300",
}

type Tab = "overview" | "dependencies" | "code"

function NodePill({ nodeId, node, onClick }: { nodeId: string; node?: ServiceNode; onClick?: () => void }) {
  const status = node?.runtimeState?.statusClass || "2xx"
  const isUnhealthy = status !== "2xx" && status !== ""
  return (
    <button
      onClick={onClick}
      className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium border transition-colors ${
        isUnhealthy
          ? "border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 hover:bg-red-100"
          : "border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800 text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-700"
      }`}
    >
      {isUnhealthy && <span className="w-1.5 h-1.5 rounded-full bg-red-500 animate-pulse" />}
      {TYPE_ICON[node?.type || "SERVICE"]}
      {node?.name || nodeId}
    </button>
  )
}

function DeploymentRow({ dep }: { dep: import("@/types/api").Deployment }) {
  const [updateDeployment, { isLoading }] = useUpdateDeploymentMutation()
  const canAction = dep.status === "deploying" || dep.status === "degraded"
  return (
    <div className={`p-2 rounded-lg border text-xs flex flex-col gap-2 ${
      dep.status === "degraded"
        ? "border-red-200 dark:border-red-800 bg-red-50/50 dark:bg-red-900/10"
        : dep.status === "healthy"
        ? "border-green-200 dark:border-green-800 bg-green-50/50 dark:bg-green-900/10"
        : "border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800"
    }`}>
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <span className="font-mono font-medium text-slate-700 dark:text-slate-300">{dep.version || dep.commitHash?.slice(0,7)}</span>
          {dep.deployedBy && <span className="ml-2 text-slate-400">by {dep.deployedBy}</span>}
          <div className="text-slate-400 mt-0.5">{dep.deployedAt}</div>
        </div>
        <span className={`px-1.5 py-0.5 rounded text-[10px] font-semibold flex-shrink-0 ${
          dep.status === "degraded" ? "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300"
          : dep.status === "healthy" ? "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300"
          : dep.status === "deploying" ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300"
          : "bg-slate-100 text-slate-500"
        }`}>{dep.status}</span>
      </div>
      {canAction && (
        <div className="flex gap-1.5">
          <button
            onClick={() => updateDeployment({ deploymentId: dep.id, status: "healthy" })}
            disabled={isLoading}
            className="flex-1 py-1 text-[10px] font-semibold rounded bg-green-600 hover:bg-green-700 text-white transition-colors disabled:opacity-50"
          >
            Mark Healthy
          </button>
          <button
            onClick={() => updateDeployment({ deploymentId: dep.id, status: "rolled_back" })}
            disabled={isLoading}
            className="flex-1 py-1 text-[10px] font-semibold rounded border border-red-300 dark:border-red-700 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950/20 transition-colors disabled:opacity-50"
          >
            Mark Rolled Back
          </button>
        </div>
      )}
    </div>
  )
}

export function NodeDetailPanel({ node, incidents, onClose, projectId, allNodes = {}, edges = [] }: NodeDetailPanelProps) {
  const [activeTab, setActiveTab] = useState<Tab>("overview")
  const [showMaintForm, setShowMaintForm] = useState(false)
  const [maintDuration, setMaintDuration] = useState<15 | 30 | 60>(15)
  const [maintReason, setMaintReason] = useState("")
  const [createMaintenance, { isLoading: isMaintLoading, isSuccess: isMaintSuccess }] = useCreateMaintenanceMutation()

  const handleMaintenanceSubmit = async () => {
    if (!projectId) return
    await createMaintenance({
      projectId,
      serviceId: node?.id,
      durationMinutes: maintDuration,
      reason: maintReason || undefined,
    })
    setShowMaintForm(false)
    setMaintReason("")
  }

  if (!node) return null

  const status = node.runtimeState?.statusClass || "2xx"
  const errorRate = node.runtimeState?.errorRate || 0
  const latency = node.runtimeState?.latencyP95 || 0
  const isHealthy = status === "2xx" && errorRate < 0.1
  const isDegraded = !isHealthy && errorRate < 0.5 && (status === "2xx" || status === "")
  const isUnhealthy = !isHealthy && !isDegraded

  // Compute direct dependencies (what this node calls → outbound)
  const { callsIds, calledByIds } = useMemo(() => ({
    callsIds:    edges.filter(e => e.from === node.id).map(e => e.to),
    calledByIds: edges.filter(e => e.to   === node.id).map(e => e.from),
  }), [edges, node.id])

  // Request path: how does a user request reach this service?
  const { data: requestPath } = useGetRequestPathQuery(
    { projectId: projectId!, serviceId: node.id },
    { skip: !projectId || activeTab !== "dependencies" }
  )
  // Recent deployments for this service
  const { data: deploymentsData } = useListServiceDeploymentsQuery(
    { projectId: projectId!, serviceId: node.id },
    { skip: !projectId || activeTab !== "dependencies" }
  )

  const { nodeIncidents, openIncidents, wasTransient } = useMemo(() => {
    const nodeIncidents = incidents.filter(
      (inc) => inc.service === node.name || inc.service === node.id ||
               inc.rootCauseCandidate === node.name || inc.rootCauseCandidate === node.id
    )
    const openIncidents = nodeIncidents.filter((i) => i.status === "OPEN")
    const wasTransient = isHealthy &&
      nodeIncidents.some(i => i.status === "RESOLVED") &&
      openIncidents.length === 0
    return { nodeIncidents, openIncidents, wasTransient }
  }, [incidents, node.id, node.name, isHealthy])

  const TABS: { id: Tab; label: string; badge?: number }[] = [
    { id: "overview",     label: "Overview" },
    { id: "dependencies", label: "Dependencies", badge: callsIds.length + calledByIds.length },
    { id: "code",         label: "Code" },
  ]

  return (
    <div className="absolute top-4 right-4 z-50 w-[440px] max-h-[calc(100%-2rem)] overflow-y-auto bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl shadow-2xl">
      {/* Header */}
      <div className="sticky top-0 bg-white dark:bg-slate-900 border-b border-slate-200 dark:border-slate-700 p-4 z-10">
        <div className="flex items-start justify-between gap-3 mb-3">
          <div className="min-w-0">
            <h3 className="font-bold text-slate-900 dark:text-white truncate">{node.name || node.id}</h3>
            <p className="text-xs text-slate-500 mt-0.5">
              {TYPE_LABELS[node.type] || node.type}
              {openIncidents.length > 0 && (
                <span className="ml-2 text-red-500 font-medium">{openIncidents.length} open incident{openIncidents.length !== 1 ? "s" : ""}</span>
              )}
            </p>
          </div>
          <button onClick={onClose} className="p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-colors shrink-0">
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Tab bar */}
        <div className="flex gap-1">
          {TABS.map(tab => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors ${
                activeTab === tab.id
                  ? "bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300"
                  : "text-slate-500 hover:text-slate-700 dark:hover:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800"
              }`}
            >
              {tab.label}
              {tab.badge != null && tab.badge > 0 && (
                <span className="text-[10px] font-bold bg-slate-200 dark:bg-slate-700 text-slate-600 dark:text-slate-400 px-1 rounded-full">
                  {tab.badge}
                </span>
              )}
            </button>
          ))}
        </div>
      </div>

      <div className="p-4 space-y-4">

        {/* ── OVERVIEW TAB ── */}
        {activeTab === "overview" && (
          <>
            {/* Health */}
            <div className="flex items-center gap-2">
              <div className={`inline-flex items-center gap-2 px-3 py-1.5 rounded-full text-sm font-semibold ${
                isUnhealthy ? "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300"
                : isDegraded  ? "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300"
                : "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300"
              }`}>
                <span className={`w-2 h-2 rounded-full ${isUnhealthy ? "bg-red-500 animate-pulse" : isDegraded ? "bg-yellow-500" : "bg-green-500"}`} />
                {isUnhealthy ? "Unhealthy" : isDegraded ? "Degraded" : "Healthy"}
              </div>
              {wasTransient && (
                <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300">
                  <CheckCircle2 className="w-3 h-3" /> Auto-recovered
                </span>
              )}
            </div>

            {/* Maintenance button */}
            {projectId && (
              <div>
                {!showMaintForm ? (
                  <button
                    onClick={() => setShowMaintForm(true)}
                    className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 hover:bg-amber-50 dark:hover:bg-amber-900/20 hover:border-amber-300 dark:hover:border-amber-700 hover:text-amber-700 dark:hover:text-amber-300 transition-colors"
                  >
                    <ShieldOff className="w-3.5 h-3.5" />
                    Suppress Alerts
                  </button>
                ) : (
                  <div className="p-3 rounded-lg border border-amber-200 dark:border-amber-700 bg-amber-50 dark:bg-amber-900/20 space-y-2">
                    <p className="text-xs font-semibold text-amber-800 dark:text-amber-300 flex items-center gap-1.5">
                      <ShieldOff className="w-3.5 h-3.5" /> Suppress alerts for:
                    </p>
                    <div className="flex gap-1.5">
                      {([15, 30, 60] as const).map((d) => (
                        <button
                          key={d}
                          onClick={() => setMaintDuration(d)}
                          className={`flex-1 py-1 text-xs font-semibold rounded border transition-colors ${
                            maintDuration === d
                              ? "bg-amber-500 text-white border-amber-500"
                              : "border-amber-300 dark:border-amber-600 text-amber-700 dark:text-amber-300 hover:bg-amber-100 dark:hover:bg-amber-900/40"
                          }`}
                        >
                          {d}m
                        </button>
                      ))}
                    </div>
                    <input
                      type="text"
                      value={maintReason}
                      onChange={(e) => setMaintReason(e.target.value)}
                      placeholder="Reason (optional)"
                      className="w-full text-xs rounded border border-amber-200 dark:border-amber-700 bg-white dark:bg-slate-900 px-2 py-1.5 text-slate-900 dark:text-white placeholder-slate-400"
                    />
                    <div className="flex gap-1.5">
                      <button
                        onClick={handleMaintenanceSubmit}
                        disabled={isMaintLoading}
                        className="flex-1 py-1 text-xs font-semibold rounded bg-amber-500 hover:bg-amber-600 text-white transition-colors disabled:opacity-50"
                      >
                        {isMaintLoading ? "Saving…" : "Confirm"}
                      </button>
                      <button
                        onClick={() => setShowMaintForm(false)}
                        className="flex-1 py-1 text-xs font-semibold rounded border border-slate-300 dark:border-slate-600 text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
                      >
                        Cancel
                      </button>
                    </div>
                    {isMaintSuccess && (
                      <p className="text-xs text-green-600 dark:text-green-400 flex items-center gap-1">
                        <CheckCircle2 className="w-3 h-3" /> Maintenance window created
                      </p>
                    )}
                  </div>
                )}
              </div>
            )}

            {/* Metrics */}
            <div className="grid grid-cols-3 gap-2">
              {[
                { icon: <Activity className="w-3 h-3" />, label: "Status", value: <span className={`text-xs font-bold px-1.5 py-0.5 rounded ${STATUS_COLORS[status] || "bg-slate-100 text-slate-600"}`}>{status}</span> },
                { icon: <TrendingUp className="w-3 h-3" />, label: "Error Rate", value: <span className={`text-sm font-bold ${errorRate > 0.5 ? "text-red-600" : errorRate > 0.1 ? "text-yellow-600" : "text-green-600"}`}>{(errorRate * 100).toFixed(1)}%</span> },
                { icon: <Clock className="w-3 h-3" />, label: "P95 Latency", value: <span className={`text-sm font-bold ${latency > 3000 ? "text-red-600" : latency > 1000 ? "text-yellow-600" : "text-slate-900 dark:text-white"}`}>{latency > 1000 ? `${(latency/1000).toFixed(1)}s` : `${latency}ms`}</span> },
              ].map(m => (
                <div key={m.label} className="p-3 rounded-lg bg-slate-50 dark:bg-slate-800">
                  <div className="flex items-center gap-1 mb-1.5 text-slate-400">{m.icon}<span className="text-[10px] text-slate-500">{m.label}</span></div>
                  {m.value}
                </div>
              ))}
            </div>

            {/* Dependency mini-summary */}
            {(callsIds.length > 0 || calledByIds.length > 0) && (
              <div className="p-3 rounded-lg bg-slate-50 dark:bg-slate-800 text-xs text-slate-500 dark:text-slate-400">
                {callsIds.length > 0 && <span>Calls <strong className="text-slate-700 dark:text-slate-300">{callsIds.length}</strong> service{callsIds.length !== 1 ? "s" : ""}</span>}
                {callsIds.length > 0 && calledByIds.length > 0 && <span className="mx-2">·</span>}
                {calledByIds.length > 0 && <span>Called by <strong className="text-slate-700 dark:text-slate-300">{calledByIds.length}</strong> service{calledByIds.length !== 1 ? "s" : ""}</span>}
                <button onClick={() => setActiveTab("dependencies")} className="ml-2 text-blue-500 hover:underline">See all →</button>
              </div>
            )}

            {/* Active Incidents */}
            {openIncidents.length > 0 && (
              <div>
                <p className="text-xs font-semibold text-slate-500 uppercase tracking-wide mb-2 flex items-center gap-1.5">
                  <AlertCircle className="w-3.5 h-3.5 text-red-500" />
                  Active Incidents ({openIncidents.length})
                </p>
                <div className="space-y-2">
                  {openIncidents.slice(0, 3).map((inc) => {
                    const sopPhase = inc.phase as IncidentPhase | undefined
                    const hasActiveSOP = sopPhase && SOP_ACTIVE_PHASES.includes(sopPhase)
                    return (
                      <Link key={inc.id} href={`/dashboard/incidents/${inc.id}${projectId ? `?project=${projectId}` : ""}`}
                        className="block p-3 rounded-lg border border-slate-200 dark:border-slate-700 hover:border-blue-400 dark:hover:border-blue-500 transition-colors">
                        <div className="flex items-center justify-between mb-1">
                          <Badge variant="destructive" className="text-xs">{inc.status}</Badge>
                          <div className="flex items-center gap-2">
                            {sopPhase && (
                              <span className={`text-[10px] font-semibold px-1.5 py-0.5 rounded ${
                                hasActiveSOP ? "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300"
                                             : "bg-slate-100 text-slate-500 dark:bg-slate-800"
                              }`}>{sopPhase}</span>
                            )}
                            <span className="text-xs text-slate-400 font-mono">{Math.round(inc.confidence * 100)}%</span>
                          </div>
                        </div>
                        {inc.rootCauseCandidate && (
                          <p className="text-xs text-slate-600 dark:text-slate-400">
                            Root cause: <span className="font-medium text-orange-600 dark:text-orange-400">{inc.rootCauseCandidate}</span>
                          </p>
                        )}
                      </Link>
                    )
                  })}
                </div>
              </div>
            )}

            {isUnhealthy && (
              <div className="p-3 rounded-lg bg-orange-50 dark:bg-orange-900/20 border border-orange-200 dark:border-orange-800">
                <p className="text-xs font-semibold text-orange-800 dark:text-orange-300 mb-1 flex items-center gap-1.5">
                  <Wrench className="w-3.5 h-3.5" /> Detected root cause origin
                </p>
                <p className="text-xs text-orange-700 dark:text-orange-400">
                  BFS traversal identified this as the failure source. Upstream callers are experiencing cascading errors.
                </p>
              </div>
            )}

            {node.tags && node.tags.length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {node.tags.map(tag => (
                  <span key={tag} className="text-xs px-2 py-0.5 rounded-full bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400">{tag}</span>
                ))}
              </div>
            )}

            {projectId && (
              <Link href={`/dashboard/incidents?project=${projectId}`}
                className="flex items-center justify-center gap-2 w-full px-4 py-2.5 rounded-lg bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 text-sm font-medium hover:bg-blue-100 dark:hover:bg-blue-900/40 transition-colors">
                <ExternalLink className="w-3.5 h-3.5" /> View All Incidents
              </Link>
            )}
          </>
        )}

        {/* ── DEPENDENCIES TAB ── */}
        {activeTab === "dependencies" && (
          <div className="space-y-5">
            {/* What this service calls (outbound) */}
            <div>
              <p className="text-xs font-semibold text-slate-500 uppercase tracking-wide mb-2 flex items-center gap-1.5">
                <ArrowRight className="w-3.5 h-3.5 text-blue-500" />
                Calls ({callsIds.length})
                <span className="font-normal normal-case text-slate-400 ml-1">— services this node depends on</span>
              </p>
              {callsIds.length === 0 ? (
                <p className="text-xs text-slate-400 dark:text-slate-500 italic">No outbound dependencies</p>
              ) : (
                <div className="flex flex-wrap gap-2">
                  {callsIds.map(id => {
                    const dep = allNodes[id]
                    const depStatus = dep?.runtimeState?.statusClass || "2xx"
                    const isDepUnhealthy = depStatus !== "2xx" && depStatus !== ""
                    return (
                      <div key={id} className="flex flex-col gap-1">
                        <NodePill key={id} nodeId={id} node={dep} />
                        {isDepUnhealthy && (
                          <span className={`text-[10px] text-center px-1.5 py-0.5 rounded ${STATUS_COLORS[depStatus] || ""}`}>
                            {depStatus}
                          </span>
                        )}
                      </div>
                    )
                  })}
                </div>
              )}
            </div>

            {/* What calls this service (inbound) */}
            <div>
              <p className="text-xs font-semibold text-slate-500 uppercase tracking-wide mb-2 flex items-center gap-1.5">
                <ArrowLeft className="w-3.5 h-3.5 text-violet-500" />
                Called By ({calledByIds.length})
                <span className="font-normal normal-case text-slate-400 ml-1">— upstream callers</span>
              </p>
              {calledByIds.length === 0 ? (
                <p className="text-xs text-slate-400 dark:text-slate-500 italic">No upstream callers — this is a leaf node</p>
              ) : (
                <div className="flex flex-wrap gap-2">
                  {calledByIds.map(id => {
                    const caller = allNodes[id]
                    const callerStatus = caller?.runtimeState?.statusClass || "2xx"
                    const isCallerUnhealthy = callerStatus !== "2xx" && callerStatus !== ""
                    return (
                      <div key={id} className="flex flex-col gap-1">
                        <NodePill nodeId={id} node={caller} />
                        {isCallerUnhealthy && (
                          <span className={`text-[10px] text-center px-1.5 py-0.5 rounded ${STATUS_COLORS[callerStatus] || ""}`}>
                            {callerStatus}
                          </span>
                        )}
                      </div>
                    )
                  })}
                </div>
              )}
            </div>

            {/* Cascade risk — show if this node is unhealthy and has callers */}
            {isUnhealthy && calledByIds.length > 0 && (
              <div className="p-3 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-xs">
                <p className="font-semibold text-red-700 dark:text-red-300 mb-1">Cascade risk</p>
                <p className="text-red-600 dark:text-red-400">
                  This service is failing and has <strong>{calledByIds.length}</strong> upstream caller{calledByIds.length !== 1 ? "s" : ""}. Their errors may be caused by this node.
                </p>
              </div>
            )}

            {/* Request Path — how a user request reaches this service */}
            {requestPath && requestPath.paths.length > 0 && (
              <div>
                <p className="text-xs font-semibold text-slate-500 uppercase tracking-wide mb-2 flex items-center gap-1.5">
                  <ChevronRight className="w-3.5 h-3.5 text-green-500" />
                  Request Path ({requestPath.pathCount} path{requestPath.pathCount !== 1 ? "s" : ""})
                  <span className="font-normal normal-case text-slate-400 ml-1">— how user requests reach this service</span>
                </p>
                <div className="space-y-2">
                  {requestPath.paths.slice(0, 2).map((path, pi) => (
                    <div key={pi} className="p-2 rounded-lg bg-slate-50 dark:bg-slate-800 flex items-center gap-1 flex-wrap text-xs">
                      {path.map((step, si) => (
                        <span key={step.serviceId} className="flex items-center gap-1">
                          <span className={`px-2 py-0.5 rounded font-medium ${
                            step.serviceId === node.id
                              ? "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300"
                              : "bg-white dark:bg-slate-700 text-slate-600 dark:text-slate-300 border border-slate-200 dark:border-slate-600"
                          }`}>
                            {step.serviceName}
                          </span>
                          {si < path.length - 1 && <ArrowRight className="w-3 h-3 text-slate-400 flex-shrink-0" />}
                        </span>
                      ))}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Recent Deployments */}
            {deploymentsData && deploymentsData.deployments.length > 0 && (
              <div>
                <p className="text-xs font-semibold text-slate-500 uppercase tracking-wide mb-2 flex items-center gap-1.5">
                  <Rocket className="w-3.5 h-3.5 text-violet-500" />
                  Recent Deployments
                </p>
                <div className="space-y-2">
                  {deploymentsData.deployments.slice(0, 3).map(dep => (
                    <DeploymentRow key={dep.id} dep={dep} />
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {/* ── CODE TAB ── */}
        {activeTab === "code" && (
          <div className="space-y-3">
            <div className="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
              <Code className="w-3.5 h-3.5" />
              <span>Code context for <strong className="text-slate-700 dark:text-slate-300">{node.name}</strong></span>
            </div>

            {projectId ? (
              <CodeContextPanel
                projectId={projectId}
                serviceId={node.id}
                serviceName={node.name}
              />
            ) : (
              <p className="text-xs text-slate-400 text-center py-4">
                No project context — open from a project's graph view
              </p>
            )}

            {projectId && (
              <Link
                href={`/dashboard/projects/${projectId}?tab=code`}
                className="flex items-center justify-center gap-1.5 w-full text-xs text-blue-500 hover:underline py-1"
              >
                <GitBranch className="w-3 h-3" />
                Configure repo for this service →
              </Link>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
