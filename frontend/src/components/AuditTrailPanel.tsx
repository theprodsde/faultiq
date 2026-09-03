"use client"

import { useGetIncidentAuditQuery } from "@/store/services"
import type { AuditEvent } from "@/types/api"

interface AuditTrailPanelProps {
  incidentId: string
}

function formatTime(iso: string): string {
  try {
    return new Date(iso).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })
  } catch {
    return iso
  }
}

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString([], { month: "short", day: "numeric", year: "numeric" })
  } catch {
    return iso
  }
}

const ACTION_DOT_COLOR: Record<string, string> = {
  phase_change:  "bg-blue-500",
  step_done:     "bg-green-500",
  step_failed:   "bg-red-500",
  auto_resolved: "bg-teal-500",
}

const ACTION_LABEL: Record<string, string> = {
  phase_change:  "Phase Change",
  step_done:     "Step Completed",
  step_failed:   "Step Failed",
  auto_resolved: "Auto Resolved",
}

function EventCard({ event, isLast }: { event: AuditEvent; isLast: boolean }) {
  const dotColor = ACTION_DOT_COLOR[event.action] ?? "bg-slate-400"
  const label = ACTION_LABEL[event.action] ?? event.action

  return (
    <div className="flex gap-4">
      {/* Left: line + dot */}
      <div className="flex flex-col items-center">
        <div className={`w-3 h-3 rounded-full flex-shrink-0 mt-1 ${dotColor} ring-2 ring-white dark:ring-slate-900`} />
        {!isLast && (
          <div className="w-0.5 flex-1 mt-1 bg-slate-200 dark:bg-slate-700 min-h-[24px]" />
        )}
      </div>

      {/* Center: event detail */}
      <div className="flex-1 pb-5 min-w-0">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="min-w-0">
            <span className="text-xs font-semibold text-slate-700 dark:text-slate-200 uppercase tracking-wide">
              {label}
            </span>

            {/* Phase change */}
            {event.action === "phase_change" && event.phaseFrom && event.phaseTo && (
              <div className="mt-1 flex items-center gap-2">
                <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300">
                  {event.phaseFrom}
                </span>
                <span className="text-slate-400 text-xs">→</span>
                <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300">
                  {event.phaseTo}
                </span>
              </div>
            )}

            {/* Step actions */}
            {(event.action === "step_done" || event.action === "step_failed") && (
              <div className="mt-1 flex items-center gap-2">
                {event.stepOrder != null && (
                  <span className="text-[11px] font-mono text-slate-500 dark:text-slate-400">
                    Step {event.stepOrder}
                  </span>
                )}
                <span
                  className={`px-2 py-0.5 rounded-full text-[11px] font-medium ${
                    event.action === "step_done"
                      ? "bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-300"
                      : "bg-red-100 dark:bg-red-900/40 text-red-700 dark:text-red-300"
                  }`}
                >
                  {event.action === "step_done" ? "Completed" : "Failed"}
                </span>
              </div>
            )}

            {/* Auto resolved */}
            {event.action === "auto_resolved" && (
              <div className="mt-1 flex items-center gap-1.5">
                <span className="text-teal-600 dark:text-teal-400 text-sm">✓</span>
                <span className="text-[11px] text-teal-700 dark:text-teal-300 font-medium">
                  Condition met — incident auto-resolved
                </span>
              </div>
            )}

            {/* Actor */}
            <p className="text-[11px] text-slate-400 dark:text-slate-500 mt-1">
              by{" "}
              <span className="font-medium text-slate-500 dark:text-slate-400">
                {event.actor === "system" ? "System" : event.actor}
              </span>
            </p>
          </div>

          {/* Right: timestamp */}
          <div className="flex-shrink-0 text-right">
            <p className="text-[11px] font-mono text-slate-500 dark:text-slate-400">{formatTime(event.createdAt)}</p>
            <p className="text-[10px] text-slate-400 dark:text-slate-500">{formatDate(event.createdAt)}</p>
          </div>
        </div>
      </div>
    </div>
  )
}

export function AuditTrailPanel({ incidentId }: AuditTrailPanelProps) {
  const { data, isLoading, isError } = useGetIncidentAuditQuery(incidentId)

  if (isLoading) {
    return (
      <div className="space-y-4 animate-pulse">
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex gap-4">
            <div className="w-3 h-3 rounded-full bg-slate-200 dark:bg-slate-700 mt-1 flex-shrink-0" />
            <div className="flex-1 space-y-2 pb-5">
              <div className="h-3 bg-slate-200 dark:bg-slate-700 rounded w-28" />
              <div className="h-2.5 bg-slate-100 dark:bg-slate-800 rounded w-48" />
            </div>
          </div>
        ))}
      </div>
    )
  }

  if (isError) {
    return (
      <div className="text-sm text-slate-400 dark:text-slate-500 text-center py-6">
        Could not load audit trail.
      </div>
    )
  }

  const events: AuditEvent[] = data?.events ?? []

  if (events.length === 0) {
    return (
      <div className="text-sm text-slate-400 dark:text-slate-500 text-center py-6">
        No audit events recorded yet for this incident.
      </div>
    )
  }

  return (
    <div>
      <h3 className="text-sm font-semibold text-slate-700 dark:text-slate-200 mb-4">
        Audit Trail
        <span className="ml-2 text-[11px] font-normal text-slate-400">{events.length} event{events.length !== 1 ? "s" : ""}</span>
      </h3>
      <div className="relative">
        {events.map((event, i) => (
          <EventCard key={event.id} event={event} isLast={i === events.length - 1} />
        ))}
      </div>
    </div>
  )
}
