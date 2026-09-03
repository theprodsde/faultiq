"use client"

import { useState, useEffect, useCallback } from "react"
import { useDispatch } from "react-redux"
import { useSearchParams } from "next/navigation"
import { useGetCurrentGraphQuery, useListProjectsQuery, useListIncidentsQuery, faultiqApi } from "@/store/services"
import { useTenant } from "@/contexts/tenant-context"
import { Card } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Loading, EmptyState, ErrorState } from "@/components/ui/states"
import { InteractiveGraphCytoscape } from "@/components/interactive-graph-cytoscape"
import { NodeDetailPanel } from "@/components/NodeDetailPanel"
import { useIncidentEvents } from "@/hooks/useIncidentEvents"
import { GitBranch, Wifi, WifiOff, CheckCircle2, AlertTriangle, Power } from "lucide-react"
import { motion } from "framer-motion"
import { Button } from "@/components/ui/button"
import { ErrorBoundary } from "@/components/ErrorBoundary"
import { useAuth } from "@/providers/auth-provider"
import { config } from "@/config"
import type { ServiceNode, ServiceEdge, NodeType, IncidentListItem } from "@/types/api"

const NODE_COLORS: Record<NodeType, string> = {
  SERVICE:  "bg-blue-100 text-blue-700 border-blue-200 dark:bg-blue-900/30 dark:text-blue-300 dark:border-blue-700",
  DATABASE: "bg-violet-100 text-violet-700 border-violet-200 dark:bg-violet-900/30 dark:text-violet-300 dark:border-violet-700",
  QUEUE:    "bg-amber-100 text-amber-700 border-amber-200 dark:bg-amber-900/30 dark:text-amber-300 dark:border-amber-700",
  GATEWAY:  "bg-cyan-100 text-cyan-700 border-cyan-200 dark:bg-cyan-900/30 dark:text-cyan-300 dark:border-cyan-700",
  EXTERNAL: "bg-slate-100 text-slate-600 border-slate-200 dark:bg-slate-800 dark:text-slate-400 dark:border-slate-600",
}

