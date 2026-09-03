"use client"

import { useState } from "react"
import { Card } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { motion, AnimatePresence } from "framer-motion"
import { Plus, MoreVertical, GitBranch, AlertTriangle, Activity, ArrowRight, Zap, X, Loader2, Edit2, Trash2, Lock } from "lucide-react"
import { useTenant } from "@/contexts/tenant-context"
import { useListProjectsQuery, useCreateProjectMutation, useDeleteProjectMutation } from "@/store/services"
import { useRouter } from "next/navigation"
import ProjectEditModal from "@/components/project-edit-modal"
import { useAuth } from "@/providers/auth-provider"

const DOMAINS = ["payments", "orders", "analytics", "auth", "notifications", "inventory", "shipping", "reporting", "other"]
const ENVIRONMENTS = ["dev", "staging", "prod"]

function toSlug(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "")
}

function NewProjectModal({ tenantId, onClose }: { tenantId: string; onClose: () => void }) {
  const [name, setName] = useState("")
  const [slug, setSlug] = useState("")
  const [slugTouched, setSlugTouched] = useState(false)
  const [domain, setDomain] = useState("payments")
  const [envs, setEnvs] = useState<string[]>(["prod"])
  const [createProject, { isLoading, error }] = useCreateProjectMutation()
  const router = useRouter()

  const handleNameChange = (v: string) => {
    setName(v)
    if (!slugTouched) setSlug(toSlug(v))
  }

  const toggleEnv = (env: string) => {
    setEnvs(prev => prev.includes(env) ? prev.filter(e => e !== env) : [...prev, env])
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !slug.trim() || envs.length === 0) return
    try {
      const result = await createProject({
        tenantId,
        body: { name: name.trim(), slug: slug.trim(), domain, environments: envs },
      }).unwrap()
      onClose()
      router.push(`/dashboard/graph?project=${result.id}&env=${envs[envs.length - 1]}`)
    } catch {
      // error shown below
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
      <motion.div
        initial={{ opacity: 0, scale: 0.95, y: 10 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.95 }}
        className="w-full max-w-md bg-white dark:bg-slate-900 rounded-2xl shadow-2xl border border-slate-200 dark:border-slate-700 p-6"
      >
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-bold text-slate-900 dark:text-white">New Project</h2>
          <button onClick={onClose} className="p-1 hover:bg-slate-100 dark:hover:bg-slate-800 rounded-lg transition-colors">
            <X className="w-5 h-5 text-slate-500" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-5">
          {/* Name */}
          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Project Name</label>
            <input
              type="text"
              value={name}
              onChange={e => handleNameChange(e.target.value)}
              placeholder="e.g. Payments Platform"
              required
              className="w-full px-3 py-2 rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm"
            />
          </div>

          {/* Slug */}
          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">
              Slug <span className="text-slate-400 font-normal">(auto-generated)</span>
            </label>
            <input
              type="text"
              value={slug}
              onChange={e => { setSlug(e.target.value); setSlugTouched(true) }}
              placeholder="payments-platform"
              required
              className="w-full px-3 py-2 rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm font-mono"
            />
          </div>

          {/* Domain */}
          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Domain</label>
            <select
              value={domain}
              onChange={e => setDomain(e.target.value)}
              className="w-full px-3 py-2 rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-slate-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm"
            >
              {DOMAINS.map(d => (
                <option key={d} value={d}>{d.charAt(0).toUpperCase() + d.slice(1)}</option>
              ))}
            </select>
          </div>

          {/* Environments */}
          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Environments</label>
            <div className="flex gap-2">
              {ENVIRONMENTS.map(env => (
                <button
                  key={env}
                  type="button"
                  onClick={() => toggleEnv(env)}
                  className={`flex-1 py-2 rounded-lg text-sm font-medium border transition-colors ${
                    envs.includes(env)
                      ? "bg-blue-600 border-blue-600 text-white"
                      : "bg-white dark:bg-slate-800 border-slate-300 dark:border-slate-600 text-slate-600 dark:text-slate-400 hover:border-blue-400"
                  }`}
                >
                  {env}
                </button>
              ))}
            </div>
            {envs.length === 0 && (
              <p className="text-xs text-red-500 mt-1">Select at least one environment.</p>
            )}
          </div>

          {error && (
            <p className="text-xs text-red-500 bg-red-50 dark:bg-red-900/20 p-2 rounded-lg border border-red-200 dark:border-red-800">
              Failed to create project. Please try again.
            </p>
          )}

          <div className="flex gap-3 pt-1">
            <Button type="button" variant="outline" className="flex-1" onClick={onClose}>
              Cancel
            </Button>
            <Button
              type="submit"
              className="flex-1 bg-blue-600 hover:bg-blue-700 text-white gap-2"
              disabled={isLoading || !name.trim() || !slug.trim() || envs.length === 0}
            >
              {isLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Plus className="w-4 h-4" />}
              {isLoading ? "Creating..." : "Create Project"}
            </Button>
          </div>
        </form>
      </motion.div>
    </div>
  )
}

