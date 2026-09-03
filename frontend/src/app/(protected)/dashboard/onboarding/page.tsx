"use client"

import { useState, useEffect } from "react"
import { useCreateProjectMutation, useStartOnboardingMutation, useGetOnboardingJobQuery, useImportServiceMapMutation } from "@/store/services"
import { useTenant } from "@/contexts/tenant-context"
import { Card } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Loading } from "@/components/ui/states"
import { CheckCircle2, Circle, Loader2, ArrowRight, ArrowLeft, FolderPlus, FileCode, Settings2 } from "lucide-react"
import { motion, AnimatePresence } from "framer-motion"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useRouter } from "next/navigation"
import { ServiceMapImport } from "@/components/ServiceMapImport"
import type { StartOnboardingInput } from "@/types/api"

// ─── Step 1: Create Project ───────────────────────────────────────────────
const projectSchema = z.object({
  name:        z.string().min(2, "Min 2 characters"),
  slug:        z.string().regex(/^[a-z0-9-]+$/, "lowercase letters, digits and hyphens only"),
  domain:      z.string().min(2),
  environments: z.string().min(1),
  services:    z.string().optional(),
})
type ProjectForm = z.infer<typeof projectSchema>

// ─── Step 2: Source config ────────────────────────────────────────────────
const sourceSchema = z.object({
  environment: z.string().min(1),
  mode: z.enum(["hybrid", "openapi", "kubernetes", "manual"]),
  openapi: z.string().optional(),
  kubernetes: z.boolean().optional(),
  traces: z.boolean().optional(),
  gatewayLogs: z.boolean().optional(),
})
type SourceForm = z.infer<typeof sourceSchema>

const STEPS = [
  { id: 1, label: "Create Project" },
  { id: 2, label: "Configure Sources" },
  { id: 3, label: "Import & Monitor" },
  { id: 4, label: "Done" },
]

function StepIndicator({ current }: { current: number }) {
  return (
    <div className="flex items-center gap-0 mb-10">
      {STEPS.map((step, i) => (
        <div key={step.id} className="flex items-center">
          <div className={`flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-semibold transition-all ${
            step.id < current
              ? "text-green-700 bg-green-50 dark:bg-green-900/20 dark:text-green-300"
              : step.id === current
                ? "text-blue-700 bg-blue-50 dark:bg-blue-900/30 dark:text-blue-300"
                : "text-slate-400 bg-slate-100 dark:bg-slate-800"
          }`}>
            {step.id < current
              ? <CheckCircle2 className="w-3.5 h-3.5" />
              : step.id === current
                ? <Circle className="w-3.5 h-3.5 fill-current" />
                : <Circle className="w-3.5 h-3.5" />}
            {step.label}
          </div>
          {i < STEPS.length - 1 && (
            <div className={`w-6 h-px mx-1 ${step.id < current ? "bg-green-300 dark:bg-green-700" : "bg-slate-200 dark:bg-slate-700"}`} />
          )}
        </div>
      ))}
    </div>
  )
}

