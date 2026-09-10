"use client"

import { useState } from "react"
import { useParams, useRouter } from "next/navigation"
import Link from "next/link"
import { useGetIncidentQuery, useSubmitFeedbackMutation, useUpdateIncidentStatusMutation, useGetRelatedIncidentsQuery, useAssignIncidentMutation, useSubmitRCAFeedbackMutation } from "@/store/services"
import { Card } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Loading, ErrorState } from "@/components/ui/states"
import { ArrowLeft, Clock, CheckCircle2, AlertCircle, Target, ThumbsUp, CheckCheck, GitBranch, Zap, Activity, UserCheck } from "lucide-react"
import { motion } from "framer-motion"
import ClientDate from "@/components/ClientDate"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import type { IncidentStatus, IncidentPhase, RootCauseCandidate, Recommendation, TriggerSignal } from "@/types/api"
import { SOPStepsPanel } from "@/components/SOPStepsPanel"
import { AuditTrailPanel } from "@/components/AuditTrailPanel"
import { CodeContextPanel } from "@/components/CodeContextPanel"
import { ErrorBoundary } from "@/components/ErrorBoundary"
import { useAuth } from "@/providers/auth-provider"

const STATUS_COLORS: Record<IncidentStatus, "destructive" | "warning" | "success"> = {
  OPEN:         "destructive",
  ACKNOWLEDGED: "warning",
  RESOLVED:     "success",
}

const feedbackSchema = z.object({
  confirmedRootCause: z.string().min(1, "Required"),
  operatorNote:       z.string().optional(),
  resolution:         z.string().optional(),
})
type FeedbackForm = z.infer<typeof feedbackSchema>

function RCACard({ candidate, rank }: { candidate: RootCauseCandidate; rank: number }) {
  const pct = Math.round(candidate.confidence * 100)
  return (
    <div className="flex items-start gap-4 p-4 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900">
      <div className={`shrink-0 w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold ${
        rank === 1
          ? "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300"
          : "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300"
      }`}>
        #{rank}
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between gap-2 mb-1">
          <span className="font-semibold text-slate-900 dark:text-white">{candidate.name}</span>
          <span className={`text-sm font-mono font-bold ${pct >= 80 ? "text-red-600" : pct >= 50 ? "text-yellow-600" : "text-blue-600"}`}>
            {pct}%
          </span>
        </div>
        <p className="text-xs text-slate-500 mb-2">{candidate.faultType}</p>
        <div className="h-1.5 rounded-full bg-slate-100 dark:bg-slate-800">
          <div
            className={`h-1.5 rounded-full ${pct >= 80 ? "bg-red-500" : pct >= 50 ? "bg-yellow-500" : "bg-blue-500"}`}
            style={{ width: `${pct}%` }}
          />
        </div>
        {candidate.impactedCallers.length > 0 && (
          <p className="text-xs text-slate-400 mt-2">
            Impacted: {candidate.impactedCallers.join(", ")}
          </p>
        )}
      </div>
    </div>
  )
}

function RecommendationItem({ rec }: { rec: Recommendation }) {
  return (
    <div className="p-4 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900">
      <div className="flex items-start gap-3">
        <div className="shrink-0 w-6 h-6 rounded-full bg-blue-100 dark:bg-blue-900/40 flex items-center justify-center">
          <span className="text-xs font-bold text-blue-700 dark:text-blue-300">{rec.rank}</span>
        </div>
        <div className="flex-1">
          <div className="flex items-center gap-2 mb-1">
            <span className="font-semibold text-slate-900 dark:text-white">{rec.title}</span>
            <Badge variant="secondary" className="text-xs">{rec.category}</Badge>
          </div>
          <p className="text-xs text-slate-500 mb-2">{rec.reason}</p>
          <ol className="list-decimal list-inside space-y-0.5">
            {(rec.steps ?? []).map((step, i) => (
              <li key={i} className="text-xs text-slate-600 dark:text-slate-400">{step}</li>
            ))}
          </ol>
        </div>
      </div>
    </div>
  )
}

