"use client"

import { useState } from "react"
import { useParams, useRouter } from "next/navigation"
import Link from "next/link"
import {
  useGetProjectQuery,
  useUpdateProjectSettingsMutation,
  useGetProjectSettingsQuery,
  useListIncidentsQuery,
  useGetCurrentGraphQuery,
  useGetCodeIndexStatusQuery,
  useUpdateServiceSLOMutation,
} from "@/store/services"
import { GraphDiffPanel } from "@/components/GraphDiffPanel"
import { ServiceMapImport } from "@/components/ServiceMapImport"
import { Card } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Loading, ErrorState } from "@/components/ui/states"
import {
  ArrowLeft, Settings, GitBranch, Activity, AlertCircle,
  ExternalLink, Plus, Trash2, CheckCircle2, Code, Globe,
  Save, Database, Layers, Server, GitCompareArrows
} from "lucide-react"
import { motion } from "framer-motion"
import type { ServiceRepo } from "@/types/api"

type Tab = "overview" | "services" | "import" | "code" | "changes"

const NODE_TYPE_ICON: Record<string, React.ReactNode> = {
  SERVICE:  <Server className="w-4 h-4" />,
  DATABASE: <Database className="w-4 h-4" />,
  QUEUE:    <Layers className="w-4 h-4" />,
  GATEWAY:  <Activity className="w-4 h-4" />,
  EXTERNAL: <Globe className="w-4 h-4" />,
}