// ─── Job monitor (step 3) ────────────────────────────────────────────────
function JobMonitor({
  projectId,
  jobId,
  onComplete,
}: {
  projectId: string
  jobId: string
  onComplete: () => void
}) {
  const { data: job, isLoading } = useGetOnboardingJobQuery(
    { projectId, jobId },
    { pollingInterval: 3000 }
  )

  useEffect(() => {
    if (job?.status === "COMPLETE" || job?.status === "VALIDATION_PENDING") {
      onComplete()
    }
  }, [job?.status, onComplete])

  if (isLoading || !job) return <Loading text="Starting import…" />

  const stages = [
    { key: "connectors",   label: "Discovering connectors" },
    { key: "catalogBuild", label: "Building service catalog" },
    { key: "graphBuild",   label: "Constructing dependency graph" },
    { key: "validation",   label: "Validating graph topology" },
  ] as const

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 mb-6">
        {job.status === "RUNNING" && <Loader2 className="w-5 h-5 text-blue-500 animate-spin" />}
        {(job.status === "COMPLETE" || job.status === "VALIDATION_PENDING") && (
          <CheckCircle2 className="w-5 h-5 text-green-500" />
        )}
        {job.status === "FAILED" && <span className="text-red-500 text-lg">⚠</span>}
        <div>
          <p className="font-semibold text-slate-900 dark:text-white">
            Import {job.status === "RUNNING" ? "in progress…" : job.status.toLowerCase()}
          </p>
          <p className="text-xs text-slate-400">Job ID: {job.jobId}</p>
        </div>
      </div>

      {job.progress && (
        <div className="space-y-3">
          {stages.map((stage) => {
            const val = job.progress![stage.key]
            const done = val === "COMPLETE"
            const running = val === "RUNNING"
            return (
              <div key={stage.key} className="flex items-center gap-3">
                {done
                  ? <CheckCircle2 className="w-4 h-4 text-green-500 shrink-0" />
                  : running
                    ? <Loader2 className="w-4 h-4 text-blue-500 animate-spin shrink-0" />
                    : <Circle className="w-4 h-4 text-slate-300 shrink-0" />}
                <span className={`text-sm ${done ? "text-slate-700 dark:text-slate-300" : "text-slate-400"}`}>
                  {stage.label}
                </span>
              </div>
            )
          })}
        </div>
      )}

      {job.validation && (
        <Card className="mt-4 p-4">
          <p className="text-xs font-semibold text-slate-500 uppercase tracking-wide mb-2">Validation Report</p>
          <dl className="grid grid-cols-2 gap-2 text-sm">
            {[
              { k: "Orphan nodes",          v: job.validation.orphanNodes },
              { k: "Low confidence edges",  v: job.validation.lowConfidenceEdges },
              { k: "Missing edges",         v: job.validation.missingEdges },
              { k: "Cycles detected",       v: job.validation.cycles },
            ].map(({ k, v }) => (
              <div key={k}>
                <dt className="text-xs text-slate-400">{k}</dt>
                <dd className={`font-semibold ${v > 0 ? "text-amber-600 dark:text-amber-400" : "text-green-600 dark:text-green-400"}`}>{v}</dd>
              </div>
            ))}
          </dl>
        </Card>
      )}

      {job.status === "FAILED" && (
        <div className="p-4 rounded-xl bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 text-sm">
          Import failed. Please check your source configuration and try again.
        </div>
      )}
    </div>
  )
}

// ─── YAML path: project name + service-map import ────────────────────────
function YamlImportPath({ tenantId, onDone }: { tenantId: string; onDone: (pid: string) => void }) {
  const [name, setName] = useState("")
  const [slug, setSlug] = useState("")
  const [slugTouched, setSlugTouched] = useState(false)
  const [projectId, setProjectId] = useState<string | null>(null)
  const [createProject, { isLoading: creating }] = useCreateProjectMutation()
  const router = useRouter()

  const toSlug = (n: string) => n.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "")

  const handleNameChange = (v: string) => {
    setName(v)
    if (!slugTouched) setSlug(toSlug(v))
  }

  const handleCreate = async () => {
    if (!name.trim() || !slug.trim()) return
    const result = await createProject({ tenantId, body: { name: name.trim(), slug: slug.trim(), environments: ["prod"] } })
    if ("data" in result && result.data) {
      setProjectId(result.data.id)
    }
  }

  if (projectId) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-2 p-3 bg-green-50 dark:bg-green-900/20 rounded-lg border border-green-200 dark:border-green-800">
          <CheckCircle2 className="w-4 h-4 text-green-600 dark:text-green-400 flex-shrink-0" />
          <p className="text-sm text-green-800 dark:text-green-300 font-medium">
            Project <strong>{name}</strong> created — now import your service topology
          </p>
        </div>
        <ServiceMapImport
          projectId={projectId}
          onSuccess={() => setTimeout(() => {
            router.push(`/dashboard/projects/${projectId}`)
          }, 2500)}
        />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div>
        <label className="text-xs font-semibold text-slate-600 dark:text-slate-400 uppercase tracking-wide block mb-1.5">
          Project Name
        </label>
        <input
          value={name}
          onChange={e => handleNameChange(e.target.value)}
          placeholder="Payments Platform"
          className="w-full text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>
      <div>
        <label className="text-xs font-semibold text-slate-600 dark:text-slate-400 uppercase tracking-wide block mb-1.5">
          Slug <span className="text-slate-400 normal-case font-normal">(URL identifier)</span>
        </label>
        <input
          value={slug}
          onChange={e => { setSlug(e.target.value); setSlugTouched(true) }}
          placeholder="payments-platform"
          className="w-full text-sm font-mono rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>
      <Button
        variant="primary"
        onClick={handleCreate}
        disabled={creating || !name.trim() || !slug.trim()}
        className="w-full"
      >
        {creating ? <><Loader2 className="w-4 h-4 mr-1.5 animate-spin" /> Creating…</> : <>Next — Import service map <ArrowRight className="w-4 h-4 ml-1.5" /></>}
      </Button>
    </div>
  )
}

