"use client"

import { useState } from "react"
import { Download, Pencil } from "lucide-react"
import { useGetSOPPlaybookQuery, useUpdateSOPStepMutation, useGetPlaybookLearningQuery } from "@/store/services"
import { SOPPlaybookEditor } from "@/components/SOPPlaybookEditor"
import type { SOPStep, IncidentPhase } from "@/types/api"

const PHASE_ORDER: IncidentPhase[] = [
  "DETECTING",
  "NARROWING",
  "CONFIRMED",
  "TRIAGING",
  "FIXING",
  "VERIFYING",
  "RESOLVED",
]

const PHASE_LABELS: Record<IncidentPhase, string> = {
  DETECTING:  "Detecting",
  NARROWING:  "Narrowing",
  CONFIRMED:  "Confirmed",
  TRIAGING:   "Triaging",
  FIXING:     "Fixing",
  VERIFYING:  "Verifying",
  RESOLVED:   "Resolved",
}

const PHASE_DESCRIPTION: Record<IncidentPhase, string> = {
  DETECTING:  "Signal received, running BFS",
  NARROWING:  "Accumulating evidence",
  CONFIRMED:  "Root cause identified",
  TRIAGING:   "SOP playbook ready",
  FIXING:     "Operator executing steps",
  VERIFYING:  "Watching for healthy signal",
  RESOLVED:   "Incident closed",
}

const STATUS_COLORS: Record<string, string> = {
  PENDING:  "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400",
  ACTIVE:   "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300",
  DONE:     "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300",
  FAILED:   "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300",
}

interface SOPStepsPanelProps {
  incidentId: string
  currentPhase?: IncidentPhase
  projectId?: string
  serviceId?: string
  faultType?: string
}

