"use client"

import { useState, useMemo } from "react"
import { useAuth } from "@/providers/auth-provider"
import { useTenant } from "@/contexts/tenant-context"
import {
  useListProjectsQuery,
  useListTenantIncidentsQuery,
  useGetCurrentGraphQuery,
  useListIncidentsQuery,
  useGetSignalHistoryBatchQuery,
} from "@/store/services"
import { Card } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Loading } from "@/components/ui/states"
import { motion } from "framer-motion"
import {
  FolderOpen,
  AlertCircle,
  CheckCircle2,
  GitBranch,
  ArrowRight,
  Plus,
  Activity,
  Database,
  Server,
  Layers,
  ExternalLink,
} from "lucide-react"
import Link from "next/link"
import dynamic from "next/dynamic"
import type { GraphStatus, IncidentPhase, NodeType } from "@/types/api"
import ClientDate from "@/components/ClientDate"
import { ErrorBoundary } from "@/components/ErrorBoundary"

const ServiceSparkline = dynamic(
  () => import("@/components/ServiceSparkline").then(m => ({ default: m.ServiceSparkline })),
  { ssr: false, loading: () => <div className="h-8 w-full animate-pulse bg-muted rounded" /> }
)

const GRAPH_STATUS_BADGE: Record<GraphStatus, "success" | "warning" | "secondary"> = {
  PUBLISHED: "success",
  DRAFT:     "warning",
  ARCHIVED:  "secondary",
}

const NODE_TYPE_ICON: Record<NodeType, React.ReactNode> = {
  SERVICE:  <Server className="w-3.5 h-3.5" />,
  DATABASE: <Database className="w-3.5 h-3.5" />,
  QUEUE:    <Layers className="w-3.5 h-3.5" />,
  GATEWAY:  <Activity className="w-3.5 h-3.5" />,
  EXTERNAL: <ExternalLink className="w-3.5 h-3.5" />,
}

const SOP_ACTIVE_PHASES: IncidentPhase[] = ["FIXING", "VERIFYING", "TRIAGING"]

function ServiceHealthGrid({ projectId }: { projectId: string }) {
  const { data: graph } = useGetCurrentGraphQuery(
    { projectId, environment: "prod" },
    { skip: !projectId, pollingInterval: 60000 }
  )
  const { data: incidentsData } = useListIncidentsQuery(
    { projectId, limit: 50 },
    { skip: !projectId, pollingInterval: 15000 }
  )

  const nodes = graph?.nodes ?? []
  const incidents = incidentsData?.incidents ?? []

  // Collect all node IDs for batch signal-history query (avoids N+1)
  const serviceIds = nodes.map((n) => n.id)
  const { data: batchSignalData } = useGetSignalHistoryBatchQuery(
    { projectId, services: serviceIds, hours: 1 },
    { skip: !projectId || serviceIds.length === 0, pollingInterval: 60000 }
  )

  // Build a map of service → open incidents
  const incidentsByService = useMemo(
    () => incidents.reduce<Record<string, typeof incidents>>((acc, inc) => {
      if (inc.service) {
        acc[inc.service] = acc[inc.service] ?? []
        acc[inc.service].push(inc)
      }
      return acc
    }, {}),
    [incidents]
  )

  if (!nodes.length) {
    return (
      <p className="text-sm text-slate-400 dark:text-slate-500 text-center py-6">
        No services found — onboard a project to see health data.
      </p>
    )
  }

  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 xl:grid-cols-4 gap-3">
      {nodes.map((node) => {
        const nodeIncidents = incidentsByService[node.id] ?? []
        const openCount = nodeIncidents.filter((i) => i.status === "OPEN").length
        const status = node.runtimeState?.statusClass ?? "2xx"
        const isUnhealthy = status !== "2xx" && status !== ""
        const activeSOP = nodeIncidents.find(
          (i) => i.phase && SOP_ACTIVE_PHASES.includes(i.phase as IncidentPhase)
        )

        return (
          <Link
            key={node.id}
            href={`/dashboard/graph?project=${projectId}&node=${node.id}`}
            className="block"
          >
            <Card className={`p-3 cursor-pointer transition-all hover:shadow-md ${
              isUnhealthy
                ? "border-red-300 dark:border-red-700 bg-red-50/30 dark:bg-red-950/10"
                : "hover:border-blue-300 dark:hover:border-blue-600"
            }`}>
              <div className="flex items-start justify-between gap-2 mb-2">
                <div className="flex items-center gap-1.5 min-w-0">
                  <span className={`flex-shrink-0 ${isUnhealthy ? "text-red-500" : "text-slate-400"}`}>
                    {NODE_TYPE_ICON[node.type as NodeType] ?? <Server className="w-3.5 h-3.5" />}
                  </span>
                  <p className="text-xs font-semibold text-slate-800 dark:text-slate-100 truncate">
                    {node.name}
                  </p>
                </div>
                <span className={`flex-shrink-0 w-2 h-2 rounded-full mt-0.5 ${
                  isUnhealthy ? "bg-red-500 animate-pulse" : "bg-green-400"
                }`} />
              </div>

              <div className="space-y-1">
                {status && (
                  <p className={`text-[10px] font-mono px-1.5 py-0.5 rounded inline-block ${
                    isUnhealthy
                      ? "bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300"
                      : "bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300"
                  }`}>
                    {status}
                  </p>
                )}
                {openCount > 0 && (
                  <p className="text-[10px] text-red-600 dark:text-red-400 font-medium">
                    {openCount} open incident{openCount !== 1 ? "s" : ""}
                  </p>
                )}
                {activeSOP && (
                  <p className="text-[10px] text-blue-600 dark:text-blue-400 font-medium flex items-center gap-1">
                    <span className="inline-block w-1.5 h-1.5 rounded-full bg-blue-500 animate-pulse" />
                    SOP: {activeSOP.phase}
                  </p>
                )}
                <ServiceSparkline
                  projectId={projectId}
                  serviceId={node.id}
                  height={30}
                  preloadedData={batchSignalData?.[node.id]}
                />
              </div>
            </Card>
          </Link>
        )
      })}
    </div>
  )
}

