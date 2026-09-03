"use client"

import { useTenant } from "@/contexts/tenant-context"
import { useAuth } from "@/providers/auth-provider"
import { useListProjectsQuery } from "@/store/services"
import { Building2, FolderOpen, Globe, Shield } from "lucide-react"

interface ContextBarProps {
  environment?: string
}

export function ContextBar({ environment }: ContextBarProps) {
  const { tenantId } = useTenant()
  const { user } = useAuth()
  const { data: projectsData } = useListProjectsQuery(tenantId ?? "", { skip: !tenantId })

  const isSuperAdmin = user?.roles?.includes("super_admin")
  const projectCount = projectsData?.projects?.length ?? 0

  return (
    <div className="flex items-center gap-3 px-4 py-2 mb-4 rounded-lg bg-slate-100/80 dark:bg-slate-800/50 border border-slate-200/60 dark:border-slate-700/50 text-xs">
      {isSuperAdmin && (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300 font-semibold">
          <Shield className="w-3 h-3" />
          Super Admin
        </span>
      )}

      <span className="inline-flex items-center gap-1.5 text-slate-600 dark:text-slate-300">
        <Building2 className="w-3 h-3 text-slate-400 dark:text-slate-500" />
        <span className="font-medium">{tenantId || "No tenant"}</span>
      </span>

      <span className="text-slate-300 dark:text-slate-600">|</span>

      <span className="inline-flex items-center gap-1.5 text-slate-600 dark:text-slate-300">
        <FolderOpen className="w-3 h-3 text-slate-400 dark:text-slate-500" />
        <span>{projectCount} project{projectCount !== 1 ? "s" : ""}</span>
      </span>

      {environment && (
        <>
          <span className="text-slate-300 dark:text-slate-600">|</span>
          <span className="inline-flex items-center gap-1.5 text-slate-600 dark:text-slate-300">
            <Globe className="w-3 h-3 text-slate-400 dark:text-slate-500" />
            <span>{environment}</span>
          </span>
        </>
      )}
    </div>
  )
}