export function SOPStepsPanel({
  incidentId,
  currentPhase = "DETECTING",
  projectId,
  serviceId,
  faultType,
}: SOPStepsPanelProps) {
  const [showEditor, setShowEditor] = useState(false)

  const { data: playbook, isLoading } = useGetSOPPlaybookQuery(incidentId, {
    pollingInterval: currentPhase === "RESOLVED" ? 0 : 5000,
  })
  const [updateStep, { isLoading: isUpdating }] = useUpdateSOPStepMutation()

  const { data: learningData } = useGetPlaybookLearningQuery(
    { projectId: projectId!, serviceId: serviceId!, faultType: faultType! },
    { skip: !projectId || !serviceId || !faultType }
  )

  const learningMap: Record<number, { successRate: number; totalCount: number }> = {}
  if (learningData?.steps) {
    for (const s of learningData.steps) {
      learningMap[s.stepOrder] = { successRate: s.successRate, totalCount: s.totalCount }
    }
  }

  const phase = (playbook?.phase ?? currentPhase) as IncidentPhase
  const phaseIndex = PHASE_ORDER.indexOf(phase)
  const steps: SOPStep[] = playbook?.steps ?? []
  const service = playbook?.service ?? ""

  const handleStep = async (order: number, status: "DONE" | "FAILED") => {
    await updateStep({ incidentId, stepOrder: order, status })
    // RTK Query invalidateTags in the mutation definition triggers a re-fetch automatically
  }

  const exportRunbook = () => {
    const md = [
      `# SOP Runbook: ${service} — ${phase}`,
      `Generated: ${new Date().toISOString()}`,
      "",
      ...steps.map((s) =>
        [
          `## Step ${s.order}: ${s.title}`,
          `**Action:** ${s.action}`,
          s.expectedSignal ? `**Expected signal:** ${s.expectedSignal}` : "",
          s.autoVerify
            ? "_Auto-verified by system when signal arrives_"
            : "_Requires operator confirmation_",
          `**Status:** ${s.status}`,
        ]
          .filter(Boolean)
          .join("\n")
      ),
    ].join("\n\n")

    const blob = new Blob([md], { type: "text/markdown" })
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = `sop-runbook-${incidentId}.md`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="space-y-6">
      {/* Phase stepper */}
      <div>
        <h3 className="text-sm font-semibold text-slate-700 dark:text-slate-200 mb-3">
          SOP Phase
        </h3>
        <div className="flex items-start gap-1 overflow-x-auto pb-2">
          {PHASE_ORDER.map((p, i) => {
            const isActive = p === phase
            const isDone = i < phaseIndex
            return (
              <div key={p} className="flex items-center min-w-0">
                <div className="flex flex-col items-center">
                  <div
                    className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold flex-shrink-0 ${
                      isActive
                        ? "bg-blue-600 text-white ring-4 ring-blue-200 dark:ring-blue-900 animate-pulse"
                        : isDone
                        ? "bg-green-500 text-white"
                        : "bg-slate-200 dark:bg-slate-700 text-slate-400 dark:text-slate-500"
                    }`}
                  >
                    {isDone ? "✓" : i + 1}
                  </div>
                  <span
                    className={`mt-1 text-[10px] font-medium text-center whitespace-nowrap ${
                      isActive
                        ? "text-blue-700 dark:text-blue-300"
                        : isDone
                        ? "text-green-600 dark:text-green-400"
                        : "text-slate-400 dark:text-slate-500"
                    }`}
                  >
                    {PHASE_LABELS[p]}
                  </span>
                  {isActive && (
                    <span className="text-[9px] text-slate-500 dark:text-slate-400 text-center max-w-[64px]">
                      {PHASE_DESCRIPTION[p]}
                    </span>
                  )}
                </div>
                {i < PHASE_ORDER.length - 1 && (
                  <div
                    className={`h-0.5 w-6 flex-shrink-0 mb-6 mx-0.5 ${
                      i < phaseIndex
                        ? "bg-green-400 dark:bg-green-600"
                        : "bg-slate-200 dark:bg-slate-700"
                    }`}
                  />
                )}
              </div>
            )
          })}
        </div>
      </div>

      {/* SOP Steps */}
      {steps.length > 0 && (
        <div>
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-semibold text-slate-700 dark:text-slate-200">
              Playbook Steps
            </h3>
            <div className="flex items-center gap-2">
              {projectId && serviceId && (phase === "TRIAGING" || phase === "FIXING") && (
                <button
                  onClick={() => setShowEditor(true)}
                  className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium rounded-lg border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 hover:border-violet-400 hover:text-violet-600 dark:hover:text-violet-400 transition-colors"
                  title="Edit playbook"
                >
                  <Pencil className="w-3.5 h-3.5" />
                  Edit Playbook
                </button>
              )}
              <button
                onClick={exportRunbook}
                className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium rounded-lg border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 hover:border-blue-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
                title="Export runbook as Markdown"
              >
                <Download className="w-3.5 h-3.5" />
                Export Runbook
              </button>
            </div>
          </div>
          {isLoading ? (
            <div className="text-sm text-slate-400 animate-pulse">Loading playbook…</div>
          ) : (
            <ol className="space-y-3">
              {steps.map((step, stepIdx) => {
                const prevStep = steps[stepIdx - 1]
                const nextStep = steps[stepIdx + 1]
                const showParallelHint =
                  step.status === "PENDING" &&
                  !step.autoVerify &&
                  prevStep?.status === "ACTIVE" &&
                  !prevStep?.autoVerify &&
                  !prevStep?.expectedSignal
                const showActiveParallelHint =
                  step.status === "ACTIVE" &&
                  !step.autoVerify &&
                  !step.expectedSignal &&
                  nextStep?.status === "PENDING" &&
                  !nextStep?.autoVerify

                const learning = learningMap[step.order]

                return (
                <li
                  key={step.order}
                  className={`rounded-lg border p-4 transition-all ${
                    step.status === "ACTIVE"
                      ? "border-blue-300 dark:border-blue-700 bg-blue-50/50 dark:bg-blue-950/30"
                      : step.status === "DONE"
                      ? "border-green-200 dark:border-green-800 bg-green-50/30 dark:bg-green-950/20 opacity-70"
                      : step.status === "FAILED"
                      ? "border-red-300 dark:border-red-700 bg-red-50/30 dark:bg-red-950/20"
                      : "border-slate-200 dark:border-slate-700 opacity-50"
                  }`}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex items-start gap-3 min-w-0">
                      <span
                        className={`mt-0.5 w-6 h-6 rounded-full flex-shrink-0 flex items-center justify-center text-xs font-bold ${
                          step.status === "DONE"
                            ? "bg-green-500 text-white"
                            : step.status === "FAILED"
                            ? "bg-red-500 text-white"
                            : step.status === "ACTIVE"
                            ? "bg-blue-600 text-white"
                            : "bg-slate-200 dark:bg-slate-700 text-slate-400"
                        }`}
                      >
                        {step.status === "DONE" ? "✓" : step.status === "FAILED" ? "✗" : step.order}
                      </span>
                      <div className="min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <p className="text-sm font-medium text-slate-800 dark:text-slate-100">
                            {step.title}
                          </p>
                          {showParallelHint && (
                            <span className="inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded text-[10px] font-semibold bg-violet-100 dark:bg-violet-900/30 text-violet-700 dark:text-violet-300">
                              ↕ parallel
                            </span>
                          )}
                          {learning && learning.totalCount > 0 && (
                            learning.totalCount < 5 ? (
                              <span
                                className="inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded text-[10px] font-semibold bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300"
                                title={`Success rate based on ${learning.totalCount} resolved incidents`}
                              >
                                ⚠ Low confidence (only {learning.totalCount} samples)
                              </span>
                            ) : (
                              <span
                                className="inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded text-[10px] font-semibold bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-300"
                                title={`Success rate based on ${learning.totalCount} resolved incidents`}
                              >
                                ✓ {Math.round(learning.successRate * 100)}% ({Math.round(learning.successRate * learning.totalCount)}/{learning.totalCount})
                              </span>
                            )
                          )}
                        </div>
                        {/* Confidence bar */}
                        {learning && learning.totalCount > 0 && (
                          <div className="mt-1 mb-0.5">
                            <div className="h-1 w-full rounded-full bg-slate-100 dark:bg-slate-800 overflow-hidden">
                              <div
                                className={`h-1 rounded-full transition-all ${
                                  learning.successRate > 0.7
                                    ? "bg-green-500"
                                    : learning.successRate >= 0.4
                                    ? "bg-yellow-500"
                                    : "bg-red-500"
                                }`}
                                style={{ width: `${Math.round(learning.successRate * 100)}%` }}
                              />
                            </div>
                          </div>
                        )}
                        <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                          {step.action}
                        </p>
                        {step.expectedSignal && (
                          <div className="mt-2 flex items-center gap-1.5">
                            {step.autoVerify ? (
                              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium bg-cyan-100 dark:bg-cyan-900/30 text-cyan-700 dark:text-cyan-300">
                                <span className={step.status === "ACTIVE" ? "animate-pulse" : ""}>●</span>
                                Auto-verify: {step.expectedSignal}
                              </span>
                            ) : (
                              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400">
                                Expected: {step.expectedSignal}
                              </span>
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                    <span className={`flex-shrink-0 px-2 py-0.5 rounded-full text-[10px] font-semibold ${STATUS_COLORS[step.status] ?? STATUS_COLORS.PENDING}`}>
                      {step.status}
                    </span>
                  </div>

                  {/* Action buttons for ACTIVE non-auto-verify steps */}
                  {step.status === "ACTIVE" && !step.autoVerify && (
                    <div className="flex gap-2 mt-3 pt-3 border-t border-slate-200 dark:border-slate-700">
                      <button
                        onClick={() => handleStep(step.order, "DONE")}
                        disabled={isUpdating}
                        className="flex-1 py-1.5 text-xs font-semibold rounded-md bg-green-600 hover:bg-green-700 text-white transition-colors disabled:opacity-50"
                      >
                        Mark Done
                      </button>
                      <button
                        onClick={() => handleStep(step.order, "FAILED")}
                        disabled={isUpdating}
                        className="flex-1 py-1.5 text-xs font-semibold rounded-md border border-red-300 dark:border-red-700 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950/20 transition-colors disabled:opacity-50"
                      >
                        Mark Failed
                      </button>
                    </div>
                  )}
                  {step.status === "ACTIVE" && step.autoVerify && (
                    <div className="mt-3 pt-3 border-t border-blue-200 dark:border-blue-800 text-xs text-blue-600 dark:text-blue-300 flex items-center gap-1.5">
                      <span className="inline-block w-2 h-2 rounded-full bg-blue-500 animate-pulse" />
                      Watching for signal: <strong>{step.expectedSignal}</strong> — will auto-advance
                    </div>
                  )}
                  {showActiveParallelHint && (
                    <div className="mt-3 pt-3 border-t border-violet-200 dark:border-violet-800 text-xs text-violet-700 dark:text-violet-300 flex items-center gap-1.5">
                      <span className="text-sm">↕</span>
                      You can work on the next step in parallel while completing this one.
                    </div>
                  )}
                </li>
                )
              })}
            </ol>
          )}
        </div>
      )}

      {steps.length === 0 && phase !== "DETECTING" && !isLoading && (
        <div className="text-sm text-slate-400 dark:text-slate-500 text-center py-4">
          Playbook will be generated once root cause is confirmed
        </div>
      )}

      {showEditor && projectId && serviceId && (
        <SOPPlaybookEditor
          projectId={projectId}
          serviceId={serviceId}
          serviceName={service || serviceId}
          faultType={faultType}
          onClose={() => setShowEditor(false)}
        />
      )}
    </div>
  )
}