export default function DashboardPage() {
  const { user } = useAuth()
  const { tenantId } = useTenant()
  const [healthProjectId, setHealthProjectId] = useState<string>("")

  const { data: projectsData, isLoading: projectsLoading } = useListProjectsQuery(
    tenantId ?? "",
    { skip: !tenantId }
  )

  // Tenant-wide incidents (all projects)
  const { data: tenantIncidents, isLoading: incidentsLoading } = useListTenantIncidentsQuery(
    { tenantId: tenantId ?? "" },
    { skip: !tenantId, pollingInterval: 30000 }
  )

  const projects = projectsData?.projects ?? []
  const totalProjects = projectsData?.total ?? 0
  const allIncidents = tenantIncidents?.incidents ?? []

  const { openIncidents, resolvedIncidents } = useMemo(() => {
    let open = 0, resolved = 0
    for (const i of allIncidents) {
      if (i.status === "OPEN") open++
      else if (i.status === "RESOLVED") resolved++
    }
    return { openIncidents: open, resolvedIncidents: resolved }
  }, [allIncidents])

  const publishedGraphs = useMemo(
    () => projects.filter((p) => p.graphStatus === "PUBLISHED").length,
    [projects]
  )

  // Default health project to first with a published graph, else first project
  const defaultHealthProject = projects.find((p) => p.graphStatus === "PUBLISHED")?.id ?? projects[0]?.id ?? ""
  const activeHealthProject = healthProjectId || defaultHealthProject

  return (
    <div>
      <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} className="mb-8">
        <h1 className="text-3xl font-bold text-slate-900 dark:text-white">
          Welcome back{user?.firstName ? `, ${user.firstName}` : ""}!
        </h1>
        <p className="text-slate-500 dark:text-slate-400 mt-1">
          Here&apos;s what&apos;s happening across your services.
        </p>
      </motion.div>

      {/* Stat tiles */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {[
          {
            label: "Projects",
            value: projectsLoading ? "…" : totalProjects,
            icon: <FolderOpen className="w-5 h-5" />,
            color: "text-blue-600 bg-blue-50 dark:bg-blue-900/20",
          },
          {
            label: "Open Incidents",
            value: incidentsLoading ? "…" : openIncidents,
            icon: <AlertCircle className="w-5 h-5" />,
            color: openIncidents > 0
              ? "text-red-600 bg-red-50 dark:bg-red-900/20"
              : "text-green-600 bg-green-50 dark:bg-green-900/20",
          },
          {
            label: "Resolved",
            value: incidentsLoading ? "…" : resolvedIncidents,
            icon: <CheckCircle2 className="w-5 h-5" />,
            color: "text-green-600 bg-green-50 dark:bg-green-900/20",
          },
          {
            label: "Published Graphs",
            value: projectsLoading ? "…" : publishedGraphs,
            icon: <GitBranch className="w-5 h-5" />,
            color: "text-violet-600 bg-violet-50 dark:bg-violet-900/20",
          },
        ].map((stat, i) => (
          <motion.div
            key={stat.label}
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: i * 0.07 }}
          >
            <Card className="p-5">
              <div className="flex items-start justify-between mb-3">
                <p className="text-xs text-slate-400 uppercase tracking-wide">{stat.label}</p>
                <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${stat.color}`}>
                  {stat.icon}
                </div>
              </div>
              <p className="text-3xl font-bold text-slate-900 dark:text-white">{stat.value}</p>
            </Card>
          </motion.div>
        ))}
      </div>

      {/* Service Health with project switcher */}
      <motion.div
        initial={{ opacity: 0, y: 16 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.28 }}
        className="mb-8"
      >
        <div className="flex items-center justify-between mb-4">
          <h2 className="font-semibold text-slate-900 dark:text-white">Service Health</h2>
          <div className="flex items-center gap-3">
            {projects.length > 1 && (
              <select
                value={activeHealthProject}
                onChange={(e) => setHealthProjectId(e.target.value)}
                className="text-xs border border-slate-200 dark:border-slate-700 rounded-md px-2 py-1.5 bg-white dark:bg-slate-900 text-slate-700 dark:text-slate-300"
              >
                {projects.map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
            )}
            <Link href={`/dashboard/graph?project=${activeHealthProject}`}>
              <Button variant="ghost" size="sm">
                View Graph <ArrowRight className="w-3.5 h-3.5 ml-1" />
              </Button>
            </Link>
          </div>
        </div>
        {projectsLoading ? (
          <Loading size="sm" />
        ) : (
          <ErrorBoundary context="Service Health">
            <ServiceHealthGrid projectId={activeHealthProject} />
          </ErrorBoundary>
        )}
      </motion.div>

      {/* Projects + Recent Incidents */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <motion.div initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.35 }}>
          <div className="flex items-center justify-between mb-4">
            <h2 className="font-semibold text-slate-900 dark:text-white">Projects</h2>
            <div className="flex gap-2">
              <Link href="/dashboard/onboarding">
                <Button variant="outline" size="sm">
                  <Plus className="w-3.5 h-3.5 mr-1" /> New
                </Button>
              </Link>
              <Link href="/dashboard/projects">
                <Button variant="ghost" size="sm">
                  View all <ArrowRight className="w-3.5 h-3.5 ml-1" />
                </Button>
              </Link>
            </div>
          </div>

          {projectsLoading ? (
            <Loading size="sm" />
          ) : !projects.length ? (
            <Card className="p-8 text-center">
              <p className="text-slate-500 text-sm mb-3">No projects yet.</p>
              <Link href="/dashboard/onboarding">
                <Button variant="primary" size="sm">Create first project</Button>
              </Link>
            </Card>
          ) : (
            <div className="space-y-3">
              {projects.slice(0, 5).map((project) => (
                <Link key={project.id} href={`/dashboard/graph?project=${project.id}`}>
                  <Card className="p-4 hover:border-blue-300 dark:hover:border-blue-600 cursor-pointer transition-all">
                    <div className="flex items-center justify-between">
                      <div>
                        <p className="font-semibold text-slate-900 dark:text-white">{project.name}</p>
                        <p className="text-xs text-slate-400 mt-0.5">
                          {project.incidentCount ?? 0} incident{project.incidentCount !== 1 ? "s" : ""}
                        </p>
                      </div>
                      <Badge variant={GRAPH_STATUS_BADGE[project.graphStatus ?? "DRAFT"]}>
                        {project.graphStatus ?? "DRAFT"}
                      </Badge>
                    </div>
                  </Card>
                </Link>
              ))}
            </div>
          )}
        </motion.div>

        <motion.div initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.42 }}>
          <div className="flex items-center justify-between mb-4">
            <h2 className="font-semibold text-slate-900 dark:text-white">Recent Incidents</h2>
            <Link href="/dashboard/incidents">
              <Button variant="ghost" size="sm">
                View all <ArrowRight className="w-3.5 h-3.5 ml-1" />
              </Button>
            </Link>
          </div>

          {incidentsLoading ? (
            <Loading size="sm" />
          ) : !allIncidents.length ? (
            <Card className="p-8 text-center">
              <CheckCircle2 className="w-8 h-8 text-green-500 mx-auto mb-2" />
              <p className="text-slate-500 text-sm">No incidents detected.</p>
            </Card>
          ) : (
            <div className="space-y-3">
              {allIncidents.slice(0, 5).map((inc) => (
                <Link key={inc.id} href={`/dashboard/incidents/${inc.id}`}>
                  <Card className="p-4 hover:border-blue-300 dark:hover:border-blue-600 cursor-pointer transition-all">
                    <div className="flex items-center gap-3">
                      <div className={`w-2 h-2 rounded-full shrink-0 ${
                        inc.status === "OPEN"
                          ? "bg-red-500"
                          : inc.status === "ACKNOWLEDGED"
                            ? "bg-yellow-500"
                            : "bg-green-500"
                      }`} />
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-semibold text-slate-900 dark:text-white truncate">
                          {inc.service}
                        </p>
                        <p className="text-xs text-slate-400">
                          <ClientDate iso={inc.detectedAt} />
                        </p>
                      </div>
                      <div className="flex items-center gap-1.5">
                        {inc.phase && inc.phase !== "RESOLVED" && (
                          <span className={`text-[10px] font-semibold px-1.5 py-0.5 rounded ${
                            (["FIXING","VERIFYING","TRIAGING"] as IncidentPhase[]).includes(inc.phase as IncidentPhase)
                              ? "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300"
                              : "bg-slate-100 text-slate-500 dark:bg-slate-800"
                          }`}>
                            {inc.phase}
                          </span>
                        )}
                        <Badge
                          variant={
                            inc.status === "OPEN"
                              ? "destructive"
                              : inc.status === "ACKNOWLEDGED"
                                ? "warning"
                                : "success"
                          }
                          className="text-xs"
                        >
                          {inc.status}
                        </Badge>
                      </div>
                    </div>
                  </Card>
                </Link>
              ))}
            </div>
          )}
        </motion.div>
      </div>
    </div>
  )
}