// ─── Main wizard ─────────────────────────────────────────────────────────
export default function OnboardingPage() {
  const { tenantId } = useTenant()
  const [path, setPath] = useState<"choose" | "yaml" | "manual">("choose")
  const [step, setStep] = useState(1)
  const [projectId, setProjectId] = useState<string | null>(null)
  const [jobId, setJobId] = useState<string | null>(null)

  const [createProject, { isLoading: creating }] = useCreateProjectMutation()
  const [startOnboarding, { isLoading: importing }] = useStartOnboardingMutation()

  const projectForm = useForm<ProjectForm>({
    resolver: zodResolver(projectSchema),
    defaultValues: { name: "", slug: "", domain: "", environments: "production,staging", services: "" },
  })

  const sourceForm = useForm<SourceForm>({
    resolver: zodResolver(sourceSchema),
    defaultValues: { environment: "production", mode: "hybrid", kubernetes: true, traces: true, gatewayLogs: false },
  })

  const onCreateProject = async (values: ProjectForm) => {
    if (!tenantId) return
    // Parse services: "api-gateway -> payment-service, payment-service -> db"
    const services: Array<{ name: string; dependencies: string[] }> = []
    if (values.services) {
      const serviceMap = new Map<string, Set<string>>()
      values.services.split(",").forEach((entry) => {
        const parts = entry.trim().split("->").map((s) => s.trim())
        if (parts.length === 2) {
          const [from, to] = parts
          if (!serviceMap.has(from)) serviceMap.set(from, new Set())
          serviceMap.get(from)!.add(to)
          if (!serviceMap.has(to)) serviceMap.set(to, new Set())
        } else if (parts.length === 1 && parts[0]) {
          if (!serviceMap.has(parts[0])) serviceMap.set(parts[0], new Set())
        }
      })
      serviceMap.forEach((deps, name) => {
        services.push({ name, dependencies: Array.from(deps) })
      })
    }

    const result = await createProject({
      tenantId,
      body: {
        name: values.name,
        slug: values.slug,
        domain: values.domain,
        environments: values.environments.split(",").map((e) => e.trim()),
        services: services.length > 0 ? services : undefined,
      },
    })
    if ("data" in result && result.data) {
      setProjectId(result.data.id)
      // If services were provided, skip to step 3 (graph is auto-built)
      if (services.length > 0) {
        setStep(3)
        // Auto-complete after brief delay since graph is built synchronously
        setTimeout(() => setStep(4), 3000)
      } else {
        setStep(2)
      }
    }
  }

  const onStartImport = async (values: SourceForm) => {
    if (!projectId) return
    const body: StartOnboardingInput = {
      environment: values.environment,
      mode: values.mode,
      sources: {
        openapi:     values.openapi ? [values.openapi] : undefined,
        kubernetes:  values.kubernetes,
        traces:      values.traces,
        gatewayLogs: values.gatewayLogs,
      },
    }
    const result = await startOnboarding({ projectId, body })
    if ("data" in result && result.data) {
      setJobId(result.data.jobId)
      setStep(3)
    }
  }

  return (
    <div>
      <motion.div initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }}>
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-slate-900 dark:text-white">New Project Onboarding</h1>
          <p className="text-slate-500 dark:text-slate-400 mt-1">
            Import your service topology and start detecting faults
          </p>
        </div>

        {/* ── Path chooser ── */}
        {path === "choose" && (
          <motion.div initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} className="max-w-lg">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-4">
              <button
                onClick={() => setPath("yaml")}
                className="group flex flex-col items-start gap-3 p-5 rounded-xl border-2 border-blue-200 dark:border-blue-700 hover:border-blue-500 dark:hover:border-blue-500 bg-blue-50/50 dark:bg-blue-900/10 text-left transition-all"
              >
                <div className="w-10 h-10 rounded-xl bg-blue-100 dark:bg-blue-900/40 flex items-center justify-center">
                  <FileCode className="w-5 h-5 text-blue-600 dark:text-blue-400" />
                </div>
                <div>
                  <p className="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-2">
                    Import YAML
                    <span className="text-[10px] font-semibold bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 rounded-full">Recommended</span>
                  </p>
                  <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
                    Paste a <code className="bg-slate-100 dark:bg-slate-800 px-1 rounded">service-map.yaml</code> — defines all services, dependencies, health endpoints, and git repos in one file. Fastest for 10+ services.
                  </p>
                </div>
                <ArrowRight className="w-4 h-4 text-blue-500 group-hover:translate-x-1 transition-transform" />
              </button>

              <button
                onClick={() => setPath("manual")}
                className="group flex flex-col items-start gap-3 p-5 rounded-xl border-2 border-slate-200 dark:border-slate-700 hover:border-slate-400 dark:hover:border-slate-500 text-left transition-all"
              >
                <div className="w-10 h-10 rounded-xl bg-slate-100 dark:bg-slate-800 flex items-center justify-center">
                  <Settings2 className="w-5 h-5 text-slate-500" />
                </div>
                <div>
                  <p className="text-sm font-bold text-slate-900 dark:text-white">Guided wizard</p>
                  <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
                    Fill in the project form step by step. Good for a first project or when you don't have a YAML file yet.
                  </p>
                </div>
                <ArrowRight className="w-4 h-4 text-slate-400 group-hover:translate-x-1 transition-transform" />
              </button>
            </div>

            <div className="p-4 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700">
              <p className="text-xs font-semibold text-slate-600 dark:text-slate-300 mb-2">service-map.yaml — why it's the right way</p>
              <ul className="space-y-1 text-xs text-slate-500 dark:text-slate-400">
                <li>✓ Define 10 services in 30 seconds — no clicking through forms</li>
                <li>✓ Same file used by health poller (autonomous fault detection)</li>
                <li>✓ Add <code className="bg-slate-100 dark:bg-slate-700 px-1 rounded">repo:</code> per service → code-level RCA shows which commit caused the fault</li>
                <li>✓ Commit it to your repo — onboarding is repeatable across environments</li>
              </ul>
            </div>
          </motion.div>
        )}

        {/* ── YAML path ── */}
        {path === "yaml" && tenantId && (
          <motion.div initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} className="max-w-2xl">
            <button onClick={() => setPath("choose")} className="flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-700 dark:hover:text-slate-300 mb-4 transition-colors">
              <ArrowLeft className="w-3.5 h-3.5" /> Back
            </button>
            <Card className="p-6">
              <h2 className="font-semibold text-slate-900 dark:text-white mb-4 flex items-center gap-2">
                <FileCode className="w-4 h-4 text-blue-500" /> Import via service-map.yaml
              </h2>
              <YamlImportPath tenantId={tenantId} onDone={() => {}} />
            </Card>
          </motion.div>
        )}

        {/* ── Manual wizard ── */}
        {path === "manual" && (
          <>
            <button onClick={() => setPath("choose")} className="flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-700 dark:hover:text-slate-300 mb-4 transition-colors">
              <ArrowLeft className="w-3.5 h-3.5" /> Back
            </button>
            <StepIndicator current={step} />
          </>
        )}

        <div className="max-w-lg">
          <AnimatePresence mode="wait">
            {/* ── Step 1 ── */}
            {path === "manual" && step === 1 && (
              <motion.div key="step1" initial={{ opacity: 0, x: 20 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -20 }}>
                <Card className="p-6">
                  <h2 className="font-semibold text-slate-900 dark:text-white mb-4 flex items-center gap-2">
                    <FolderPlus className="w-4 h-4 text-blue-500" /> Create Project
                  </h2>
                  <form onSubmit={projectForm.handleSubmit(onCreateProject)} className="space-y-4">
                    {[
                      { name: "name" as const,        label: "Project Name",       placeholder: "My Platform" },
                      { name: "slug" as const,        label: "Slug",               placeholder: "my-platform" },
                      { name: "domain" as const,      label: "Primary Domain",     placeholder: "payments" },
                      { name: "environments" as const, label: "Environments (comma separated)", placeholder: "production,staging" },
                    ].map((f) => (
                      <div key={f.name}>
                        <label className="text-xs text-slate-500 uppercase tracking-wide block mb-1">{f.label}</label>
                        <input
                          {...projectForm.register(f.name)}
                          placeholder={f.placeholder}
                          className="w-full text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-900 dark:text-white"
                        />
                        {projectForm.formState.errors[f.name] && (
                          <p className="text-xs text-red-500 mt-0.5">
                            {projectForm.formState.errors[f.name]?.message}
                          </p>
                        )}
                      </div>
                    ))}

                    {/* Services with dependencies (optional — auto-builds graph) */}
                    <div>
                      <label className="text-xs text-slate-500 uppercase tracking-wide block mb-1">
                        Services & Dependencies (optional)
                      </label>
                      <textarea
                        {...projectForm.register("services")}
                        placeholder="api-gateway -> payment-service, payment-service -> postgres-db, payment-service -> redis-cache"
                        rows={3}
                        className="w-full text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-900 dark:text-white"
                      />
                      <p className="text-xs text-slate-400 mt-1">
                        Format: <code className="bg-slate-100 dark:bg-slate-800 px-1 rounded">serviceA -&gt; serviceB</code>, separated by commas. This auto-builds your service graph.
                      </p>
                    </div>

                    <Button type="submit" variant="primary" disabled={creating} className="w-full">
                      {creating ? "Creating…" : <>Next <ArrowRight className="w-4 h-4 ml-1" /></>}
                    </Button>
                  </form>
                </Card>
              </motion.div>
            )}

            {/* ── Step 2 ── */}
            {path === "manual" && step === 2 && (
              <motion.div key="step2" initial={{ opacity: 0, x: 20 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -20 }}>
                <Card className="p-6">
                  <h2 className="font-semibold text-slate-900 dark:text-white mb-4">Configure Import Sources</h2>
                  <form onSubmit={sourceForm.handleSubmit(onStartImport)} className="space-y-4">
                    <div className="grid grid-cols-2 gap-3">
                      <div>
                        <label className="text-xs text-slate-500 uppercase tracking-wide block mb-1">Environment</label>
                        <select {...sourceForm.register("environment")} className="w-full text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-700 dark:text-slate-300">
                          <option value="production">production</option>
                          <option value="staging">staging</option>
                          <option value="development">development</option>
                        </select>
                      </div>
                      <div>
                        <label className="text-xs text-slate-500 uppercase tracking-wide block mb-1">Discovery Mode</label>
                        <select {...sourceForm.register("mode")} className="w-full text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-700 dark:text-slate-300">
                          <option value="hybrid">Hybrid (recommended)</option>
                          <option value="openapi">OpenAPI only</option>
                          <option value="kubernetes">Kubernetes only</option>
                          <option value="manual">Manual</option>
                        </select>
                      </div>
                    </div>

                    <div>
                      <label className="text-xs text-slate-500 uppercase tracking-wide block mb-1">OpenAPI spec URLs (optional)</label>
                      <input {...sourceForm.register("openapi")} placeholder="https://api.example.com/openapi.json" className="w-full text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-900 dark:text-white" />
                    </div>

                    <div className="space-y-2">
                      <p className="text-xs text-slate-500 uppercase tracking-wide">Additional sources</p>
                      {[
                        { name: "kubernetes" as const,  label: "Kubernetes service discovery" },
                        { name: "traces" as const,      label: "Distributed traces (OTLP)" },
                        { name: "gatewayLogs" as const, label: "API Gateway access logs" },
                      ].map((s) => (
                        <label key={s.name} className="flex items-center gap-2 cursor-pointer">
                          <input type="checkbox" {...sourceForm.register(s.name)} className="rounded border-slate-300" />
                          <span className="text-sm text-slate-700 dark:text-slate-300">{s.label}</span>
                        </label>
                      ))}
                    </div>

                    <div className="flex gap-3">
                      <Button type="button" variant="outline" size="sm" onClick={() => setStep(1)}>
                        <ArrowLeft className="w-4 h-4 mr-1" /> Back
                      </Button>
                      <Button type="submit" variant="primary" disabled={importing} className="flex-1">
                        {importing ? "Starting…" : <>Start Import <ArrowRight className="w-4 h-4 ml-1" /></>}
                      </Button>
                    </div>
                  </form>
                </Card>
              </motion.div>
            )}

            {/* ── Step 3 ── */}
            {path === "manual" && step === 3 && projectId && jobId && (
              <motion.div key="step3" initial={{ opacity: 0, x: 20 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -20 }}>
                <Card className="p-6">
                  <h2 className="font-semibold text-slate-900 dark:text-white mb-4">Importing Service Graph</h2>
                  <JobMonitor projectId={projectId} jobId={jobId} onComplete={() => setStep(4)} />
                </Card>
              </motion.div>
            )}

            {/* ── Step 4 ── */}
            {path === "manual" && step === 4 && (
              <motion.div key="step4" initial={{ opacity: 0, scale: 0.95 }} animate={{ opacity: 1, scale: 1 }}>
                <Card className="p-8 text-center">
                  <CheckCircle2 className="w-12 h-12 text-green-500 mx-auto mb-4" />
                  <h2 className="text-xl font-bold text-slate-900 dark:text-white mb-2">Import Complete!</h2>
                  <p className="text-slate-500 text-sm mb-6">
                    Your service graph is ready. Submit signals to trigger fault analysis.
                  </p>
                  <div className="flex justify-center gap-3">
                    <Button variant="outline" size="sm" onClick={() => window.location.href = "/dashboard/graph"}>
                      View Graph
                    </Button>
                    <Button variant="primary" size="sm" onClick={() => window.location.href = "/dashboard/graph"}>
                      View Graph
                    </Button>
                  </div>
                </Card>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </motion.div>
    </div>
  )
}
