"use client"

import { useState } from "react"
import { Card } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { motion } from "framer-motion"
import { X, Plus, Trash2, AlertCircle } from "lucide-react"
import { useUpdateProjectMutation } from "@/store/services"
import { baseApi } from "@/store/api"
import { useDispatch } from "react-redux"
import type { Project } from "@/types/api"

interface Service {
  name: string
  dependencies: string[]
}

interface ProjectEditModalProps {
  project: Project
  tenantId: string
  onClose: () => void
  isAdmin: boolean
}

export default function ProjectEditModal({
  project,
  tenantId,
  onClose,
  isAdmin,
}: ProjectEditModalProps) {
  const [name, setName] = useState(project.name || "")
  const [services, setServices] = useState<Service[]>([])
  const [newServiceName, setNewServiceName] = useState("")
  const [editingService, setEditingService] = useState<string | null>(null)
  const dispatch = useDispatch()
  const [updateProject, { isLoading, error }] = useUpdateProjectMutation()

  const handleAddService = () => {
    if (!newServiceName.trim()) return
    if (services.some((s) => s.name === newServiceName)) return
    setServices([...services, { name: newServiceName, dependencies: [] }])
    setNewServiceName("")
  }

  const handleRemoveService = (serviceName: string) => {
    setServices(services.filter((s) => s.name !== serviceName))
    setServices(
      services.map((s) => ({
        ...s,
        dependencies: s.dependencies.filter((d) => d !== serviceName),
      }))
    )
  }

  const toggleDependency = (serviceName: string, depName: string) => {
    setServices(
      services.map((s) => {
        if (s.name !== serviceName) return s
        const deps = s.dependencies || []
        return {
          ...s,
          dependencies: deps.includes(depName)
            ? deps.filter((d) => d !== depName)
            : [...deps, depName],
        }
      })
    )
  }

  const handleSubmit = async () => {
    if (!isAdmin) return

    try {
      await updateProject({
        tenantId,
        projectId: project.id,
        body: {
          name: name.trim() || project.name,
          services,
        },
      }).unwrap()
      // Invalidate graph cache so the graph page auto-refreshes with new topology
      dispatch(baseApi.util.invalidateTags([{ type: "Graph", id: project.id }]))
      onClose()
    } catch (err) {
      // error shown below
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm overflow-y-auto">
      <motion.div
        initial={{ opacity: 0, scale: 0.95, y: 10 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.95 }}
        className="w-full max-w-3xl bg-white dark:bg-slate-900 rounded-2xl shadow-2xl border border-slate-200 dark:border-slate-700 p-6 my-8"
      >
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-2xl font-bold text-slate-900 dark:text-white">
            Edit Project: {project.name}
          </h2>
          <button
            onClick={onClose}
            className="p-1 hover:bg-slate-100 dark:hover:bg-slate-800 rounded-lg transition-colors"
          >
            <X className="w-5 h-5 text-slate-500" />
          </button>
        </div>

        {!isAdmin && (
          <div className="mb-6 p-4 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700 rounded-lg flex gap-3">
            <AlertCircle className="w-5 h-5 text-amber-600 dark:text-amber-500 flex-shrink-0 mt-0.5" />
            <p className="text-sm text-amber-700 dark:text-amber-200">
              You don't have permission to edit this project. Only tenant admins can modify projects.
            </p>
          </div>
        )}

        <div className="space-y-6">
          {/* Project Name */}
          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">
              Project Name
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={!isAdmin}
              className="w-full px-3 py-2 rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed text-sm"
            />
          </div>

          {/* Graph State / Services */}
          <div>
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold text-slate-900 dark:text-white">
                Service Topology
              </h3>
              <Badge variant="secondary" className="text-xs">
                {services.length} services
              </Badge>
            </div>

            {/* Add Service */}
            {isAdmin && (
              <div className="mb-4 flex gap-2">
                <input
                  type="text"
                  value={newServiceName}
                  onChange={(e) => setNewServiceName(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && handleAddService()}
                  placeholder="e.g. api-gateway"
                  className="flex-1 px-3 py-2 rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm"
                />
                <button
                  onClick={handleAddService}
                  disabled={!newServiceName.trim()}
                  className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex items-center gap-2 text-sm font-medium"
                >
                  <Plus className="w-4 h-4" />
                  Add
                </button>
              </div>
            )}

            {/* Services List */}
            {services.length === 0 ? (
              <div className="p-4 text-center text-slate-500 dark:text-slate-400 border border-dashed border-slate-300 dark:border-slate-700 rounded-lg bg-slate-50 dark:bg-slate-800/50">
                No services defined yet. {isAdmin && "Add services to define your architecture."}
              </div>
            ) : (
              <div className="space-y-3">
                {services.map((service) => (
                  <Card
                    key={service.name}
                    className="p-4 border border-slate-200 dark:border-slate-700"
                  >
                    <div className="flex items-start justify-between mb-3">
                      <div>
                        <h4 className="font-semibold text-slate-900 dark:text-white">
                          {service.name}
                        </h4>
                        <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
                          {service.dependencies.length === 0
                            ? "No dependencies"
                            : `Depends on: ${service.dependencies.join(", ")}`}
                        </p>
                      </div>
                      {isAdmin && (
                        <button
                          onClick={() => handleRemoveService(service.name)}
                          className="p-1.5 hover:bg-red-100 dark:hover:bg-red-900/30 rounded-lg transition-colors text-red-600 dark:text-red-400"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      )}
                    </div>

                    {/* Dependencies */}
                    {isAdmin && services.length > 1 && (
                      <div className="mt-3 pt-3 border-t border-slate-200 dark:border-slate-700">
                        <p className="text-xs font-medium text-slate-600 dark:text-slate-400 mb-2">
                          Dependencies:
                        </p>
                        <div className="flex flex-wrap gap-2">
                          {services
                            .filter((s) => s.name !== service.name)
                            .map((other) => (
                              <button
                                key={other.name}
                                onClick={() =>
                                  toggleDependency(service.name, other.name)
                                }
                                className={`px-2.5 py-1 text-xs font-medium rounded-full border transition-colors ${
                                  service.dependencies.includes(other.name)
                                    ? "bg-blue-100 dark:bg-blue-900/40 border-blue-300 dark:border-blue-700 text-blue-700 dark:text-blue-300"
                                    : "border-slate-300 dark:border-slate-600 text-slate-600 dark:text-slate-400 hover:border-blue-400"
                                }`}
                              >
                                {other.name}
                              </button>
                            ))}
                        </div>
                      </div>
                    )}
                  </Card>
                ))}
              </div>
            )}
          </div>

          {/* Error */}
          {error && (
            <div className="p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 rounded-lg">
              <p className="text-sm text-red-700 dark:text-red-200">
                {typeof error === "string" ? error : "Failed to update project"}
              </p>
            </div>
          )}

          {/* Actions */}
          <div className="flex justify-end gap-3 pt-6 border-t border-slate-200 dark:border-slate-700">
            <button
              onClick={onClose}
              className="px-4 py-2 rounded-lg border border-slate-300 dark:border-slate-600 text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors font-medium text-sm"
            >
              Cancel
            </button>
            {isAdmin && (
              <button
                onClick={handleSubmit}
                disabled={isLoading}
                className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors font-medium text-sm"
              >
                {isLoading ? "Saving..." : "Save Changes"}
              </button>
            )}
          </div>
        </div>
      </motion.div>
    </div>
  )
}