export default function GraphPage() {
  const { tenantId } = useTenant()
  const { token } = useAuth()
  const dispatch = useDispatch()
  const searchParams = useSearchParams()
  const projectFromUrl = searchParams?.get("project")
  const envFromUrl = searchParams?.get("env")

  const [selectedProjectId, setSelectedProjectId] = useState<string>("")
  const [selectedEnv, setSelectedEnv] = useState<string>("")
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [takingDown, setTakingDown] = useState<string | null>(null)

  const { data: projectsData } = useListProjectsQuery(tenantId ?? "", {
    skip: !tenantId,
  })

  // Initialize project and environment from URL or defaults
  useEffect(() => {
    if (projectFromUrl) {
      setSelectedProjectId(projectFromUrl)
    } else if (projectsData?.projects?.[0]?.id) {
      setSelectedProjectId(projectsData.projects[0].id)
    }
  }, [projectFromUrl, projectsData])

  useEffect(() => {
    const activeProject = projectsData?.projects?.find(p => p.id === selectedProjectId)
    const envs = activeProject?.environments ?? []
    
    if (envFromUrl && envs.some((e) => e.name === envFromUrl)) {
      setSelectedEnv(envFromUrl)
    } else if (envs.length > 0) {
      setSelectedEnv(envs[0].name)
    } else {
      setSelectedEnv("prod")
    }
  }, [envFromUrl, selectedProjectId, projectsData])

  const activeProjectId = selectedProjectId || projectsData?.projects?.[0]?.id || ""
  const activeProject = projectsData?.projects?.find(p => p.id === activeProjectId)
  const projectEnvironments: Array<{ id: string; name: string; namespace: string }> = activeProject?.environments || []
  const availableEnvs: Array<{ id: string; name: string; namespace: string }> = projectEnvironments.length > 0 ? projectEnvironments : [{ id: "1", name: "prod", namespace: "prod" }, { id: "2", name: "staging", namespace: "staging" }, { id: "3", name: "dev", namespace: "dev" }]

  const {
    data: graph,
    isLoading,
    isError,
    refetch,
  } = useGetCurrentGraphQuery(
    { projectId: activeProjectId, environment: selectedEnv },
    { skip: !activeProjectId || !selectedEnv, pollingInterval: 120000 } // Poll every 2 minutes to keep graph static
  )

  // Real-time SSE connection for instant incident updates (must come before incidents query)
  const { connected: sseConnected } = useIncidentEvents({
    projectId: activeProjectId,
    tenantId: tenantId ?? undefined,
    onEvent: useCallback(() => {
      // When an incident event arrives via SSE, refetch graph and force incident cache invalidation
      refetch()
      dispatch(faultiqApi.util.invalidateTags([{ type: "Incident", id: "LIST" }]))
    }, [refetch, dispatch]),
  })

  // Dynamic polling: fast when SSE disconnected (fallback), slow when SSE connected (SSE handles real-time)
  const incidentPollInterval = sseConnected ? 30000 : 5000

  // Fetch incidents for this project — used for real-time node coloring
  const { data: incidentsData, refetch: refetchIncidents } = useListIncidentsQuery(
    { projectId: activeProjectId, limit: 100 },
    { skip: !activeProjectId, pollingInterval: incidentPollInterval }
  )

  // Build incident node states to pass to graph component
  // Only show OPEN and ACKNOWLEDGED incidents (hide RESOLVED)
  const incidentStates = (incidentsData?.incidents ?? [])
    .filter(inc => inc.status !== "RESOLVED")
    .map((inc) => ({
      serviceName: inc.service,
      status: inc.status as "OPEN" | "ACKNOWLEDGED",
      confidence: inc.confidence,
      incidentId: inc.id,
    }))

  // Normalize graph nodes/edges: backend may return maps or arrays
  const nodesArray: ServiceNode[] = Array.isArray(graph?.nodes)
    ? (graph!.nodes as ServiceNode[])
    : graph?.nodes
      ? Object.values(graph.nodes as Record<string, ServiceNode>)
      : []

  const edgesArray: ServiceEdge[] = Array.isArray(graph?.edges)
    ? (graph!.edges as ServiceEdge[])
    : graph?.edges
      ? Object.entries(graph.edges as Record<string, string[]>)
          .flatMap(([from, tos], groupIdx) => (Array.isArray(tos) ? tos.map((to, i) => ({ id: `edge-${groupIdx}-${i}`, from, to, type: "CALLS" as any, confidence: 0.9 } as ServiceEdge)) : []))
      : []

  const nodeMap = Object.fromEntries(nodesArray.map((n) => [n.id, n.name]))

  // Find selected node details for the panel
  const selectedNode = selectedNodeId
    ? nodesArray.find((n) => n.id === selectedNodeId) || null
    : null

  const handleNodeSelect = useCallback((nodeId: string | null) => {
    setSelectedNodeId(nodeId)
  }, [])

  // Take down a service by sending an error signal
  const handleTakeDown = async (serviceId: string) => {
    setTakingDown(serviceId)
    try {
      if (!tenantId) {
        console.error("Tenant not configured — cannot send signal")
        return
      }
      const payload = {
        tenantId,
        projectId: activeProjectId,
        service: serviceId,
        statusClass: "5xx",
        errorRate: 0.95,
        latencyP95: 5000,
        timestamp: Date.now(),
        source: "ui-takedown",
      }
      await fetch(`${config.apiUrl}/signals`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token || ""}`,
        },
        body: JSON.stringify(payload),
      })
    } catch (err) {
      console.error("Take down signal failed:", err)
    } finally {
      setTimeout(() => setTakingDown(null), 1500)
    }
  }

  // Determine overall graph health
  const hasOpenIncidents = incidentStates.length > 0
  const graphHealthy = nodesArray.length > 0 && !hasOpenIncidents

  return (
    <div>
      <motion.div initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }}>
        {/* Header */}
        <div className="flex flex-wrap items-start justify-between gap-4 mb-8">
          <div>
            <h1 className="text-3xl font-bold text-slate-900 dark:text-white">Service Graph</h1>
            <p className="text-slate-500 dark:text-slate-400 mt-1">
              Interactive dependency visualization with real-time health metrics
            </p>
          </div>
          {graph && nodesArray.length > 0 && (
            <Badge variant="secondary" className="text-xs">
              {nodesArray.length} nodes · {selectedEnv}
            </Badge>
          )}
        </div>

        {/* Controls */}
        <div className="flex flex-wrap gap-3 mb-6">
          {(projectsData?.projects?.length ?? 0) > 0 ? (
            <select
              value={activeProjectId}
              onChange={(e) => setSelectedProjectId(e.target.value)}
              className="text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-700 dark:text-slate-300"
            >
              {projectsData?.projects?.map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
          ) : null}

          {availableEnvs.length > 0 && (
            <select
              value={selectedEnv}
              onChange={(e) => setSelectedEnv(e.target.value)}
              className="text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-700 dark:text-slate-300"
            >
              {availableEnvs.map((env) => (
                <option key={env.name} value={env.name}>
                  {env.name.charAt(0).toUpperCase() + env.name.slice(1)}
                </option>
              ))}
            </select>
          )}

          <button
            onClick={() => refetch()}
            className="px-3 py-2 text-xs font-semibold rounded-lg bg-blue-600 text-white hover:bg-blue-700 transition-colors"
          >
            Refresh
          </button>

          {/* Real-time connection indicator */}
          <div className={`flex items-center gap-1.5 px-2.5 py-1.5 rounded-full text-xs font-medium ${
            sseConnected
              ? "bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-300"
              : "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400"
          }`}>
            {sseConnected ? <Wifi className="w-3 h-3" /> : <WifiOff className="w-3 h-3" />}
            {sseConnected ? "Live" : "Polling"}
          </div>
        </div>

        {/* Content */}
        {!activeProjectId || !selectedEnv ? (
          <EmptyState
            icon={<GitBranch className="w-6 h-6" />}
            title="No project selected"
            description="Create or select a project to view its service graph."
          />
        ) : isLoading ? (
          <Loading text="Loading graph…" />
        ) : isError ? (
          <ErrorState message="Could not load graph." retry={refetch} />
        ) : !graph ? (
          <EmptyState
            icon={<GitBranch className="w-6 h-6" />}
            title="No graph available"
            description="Complete the onboarding wizard to generate a service graph for this environment."
          />
        ) : (
          <div>
            {/* Stats bar */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-6">
              {[
                { label: "Nodes", value: nodesArray.length },
                { label: "Edges", value: edgesArray.length },
                { label: "Namespace", value: graph.namespace || `${activeProject?.slug || "project"}:${selectedEnv}` },
                { label: "Status", value: graph.status || "PUBLISHED" },
              ].map((s) => (
                <Card key={s.label} className="px-4 py-3">
                  <p className="text-xs text-slate-400 uppercase tracking-wide mb-1">{s.label}</p>
                  <p className="font-bold text-slate-900 dark:text-white">{s.value}</p>
                </Card>
              ))}
            </div>

            {/* Interactive Graph */}
            {nodesArray.length > 0 ? (
              <motion.div
                initial={{ opacity: 0, scale: 0.95 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ duration: 0.5 }}
                className="relative"
              >
                {/* Graph Health Banner */}
                <div className={`mb-4 px-4 py-3 rounded-lg border flex items-center justify-between ${
                  graphHealthy
                    ? "bg-green-50 dark:bg-green-900/10 border-green-200 dark:border-green-800"
                    : "bg-red-50 dark:bg-red-900/10 border-red-200 dark:border-red-800"
                }`}>
                  <div className="flex items-center gap-2">
                    {graphHealthy ? (
                      <CheckCircle2 className="w-5 h-5 text-green-600 dark:text-green-400" />
                    ) : (
                      <AlertTriangle className="w-5 h-5 text-red-600 dark:text-red-400" />
                    )}
                    <span className={`text-sm font-semibold ${
                      graphHealthy
                        ? "text-green-800 dark:text-green-200"
                        : "text-red-800 dark:text-red-200"
                    }`}>
                      {graphHealthy
                        ? "All services healthy — no active incidents"
                        : `${incidentStates.length} service${incidentStates.length !== 1 ? "s" : ""} affected`}
                    </span>
                  </div>
                  <span className={`text-xs px-2 py-1 rounded-full font-medium ${
                    graphHealthy
                      ? "bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300"
                      : "bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300"
                  }`}>
                    {graphHealthy ? "✓ Healthy" : "⚠ Degraded"}
                  </span>
                </div>

                {/* Take Down Service Panel */}
                {!tenantId && (
                  <div className="mb-4 p-3 rounded-lg border border-amber-300 dark:border-amber-700 bg-amber-50 dark:bg-amber-900/20 text-amber-700 dark:text-amber-300 text-xs">
                    Tenant not configured — signal simulation disabled. Contact your administrator.
                  </div>
                )}
                {tenantId && nodesArray.filter(n => n.type === "SERVICE" || n.type === "QUEUE").length > 0 && (
                  <div className="mb-4 p-3 rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/50">
                    <p className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wide mb-2 flex items-center gap-1.5">
                      <Power className="w-3 h-3" />
                      Simulate Service Failure
                    </p>
                    <div className="flex flex-wrap gap-2">
                      {nodesArray
                        .filter(n => n.type === "SERVICE" || n.type === "QUEUE")
                        .map(node => (
                          <button
                            key={node.id}
                            onClick={() => handleTakeDown(node.id)}
                            disabled={takingDown === node.id}
                            className={`px-3 py-1.5 text-xs font-medium rounded-lg border transition-all ${
                              takingDown === node.id
                                ? "bg-red-600 text-white border-red-600"
                                : "border-slate-300 dark:border-slate-600 text-slate-700 dark:text-slate-300 hover:bg-red-50 dark:hover:bg-red-900/20 hover:border-red-300 dark:hover:border-red-700 hover:text-red-700 dark:hover:text-red-300"
                            }`}
                          >
                            {takingDown === node.id ? "⚡ Sent!" : node.name || node.id}
                          </button>
                        ))}
                    </div>
                  </div>
                )}

                <ErrorBoundary context="Graph Visualization">
                  <InteractiveGraphCytoscape nodes={nodesArray} edges={edgesArray} nodeMap={nodeMap} incidentStates={incidentStates} onNodeSelect={handleNodeSelect} />
                </ErrorBoundary>
                
                {/* Node Detail Panel */}
                <NodeDetailPanel
                  node={selectedNode}
                  incidents={(incidentsData?.incidents ?? []) as IncidentListItem[]}
                  onClose={() => setSelectedNodeId(null)}
                  projectId={activeProjectId}
                  allNodes={Object.fromEntries(nodesArray.map(n => [n.id, n]))}
                  edges={edgesArray}
                />
              </motion.div>
            ) : (
              <EmptyState
                icon={<GitBranch className="w-6 h-6" />}
                title="No nodes in graph"
                description="The service graph is empty. Run the seed script to populate demo data."
              />
            )}
          </div>
        )}
      </motion.div>
    </div>
  )
}
