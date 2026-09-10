"use client"

import { useState, useEffect } from "react"
import { useGetGraphDiffQuery, useTakeGraphSnapshotMutation } from "@/store/services"
import { Camera, Plus, Minus, RefreshCw, Server, Database, Layers, Activity, Globe } from "lucide-react"
import { Card } from "@/components/ui/card"

interface GraphDiffPanelProps {
  projectId: string
}

const TYPE_ICON: Record<string, React.ReactNode> = {
  SERVICE:  <Server className="w-3.5 h-3.5" />,
  DATABASE: <Database className="w-3.5 h-3.5" />,
  QUEUE:    <Layers className="w-3.5 h-3.5" />,
  GATEWAY:  <Activity className="w-3.5 h-3.5" />,
  EXTERNAL: <Globe className="w-3.5 h-3.5" />,
}

export function GraphDiffPanel({ projectId }: GraphDiffPanelProps) {
  const [snapSuccess, setSnapSuccess] = useState(false)

  useEffect(() => {
    if (snapSuccess) {
      const t = setTimeout(() => setSnapSuccess(false), 2500)
      return () => clearTimeout(t)
    }
  }, [snapSuccess])

  const {
    data: diff,
    isLoading,
    isError,
    refetch,
  } = useGetGraphDiffQuery(projectId, { skip: !projectId })

  const [takeSnapshot, { isLoading: snapping }] = useTakeGraphSnapshotMutation()

  const handleSnapshot = async () => {
    const result = await takeSnapshot(projectId)
    if ("data" in result) {
      setSnapSuccess(true)
      refetch()
    }
  }

  const noSnapshot = isError || (!isLoading && (!diff || !diff.snapshotAge))

  const added   = diff?.added   ?? []
  const removed = diff?.removed ?? []
  const changed = diff?.changed ?? []

  const totalChanges = added.length + removed.length + changed.length

  return (
    <div className="space-y-5">
      {/* Header bar */}
      <div className="flex items-center justify-between gap-3">
        <div>
          {noSnapshot ? (
            <p className="text-sm text-slate-500 dark:text-slate-400">
              No snapshot — click &quot;Take Snapshot&quot; to start tracking changes.
            </p>
          ) : (
            <p className="text-sm text-slate-600 dark:text-slate-300">
              Snapshot age:{" "}
              <span className="font-medium text-slate-900 dark:text-white">
                {diff?.snapshotAge}
              </span>
              {totalChanges === 0 && (
                <span className="ml-2 text-xs text-green-600 dark:text-green-400 font-medium">
                  — no changes detected
                </span>
              )}
            </p>
          )}
        </div>
        <div className="flex items-center gap-2">
          {snapSuccess && (
            <span className="text-xs text-green-600 dark:text-green-400 font-medium">
              Snapshot saved!
            </span>
          )}
          <button
            onClick={handleSnapshot}
            disabled={snapping}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-600 hover:bg-blue-700 text-white transition-colors disabled:opacity-50"
          >
            <Camera className="w-3.5 h-3.5" />
            {snapping ? "Taking snapshot…" : "Take Snapshot"}
          </button>
        </div>
      </div>

      {isLoading && (
        <div className="text-sm text-slate-400 animate-pulse py-4 text-center">Loading diff…</div>
      )}

      {!isLoading && !noSnapshot && totalChanges === 0 && (
        <Card className="p-8 text-center">
          <RefreshCw className="w-8 h-8 text-green-400 mx-auto mb-3" />
          <p className="text-sm font-medium text-slate-600 dark:text-slate-400">
            Graph matches snapshot — no changes detected.
          </p>
        </Card>
      )}

      {/* Added services */}
      {added.length > 0 && (
        <div>
          <h3 className="flex items-center gap-2 text-sm font-semibold text-green-700 dark:text-green-400 mb-2">
            <Plus className="w-4 h-4" />
            Added ({added.length})
          </h3>
          <div className="space-y-2">
            {added.map((svc) => (
              <div
                key={svc.id}
                className="flex items-center gap-3 p-3 rounded-lg border border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-950/30"
              >
                <span className="text-green-600 dark:text-green-400 flex-shrink-0">
                  {TYPE_ICON[svc.type] ?? <Server className="w-3.5 h-3.5" />}
                </span>
                <div className="min-w-0">
                  <p className="text-sm font-medium text-slate-900 dark:text-white">{svc.name}</p>
                  <p className="text-xs text-slate-400 font-mono">{svc.id}</p>
                </div>
                <span className="ml-auto text-[10px] font-semibold text-green-700 dark:text-green-400 bg-green-100 dark:bg-green-900/40 px-2 py-0.5 rounded-full">
                  NEW
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Removed services */}
      {removed.length > 0 && (
        <div>
          <h3 className="flex items-center gap-2 text-sm font-semibold text-red-700 dark:text-red-400 mb-2">
            <Minus className="w-4 h-4" />
            Removed ({removed.length})
          </h3>
          <div className="space-y-2">
            {removed.map((svc) => (
              <div
                key={svc.id}
                className="flex items-center gap-3 p-3 rounded-lg border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-950/30"
              >
                <span className="text-red-500 flex-shrink-0">
                  <Server className="w-3.5 h-3.5" />
                </span>
                <div className="min-w-0">
                  <p className="text-sm font-medium text-slate-900 dark:text-white">{svc.name}</p>
                  <p className="text-xs text-slate-400 font-mono">{svc.id}</p>
                </div>
                <span className="ml-auto text-[10px] font-semibold text-red-700 dark:text-red-400 bg-red-100 dark:bg-red-900/40 px-2 py-0.5 rounded-full">
                  REMOVED
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Changed services */}
      {changed.length > 0 && (
        <div>
          <h3 className="flex items-center gap-2 text-sm font-semibold text-yellow-700 dark:text-yellow-400 mb-2">
            <RefreshCw className="w-4 h-4" />
            Changed ({changed.length})
          </h3>
          <div className="space-y-2">
            {changed.map((svc) => (
              <div
                key={svc.id}
                className="p-3 rounded-lg border border-yellow-200 dark:border-yellow-700 bg-yellow-50 dark:bg-yellow-950/20"
              >
                <div className="flex items-center gap-3 mb-2">
                  <span className="text-yellow-600 dark:text-yellow-400 flex-shrink-0">
                    <Server className="w-3.5 h-3.5" />
                  </span>
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-slate-900 dark:text-white">{svc.name}</p>
                    <p className="text-xs text-slate-400 font-mono">{svc.id}</p>
                  </div>
                  <span className="ml-auto text-[10px] font-semibold text-yellow-700 dark:text-yellow-400 bg-yellow-100 dark:bg-yellow-900/40 px-2 py-0.5 rounded-full">
                    CHANGED
                  </span>
                </div>
                <div className="space-y-1 pl-6">
                  {svc.changes.map((change, i) => (
                    <p key={i} className="text-xs text-slate-600 dark:text-slate-400 font-mono">
                      {change}
                    </p>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Edge changes summary */}
      {((diff?.addedEdges?.length ?? 0) > 0 || (diff?.removedEdges?.length ?? 0) > 0) && (
        <div className="p-3 rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/50">
          <p className="text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1">Edge changes</p>
          <div className="flex gap-4 text-xs text-slate-500 dark:text-slate-400">
            {(diff?.addedEdges?.length ?? 0) > 0 && (
              <span className="text-green-600 dark:text-green-400 font-medium">
                +{diff!.addedEdges.length} edge{diff!.addedEdges.length !== 1 ? "s" : ""}
              </span>
            )}
            {(diff?.removedEdges?.length ?? 0) > 0 && (
              <span className="text-red-600 dark:text-red-400 font-medium">
                -{diff!.removedEdges.length} edge{diff!.removedEdges.length !== 1 ? "s" : ""}
              </span>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