export default function ProjectsPage() {
  const { tenantId } = useTenant()
  const { data, isLoading } = useListProjectsQuery(tenantId ?? "", { skip: !tenantId })
  const router = useRouter()
  const { user } = useAuth()
  const [showNewProject, setShowNewProject] = useState(false)
  const [selectedProjectToEdit, setSelectedProjectToEdit] = useState<any | null>(null)
  const [projectToDelete, setProjectToDelete] = useState<any | null>(null)
  const [deleteProject, { isLoading: isDeleting }] = useDeleteProjectMutation()

  // Check if user has admin role
  const isAdmin = !!(user?.roles?.includes("super_admin") || user?.roles?.includes("tenant_admin"))

  const getStatusColor = (status: string) => {
    switch (status) {
      case "healthy":
        return "success"
      case "degraded":
        return "warning"
      case "unhealthy":
        return "destructive"
      default:
        return "default"
    }
  }

  const handleDeleteConfirm = async () => {
    if (!projectToDelete || !tenantId) return
    try {
      await deleteProject({ tenantId, projectId: projectToDelete.id }).unwrap()
      setProjectToDelete(null)
    } catch (err) {
      console.error("Failed to delete project:", err)
    }
  }

  const projects = data?.projects ?? []

  return (
    <div>
      <AnimatePresence>
        {showNewProject && tenantId && (
          <NewProjectModal tenantId={tenantId} onClose={() => setShowNewProject(false)} />
        )}
        {selectedProjectToEdit && (
          <ProjectEditModal
            project={selectedProjectToEdit}
            tenantId={tenantId ?? ""}
            onClose={() => setSelectedProjectToEdit(null)}
            isAdmin={isAdmin}
          />
        )}
        {projectToDelete && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
            <motion.div
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.95 }}
              className="w-full max-w-sm bg-white dark:bg-slate-900 rounded-2xl shadow-2xl border border-slate-200 dark:border-slate-700 p-6"
            >
              <div className="flex items-center justify-center w-10 h-10 rounded-full bg-red-100 dark:bg-red-900/30 mx-auto mb-4">
                <AlertTriangle className="w-6 h-6 text-red-600 dark:text-red-400" />
              </div>
              <h3 className="text-lg font-bold text-slate-900 dark:text-white text-center mb-2">
                Delete Project?
              </h3>
              <p className="text-sm text-slate-600 dark:text-slate-400 text-center mb-6">
                Are you sure you want to delete <span className="font-semibold">{projectToDelete.name}</span>? This action cannot be undone.
              </p>
              <div className="flex gap-3">
                <Button
                  type="button"
                  variant="outline"
                  className="flex-1"
                  onClick={() => setProjectToDelete(null)}
                  disabled={isDeleting}
                >
                  Cancel
                </Button>
                <Button
                  type="button"
                  className="flex-1 bg-red-600 hover:bg-red-700 text-white gap-2"
                  onClick={handleDeleteConfirm}
                  disabled={isDeleting}
                >
                  {isDeleting ? <Loader2 className="w-4 h-4 animate-spin" /> : <Trash2 className="w-4 h-4" />}
                  {isDeleting ? "Deleting..." : "Delete"}
                </Button>
              </div>
            </motion.div>
          </div>
        )}
      </AnimatePresence>

      {/* Header */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5 }}
        className="flex items-center justify-between mb-6"
      >
        <div>
          <h1 className="text-4xl font-bold text-slate-900 dark:text-white mb-2">Projects</h1>
          <p className="text-slate-600 dark:text-slate-400">Manage and monitor your service dependency projects</p>
        </div>
        <Button className="gap-2" onClick={() => setShowNewProject(true)}>
          <Plus className="w-4 h-4" />
          New Project
        </Button>
      </motion.div>

      {/* Projects Grid */}
      <div className="grid gap-6">
        {isLoading && <div className="p-8 text-center text-slate-500">Loading projects...</div>}
        {!isLoading && projects.length === 0 && (
          <div className="p-12 text-center border-2 border-dashed border-slate-200 dark:border-slate-700 rounded-xl">
            <div className="text-4xl mb-3">📂</div>
            <p className="text-slate-600 dark:text-slate-400 font-medium">No projects yet</p>
            <p className="text-sm text-slate-500 dark:text-slate-500 mt-1">Create your first project to start monitoring services.</p>
          </div>
        )}
        {projects.map((project: any, idx: number) => (
          <motion.div
            key={project.id}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: idx * 0.05 }}
          >
            <Card className="p-6 hover:shadow-lg hover:border-blue-400/50 transition-all bg-gradient-to-br from-slate-50 to-white dark:from-slate-900/50 dark:to-slate-900 border border-slate-200 dark:border-slate-800">
              <div className="flex items-start justify-between mb-4">
                <div className="flex-1">
                  <div className="flex items-center gap-3 mb-1">
                    <h3 className="text-xl font-bold text-slate-900 dark:text-white">{project.name}</h3>
                    <Badge variant={project.graphStatus === "published" ? "success" : "warning"}>
                      {project.graphStatus ?? "draft"}
                    </Badge>
                    {(project.incidentCount ?? 0) > 0 && (
                      <Badge variant="destructive">{project.incidentCount} open incidents</Badge>
                    )}
                  </div>
                  <p className="text-slate-500 dark:text-slate-400 text-sm font-mono">{project.slug}</p>
                </div>
                <div className="flex gap-1">
                  {!isAdmin && (
                    <div className="p-2 rounded-lg bg-slate-100 dark:bg-slate-800" title="Admin only">
                      <Lock className="w-5 h-5 text-slate-400" />
                    </div>
                  )}
                  {isAdmin && (
                    <>
                      <button
                        onClick={() => setSelectedProjectToEdit(project)}
                        className="p-2 hover:bg-slate-100 dark:hover:bg-slate-700 rounded-lg transition-colors"
                        title="Edit project"
                      >
                        <Edit2 className="w-5 h-5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300" />
                      </button>
                      <button
                        onClick={() => setProjectToDelete(project)}
                        className="p-2 hover:bg-red-100 dark:hover:bg-red-900/20 rounded-lg transition-colors"
                        title="Delete project"
                      >
                        <Trash2 className="w-5 h-5 text-slate-400 hover:text-red-600 dark:hover:text-red-400" />
                      </button>
                    </>
                  )}
                </div>
              </div>

              {/* Stats Grid */}
              <div className="grid grid-cols-3 gap-3 mt-4 pt-4 border-t border-slate-200 dark:border-slate-700">
                <div className="p-3 bg-gradient-to-br from-blue-50 to-blue-100 dark:from-blue-900/20 dark:to-blue-900/10 rounded-lg">
                  <p className="text-xs text-slate-600 dark:text-slate-400 font-medium flex items-center gap-1">
                    <AlertTriangle className="w-3 h-3" /> Open Incidents
                  </p>
                  <p className={`text-2xl font-bold mt-1 ${(project.incidentCount ?? 0) > 0 ? "text-red-600 dark:text-red-400" : "text-green-600 dark:text-green-400"}`}>
                    {project.incidentCount ?? 0}
                  </p>
                </div>
                <div className="p-3 bg-gradient-to-br from-purple-50 to-purple-100 dark:from-purple-900/20 dark:to-purple-900/10 rounded-lg">
                  <p className="text-xs text-slate-600 dark:text-slate-400 font-medium flex items-center gap-1">
                    <GitBranch className="w-3 h-3" /> Graph
                  </p>
                  <p className="text-sm font-semibold text-purple-600 dark:text-purple-400 mt-2">
                    {project.graphStatus === "published" ? "✓ Published" : "⚠ Draft"}
                  </p>
                </div>
                <div className="p-3 bg-gradient-to-br from-green-50 to-green-100 dark:from-green-900/20 dark:to-green-900/10 rounded-lg">
                  <p className="text-xs text-slate-600 dark:text-slate-400 font-medium flex items-center gap-1">
                    <Activity className="w-3 h-3" /> Status
                  </p>
                  <p className="text-sm font-semibold text-green-600 dark:text-green-400 mt-2">✓ Active</p>
                </div>
              </div>

              {/* Action Buttons */}
              <div className="mt-4 flex gap-2 flex-wrap">
                <Button
                  variant="outline"
                  className="flex-1 text-sm gap-2 min-w-[120px]"
                  onClick={() => router.push(`/dashboard/graph?project=${project.id}&env=prod`)}
                >
                  <GitBranch className="w-4 h-4" />
                  Graph
                </Button>
                <Button
                  variant="outline"
                  className="flex-1 text-sm gap-2 min-w-[120px]"
                  onClick={() => router.push(`/dashboard/incidents?project=${project.id}`)}
                >
                  <AlertTriangle className="w-4 h-4" />
                  Incidents
                </Button>
                <Button
                  variant="outline"
                  className="flex-1 text-sm gap-2 min-w-[120px]"
                  onClick={() => router.push(`/dashboard/projects/${project.id}`)}
                >
                  <Activity className="w-4 h-4" />
                  Settings
                  <ArrowRight className="w-3 h-3 ml-auto" />
                </Button>
              </div>
            </Card>
          </motion.div>
        ))}
      </div>
    </div>
  )
}

