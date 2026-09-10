"use client"

import { useState, useEffect } from "react"
import { X, Plus, Trash2, ChevronUp, ChevronDown, Save, History, RotateCcw } from "lucide-react"
import {
  useGetSOPPlaybookHistoryQuery,
  useUpdateSOPPlaybookMutation,
  useRevertSOPPlaybookMutation,
} from "@/store/services"
import type { SOPStep, SOPStepStatus } from "@/types/api"

interface SOPPlaybookEditorProps {
  projectId: string
  serviceId: string
  serviceName: string
  faultType?: string
  onClose: () => void
}

const EMPTY_STEP = (order: number): SOPStep => ({
  order,
  title: "",
  action: "",
  expectedSignal: "",
  autoVerify: false,
  status: "PENDING" as SOPStepStatus,
})

export function SOPPlaybookEditor({
  projectId,
  serviceId,
  serviceName,
  faultType,
  onClose,
}: SOPPlaybookEditorProps) {
  const [steps, setSteps] = useState<SOPStep[]>([])
  const [initialized, setInitialized] = useState(false)
  const [showHistory, setShowHistory] = useState(false)
  const [saveSuccess, setSaveSuccess] = useState(false)

  const { data: historyData, isLoading: historyLoading } = useGetSOPPlaybookHistoryQuery(
    { projectId, serviceId },
    { skip: !projectId || !serviceId }
  )
  const [updatePlaybook, { isLoading: saving }] = useUpdateSOPPlaybookMutation()
  const [revertPlaybook, { isLoading: reverting }] = useRevertSOPPlaybookMutation()

  // Pre-populate steps from the latest version
  useEffect(() => {
    if (!initialized && historyData) {
      const latest = historyData.versions[0]
      if (latest?.steps?.length) {
        setSteps(latest.steps.map((s) => ({ ...s })))
      } else {
        setSteps([EMPTY_STEP(1)])
      }
      setInitialized(true)
    }
  }, [historyData, initialized])

  // If no history at all, give one blank step
  useEffect(() => {
    if (!historyLoading && !initialized) {
      setSteps([EMPTY_STEP(1)])
      setInitialized(true)
    }
  }, [historyLoading, initialized])

  const renumber = (arr: SOPStep[]): SOPStep[] =>
    arr.map((s, i) => ({ ...s, order: i + 1 }))

  const addStep = () => {
    setSteps((prev) => renumber([...prev, EMPTY_STEP(prev.length + 1)]))
  }

  const deleteStep = (idx: number) => {
    setSteps((prev) => renumber(prev.filter((_, i) => i !== idx)))
  }

  const moveUp = (idx: number) => {
    if (idx === 0) return
    setSteps((prev) => {
      const next = [...prev]
      ;[next[idx - 1], next[idx]] = [next[idx], next[idx - 1]]
      return renumber(next)
    })
  }

  const moveDown = (idx: number) => {
    setSteps((prev) => {
      if (idx >= prev.length - 1) return prev
      const next = [...prev]
      ;[next[idx], next[idx + 1]] = [next[idx + 1], next[idx]]
      return renumber(next)
    })
  }

  const updateField = <K extends keyof SOPStep>(idx: number, field: K, value: SOPStep[K]) => {
    setSteps((prev) => prev.map((s, i) => (i === idx ? { ...s, [field]: value } : s)))
  }

  useEffect(() => {
    if (saveSuccess) {
      const t = setTimeout(() => setSaveSuccess(false), 2500)
      return () => clearTimeout(t)
    }
  }, [saveSuccess])

  const handleSave = async () => {
    const result = await updatePlaybook({ projectId, serviceId, steps })
    if ("data" in result) {
      setSaveSuccess(true)
    }
  }

  const handleRevert = async (version: number) => {
    const result = await revertPlaybook({ projectId, serviceId, version })
    if ("data" in result && result.data) {
      setSteps(result.data.steps.map((s) => ({ ...s })))
      setShowHistory(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
      <div className="w-full max-w-3xl max-h-[90vh] flex bg-white dark:bg-slate-900 rounded-2xl shadow-2xl border border-slate-200 dark:border-slate-700 overflow-hidden">
        {/* Main editor */}
        <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
          {/* Header */}
          <div className="flex items-center justify-between px-6 py-4 border-b border-slate-200 dark:border-slate-700 flex-shrink-0">
            <div>
              <h2 className="text-base font-semibold text-slate-900 dark:text-white">
                Edit SOP Playbook
              </h2>
              <p className="text-xs text-slate-500 mt-0.5">
                {serviceName}
                {faultType ? ` — ${faultType}` : ""}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setShowHistory((v) => !v)}
                className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg border transition-colors ${
                  showHistory
                    ? "border-blue-400 text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/20"
                    : "border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 hover:border-blue-400 hover:text-blue-600 dark:hover:text-blue-400"
                }`}
              >
                <History className="w-3.5 h-3.5" />
                Version History
              </button>
              <button
                onClick={onClose}
                className="p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-colors"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
          </div>

          {/* Steps list */}
          <div className="flex-1 overflow-y-auto px-6 py-4 space-y-4">
            {historyLoading && !initialized ? (
              <div className="text-sm text-slate-400 animate-pulse py-8 text-center">Loading playbook…</div>
            ) : (
              steps.map((step, idx) => (
                <div
                  key={step.order}
                  className="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/50 p-4"
                >
                  <div className="flex items-center justify-between gap-2 mb-3">
                    <span className="w-6 h-6 rounded-full bg-blue-600 text-white flex items-center justify-center text-xs font-bold flex-shrink-0">
                      {step.order}
                    </span>
                    <div className="flex items-center gap-1 ml-auto">
                      <button
                        onClick={() => moveUp(idx)}
                        disabled={idx === 0}
                        className="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 disabled:opacity-30 transition-colors"
                        title="Move up"
                      >
                        <ChevronUp className="w-3.5 h-3.5 text-slate-500" />
                      </button>
                      <button
                        onClick={() => moveDown(idx)}
                        disabled={idx === steps.length - 1}
                        className="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 disabled:opacity-30 transition-colors"
                        title="Move down"
                      >
                        <ChevronDown className="w-3.5 h-3.5 text-slate-500" />
                      </button>
                      <button
                        onClick={() => deleteStep(idx)}
                        className="p-1 rounded hover:bg-red-100 dark:hover:bg-red-900/30 text-red-500 transition-colors"
                        title="Delete step"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>

                  <div className="space-y-3">
                    <div>
                      <label className="text-xs font-medium text-slate-600 dark:text-slate-400 block mb-1">
                        Title
                      </label>
                      <input
                        value={step.title}
                        onChange={(e) => updateField(idx, "title", e.target.value)}
                        placeholder="Step title"
                        className="w-full text-sm px-3 py-2 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                      />
                    </div>

                    <div>
                      <label className="text-xs font-medium text-slate-600 dark:text-slate-400 block mb-1">
                        Action
                      </label>
                      <textarea
                        value={step.action}
                        onChange={(e) => updateField(idx, "action", e.target.value)}
                        placeholder="What the operator should do"
                        rows={2}
                        className="w-full text-sm px-3 py-2 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
                      />
                    </div>

                    <div className="flex gap-3">
                      <div className="flex-1">
                        <label className="text-xs font-medium text-slate-600 dark:text-slate-400 block mb-1">
                          Expected Signal
                        </label>
                        <input
                          value={step.expectedSignal}
                          onChange={(e) => updateField(idx, "expectedSignal", e.target.value)}
                          placeholder="e.g. 2xx on /health"
                          className="w-full text-sm px-3 py-2 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                      </div>

                      <div className="flex flex-col justify-end pb-0.5">
                        <label className="text-xs font-medium text-slate-600 dark:text-slate-400 block mb-2">
                          Auto-Verify
                        </label>
                        <button
                          type="button"
                          onClick={() => updateField(idx, "autoVerify", !step.autoVerify)}
                          className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors focus:outline-none ${
                            step.autoVerify
                              ? "bg-cyan-500"
                              : "bg-slate-300 dark:bg-slate-600"
                          }`}
                        >
                          <span
                            className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow transition-transform ${
                              step.autoVerify ? "translate-x-[18px]" : "translate-x-[2px]"
                            }`}
                          />
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
              ))
            )}
          </div>

          {/* Footer */}
          <div className="flex items-center justify-between px-6 py-4 border-t border-slate-200 dark:border-slate-700 flex-shrink-0 gap-3">
            <button
              onClick={addStep}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg border border-dashed border-slate-300 dark:border-slate-600 text-slate-500 dark:text-slate-400 hover:border-blue-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
            >
              <Plus className="w-3.5 h-3.5" />
              Add Step
            </button>

            <div className="flex items-center gap-3">
              {saveSuccess && (
                <span className="text-xs text-green-600 dark:text-green-400 font-medium">
                  Playbook saved!
                </span>
              )}
              <button
                onClick={onClose}
                className="px-4 py-1.5 text-sm font-medium rounded-lg border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleSave}
                disabled={saving}
                className="flex items-center gap-1.5 px-4 py-1.5 text-sm font-medium rounded-lg bg-blue-600 hover:bg-blue-700 text-white transition-colors disabled:opacity-50"
              >
                <Save className="w-3.5 h-3.5" />
                {saving ? "Saving…" : "Save Playbook"}
              </button>
            </div>
          </div>
        </div>

        {/* Version History side panel */}
        {showHistory && (
          <div className="w-72 flex-shrink-0 border-l border-slate-200 dark:border-slate-700 flex flex-col overflow-hidden">
            <div className="px-4 py-3 border-b border-slate-200 dark:border-slate-700 flex-shrink-0">
              <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-100">
                Version History
              </h3>
            </div>
            <div className="flex-1 overflow-y-auto p-4 space-y-3">
              {historyLoading ? (
                <p className="text-xs text-slate-400 animate-pulse">Loading…</p>
              ) : (historyData?.versions?.length ?? 0) === 0 ? (
                <p className="text-xs text-slate-400 text-center py-4">No previous versions</p>
              ) : (
                historyData!.versions.map((v) => (
                  <div
                    key={v.version}
                    className="rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 p-3"
                  >
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-xs font-semibold text-slate-800 dark:text-slate-100">
                        v{v.version}
                      </span>
                      <span className="text-[10px] text-slate-400">
                        {v.steps.length} step{v.steps.length !== 1 ? "s" : ""}
                      </span>
                    </div>
                    <p className="text-[10px] text-slate-500 mb-1">
                      {new Date(v.createdAt).toLocaleString()}
                    </p>
                    <p className="text-[10px] text-slate-400 mb-2 truncate">by {v.createdBy}</p>
                    <button
                      onClick={() => handleRevert(v.version)}
                      disabled={reverting}
                      className="flex items-center gap-1 text-[10px] font-medium text-blue-600 dark:text-blue-400 hover:underline disabled:opacity-50"
                    >
                      <RotateCcw className="w-2.5 h-2.5" />
                      Restore this version
                    </button>
                  </div>
                ))
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