const FAULT_DESCRIPTIONS: Record<string, string> = {
  "high-error-rate": "Elevated error rate detected — service returning failures above normal threshold",
  "latency-anomaly": "Latency spike detected — response times significantly above baseline",
  "timeout-burst": "Timeout burst — service failing to respond within deadline",
  "unknown": "Anomalous behavior detected by the graph traversal engine",
}

function TriggerSignalRow({ signal, index }: { signal: TriggerSignal; index: number }) {
  const pct = Math.round(signal.confidence * 100)
  return (
    <div className="flex items-center gap-3 p-3 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900">
      <div className={`shrink-0 w-7 h-7 rounded-full flex items-center justify-center ${
        pct >= 80 ? "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300"
        : pct >= 50 ? "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300"
        : "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300"
      }`}>
        <Zap className="w-3.5 h-3.5" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between gap-2 mb-0.5">
          <span className="font-semibold text-sm text-slate-900 dark:text-white truncate">{signal.service}</span>
          <span className={`text-xs font-mono font-bold shrink-0 ${
            pct >= 80 ? "text-red-600" : pct >= 50 ? "text-yellow-600" : "text-blue-600"
          }`}>{pct}% confidence</span>
        </div>
        <p className="text-xs text-slate-500">{FAULT_DESCRIPTIONS[signal.faultType] ?? FAULT_DESCRIPTIONS["unknown"]}</p>
      </div>
    </div>
  )
}

