"use client"

import { useState, useEffect, useRef } from "react"
import { useListIncidentsQuery, useListProjectsQuery } from "@/store/services"
import { useTenant } from "@/contexts/tenant-context"
import { useIncidentEvents } from "@/hooks/useIncidentEvents"
import Link from "next/link"
import { motion, AnimatePresence } from "framer-motion"
import {
  CheckCircle2,
  Circle,
  Play,
  Zap,
  Activity,
  Wifi,
  WifiOff,
  RefreshCw,
  ExternalLink,
  AlertCircle,
  ChevronRight,
  Shield,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import ClientDate from "@/components/ClientDate"
import type { IncidentListItem } from "@/types/api"

// demo-services admin API is exposed on port 8091 locally
const DEMO_API = process.env.NEXT_PUBLIC_DEMO_SERVICES_URL ?? "http://localhost:8091"

// The three key services used in the demo fault scenario
const DEMO_SERVICES = [
  { name: "api-gateway",    label: "api-gateway",    role: "Entry point" },
  { name: "payment-api",    label: "payment-api",    role: "Payment flow" },
  { name: "ledger-service", label: "ledger-service", role: "Ledger" },
]

type ServiceHealth = {
  status: string
  latency_p95_ms: number
  error_rate: number
  service: string
}

type StepStatus = "pending" | "active" | "done"

interface DemoStep {
  id: number
  title: string
  description: string
}

const STEPS: DemoStep[] = [
  { id: 1, title: "Your stack is running",    description: "Verify all key services are healthy before injecting a fault." },
  { id: 2, title: "Inject a fault",           description: "Simulate ledger-service failing — watch TechGraph detect it autonomously." },
  { id: 3, title: "Watch detection",          description: "TechGraph's health poller detects the fault and opens an incident via SSE." },
  { id: 4, title: "Work the SOP",             description: "Navigate to the incident and follow the AI-generated runbook." },
  { id: 5, title: "Restore service",          description: "Heal ledger-service and watch TechGraph auto-resolve the incident." },
  { id: 6, title: "See auto-resolution",      description: "Review the audit trail and RCA timeline in the resolved incident." },
]

function incidentLabel(inc: IncidentListItem): string {
  return `${inc.service} fault detected (confidence ${Math.round(inc.confidence * 100)}%)`
}

export default function DemoPage() {
  const { tenantId } = useTenant()
  const [activeStep, setActiveStep] = useState(1)
  const [stepStatus, setStepStatus] = useState<Record<number, StepStatus>>({ 1: "active" })
  const [healths, setHealths] = useState<Record<string, ServiceHealth>>({})
  const [healthLoading, setHealthLoading] = useState(false)
  const [faulting, setFaulting] = useState(false)
  const [healing, setHealing] = useState(false)
  const [faultActive, setFaultActive] = useState(false)
  const [events, setEvents] = useState<Array<{ id: string; text: string; ts: number; type: string }>>([])
  const eventsEndRef = useRef<HTMLDivElement>(null)

  // Resolve the first available project for incident polling
  const { data: projectsData } = useListProjectsQuery(tenantId ?? "", {
    skip: !tenantId,
  })
  const firstProjectId = projectsData?.projects?.[0]?.id ?? ""

  // Pull live incidents for the first project
  const { data: incidentsData, refetch: refetchIncidents } = useListIncidentsQuery(
    { projectId: firstProjectId, limit: 10 },
    { skip: !firstProjectId, pollingInterval: faultActive ? 3000 : 10000 }
  )

  // SSE incident events
  const { connected: sseConnected } = useIncidentEvents({
    onEvent: (event) => {
      const text =
        event.type === "incident_created"
          ? `Incident opened — ${event.service ?? "unknown"} (${event.rootCause ?? "root cause pending"})`
          : event.type === "incident_resolved"
          ? `Incident auto-resolved — ${event.service ?? "unknown"}`
          : `Signal: ${event.service ?? "unknown"} — confidence ${((event.confidence ?? 0) * 100).toFixed(0)}%`
      setEvents((prev) => [
        ...prev.slice(-49),
        { id: `${Date.now()}-${Math.random()}`, text, ts: Date.now(), type: event.type },
      ])

      if (event.type === "incident_created" && faultActive) {
        markDone(3)
        advance(4)
      }
      if (event.type === "incident_resolved") {
        markDone(5)
        advance(6)
      }
    },
  })

  const fetchHealths = async () => {
    setHealthLoading(true)
    const results: Record<string, ServiceHealth> = {}
    await Promise.all(
      DEMO_SERVICES.map(async (svc) => {
        try {
          const res = await fetch(`${DEMO_API}/services/${svc.name}/health`)
          if (res.ok) {
            results[svc.name] = (await res.json()) as ServiceHealth
          } else {
            results[svc.name] = { status: "down", latency_p95_ms: 0, error_rate: 1, service: svc.name }
          }
        } catch {
          results[svc.name] = { status: "unreachable", latency_p95_ms: 0, error_rate: 1, service: svc.name }
        }
      })
    )
    setHealths(results)
    setHealthLoading(false)
  }

  useEffect(() => {
    fetchHealths()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeStep])

  useEffect(() => {
    eventsEndRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [events])

  const markDone = (step: number) => {
    setStepStatus((prev) => ({ ...prev, [step]: "done" }))
  }

  const advance = (step: number) => {
    setActiveStep(step)
    setStepStatus((prev) => ({ ...prev, [step]: "active" }))
  }

  const handleFaultLedger = async () => {
    setFaulting(true)
    try {
      await fetch(`${DEMO_API}/admin/services/ledger-service/status`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ statusClass: "5xx", errorRate: 0.85, latencyP95: 5000 }),
      })
      setFaultActive(true)
      markDone(2)
      advance(3)
      await fetchHealths()
      refetchIncidents()
      setEvents((prev) => [
        ...prev,
        { id: `fault-${Date.now()}`, text: `PUT /admin/services/ledger-service/status → 5xx injected (errorRate=0.85)`, ts: Date.now(), type: "signal" },
      ])
    } catch {
      setEvents((prev) => [
        ...prev,
        { id: `fault-err-${Date.now()}`, text: "Could not reach demo-services at localhost:8091 — is the stack running?", ts: Date.now(), type: "incident_created" },
      ])
    } finally {
      setFaulting(false)
    }
  }

  const handleHealLedger = async () => {
    setHealing(true)
    try {
      await fetch(`${DEMO_API}/admin/services/ledger-service/status`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ statusClass: "2xx", errorRate: 0.002, latencyP95: 45 }),
      })
      setFaultActive(false)
      markDone(5)
      advance(6)
      await fetchHealths()
      refetchIncidents()
      setEvents((prev) => [
        ...prev,
        { id: `heal-${Date.now()}`, text: `PUT /admin/services/ledger-service/status → 2xx restored (errorRate=0.002)`, ts: Date.now(), type: "incident_resolved" },
      ])
    } catch {
      setEvents((prev) => [
        ...prev,
        { id: `heal-err-${Date.now()}`, text: "Could not reach demo-services — is the stack running?", ts: Date.now(), type: "incident_created" },
      ])
    } finally {
      setHealing(false)
    }
  }

  const resetDemo = async () => {
    try {
      await fetch(`${DEMO_API}/admin/reset`, { method: "POST" })
    } catch {
      // best-effort
    }
    setFaultActive(false)
    setActiveStep(1)
    setStepStatus({ 1: "active" })
    setEvents([])
    await fetchHealths()
  }

  const statusColor = (h: ServiceHealth | undefined) => {
    if (!h) return "bg-slate-200 dark:bg-slate-700"
    if (h.status === "ok" || h.status === "2xx") return "bg-green-500"
    if (h.status === "degraded") return "bg-amber-400"
    return "bg-red-500"
  }

  const statusLabel = (h: ServiceHealth | undefined) => {
    if (!h) return "unknown"
    if (h.status === "ok" || h.status === "2xx") return "healthy"
    return h.status
  }

  const openIncidents = incidentsData?.incidents?.filter((i) => i.status === "OPEN") ?? []

  return (
    <div className="max-w-4xl mx-auto">
      <motion.div initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} className="mb-8">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold text-slate-900 dark:text-white flex items-center gap-2">
              <span>🎯</span> TechGraph Live Demo
            </h1>
            <p className="text-sm text-slate-500 mt-1">
              Walk through a complete fault-injection → detection → RCA → resolution cycle in your
              running demo stack. Requires{" "}
              <code className="bg-slate-100 dark:bg-slate-800 px-1 rounded">docker compose up</code> to be running.
            </p>
          </div>
          <div className="flex items-center gap-2 flex-shrink-0">
            {sseConnected ? (
              <span className="flex items-center gap-1.5 text-xs text-green-600 dark:text-green-400">
                <Wifi className="w-3.5 h-3.5" /> SSE live
              </span>
            ) : (
              <span className="flex items-center gap-1.5 text-xs text-slate-400">
                <WifiOff className="w-3.5 h-3.5" /> SSE offline
              </span>
            )}
          </div>
        </div>
      </motion.div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* ── Step sidebar ── */}
        <div className="lg:col-span-1">
          <Card className="p-4">
            <h2 className="text-xs font-semibold text-slate-500 uppercase tracking-wide mb-3">
              Demo steps
            </h2>
            <ol className="space-y-1">
              {STEPS.map((step) => {
                const status: StepStatus = stepStatus[step.id] ?? "pending"
                return (
                  <li key={step.id}>
                    <button
                      onClick={() => {
                        setActiveStep(step.id)
                        setStepStatus((prev) => ({
                          ...prev,
                          [step.id]: prev[step.id] === "done" ? "done" : "active",
                        }))
                      }}
                      className={`w-full text-left flex items-start gap-3 px-3 py-2.5 rounded-lg transition-colors ${
                        activeStep === step.id
                          ? "bg-blue-50 dark:bg-blue-900/20 text-blue-800 dark:text-blue-200"
                          : "text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800"
                      }`}
                    >
                      <span className="flex-shrink-0 mt-0.5">
                        {status === "done" ? (
                          <CheckCircle2 className="w-4 h-4 text-green-500" />
                        ) : status === "active" ? (
                          <Play className="w-4 h-4 text-blue-500" />
                        ) : (
                          <Circle className="w-4 h-4 text-slate-300 dark:text-slate-600" />
                        )}
                      </span>
                      <span>
                        <span className="text-xs font-semibold block">Step {step.id}</span>
                        <span className="text-xs">{step.title}</span>
                      </span>
                    </button>
                  </li>
                )
              })}
            </ol>

            <div className="mt-4 pt-3 border-t border-slate-200 dark:border-slate-700">
              <button
                onClick={resetDemo}
                className="text-xs text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 flex items-center gap-1"
              >
                <RefreshCw className="w-3 h-3" /> Reset all services
              </button>
            </div>
          </Card>
        </div>

        {/* ── Main content ── */}
        <div className="lg:col-span-2 space-y-4">
          <AnimatePresence mode="wait">
            {/* Step 1 — Stack status */}
            {activeStep === 1 && (
              <motion.div
                key="step1"
                initial={{ opacity: 0, x: 12 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -12 }}
              >
                <Card className="p-5">
                  <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-100 mb-4 flex items-center gap-2">
                    <Shield className="w-4 h-4 text-green-500" />
                    Step 1 — Your stack is running
                  </h3>
                  <div className="space-y-3 mb-4">
                    {DEMO_SERVICES.map((svc) => {
                      const h = healths[svc.name]
                      return (
                        <div
                          key={svc.name}
                          className="flex items-center justify-between p-3 rounded-lg border border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-slate-900"
                        >
                          <div className="flex items-center gap-3">
                            <span className={`w-2.5 h-2.5 rounded-full flex-shrink-0 ${statusColor(h)}`} />
                            <div>
                              <p className="text-sm font-medium text-slate-800 dark:text-slate-100">{svc.label}</p>
                              <p className="text-xs text-slate-400">{svc.role}</p>
                            </div>
                          </div>
                          <div className="text-right">
                            <p
                              className={`text-xs font-semibold ${
                                h?.status === "ok" || h?.status === "2xx"
                                  ? "text-green-600 dark:text-green-400"
                                  : "text-red-500"
                              }`}
                            >
                              {statusLabel(h)}
                            </p>
                            {h && (
                              <p className="text-[11px] text-slate-400">
                                {h.latency_p95_ms}ms · err {(h.error_rate * 100).toFixed(1)}%
                              </p>
                            )}
                          </div>
                        </div>
                      )
                    })}
                  </div>
                  <div className="flex items-center gap-3">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={fetchHealths}
                      disabled={healthLoading}
                      className="text-xs"
                    >
                      {healthLoading ? (
                        <><RefreshCw className="w-3 h-3 mr-1.5 animate-spin" />Checking…</>
                      ) : (
                        <><RefreshCw className="w-3 h-3 mr-1.5" />Refresh</>
                      )}
                    </Button>
                    <Button
                      variant="primary"
                      size="sm"
                      onClick={() => { markDone(1); advance(2) }}
                      className="text-xs"
                    >
                      All healthy — continue <ChevronRight className="w-3.5 h-3.5 ml-1" />
                    </Button>
                  </div>
                </Card>
              </motion.div>
            )}

            {/* Step 2 — Inject fault */}
            {activeStep === 2 && (
              <motion.div
                key="step2"
                initial={{ opacity: 0, x: 12 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -12 }}
              >
                <Card className="p-5">
                  <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-100 mb-2 flex items-center gap-2">
                    <Zap className="w-4 h-4 text-amber-500" />
                    Step 2 — Inject a fault
                  </h3>
                  <p className="text-xs text-slate-500 dark:text-slate-400 mb-4">
                    Clicking the button below calls the demo-services admin API to set{" "}
                    <code className="bg-slate-100 dark:bg-slate-800 px-1 rounded">ledger-service</code>{" "}
                    to return 5xx errors at 85% error rate. TechGraph&apos;s health poller will detect
                    this and open an incident automatically.
                  </p>
                  <div className="mb-4 p-3 rounded-lg bg-slate-900 dark:bg-black font-mono text-xs text-green-400 overflow-x-auto">
                    <p className="text-slate-500 mb-1"># Admin API call that will be sent:</p>
                    <p>PUT {DEMO_API}/admin/services/ledger-service/status</p>
                    <p className="text-slate-400">
                      {`{ statusClass: "5xx", errorRate: 0.85, latencyP95: 5000 }`}
                    </p>
                  </div>
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={handleFaultLedger}
                    disabled={faulting || faultActive}
                    className="bg-red-600 hover:bg-red-700 text-white text-xs"
                  >
                    {faulting ? (
                      <><RefreshCw className="w-3.5 h-3.5 mr-1.5 animate-spin" />Injecting…</>
                    ) : faultActive ? (
                      <>Fault active — proceed to Step 3</>
                    ) : (
                      <><Zap className="w-3.5 h-3.5 mr-1.5" />Fault ledger-service</>
                    )}
                  </Button>
                </Card>
              </motion.div>
            )}

            {/* Step 3 — Watch detection */}
            {activeStep === 3 && (
              <motion.div
                key="step3"
                initial={{ opacity: 0, x: 12 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -12 }}
              >
                <Card className="p-5">
                  <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-100 mb-2 flex items-center gap-2">
                    <Activity className="w-4 h-4 text-blue-500" />
                    Step 3 — Watch detection
                  </h3>
                  <p className="text-xs text-slate-500 dark:text-slate-400 mb-3">
                    TechGraph&apos;s health poller runs every ~30 seconds. When{" "}
                    <code className="bg-slate-100 dark:bg-slate-800 px-1 rounded">ledger-service</code>{" "}
                    fails the configured SLO, an incident opens automatically and appears below.
                  </p>

                  {/* Live service health */}
                  <div className="grid grid-cols-3 gap-2 mb-4">
                    {DEMO_SERVICES.map((svc) => {
                      const h = healths[svc.name]
                      return (
                        <div
                          key={svc.name}
                          className="p-2 rounded-lg border border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-slate-900 text-center"
                        >
                          <span className={`inline-block w-2 h-2 rounded-full mb-1 ${statusColor(h)}`} />
                          <p className="text-[11px] font-medium text-slate-700 dark:text-slate-300">{svc.label}</p>
                          <p className="text-[10px] text-slate-400">{statusLabel(h)}</p>
                        </div>
                      )
                    })}
                  </div>

                  {/* Incident list */}
                  {openIncidents.length > 0 ? (
                    <div className="mb-4">
                      <p className="text-xs font-semibold text-slate-600 dark:text-slate-300 mb-2 flex items-center gap-1">
                        <AlertCircle className="w-3.5 h-3.5 text-red-500" />
                        Open incidents detected
                      </p>
                      <div className="space-y-1">
                        {openIncidents.slice(0, 3).map((inc) => (
                          <Link
                            key={inc.id}
                            href={`/dashboard/incidents/${inc.id}`}
                            className="flex items-center justify-between gap-2 px-3 py-2 rounded-lg border border-red-100 dark:border-red-900/30 bg-red-50 dark:bg-red-900/10 hover:bg-red-100 dark:hover:bg-red-900/20 transition-colors"
                          >
                            <div>
                              <p className="text-xs font-medium text-red-800 dark:text-red-300">
                                {incidentLabel(inc)}
                              </p>
                              <p className="text-[11px] text-red-500">
                                <ClientDate iso={inc.detectedAt} />
                              </p>
                            </div>
                            <ChevronRight className="w-3.5 h-3.5 text-red-400 flex-shrink-0" />
                          </Link>
                        ))}
                      </div>
                    </div>
                  ) : (
                    <div className="mb-4 p-3 rounded-lg bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800">
                      <p className="text-xs text-amber-700 dark:text-amber-400">
                        Waiting for health poller to detect the fault (runs every ~30s). Watch the event log.
                      </p>
                    </div>
                  )}

                  {/* SSE event log */}
                  <div>
                    <p className="text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1 flex items-center gap-2">
                      Event log
                      {sseConnected ? (
                        <span className="text-[10px] text-green-500 font-normal">● live</span>
                      ) : (
                        <span className="text-[10px] text-slate-400 font-normal">○ offline</span>
                      )}
                    </p>
                    <div className="h-36 overflow-y-auto bg-slate-900 dark:bg-black rounded-lg p-3 font-mono text-[11px] space-y-1">
                      {events.length === 0 ? (
                        <p className="text-slate-500">Waiting for events…</p>
                      ) : (
                        events.map((ev) => (
                          <p
                            key={ev.id}
                            className={
                              ev.type === "incident_created"
                                ? "text-red-400"
                                : ev.type === "incident_resolved"
                                ? "text-green-400"
                                : "text-slate-400"
                            }
                          >
                            {new Date(ev.ts).toLocaleTimeString()} {ev.text}
                          </p>
                        ))
                      )}
                      <div ref={eventsEndRef} />
                    </div>
                  </div>

                  {openIncidents.length > 0 && (
                    <Button
                      variant="primary"
                      size="sm"
                      onClick={() => { markDone(3); advance(4) }}
                      className="mt-3 text-xs"
                    >
                      Incident detected — work the SOP <ChevronRight className="w-3.5 h-3.5 ml-1" />
                    </Button>
                  )}
                </Card>
              </motion.div>
            )}

            {/* Step 4 — Work the SOP */}
            {activeStep === 4 && (
              <motion.div
                key="step4"
                initial={{ opacity: 0, x: 12 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -12 }}
              >
                <Card className="p-5">
                  <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-100 mb-2 flex items-center gap-2">
                    <ExternalLink className="w-4 h-4 text-violet-500" />
                    Step 4 — Work the SOP
                  </h3>
                  <p className="text-xs text-slate-500 dark:text-slate-400 mb-4">
                    Open the incident to see the AI-generated root-cause analysis, propagation path,
                    and SOP runbook. Each step has a &quot;Mark done&quot; button to track remediation progress.
                  </p>

                  {openIncidents.length > 0 ? (
                    <div className="space-y-2 mb-4">
                      {openIncidents.slice(0, 3).map((inc) => (
                        <Link
                          key={inc.id}
                          href={`/dashboard/incidents/${inc.id}`}
                          className="flex items-center justify-between gap-3 p-4 rounded-xl border border-violet-200 dark:border-violet-800 bg-violet-50 dark:bg-violet-900/10 hover:bg-violet-100 dark:hover:bg-violet-900/20 transition-colors"
                        >
                          <div>
                            <p className="text-sm font-semibold text-violet-800 dark:text-violet-200">
                              {incidentLabel(inc)}
                            </p>
                            <p className="text-xs text-violet-500 mt-0.5">
                              <ClientDate iso={inc.detectedAt} />
                              {inc.rootCauseCandidate ? ` · root cause: ${inc.rootCauseCandidate}` : " · analysis pending"}
                            </p>
                          </div>
                          <ChevronRight className="w-4 h-4 text-violet-400 flex-shrink-0" />
                        </Link>
                      ))}
                    </div>
                  ) : (
                    <div className="mb-4 p-3 rounded-lg bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700">
                      <p className="text-xs text-slate-500">
                        No open incidents yet — go back to Step 3 and wait for detection.
                      </p>
                    </div>
                  )}

                  <div className="flex items-center gap-3">
                    <Link href="/dashboard/incidents">
                      <Button variant="outline" size="sm" className="text-xs">
                        <ExternalLink className="w-3.5 h-3.5 mr-1.5" /> View all incidents
                      </Button>
                    </Link>
                    <Button
                      variant="primary"
                      size="sm"
                      onClick={() => { markDone(4); advance(5) }}
                      className="text-xs"
                    >
                      SOP reviewed — restore service <ChevronRight className="w-3.5 h-3.5 ml-1" />
                    </Button>
                  </div>
                </Card>
              </motion.div>
            )}

            {/* Step 5 — Restore */}
            {activeStep === 5 && (
              <motion.div
                key="step5"
                initial={{ opacity: 0, x: 12 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -12 }}
              >
                <Card className="p-5">
                  <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-100 mb-2 flex items-center gap-2">
                    <RefreshCw className="w-4 h-4 text-green-500" />
                    Step 5 — Restore service
                  </h3>
                  <p className="text-xs text-slate-500 dark:text-slate-400 mb-4">
                    Heal{" "}
                    <code className="bg-slate-100 dark:bg-slate-800 px-1 rounded">ledger-service</code>.
                    TechGraph will detect the service is healthy again and auto-resolve the incident
                    after the required number of consecutive healthy polls.
                  </p>
                  <div className="mb-4 p-3 rounded-lg bg-slate-900 dark:bg-black font-mono text-xs text-green-400 overflow-x-auto">
                    <p className="text-slate-500 mb-1"># Admin API call that will be sent:</p>
                    <p>PUT {DEMO_API}/admin/services/ledger-service/status</p>
                    <p className="text-slate-400">
                      {`{ statusClass: "2xx", errorRate: 0.002, latencyP95: 45 }`}
                    </p>
                  </div>
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={handleHealLedger}
                    disabled={healing || !faultActive}
                    className="bg-green-600 hover:bg-green-700 text-white text-xs"
                  >
                    {healing ? (
                      <><RefreshCw className="w-3.5 h-3.5 mr-1.5 animate-spin" />Healing…</>
                    ) : !faultActive ? (
                      <>Service already healthy</>
                    ) : (
                      <><RefreshCw className="w-3.5 h-3.5 mr-1.5" />Heal ledger-service</>
                    )}
                  </Button>
                </Card>
              </motion.div>
            )}

            {/* Step 6 — Auto-resolution */}
            {activeStep === 6 && (
              <motion.div
                key="step6"
                initial={{ opacity: 0, x: 12 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -12 }}
              >
                <Card className="p-5">
                  <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-100 mb-2 flex items-center gap-2">
                    <CheckCircle2 className="w-4 h-4 text-green-500" />
                    Step 6 — See auto-resolution
                  </h3>
                  <p className="text-xs text-slate-500 dark:text-slate-400 mb-4">
                    Once the service is healthy for the configured number of consecutive polls,
                    TechGraph automatically resolves the incident. The audit trail records the full
                    timeline: fault detected → SOP worked → service restored → resolved.
                  </p>

                  <div className="flex items-center gap-3 p-4 rounded-xl bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 mb-4">
                    <CheckCircle2 className="w-5 h-5 text-green-600 dark:text-green-400 flex-shrink-0" />
                    <div>
                      <p className="text-sm font-semibold text-green-800 dark:text-green-300">Demo complete!</p>
                      <p className="text-xs text-green-600 dark:text-green-400 mt-0.5">
                        You&apos;ve seen fault injection, autonomous detection, SOP-guided remediation,
                        and auto-resolution — the full TechGraph loop.
                      </p>
                    </div>
                  </div>

                  <div className="flex flex-wrap gap-3">
                    <Link href="/dashboard/incidents">
                      <Button variant="outline" size="sm" className="text-xs">
                        <ExternalLink className="w-3.5 h-3.5 mr-1.5" /> View resolved incidents
                      </Button>
                    </Link>
                    <Link href="/dashboard/graph">
                      <Button variant="outline" size="sm" className="text-xs">
                        View service graph
                      </Button>
                    </Link>
                    <button
                      onClick={resetDemo}
                      className="flex items-center gap-1.5 text-xs text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"
                    >
                      <RefreshCw className="w-3.5 h-3.5" /> Run demo again
                    </button>
                  </div>
                </Card>
              </motion.div>
            )}
          </AnimatePresence>

          {/* ── SSE event log (always visible from step 3 to step 5) ── */}
          {activeStep >= 3 && activeStep < 6 && (
            <Card className="p-4">
              <h4 className="text-xs font-semibold text-slate-600 dark:text-slate-300 mb-2 flex items-center gap-2">
                Real-time event log
                {sseConnected ? (
                  <span className="text-[10px] font-normal text-green-500">● SSE connected</span>
                ) : (
                  <span className="text-[10px] font-normal text-slate-400">○ SSE offline (polling active)</span>
                )}
              </h4>
              <div className="h-32 overflow-y-auto bg-slate-900 dark:bg-black rounded-lg p-3 font-mono text-[11px] space-y-1">
                {events.length === 0 ? (
                  <p className="text-slate-500">No events yet…</p>
                ) : (
                  events.slice(-20).map((ev) => (
                    <p
                      key={ev.id}
                      className={
                        ev.type === "incident_created"
                          ? "text-red-400"
                          : ev.type === "incident_resolved"
                          ? "text-green-400"
                          : "text-slate-400"
                      }
                    >
                      {new Date(ev.ts).toLocaleTimeString()} {ev.text}
                    </p>
                  ))
                )}
                <div ref={eventsEndRef} />
              </div>
            </Card>
          )}
        </div>
      </div>
    </div>
  )
}
