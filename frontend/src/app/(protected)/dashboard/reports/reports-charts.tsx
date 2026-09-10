"use client"

import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  Legend,
} from "recharts"
import { Card } from "@/components/ui/card"
import { ErrorBoundary } from "@/components/ErrorBoundary"
import type { IncidentStatus, IncidentListItem } from "@/types/api"

const STATUS_PALETTE: Record<IncidentStatus, string> = {
  OPEN:         "#ef4444",
  ACKNOWLEDGED: "#f59e0b",
  RESOLVED:     "#22c55e",
}

interface ReportsChartsProps {
  dailyData: { date: string; count: number }[]
  statusData: { status: string; value: number }[]
  confData: { range: string; count: number }[]
  mttrData: { service: string; avgMttrMin: number }[]
  incidents: IncidentListItem[]
}

export default function ReportsCharts({
  dailyData,
  statusData,
  confData,
  mttrData,
  incidents,
}: ReportsChartsProps) {
  const phaseCounts: Record<string, number> = {
    DETECTING: 0,
    NARROWING:  0,
    CONFIRMED:  0,
    TRIAGING:   0,
    FIXING:     0,
    VERIFYING:  0,
    RESOLVED:   0,
  }
  for (const inc of incidents) {
    if (inc.phase) {
      phaseCounts[inc.phase] = (phaseCounts[inc.phase] ?? 0) + 1
    }
  }
  const phaseData = Object.entries(phaseCounts)
    .filter(([, v]) => v > 0)
    .map(([phase, count]) => ({ phase, count }))

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
      {/* Daily incidents */}
      <ErrorBoundary context="Analytics Chart">
        <Card className="p-5">
          <p className="font-semibold text-slate-900 dark:text-white mb-4">Daily Incidents (last 14 days)</p>
          <ResponsiveContainer width="100%" height={200}>
            <BarChart data={dailyData} margin={{ left: -20 }}>
              <XAxis dataKey="date" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} allowDecimals={false} />
              <Tooltip />
              <Bar dataKey="count" name="Incidents" fill="#3b82f6" radius={[3, 3, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </Card>
      </ErrorBoundary>

      {/* Status distribution */}
      <ErrorBoundary context="Analytics Chart">
        <Card className="p-5">
          <p className="font-semibold text-slate-900 dark:text-white mb-4">Status Distribution</p>
          <ResponsiveContainer width="100%" height={200}>
            <PieChart>
              <Pie
                data={statusData}
                dataKey="value"
                nameKey="status"
                cx="50%"
                cy="50%"
                outerRadius={70}
                label={({ name, percent }) => `${name} ${Math.round((percent ?? 0) * 100)}%`}
                labelLine={false}
              >
                {statusData.map((entry) => (
                  <Cell
                    key={entry.status}
                    fill={STATUS_PALETTE[entry.status as IncidentStatus] ?? "#94a3b8"}
                  />
                ))}
              </Pie>
              <Legend />
              <Tooltip />
            </PieChart>
          </ResponsiveContainer>
        </Card>
      </ErrorBoundary>

      {/* Confidence histogram */}
      <ErrorBoundary context="Analytics Chart">
        <Card className="p-5 lg:col-span-2">
          <p className="font-semibold text-slate-900 dark:text-white mb-4">RCA Confidence Distribution</p>
          <ResponsiveContainer width="100%" height={180}>
            <BarChart data={confData} margin={{ left: -20 }}>
              <XAxis dataKey="range" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} allowDecimals={false} />
              <Tooltip />
              <Bar dataKey="count" name="Incidents" fill="#8b5cf6" radius={[3, 3, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </Card>
      </ErrorBoundary>

      {/* MTTR per service */}
      {mttrData.length > 0 && (
        <ErrorBoundary context="Analytics Chart">
          <Card className="p-5 lg:col-span-2">
            <div className="mb-4">
              <p className="font-semibold text-slate-900 dark:text-white">Mean Time to Resolve (MTTR) per Service</p>
              <p className="text-xs text-slate-400 mt-0.5">Average minutes from detection to resolution, for resolved incidents</p>
            </div>
            <ResponsiveContainer width="100%" height={200}>
              <BarChart data={mttrData} margin={{ left: -10 }}>
                <XAxis dataKey="service" tick={{ fontSize: 11 }} />
                <YAxis tick={{ fontSize: 11 }} allowDecimals={false} label={{ value: "min", angle: -90, position: "insideLeft", style: { fontSize: 10, fill: "#94a3b8" } }} />
                <Tooltip formatter={(value) => [`${value} min`, "Avg MTTR"]} />
                <Bar dataKey="avgMttrMin" name="Avg MTTR (min)" fill="#f59e0b" radius={[3, 3, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </Card>
        </ErrorBoundary>
      )}

      {/* Phase distribution */}
      {phaseData.length > 0 && (
        <ErrorBoundary context="Analytics Chart">
          <Card className="p-5 lg:col-span-2">
            <div className="mb-4">
              <p className="font-semibold text-slate-900 dark:text-white">Incident Phase Distribution</p>
              <p className="text-xs text-slate-400 mt-0.5">Number of incidents currently in each SOP phase</p>
            </div>
            <ResponsiveContainer width="100%" height={180}>
              <BarChart data={phaseData} layout="vertical" margin={{ left: 20 }}>
                <XAxis type="number" tick={{ fontSize: 11 }} allowDecimals={false} />
                <YAxis dataKey="phase" type="category" tick={{ fontSize: 11 }} width={80} />
                <Tooltip />
                <Bar dataKey="count" name="Incidents" fill="#06b6d4" radius={[0, 3, 3, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </Card>
        </ErrorBoundary>
      )}
    </div>
  )
}