function ServiceRepoRow({
  serviceId,
  serviceName,
  serviceType,
  repo,
  onChange,
  onRemoveRepo,
}: {
  serviceId: string
  serviceName: string
  serviceType: string
  repo?: ServiceRepo
  onChange: (r: ServiceRepo) => void
  onRemoveRepo: () => void
}) {
  const [editing, setEditing] = useState(false)
  const [repoUrl, setRepoUrl] = useState(repo?.repoUrl ?? "")
  const [branch, setBranch] = useState(repo?.branch ?? "main")
  const [codePath, setCodePath] = useState(repo?.codePath ?? "")

  const save = () => {
    if (repoUrl.trim()) {
      onChange({ serviceId, repoUrl: repoUrl.trim(), branch: branch.trim() || "main", codePath: codePath.trim() || undefined })
    }
    setEditing(false)
  }

  return (
    <div className="p-4 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-2 min-w-0">
          <span className="text-slate-400 flex-shrink-0">
            {NODE_TYPE_ICON[serviceType] ?? <Server className="w-4 h-4" />}
          </span>
          <div className="min-w-0">
            <p className="text-sm font-semibold text-slate-800 dark:text-slate-100">{serviceName}</p>
            <p className="text-xs text-slate-400 font-mono">{serviceId}</p>
          </div>
        </div>
        <div className="flex items-center gap-2 flex-shrink-0">
          {repo?.repoUrl ? (
            <span className="inline-flex items-center gap-1 text-xs text-green-600 dark:text-green-400 bg-green-50 dark:bg-green-900/20 px-2 py-0.5 rounded-full">
              <CheckCircle2 className="w-3 h-3" /> Repo linked
            </span>
          ) : (
            <span className="text-xs text-slate-400">No repo</span>
          )}
          <button
            onClick={() => setEditing((v) => !v)}
            className="text-xs text-blue-600 dark:text-blue-400 hover:underline"
          >
            {editing ? "Cancel" : "Edit"}
          </button>
        </div>
      </div>

      {/* Existing repo info */}
      {!editing && repo?.repoUrl && (
        <div className="mt-2 pt-2 border-t border-slate-100 dark:border-slate-800 flex flex-wrap gap-3 text-xs text-slate-500">
          <a href={repo.repoUrl} target="_blank" rel="noopener" className="flex items-center gap-1 text-blue-500 hover:underline">
            <GitBranch className="w-3 h-3" />{repo.repoUrl.replace("https://github.com/", "")}
          </a>
          <span>branch: {repo.branch ?? "main"}</span>
          {repo.codePath && <span>path: {repo.codePath}</span>}
          {repo.lastIndexed && <span>last indexed: {new Date(repo.lastIndexed).toLocaleDateString()}</span>}
        </div>
      )}

      {/* Edit form */}
      {editing && (
        <div className="mt-3 pt-3 border-t border-slate-100 dark:border-slate-800 space-y-2">
          <div>
            <label className="text-xs font-medium text-slate-600 dark:text-slate-400 block mb-1">
              Repository URL
            </label>
            <input
              value={repoUrl}
              onChange={(e) => setRepoUrl(e.target.value)}
              placeholder="https://github.com/your-org/service-repo"
              className="w-full text-xs px-3 py-2 rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
          <div className="flex gap-3">
            <div className="flex-1">
              <label className="text-xs font-medium text-slate-600 dark:text-slate-400 block mb-1">Branch</label>
              <input
                value={branch}
                onChange={(e) => setBranch(e.target.value)}
                placeholder="main"
                className="w-full text-xs px-3 py-2 rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div className="flex-1">
              <label className="text-xs font-medium text-slate-600 dark:text-slate-400 block mb-1">
                Code path <span className="text-slate-400">(monolith only)</span>
              </label>
              <input
                value={codePath}
                onChange={(e) => setCodePath(e.target.value)}
                placeholder="src/payments/ (leave empty for microservice)"
                className="w-full text-xs px-3 py-2 rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          </div>
          <div className="flex gap-2">
            <Button size="sm" variant="primary" onClick={save} className="text-xs">
              <Save className="w-3 h-3 mr-1.5" /> Save
            </Button>
            {repo?.repoUrl && (
              <Button size="sm" variant="ghost" onClick={onRemoveRepo} className="text-xs text-red-500 hover:text-red-600">
                <Trash2 className="w-3 h-3 mr-1.5" /> Remove
              </Button>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

export default function ProjectSettingsPage() {
  const { id: projectId } = useParams()
  const router = useRouter()
  const [activeTab, setActiveTab] = useState<Tab>("overview")
  const [serviceRepos, setServiceRepos] = useState<ServiceRepo[]>([])
  const [reposLoaded, setReposLoaded] = useState(false)
  const [saved, setSaved] = useState(false)
  const [webhookUrl, setWebhookUrl] = useState("")
  const [webhookSaved, setWebhookSaved] = useState(false)
  const [webhookSaving, setWebhookSaving] = useState(false)

  const { data: project, isLoading: projectLoading } = useGetProjectQuery(projectId as string, {
    skip: !projectId,
  })
  const { data: settings } = useGetProjectSettingsQuery(projectId as string, {
    skip: !projectId,
    // Load repos and webhook from settings on first fetch
    selectFromResult: ({ data, ...rest }) => {
      if (data && !reposLoaded) {
        setServiceRepos(data.services ?? [])
        setWebhookUrl(data.notificationWebhook ?? "")
        setReposLoaded(true)
      }
      return { data, ...rest }
    },
  })
  const { data: graph } = useGetCurrentGraphQuery(
    { projectId: projectId as string, environment: "prod" },
    { skip: !projectId }
  )
  const { data: incidentsData } = useListIncidentsQuery(
    { projectId: projectId as string, limit: 5 },
    { skip: !projectId }
  )
  const [updateSettings, { isLoading: saving }] = useUpdateProjectSettingsMutation()
  const { data: codeIndexData } = useGetCodeIndexStatusQuery(projectId as string, {
    skip: !projectId || activeTab !== "code",
  })
  const [reindexing, setReindexing] = useState<Record<string, boolean>>({})
  const [sloEditing, setSloEditing] = useState<string | null>(null)
  const [sloValues, setSloValues] = useState<Record<string, { errorRateThreshold: number; latencyThresholdMs: number; minSignalCount: number }>>({})
  const [sloEnv, setSloEnv] = useState<Record<string, string>>({})
  const [sloSaving, setSloSaving] = useState<Record<string, boolean>>({})
  const [sloSaved, setSloSaved] = useState<Record<string, boolean>>({})
  const [updateServiceSLO] = useUpdateServiceSLOMutation()

  const triggerReindex = async (serviceId: string) => {
    setReindexing((prev) => ({ ...prev, [serviceId]: true }))
    try {
      await fetch(`http://localhost:8092/index/service/${serviceId}`, { method: "POST" })
    } catch {
      // best-effort
    } finally {
      setReindexing((prev) => ({ ...prev, [serviceId]: false }))
    }
  }

  const nodes = graph?.nodes ?? []
  const openIncidents = incidentsData?.incidents?.filter(i => i.status === "OPEN").length ?? 0

  const handleSaveRepos = async () => {
    const result = await updateSettings({
      projectId: projectId as string,
      body: { services: serviceRepos },
    })
    if ("data" in result) {
      setSaved(true)
      setTimeout(() => setSaved(false), 2500)
    }
  }

  const updateRepo = (serviceId: string, repo: ServiceRepo) => {
    setServiceRepos(prev => {
      const existing = prev.findIndex(r => r.serviceId === serviceId)
      if (existing >= 0) {
        const next = [...prev]
        next[existing] = repo
        return next
      }
      return [...prev, repo]
    })
  }

  const removeRepo = (serviceId: string) => {
    setServiceRepos(prev => prev.filter(r => r.serviceId !== serviceId))
  }

  const openSloEditor = (serviceId: string) => {
    if (!sloValues[serviceId]) {
      setSloValues((prev) => ({
        ...prev,
        [serviceId]: { errorRateThreshold: 0.005, latencyThresholdMs: 500, minSignalCount: 3 },
      }))
    }
    if (!sloEnv[serviceId]) {
      setSloEnv((prev) => ({ ...prev, [serviceId]: "prod" }))
    }
    setSloEditing(serviceId)
  }

  const saveSlo = async (serviceId: string) => {
    const body = sloValues[serviceId]
    if (!body) return
    setSloSaving((prev) => ({ ...prev, [serviceId]: true }))
    try {
      await updateServiceSLO({ projectId: projectId as string, serviceId, env: sloEnv[serviceId] ?? "prod", body })
      setSloSaved((prev) => ({ ...prev, [serviceId]: true }))
      setTimeout(() => setSloSaved((prev) => ({ ...prev, [serviceId]: false })), 2500)
      setSloEditing(null)
    } finally {
      setSloSaving((prev) => ({ ...prev, [serviceId]: false }))
    }
  }

  const saveWebhook = async () => {
    setWebhookSaving(true)
    try {
      await updateSettings({
        projectId: projectId as string,
        body: { notificationWebhook: webhookUrl },
      })
      setWebhookSaved(true)
      setTimeout(() => setWebhookSaved(false), 2500)
    } finally {
      setWebhookSaving(false)
    }
  }

  const linkedCount = serviceRepos.filter(r => r.repoUrl).length

  if (projectLoading) return <Loading />
  if (!project) return <ErrorState message="Project not found" />

  const TABS: { id: Tab; label: string; icon: React.ReactNode }[] = [
    { id: "overview",  label: "Overview",  icon: <Activity className="w-4 h-4" /> },
    { id: "services",  label: "Services",  icon: <Server className="w-4 h-4" /> },
    { id: "code",      label: "Code Repos", icon: <Code className="w-4 h-4" /> },
    { id: "import",    label: "Re-import",  icon: <GitBranch className="w-4 h-4" /> },
    { id: "changes",   label: "Changes",    icon: <GitCompareArrows className="w-4 h-4" /> },
  ]

  return (
    <div>
      {/* Header */}
      <motion.div initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} className="mb-6">
        <Link href="/dashboard/projects" className="inline-flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-700 dark:hover:text-slate-300 mb-4 transition-colors">
          <ArrowLeft className="w-4 h-4" /> All Projects
        </Link>
        <div className="flex items-start justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold text-slate-900 dark:text-white">{project.name}</h1>
            <p className="text-sm text-slate-500 mt-0.5 font-mono">{project.slug} · {project.environments?.[0]?.name ?? "prod"}</p>
          </div>
          <div className="flex items-center gap-2">
            <Badge variant={project.graphStatus === "PUBLISHED" ? "success" : "warning"}>
              {project.graphStatus ?? "DRAFT"}
            </Badge>
            {openIncidents > 0 && (
              <Link href={`/dashboard/incidents?project=${projectId}`}>
                <Badge variant="destructive" className="cursor-pointer">
                  {openIncidents} open incident{openIncidents !== 1 ? "s" : ""}
                </Badge>
              </Link>
            )}
            <Link href={`/dashboard/graph?project=${projectId}`}>
              <Button variant="outline" size="sm">
                <GitBranch className="w-3.5 h-3.5 mr-1.5" /> View Graph
              </Button>
            </Link>
          </div>
        </div>
      </motion.div>

      {/* Tab nav */}
      <div className="flex gap-1 p-1 bg-slate-100 dark:bg-slate-800 rounded-xl w-fit mb-6">
        {TABS.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`flex items-center gap-1.5 px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
              activeTab === tab.id
                ? "bg-white dark:bg-slate-700 text-slate-900 dark:text-white shadow-sm"
                : "text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200"
            }`}
          >
            {tab.icon}
            {tab.label}
            {tab.id === "code" && linkedCount > 0 && (
              <span className="ml-1 text-[10px] font-bold bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 rounded-full">
                {linkedCount}
              </span>
            )}
          </button>
        ))}
      </div>

      {/* ── Overview ── */}
      {activeTab === "overview" && (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {[
            { label: "Services", value: nodes.length, icon: <Server className="w-5 h-5" />, color: "text-blue-600 bg-blue-50 dark:bg-blue-900/20" },
            { label: "Open Incidents", value: openIncidents, icon: <AlertCircle className="w-5 h-5" />, color: openIncidents > 0 ? "text-red-600 bg-red-50 dark:bg-red-900/20" : "text-green-600 bg-green-50 dark:bg-green-900/20" },
            { label: "Repos Linked", value: linkedCount, icon: <Code className="w-5 h-5" />, color: linkedCount > 0 ? "text-violet-600 bg-violet-50 dark:bg-violet-900/20" : "text-slate-500 bg-slate-100 dark:bg-slate-800" },
          ].map(stat => (
            <Card key={stat.label} className="p-5">
              <div className="flex items-start justify-between mb-3">
                <p className="text-xs text-slate-400 uppercase tracking-wide">{stat.label}</p>
                <div className={`w-9 h-9 rounded-lg flex items-center justify-center ${stat.color}`}>{stat.icon}</div>
              </div>
              <p className="text-3xl font-bold text-slate-900 dark:text-white">{stat.value}</p>
            </Card>
          ))}

          <Card className="p-5 md:col-span-3">
            <h3 className="text-sm font-semibold text-slate-700 dark:text-slate-200 mb-3">Quick Actions</h3>
            <div className="flex flex-wrap gap-3">
              <button onClick={() => setActiveTab("import")}
                className="flex items-center gap-2 px-4 py-2 text-sm rounded-lg border border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-300 bg-blue-50 dark:bg-blue-900/20 hover:bg-blue-100 dark:hover:bg-blue-900/40 transition-colors">
                <GitBranch className="w-4 h-4" /> Re-import service map
              </button>
              <button onClick={() => setActiveTab("code")}
                className="flex items-center gap-2 px-4 py-2 text-sm rounded-lg border border-violet-200 dark:border-violet-800 text-violet-700 dark:text-violet-300 bg-violet-50 dark:bg-violet-900/20 hover:bg-violet-100 dark:hover:bg-violet-900/40 transition-colors">
                <Code className="w-4 h-4" /> Configure repos for code-level RCA
              </button>
              <Link href={`/dashboard/incidents?project=${projectId}`}
                className="flex items-center gap-2 px-4 py-2 text-sm rounded-lg border border-slate-200 dark:border-slate-700 text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors">
                <AlertCircle className="w-4 h-4" /> View all incidents
              </Link>
            </div>
          </Card>

          {/* ── Setup checklist ── */}
          <Card className="p-5 md:col-span-3">
            <h3 className="text-sm font-semibold text-slate-700 dark:text-slate-200 mb-4">Setup checklist</h3>
            <div className="space-y-3">
              {/* 1. Graph imported */}
              {(() => {
                const done = nodes.length > 0
                return (
                  <div className="flex items-center justify-between gap-3 py-2 border-b border-slate-100 dark:border-slate-800 last:border-0">
                    <div className="flex items-center gap-3">
                      {done
                        ? <CheckCircle2 className="w-4 h-4 text-green-500 flex-shrink-0" />
                        : <span className="w-4 h-4 rounded-full border-2 border-slate-300 dark:border-slate-600 flex-shrink-0" />}
                      <div>
                        <p className={`text-sm font-medium ${done ? "text-slate-800 dark:text-slate-100" : "text-slate-500 dark:text-slate-400"}`}>
                          Graph imported
                        </p>
                        <p className="text-xs text-slate-400">
                          {done ? `${nodes.length} services in graph` : "No services yet — import a service-map.yaml"}
                        </p>
                      </div>
                    </div>
                    {!done && (
                      <button
                        onClick={() => setActiveTab("import")}
                        className="text-xs text-blue-600 dark:text-blue-400 hover:underline flex-shrink-0"
                      >
                        Fix →
                      </button>
                    )}
                  </div>
                )
              })()}

              {/* 2. Health poller watching */}
              {(() => {
                const done = nodes.length > 0
                return (
                  <div className="flex items-center justify-between gap-3 py-2 border-b border-slate-100 dark:border-slate-800 last:border-0">
                    <div className="flex items-center gap-3">
                      {done
                        ? <CheckCircle2 className="w-4 h-4 text-green-500 flex-shrink-0" />
                        : <span className="w-4 h-4 rounded-full border-2 border-slate-300 dark:border-slate-600 flex-shrink-0" />}
                      <div>
                        <p className={`text-sm font-medium ${done ? "text-slate-800 dark:text-slate-100" : "text-slate-500 dark:text-slate-400"}`}>
                          Health poller watching
                        </p>
                        <p className="text-xs text-slate-400">
                          {done
                            ? "Health poller is monitoring all services with healthUrl"
                            : "Import a service map with healthUrl fields to enable polling"}
                        </p>
                      </div>
                    </div>
                    {!done && (
                      <button
                        onClick={() => setActiveTab("import")}
                        className="text-xs text-blue-600 dark:text-blue-400 hover:underline flex-shrink-0"
                      >
                        Fix →
                      </button>
                    )}
                  </div>
                )
              })()}

              {/* 3. Repos configured */}
              {(() => {
                const done = linkedCount > 0
                return (
                  <div className="flex items-center justify-between gap-3 py-2 border-b border-slate-100 dark:border-slate-800 last:border-0">
                    <div className="flex items-center gap-3">
                      {done
                        ? <CheckCircle2 className="w-4 h-4 text-green-500 flex-shrink-0" />
                        : <span className="w-4 h-4 rounded-full border-2 border-slate-300 dark:border-slate-600 flex-shrink-0" />}
                      <div>
                        <p className={`text-sm font-medium ${done ? "text-slate-800 dark:text-slate-100" : "text-slate-500 dark:text-slate-400"}`}>
                          Repos configured for code-level RCA
                        </p>
                        <p className="text-xs text-slate-400">
                          {done
                            ? `${linkedCount}/${nodes.length} services linked — incidents show the commit that caused the fault`
                            : "Link Git repos to see which commit caused each fault"}
                        </p>
                      </div>
                    </div>
                    {!done && (
                      <button
                        onClick={() => setActiveTab("code")}
                        className="text-xs text-blue-600 dark:text-blue-400 hover:underline flex-shrink-0"
                      >
                        Fix →
                      </button>
                    )}
                  </div>
                )
              })()}

              {/* 4. CI/CD webhook */}
              {(() => {
                return (
                  <div className="flex items-center justify-between gap-3 py-2 border-b border-slate-100 dark:border-slate-800 last:border-0">
                    <div className="flex items-center gap-3">
                      <span className="w-4 h-4 rounded-full border-2 border-slate-300 dark:border-slate-600 flex-shrink-0" />
                      <div>
                        <p className="text-sm font-medium text-slate-500 dark:text-slate-400">CI/CD webhook configured</p>
                        <p className="text-xs text-slate-400">
                          POST deployments to TechGraph so incidents are correlated with deploys
                        </p>
                        <code className="mt-1 block text-[10px] bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 px-2 py-1 rounded font-mono overflow-x-auto">
                          POST {`${process.env.NEXT_PUBLIC_API_GATEWAY_URL ?? "http://localhost:8080/api/v1"}/projects/${projectId as string}/deployments`}
                        </code>
                      </div>
                    </div>
                    <a
                      href="/docs/CICD_WEBHOOK.md"
                      className="text-xs text-blue-600 dark:text-blue-400 hover:underline flex-shrink-0"
                    >
                      Docs →
                    </a>
                  </div>
                )
              })()}

              {/* 5. Notification webhook */}
              {(() => {
                const done = !!settings?.notificationWebhook
                return (
                  <div className="flex items-center justify-between gap-3 py-2">
                    <div className="flex items-center gap-3">
                      {done
                        ? <CheckCircle2 className="w-4 h-4 text-green-500 flex-shrink-0" />
                        : <span className="w-4 h-4 rounded-full border-2 border-slate-300 dark:border-slate-600 flex-shrink-0" />}
                      <div>
                        <p className={`text-sm font-medium ${done ? "text-slate-800 dark:text-slate-100" : "text-slate-500 dark:text-slate-400"}`}>
                          Notification webhook set
                        </p>
                        <p className="text-xs text-slate-400">
                          {done
                            ? "Incident alerts will fire to your webhook when root cause is confirmed"
                            : "Configure a Slack/PagerDuty webhook to get alerted on new incidents"}
                        </p>
                      </div>
                    </div>
                    {!done && (
                      <button
                        onClick={() => {
                          const el = document.getElementById("notification-webhook-input")
                          el?.focus()
                          el?.scrollIntoView({ behavior: "smooth", block: "center" })
                        }}
                        className="text-xs text-blue-600 dark:text-blue-400 hover:underline flex-shrink-0"
                      >
                        Fix →
                      </button>
                    )}
                  </div>
                )
              })()}
            </div>
          </Card>

          {/* Notifications */}
          <Card className="p-5 md:col-span-3">
            <h3 className="text-sm font-semibold text-slate-700 dark:text-slate-200 mb-3">Notifications</h3>
            <div className="space-y-2">
              <label className="block text-xs text-slate-600 dark:text-slate-400">
                Webhook URL <span className="text-slate-400">(fires when root cause is CONFIRMED)</span>
              </label>
              <input
                id="notification-webhook-input"
                value={webhookUrl}
                onChange={(e) => setWebhookUrl(e.target.value)}
                placeholder="https://hooks.slack.com/services/..."
                className="w-full text-sm px-3 py-2 rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-blue-500 text-slate-900 dark:text-white"
              />
              <p className="text-xs text-slate-400">Works with Slack, PagerDuty, or any HTTP endpoint receiving a JSON POST.</p>
              <Button
                size="sm"
                variant="primary"
                onClick={saveWebhook}
                disabled={webhookSaving}
              >
                <Save className="w-3 h-3 mr-1.5" />
                {webhookSaving ? "Saving…" : webhookSaved ? "✓ Saved" : "Save"}
              </Button>
            </div>
          </Card>
        </div>
      )}

      {/* ── Services ── */}
      {activeTab === "services" && (
        <div>
          <div className="flex items-center justify-between mb-4">
            <div>
              <h2 className="text-base font-semibold text-slate-900 dark:text-white">Service Topology</h2>
              <p className="text-xs text-slate-500 mt-0.5">
                {nodes.length} services in graph. To change topology, re-import your service-map.yaml.
              </p>
            </div>
            <button onClick={() => setActiveTab("import")}
              className="text-xs text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1">
              <Plus className="w-3.5 h-3.5" /> Re-import YAML
            </button>
          </div>

          {nodes.length === 0 ? (
            <Card className="p-8 text-center">
              <GitBranch className="w-8 h-8 text-slate-300 mx-auto mb-3" />
              <p className="text-sm font-medium text-slate-600 dark:text-slate-400 mb-2">
                No services in graph yet
              </p>
              <p className="text-xs text-slate-400 mb-4">
                Import a service-map.yaml to define your service topology
              </p>
              <Button variant="primary" size="sm" onClick={() => setActiveTab("import")}>
                Import service map
              </Button>
            </Card>
          ) : (
            <div className="space-y-2">
              {nodes.map(node => {
                const isLinked = serviceRepos.some(r => r.serviceId === node.id && r.repoUrl)
                const isEditingSlo = sloEditing === node.id
                const slo = sloValues[node.id] ?? { errorRateThreshold: 0.005, latencyThresholdMs: 500, minSignalCount: 3 }
                return (
                  <div key={node.id} className="rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 overflow-hidden">
                    <div className="flex items-center gap-3 p-3">
                      <span className="text-slate-400">
                        {NODE_TYPE_ICON[node.type] ?? <Server className="w-4 h-4" />}
                      </span>
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-slate-800 dark:text-slate-100">{node.name}</p>
                        <p className="text-xs text-slate-400 font-mono">{node.id}</p>
                      </div>
                      <Badge variant="secondary" className="text-[10px]">{node.type}</Badge>
                      {isLinked ? (
                        <span className="text-xs text-green-600 dark:text-green-400 flex items-center gap-1">
                          <Code className="w-3 h-3" /> Repo
                        </span>
                      ) : (
                        <button
                          onClick={() => setActiveTab("code")}
                          className="text-xs text-slate-400 hover:text-blue-500 flex items-center gap-1 transition-colors"
                        >
                          <Plus className="w-3 h-3" /> Add repo
                        </button>
                      )}
                      <button
                        onClick={() => isEditingSlo ? setSloEditing(null) : openSloEditor(node.id)}
                        className={`flex items-center gap-1 text-xs px-2 py-1 rounded-md border transition-colors ${
                          isEditingSlo
                            ? "border-violet-400 text-violet-600 dark:text-violet-400 bg-violet-50 dark:bg-violet-900/20"
                            : "border-slate-200 dark:border-slate-700 text-slate-500 dark:text-slate-400 hover:border-violet-400 hover:text-violet-600 dark:hover:text-violet-400"
                        }`}
                        title="Edit SLO thresholds"
                      >
                        <Settings className="w-3 h-3" />
                        SLO
                      </button>
                    </div>

                    {/* Inline SLO form */}
                    {isEditingSlo && (
                      <div className="border-t border-slate-100 dark:border-slate-800 px-4 py-3 bg-slate-50 dark:bg-slate-800/50">
                        <p className="text-xs font-semibold text-slate-600 dark:text-slate-300 mb-3">
                          SLO Thresholds — {node.name}
                        </p>
                        {/* Environment selector */}
                        <div className="mb-3">
                          <label className="text-[11px] font-medium text-slate-500 dark:text-slate-400 block mb-1">
                            Environment
                          </label>
                          <select
                            value={sloEnv[node.id] ?? "prod"}
                            onChange={(e) => setSloEnv((prev) => ({ ...prev, [node.id]: e.target.value }))}
                            className="text-xs px-2 py-1.5 rounded border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                          >
                            <option value="prod">prod</option>
                            <option value="staging">staging</option>
                            <option value="dev">dev</option>
                          </select>
                        </div>
                        <div className="grid grid-cols-3 gap-3 mb-3">
                          <div>
                            <label className="text-[11px] font-medium text-slate-500 dark:text-slate-400 block mb-1">
                              Error rate threshold
                            </label>
                            <input
                              type="number"
                              step="0.001"
                              min="0"
                              max="1"
                              value={slo.errorRateThreshold}
                              onChange={(e) =>
                                setSloValues((prev) => ({
                                  ...prev,
                                  [node.id]: { ...slo, errorRateThreshold: parseFloat(e.target.value) || 0 },
                                }))
                              }
                              className="w-full text-xs px-2 py-1.5 rounded border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                            />
                            <p className="text-[10px] text-slate-400 mt-0.5">e.g. 0.005 = 0.5%</p>
                          </div>
                          <div>
                            <label className="text-[11px] font-medium text-slate-500 dark:text-slate-400 block mb-1">
                              Latency threshold (ms)
                            </label>
                            <input
                              type="number"
                              min="0"
                              value={slo.latencyThresholdMs}
                              onChange={(e) =>
                                setSloValues((prev) => ({
                                  ...prev,
                                  [node.id]: { ...slo, latencyThresholdMs: parseInt(e.target.value) || 0 },
                                }))
                              }
                              className="w-full text-xs px-2 py-1.5 rounded border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                            />
                            <p className="text-[10px] text-slate-400 mt-0.5">p95 latency</p>
                          </div>
                          <div>
                            <label className="text-[11px] font-medium text-slate-500 dark:text-slate-400 block mb-1">
                              Min signal count
                            </label>
                            <input
                              type="number"
                              min="1"
                              value={slo.minSignalCount}
                              onChange={(e) =>
                                setSloValues((prev) => ({
                                  ...prev,
                                  [node.id]: { ...slo, minSignalCount: parseInt(e.target.value) || 1 },
                                }))
                              }
                              className="w-full text-xs px-2 py-1.5 rounded border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                            />
                            <p className="text-[10px] text-slate-400 mt-0.5">consecutive polls</p>
                          </div>
                        </div>
                        <div className="flex items-center gap-3">
                          <Button
                            size="sm"
                            variant="primary"
                            onClick={() => saveSlo(node.id)}
                            disabled={sloSaving[node.id]}
                            className="text-xs"
                          >
                            <Save className="w-3 h-3 mr-1" />
                            {sloSaving[node.id] ? "Saving…" : sloSaved[node.id] ? "✓ Saved" : "Save SLO"}
                          </Button>
                          <button
                            onClick={() => setSloEditing(null)}
                            className="text-xs text-slate-400 hover:text-slate-600 dark:hover:text-slate-300"
                          >
                            Cancel
                          </button>
                        </div>
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          )}
        </div>
      )}

      {/* ── Code Repos ── */}
      {activeTab === "code" && (
        <div>
          <div className="flex items-center justify-between mb-2">
            <div>
              <h2 className="text-base font-semibold text-slate-900 dark:text-white">Code Repository Mapping</h2>
              <p className="text-xs text-slate-500 mt-0.5">
                Link each service to its Git repo. When a fault is detected, TechGraph will show which
                recent commit likely caused it.
              </p>
            </div>
          </div>

          {/* Architecture guide */}
          <div className="mb-4 p-4 rounded-xl bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800">
            <p className="text-xs font-semibold text-amber-800 dark:text-amber-300 mb-1">
              Microservices vs Monolith
            </p>
            <p className="text-xs text-amber-700 dark:text-amber-400">
              <strong>Microservices:</strong> each service has its own repo — set <code className="bg-amber-100 dark:bg-amber-900 px-1 rounded">repo</code> only. &nbsp;
              <strong>Monolith:</strong> all services share one repo — set <code className="bg-amber-100 dark:bg-amber-900 px-1 rounded">repo</code> the same + use <code className="bg-amber-100 dark:bg-amber-900 px-1 rounded">codePath</code> to scope each service to its subdirectory.
            </p>
          </div>

          <div className="space-y-3 mb-5">
            {nodes.length === 0 && (
              <Card className="p-6 text-center">
                <p className="text-sm text-slate-500">Import a service map first to see services here.</p>
              </Card>
            )}
            {nodes.map(node => (
              <ServiceRepoRow
                key={node.id}
                serviceId={node.id}
                serviceName={node.name}
                serviceType={node.type}
                repo={serviceRepos.find(r => r.serviceId === node.id)}
                onChange={(r) => updateRepo(node.id, r)}
                onRemoveRepo={() => removeRepo(node.id)}
              />
            ))}
          </div>

          {nodes.length > 0 && (
            <div className="flex items-center gap-3">
              <Button
                variant="primary"
                onClick={handleSaveRepos}
                disabled={saving}
              >
                {saving ? "Saving…" : saved ? "✓ Saved" : "Save repo configuration"}
              </Button>
              <p className="text-xs text-slate-400">
                {linkedCount}/{nodes.length} services linked to repos
              </p>
            </div>
          )}

          {/* Code Indexer Status */}
          {(codeIndexData?.services?.length ?? 0) > 0 && (
            <div className="mt-6">
              <h3 className="text-sm font-semibold text-slate-700 dark:text-slate-200 mb-3 flex items-center gap-2">
                <Code className="w-4 h-4 text-violet-500" />
                Code Indexer Status
              </h3>
              <div className="space-y-2">
                {codeIndexData!.services.map((svc) => {
                  const node = nodes.find((n) => n.id === svc.serviceId)
                  return (
                    <div
                      key={svc.serviceId}
                      className="flex items-center justify-between gap-4 p-3 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900"
                    >
                      <div className="min-w-0 flex-1">
                        <p className="text-sm font-medium text-slate-800 dark:text-slate-100">
                          {node?.name ?? svc.serviceId}
                        </p>
                        <div className="flex flex-wrap items-center gap-3 mt-0.5 text-[11px] text-slate-400 dark:text-slate-500">
                          {svc.lastIndexedAt ? (
                            <span>
                              Last indexed:{" "}
                              <span className="text-slate-500 dark:text-slate-400">
                                {new Date(svc.lastIndexedAt).toLocaleString()}
                              </span>
                            </span>
                          ) : (
                            <span className="text-amber-500">Not yet indexed</span>
                          )}
                          {svc.lastCommitHash && (
                            <span className="font-mono">
                              Commit:{" "}
                              <span className="text-slate-600 dark:text-slate-300">
                                {svc.lastCommitHash.slice(0, 7)}
                              </span>
                            </span>
                          )}
                        </div>
                      </div>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => triggerReindex(svc.serviceId)}
                        disabled={reindexing[svc.serviceId]}
                        className="flex-shrink-0 text-xs"
                      >
                        {reindexing[svc.serviceId] ? "Triggering…" : "Trigger re-index"}
                      </Button>
                    </div>
                  )
                })}
              </div>
            </div>
          )}
        </div>
      )}

      {/* ── Re-import ── */}
      {activeTab === "import" && (
        <div className="max-w-2xl">
          <div className="mb-4">
            <h2 className="text-base font-semibold text-slate-900 dark:text-white">Re-import Service Map</h2>
            <p className="text-xs text-slate-500 mt-0.5">
              Paste an updated service-map.yaml to rebuild the graph. Existing incidents are preserved.
            </p>
          </div>
          <Card className="p-6">
            <ServiceMapImport
              projectId={projectId as string}
              onSuccess={(_jobId) => {
                setTimeout(() => router.push(`/dashboard/graph?project=${projectId}`), 3000)
              }}
            />
          </Card>

          <div className="mt-4 p-4 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700">
            <p className="text-xs font-semibold text-slate-600 dark:text-slate-300 mb-2">
              What re-importing does
            </p>
            <ul className="space-y-1 text-xs text-slate-500 dark:text-slate-400">
              <li>✓ Rebuilds the Neo4j graph (adds new services, updates dependencies)</li>
              <li>✓ Preserves all open incidents and SOP playbooks</li>
              <li>✓ Updates health poller targets immediately</li>
              <li>✗ Does not delete services removed from the YAML (use the API to remove manually)</li>
            </ul>
          </div>
        </div>
      )}

      {/* ── Changes ── */}
      {activeTab === "changes" && (
        <div className="max-w-2xl">
          <div className="mb-4">
            <h2 className="text-base font-semibold text-slate-900 dark:text-white">Graph Diff</h2>
            <p className="text-xs text-slate-500 mt-0.5">
              Compare the current graph against a saved snapshot to see what changed.
            </p>
          </div>
          <Card className="p-6">
            <GraphDiffPanel projectId={projectId as string} />
          </Card>
        </div>
      )}
    </div>
  )
}