export default function IncidentDetailPage() {
  const { incidentId } = useParams<{ incidentId: string }>()
  const router = useRouter()
  const { user } = useAuth()
  const [showFeedbackForm, setShowFeedbackForm] = useState(false)
  const [showResolveConfirm, setShowResolveConfirm] = useState(false)
  const [resolved, setResolved] = useState(false)
  const [sopAuditView, setSopAuditView] = useState<"sop" | "audit">("sop")
  const [showAssignInput, setShowAssignInput] = useState(false)
  const [assignEmail, setAssignEmail] = useState("")
  const [showWrongFeedback, setShowWrongFeedback] = useState(false)
  const [wrongFeedbackRoot, setWrongFeedbackRoot] = useState("")

  const { data: incident, isLoading, isError, refetch } = useGetIncidentQuery(incidentId)
  const [submitFeedback, { isLoading: submitting, isSuccess: submitted }] = useSubmitFeedbackMutation()
  const [updateIncidentStatus, { isLoading: isResolving }] = useUpdateIncidentStatusMutation()
  const [assignIncident, { isLoading: isAssigning }] = useAssignIncidentMutation()
  const [submitRCAFeedback, { isLoading: isSubmittingRCA, isSuccess: rcaFeedbackSent }] = useSubmitRCAFeedbackMutation()
  const { data: relatedData } = useGetRelatedIncidentsQuery(incidentId, { skip: !incidentId })
  const relatedIncidents = relatedData?.incidents ?? []

  const handleAssignToMe = () => {
    if (!user?.email) return
    assignIncident({ incidentId, assignTo: user.email })
  }

  const handleAssignCustom = () => {
    if (!assignEmail.trim()) return
    assignIncident({ incidentId, assignTo: assignEmail.trim() })
    setShowAssignInput(false)
    setAssignEmail("")
  }

  const handleUnassign = () => {
    assignIncident({ incidentId, assignTo: null })
  }

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FeedbackForm>({ resolver: zodResolver(feedbackSchema) })

  const onFeedback = async (values: FeedbackForm) => {
    await submitFeedback({
      incidentId,
      body: {
        ...values,
        resolvedAt: new Date().toISOString(),
      },
    })
    setShowFeedbackForm(false)
  }

  const onResolveIncident = async () => {
    await updateIncidentStatus({
      incidentId,
      status: "RESOLVED",
    })
    setShowResolveConfirm(false)
    setResolved(true)
  }

  if (isLoading) return <Loading text="Loading incident…" />
  if (isError || !incident) return <ErrorState message="Incident not found." retry={refetch} />

  const graphUrl = incident.projectId
    ? `/dashboard/graph?project=${incident.projectId}`
    : "/dashboard/graph"

  // Post-resolve success screen
  if (resolved) {
    return (
      <div className="max-w-2xl mx-auto">
        <motion.div
          initial={{ opacity: 0, scale: 0.96 }}
          animate={{ opacity: 1, scale: 1 }}
          className="text-center py-16"
        >
          <div className="w-16 h-16 rounded-full bg-green-100 dark:bg-green-900/30 flex items-center justify-center mx-auto mb-6">
            <CheckCircle2 className="w-9 h-9 text-green-600 dark:text-green-400" />
          </div>
          <h2 className="text-2xl font-bold text-slate-900 dark:text-white mb-2">Incident Resolved</h2>
          <p className="text-slate-500 dark:text-slate-400 mb-8 max-w-sm mx-auto">
            The incident has been marked as resolved. Check the service graph to confirm all nodes are healthy.
          </p>
          <div className="flex gap-3 justify-center">
            <Link
              href={graphUrl}
              className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-700 text-white font-semibold text-sm transition-colors"
            >
              <GitBranch className="w-4 h-4" />
              View Project Graph
            </Link>
            <button
              onClick={() => router.push(`/dashboard/incidents?project=${incident.projectId}`)}
              className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl border border-slate-200 dark:border-slate-700 text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800 font-semibold text-sm transition-colors"
            >
              <ArrowLeft className="w-4 h-4" />
              Back to Incidents
            </button>
          </div>
        </motion.div>
      </div>
    )
  }

  return (
    <div className="max-w-4xl mx-auto">
      <motion.div initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }}>
        {/* Navigation bar */}
        <div className="flex items-center justify-between mb-6">
          <Link
            href={graphUrl}
            className="flex items-center gap-1 text-sm text-slate-500 hover:text-slate-900 dark:hover:text-white transition-colors"
          >
            <ArrowLeft className="w-4 h-4" /> Back to Service Graph
          </Link>
          <Link
            href={graphUrl}
            className="flex items-center gap-1.5 text-sm font-medium text-blue-600 dark:text-blue-400 hover:underline"
          >
            <GitBranch className="w-3.5 h-3.5" />
            View in Project Graph
          </Link>
        </div>

        <div className="flex flex-wrap items-start justify-between gap-4 mb-8">
          <div>
            <div className="flex items-center gap-3 mb-2">
              <h1 className="text-2xl font-bold text-slate-900 dark:text-white">Incident Analysis</h1>
              <Badge variant={STATUS_COLORS[incident.status]}>{incident.status}</Badge>
            </div>
            <p className="text-sm text-slate-500 flex items-center gap-1.5">
              <Clock className="w-3.5 h-3.5" />
              Detected <ClientDate iso={incident.detectedAt} />
              {incident.resolvedAt && (
                <> · Resolved <ClientDate iso={incident.resolvedAt} /></>
              )}
            </p>
            <p className="text-xs text-slate-400 mt-0.5">
              Environment: {incident.environment} · Graph v{incident.graphVersion}
            </p>
          </div>

          <div className="flex flex-wrap gap-2 items-center">
            {/* Assignment section */}
            {incident.assignedTo ? (
              <div className="flex items-center gap-2">
                <span className="text-xs text-slate-500 flex items-center gap-1">
                  <UserCheck className="w-3.5 h-3.5 text-blue-500" />
                  Assigned to: <span className="font-medium text-slate-700 dark:text-slate-300">{incident.assignedTo}</span>
                </span>
                <Button variant="outline" size="sm" onClick={handleUnassign} disabled={isAssigning}>
                  Unassign
                </Button>
              </div>
            ) : (
              <div className="flex items-center gap-1.5">
                <Button variant="outline" size="sm" onClick={handleAssignToMe} disabled={isAssigning || !user?.email}>
                  <UserCheck className="w-3.5 h-3.5 mr-1" />
                  Assign to me
                </Button>
                {showAssignInput ? (
                  <div className="flex gap-1">
                    <input
                      type="email"
                      value={assignEmail}
                      onChange={(e) => setAssignEmail(e.target.value)}
                      placeholder="user@company.com"
                      className="text-xs rounded border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-1 w-44"
                      onKeyDown={(e) => e.key === "Enter" && handleAssignCustom()}
                    />
                    <Button size="sm" onClick={handleAssignCustom} disabled={isAssigning}>Assign</Button>
                    <Button size="sm" variant="ghost" onClick={() => setShowAssignInput(false)}>Cancel</Button>
                  </div>
                ) : (
                  <Button variant="ghost" size="sm" onClick={() => setShowAssignInput(true)}>
                    Other…
                  </Button>
                )}
              </div>
            )}

            {incident.status !== "RESOLVED" && !submitted && (
              <>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setShowFeedbackForm(!showFeedbackForm)}
                >
                  <ThumbsUp className="w-3.5 h-3.5 mr-1.5" />
                  Submit Feedback
                </Button>
                <Button
                  size="sm"
                  className="bg-green-600 hover:bg-green-700 text-white"
                  onClick={() => setShowResolveConfirm(true)}
                >
                  <CheckCheck className="w-3.5 h-3.5 mr-1.5" />
                  Mark Resolved
                </Button>
              </>
            )}
          </div>
        </div>

        {/* Resolve confirmation */}
        {showResolveConfirm && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
            <motion.div
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              className="w-full max-w-sm bg-white dark:bg-slate-900 rounded-2xl shadow-2xl border border-slate-200 dark:border-slate-700 p-6"
            >
              <div className="flex items-center justify-center w-10 h-10 rounded-full bg-green-100 dark:bg-green-900/30 mx-auto mb-4">
                <CheckCircle2 className="w-6 h-6 text-green-600 dark:text-green-400" />
              </div>
              <h3 className="text-lg font-bold text-slate-900 dark:text-white text-center mb-2">
                Mark Incident as Resolved?
              </h3>
              <p className="text-sm text-slate-600 dark:text-slate-400 text-center mb-6">
                This will update the incident status to RESOLVED and record the resolution time.
              </p>
              <div className="flex gap-3">
                <Button
                  type="button"
                  variant="outline"
                  className="flex-1"
                  onClick={() => setShowResolveConfirm(false)}
                  disabled={isResolving}
                >
                  Cancel
                </Button>
                <Button
                  type="button"
                  className="flex-1 bg-green-600 hover:bg-green-700 text-white gap-2"
                  onClick={onResolveIncident}
                  disabled={isResolving}
                >
                  {isResolving ? "Resolving..." : "Mark Resolved"}
                </Button>
              </div>
            </motion.div>
          </div>
        )}

        {/* Feedback form */}
        {showFeedbackForm && (
          <Card className="mb-8 p-5 border-blue-200 dark:border-blue-700">
            <h3 className="font-semibold text-slate-900 dark:text-white mb-4">Operator Feedback</h3>
            <form onSubmit={handleSubmit(onFeedback)} className="space-y-4">
              <div>
                <label className="text-sm text-slate-700 dark:text-slate-300 block mb-1">
                  Confirmed root cause service <span className="text-red-500">*</span>
                </label>
                <input
                  {...register("confirmedRootCause")}
                  className="w-full text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-900 dark:text-white"
                  placeholder="e.g. payment-service"
                />
                {errors.confirmedRootCause && (
                  <p className="text-xs text-red-500 mt-1">{errors.confirmedRootCause.message}</p>
                )}
              </div>
              <div>
                <label className="text-sm text-slate-700 dark:text-slate-300 block mb-1">Notes (optional)</label>
                <textarea
                  {...register("operatorNote")}
                  className="w-full text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-900 dark:text-white"
                  rows={2}
                  placeholder="Additional context…"
                />
              </div>
              <div>
                <label className="text-sm text-slate-700 dark:text-slate-300 block mb-1">Resolution taken (optional)</label>
                <input
                  {...register("resolution")}
                  className="w-full text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-900 dark:text-white"
                  placeholder="e.g. Rolled back v2.1.4 → v2.1.3"
                />
              </div>
              <div className="flex gap-3">
                <Button type="submit" variant="primary" size="sm" disabled={submitting}>
                  {submitting ? "Submitting…" : "Submit"}
                </Button>
                <Button type="button" variant="ghost" size="sm" onClick={() => setShowFeedbackForm(false)}>
                  Cancel
                </Button>
              </div>
            </form>
          </Card>
        )}

        {submitted && (
          <div className="mb-6 flex items-center gap-2 px-4 py-3 rounded-xl bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-300 text-sm">
            <CheckCircle2 className="w-4 h-4" /> Feedback recorded successfully.
          </div>
        )}

        {/* Triggering Signals */}
        {(incident.triggerSignals ?? []).length > 0 && (
          <div className="mb-6">
            <h2 className="text-base font-semibold text-slate-900 dark:text-white mb-3 flex items-center gap-2">
              <Activity className="w-4 h-4 text-orange-500" /> What triggered this incident
            </h2>
            <div className="space-y-2">
              {(incident.triggerSignals ?? []).map((sig, i) => (
                <TriggerSignalRow key={`${sig.nodeId || 'signal'}-${i}`} signal={sig} index={i} />
              ))}
            </div>
          </div>
        )}

        {/* Triggering service fallback */}
        {(incident.triggerSignals ?? []).length === 0 && incident.service && (
          <div className="mb-6 px-4 py-3 rounded-lg bg-orange-50 dark:bg-orange-900/20 border border-orange-200 dark:border-orange-700 text-sm">
            <span className="font-semibold text-orange-800 dark:text-orange-300">Triggered by: </span>
            <span className="text-orange-700 dark:text-orange-400 font-mono">{incident.service}</span>
            <span className="text-orange-600 dark:text-orange-500"> — anomalous signal received from this service</span>
          </div>
        )}

        {/* SOP Phase + Playbook — the core SOP sequencing panel */}
        <div className="mb-6">
          {/* SOP / Audit toggle */}
          <div className="flex gap-1 p-1 bg-slate-100 dark:bg-slate-800 rounded-xl w-fit mb-3">
            <button
              onClick={() => setSopAuditView("sop")}
              className={`px-4 py-1.5 text-sm font-medium rounded-lg transition-colors ${
                sopAuditView === "sop"
                  ? "bg-white dark:bg-slate-700 text-slate-900 dark:text-white shadow-sm"
                  : "text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200"
              }`}
            >
              SOP Playbook
            </button>
            <button
              onClick={() => setSopAuditView("audit")}
              className={`px-4 py-1.5 text-sm font-medium rounded-lg transition-colors ${
                sopAuditView === "audit"
                  ? "bg-white dark:bg-slate-700 text-slate-900 dark:text-white shadow-sm"
                  : "text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200"
              }`}
            >
              Audit Trail
            </button>
          </div>
          <Card className="p-6">
            {sopAuditView === "sop" ? (
              <ErrorBoundary context="SOP Playbook">
                <SOPStepsPanel
                  incidentId={incidentId as string}
                  currentPhase={(incident.phase ?? "DETECTING") as IncidentPhase}
                  projectId={incident.projectId}
                  serviceId={incident.rootCauseCandidates?.[0]?.nodeId ?? incident.service}
                  faultType={incident.rootCauseCandidates?.[0]?.faultType}
                />
              </ErrorBoundary>
            ) : (
              <AuditTrailPanel incidentId={incidentId as string} />
            )}
          </Card>
        </div>

        {/* Code Context — shows recent commits when repo is configured */}
        {incident.projectId && (incident.rootCauseCandidates?.[0]?.nodeId ?? incident.service) && (
          <div className="mb-6">
            <Card className="p-6">
              <h2 className="text-base font-semibold text-slate-900 dark:text-white mb-4 flex items-center gap-2">
                <span className="text-base">🔗</span> Code Context
                <span className="text-xs font-normal text-slate-400 ml-1">— which commit likely caused this</span>
              </h2>
              <ErrorBoundary context="Code Context">
                <CodeContextPanel
                  projectId={incident.projectId}
                  serviceId={incident.rootCauseCandidates?.[0]?.nodeId ?? incident.service ?? ""}
                  serviceName={incident.rootCauseCandidates?.[0]?.name ?? incident.service}
                />
              </ErrorBoundary>
            </Card>
          </div>
        )}

        {/* Related Incidents — same root cause */}
        {relatedIncidents.length > 0 && (
          <div className="mb-6">
            <Card className="p-5">
              <p className="text-sm font-semibold text-amber-700 dark:text-amber-300 mb-3 flex items-center gap-2">
                <span>⚠</span> {relatedIncidents.length} other incident{relatedIncidents.length !== 1 ? "s" : ""} share the same root cause
              </p>
              <div className="space-y-2">
                {relatedIncidents.map((rel) => (
                  <Link key={rel.id} href={`/dashboard/incidents/${rel.id}`} className="block p-2 rounded-lg border border-amber-200 dark:border-amber-800 hover:bg-amber-50 dark:hover:bg-amber-900/20 transition-colors">
                    <div className="flex items-center justify-between text-xs">
                      <span className="font-medium">{rel.service}</span>
                      <span className="text-slate-400">{Math.round(rel.confidence * 100)}% confidence</span>
                    </div>
                  </Link>
                ))}
              </div>
            </Card>
          </div>
        )}

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* RCA Candidates */}
          <div>
            <h2 className="text-lg font-semibold text-slate-900 dark:text-white mb-3 flex items-center gap-2">
              <Target className="w-5 h-5 text-red-500" /> Root Cause Analysis
            </h2>
            <div className="space-y-3">
              {(incident.rootCauseCandidates ?? []).length === 0 ? (
                <p className="text-sm text-slate-500 dark:text-slate-400">No root cause candidates identified yet.</p>
              ) : (
                (incident.rootCauseCandidates ?? []).map((c) => (
                  <RCACard key={c.nodeId} candidate={c} rank={c.rank} />
                ))
              )}
            </div>

            {/* RCA feedback — was the root cause correct? */}
            {incident.rootCauseCandidates?.[0] && incident.status === "RESOLVED" && !rcaFeedbackSent && (
              <div className="flex items-center gap-2 mt-3 pt-3 border-t border-slate-100 dark:border-slate-800 text-xs text-slate-500">
                <span>Was this root cause correct?</span>
                <button
                  onClick={() => submitRCAFeedback({ incidentId, wasCorrect: true })}
                  disabled={isSubmittingRCA}
                  className="px-2 py-1 rounded bg-green-50 text-green-700 hover:bg-green-100 dark:bg-green-900/20 dark:text-green-400 disabled:opacity-50"
                >
                  ✓ Yes
                </button>
                <button
                  onClick={() => setShowWrongFeedback(true)}
                  disabled={isSubmittingRCA}
                  className="px-2 py-1 rounded bg-red-50 text-red-700 hover:bg-red-100 dark:bg-red-900/20 dark:text-red-400 disabled:opacity-50"
                >
                  ✗ No
                </button>
              </div>
            )}
            {rcaFeedbackSent && (
              <p className="mt-2 pt-2 border-t border-slate-100 dark:border-slate-800 text-xs text-green-600 dark:text-green-400">
                Feedback recorded. Thank you.
              </p>
            )}
            {showWrongFeedback && (
              <div className="mt-3 pt-3 border-t border-slate-100 dark:border-slate-800 space-y-2">
                <p className="text-xs text-slate-500">What was the actual root cause?</p>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={wrongFeedbackRoot}
                    onChange={(e) => setWrongFeedbackRoot(e.target.value)}
                    placeholder="e.g. svc_database"
                    className="flex-1 text-xs rounded border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-1 text-slate-900 dark:text-white"
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && wrongFeedbackRoot.trim()) {
                        submitRCAFeedback({ incidentId, wasCorrect: false, actualRoot: wrongFeedbackRoot.trim() })
                        setShowWrongFeedback(false)
                      }
                    }}
                  />
                  <button
                    onClick={() => {
                      if (wrongFeedbackRoot.trim()) {
                        submitRCAFeedback({ incidentId, wasCorrect: false, actualRoot: wrongFeedbackRoot.trim() })
                        setShowWrongFeedback(false)
                      }
                    }}
                    disabled={isSubmittingRCA || !wrongFeedbackRoot.trim()}
                    className="px-3 py-1 text-xs rounded bg-slate-800 text-white hover:bg-slate-700 disabled:opacity-50"
                  >
                    Submit
                  </button>
                  <button
                    onClick={() => setShowWrongFeedback(false)}
                    className="px-2 py-1 text-xs text-slate-500 hover:text-slate-700"
                  >
                    Cancel
                  </button>
                </div>
              </div>
            )}

            {/* Blast radius */}
            {(incident.blastRadius ?? []).length > 0 && (
              <div className="mt-4 p-4 rounded-xl bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700">
                <p className="text-xs font-semibold text-amber-800 dark:text-amber-300 mb-1 flex items-center gap-1.5">
                  <AlertCircle className="w-3.5 h-3.5" /> Blast Radius
                </p>
                <div className="flex flex-wrap gap-1.5">
                  {(incident.blastRadius ?? []).map((s) => (
                    <span
                      key={s}
                      className="text-xs px-2 py-0.5 rounded-full bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300"
                    >
                      {s}
                    </span>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Recommendations */}
          <div>
            <h2 className="text-lg font-semibold text-slate-900 dark:text-white mb-3 flex items-center gap-2">
              <CheckCircle2 className="w-5 h-5 text-green-500" /> Recommendations
            </h2>
            <div className="space-y-3">
              {(incident.recommendations ?? []).length === 0 ? (
                <p className="text-sm text-slate-500 dark:text-slate-400">No recommendations available.</p>
              ) : (
                (incident.recommendations ?? []).map((r) => (
                  <RecommendationItem key={r.rank} rec={r} />
                ))
              )}
            </div>
          </div>
        </div>

        {/* Previous feedback */}
        {incident.feedback && (
          <Card className="mt-8 p-5">
            <h3 className="font-semibold text-slate-900 dark:text-white mb-3">Recorded Feedback</h3>
            <dl className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
              <div>
                <dt className="text-xs text-slate-400 uppercase tracking-wide mb-0.5">Confirmed Root Cause</dt>
                <dd className="font-medium text-slate-900 dark:text-white">{incident.feedback.confirmedRootCause}</dd>
              </div>
              {incident.feedback.resolution && (
                <div>
                  <dt className="text-xs text-slate-400 uppercase tracking-wide mb-0.5">Resolution</dt>
                  <dd className="text-slate-700 dark:text-slate-300">{incident.feedback.resolution}</dd>
                </div>
              )}
              {incident.feedback.operatorNote && (
                <div className="sm:col-span-2">
                  <dt className="text-xs text-slate-400 uppercase tracking-wide mb-0.5">Notes</dt>
                  <dd className="text-slate-700 dark:text-slate-300">{incident.feedback.operatorNote}</dd>
                </div>
              )}
            </dl>
          </Card>
        )}
      </motion.div>
    </div>
  )
}
