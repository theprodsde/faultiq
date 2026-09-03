"use client"

import React, { useEffect, useRef, useState, useCallback, useMemo } from "react"
import type { ServiceNode, ServiceEdge } from "@/types/api"
import { ZoomIn, ZoomOut, Maximize2, RefreshCw } from "lucide-react"

interface IncidentNodeState {
  serviceName: string
  status: "OPEN" | "ACKNOWLEDGED" | "RESOLVED"
  confidence: number
  incidentId?: string
}

interface Props {
  nodes: ServiceNode[]
  edges: ServiceEdge[]
  nodeMap: Record<string, string>
  incidentStates?: IncidentNodeState[]
  onNodeSelect?: (nodeId: string | null) => void
}

const NODE_COLORS: Record<string, string> = {
  SERVICE: "#3b82f6",
  DATABASE: "#a78bfa",
  QUEUE: "#f59e0b",
  GATEWAY: "#06b6d4",
  EXTERNAL: "#64748b",
}

const NODE_LABELS: Record<string, string> = {
  SERVICE: "Microservice",
  DATABASE: "Database",
  QUEUE: "Message Queue",
  GATEWAY: "API Gateway",
  EXTERNAL: "External System",
}

export function InteractiveGraphCytoscape({ nodes, edges, nodeMap, incidentStates = [], onNodeSelect }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const cyRef = useRef<any>(null)
  const [selectedNode, setSelectedNode] = useState<string | null>(null)
  const [isDark, setIsDark] = useState(false)
  const [mounted, setMounted] = useState(false)

  // Keep a ref to the latest incidentMap so the init effect doesn't depend on it
  type IncidentEntry = { status: "OPEN" | "ACKNOWLEDGED" | "RESOLVED"; incidentId?: string }
  const incidentMapRef = useRef<Record<string, IncidentEntry>>({})
  // Track graph topology — only re-initialize Cytoscape when node/edge IDs actually change
  const topologyFingerprintRef = useRef<string>("")
  const incidentMap = useMemo(() => {
    const map: Record<string, IncidentEntry> = {}
    for (const s of incidentStates) {
      // OPEN takes priority over ACKNOWLEDGED
      if (!map[s.serviceName] || s.status === "OPEN") {
        map[s.serviceName] = { status: s.status, incidentId: s.incidentId }
      }
    }
    return map
  }, [incidentStates])

  // Keep ref in sync (used inside init effect to avoid re-initialization)
  useEffect(() => {
    incidentMapRef.current = incidentMap
  }, [incidentMap])
  // (incidentMapRef2 removed — merged into incidentMapRef)

  // Zoom controls – guard against destroyed instance
  const zoomIn = useCallback(() => {
    const cy = cyRef.current
    if (!cy || !cy.container()) return
    cy.zoom(cy.zoom() * 1.3)
  }, [])
  const zoomOut = useCallback(() => {
    const cy = cyRef.current
    if (!cy || !cy.container()) return
    cy.zoom(cy.zoom() * 0.77)
  }, [])
  const fitGraph = useCallback(() => {
    const cy = cyRef.current
    if (!cy || !cy.container()) return
    cy.fit(undefined, 30)
  }, [])
  const resetLayout = useCallback(() => {
    const cy = cyRef.current
    if (!cy || !cy.container()) return
    cy.layout({ name: "cose", animate: true, animationDuration: 600, fit: true, padding: 30, nodeRepulsion: 8000, edgeElasticity: 100 } as any).run()
  }, [])

  // Detect dark mode
  useEffect(() => {
    setMounted(true)
    const isDarkMode = document.documentElement.classList.contains("dark")
    setIsDark(isDarkMode)

    const observer = new MutationObserver(() => {
      const dark = document.documentElement.classList.contains("dark")
      setIsDark(dark)
    })

    observer.observe(document.documentElement, { attributes: true })
    return () => observer.disconnect()
  }, [])

  // Initialize and configure Cytoscape
  useEffect(() => {
    if (!mounted || !containerRef.current || nodes.length === 0) return

    // Only re-initialize if TOPOLOGY changes (node IDs or edge connections).
    // Status/health changes are handled by the patch effect below — never re-layout for data updates.
    const newFingerprint = [
      nodes.map((n) => n.id).sort().join(","),
      edges.map((e) => `${e.from}->${e.to}`).sort().join(","),
    ].join("|");
    if (cyRef.current && newFingerprint === topologyFingerprintRef.current) return
    topologyFingerprintRef.current = newFingerprint

    // Cancelled flag prevents the async init from writing to refs after unmount
    let cancelled = false

    // Dynamically import Cytoscape to avoid SSR issues
    const initializeCytoscape = async () => {
      try {
        // Ensure container is mounted and has dimensions
        if (!containerRef.current) return
        
        // @ts-ignore - Cytoscape module doesn't have full type definitions
        const Cytoscape = (await import("cytoscape")).default

        // Destroy previous instance if any
        if (cyRef.current) {
          try { cyRef.current.destroy() } catch {}
          cyRef.current = null
        }

        // Convert nodes and edges to Cytoscape format
        const elements: any[] = []

        const darkNow = document.documentElement.classList.contains("dark")

        // Add nodes — priority: incident status > runtime error rate
        nodes.forEach((node) => {
          const errRate = node.runtimeState?.errorRate || 0
          const currentMap = incidentMapRef.current
          const incidentEntry = currentMap[node.name] || currentMap[node.id]
          const incidentStatus = incidentEntry?.status
          // Incident-driven health (highest priority)
          const hasOpenIncident = incidentStatus === "OPEN"
          const hasAcknowledgedIncident = incidentStatus === "ACKNOWLEDGED"
          // Fallback to runtime metrics
          const isUnhealthy = hasOpenIncident || errRate > 0.5 || 
            node.runtimeState?.statusClass === "5xx" || 
            node.runtimeState?.statusClass === "timeout" ||
            node.runtimeState?.statusClass === "connection_error" ||
            node.runtimeState?.statusClass === "down" ||
            node.runtimeState?.statusClass === "503" ||
            node.runtimeState?.statusClass === "504"
          const isDegraded = hasAcknowledgedIncident || (errRate > 0.1 && !isUnhealthy)
          elements.push({
            data: {
              id: node.id,
              label: node.name || node.id,
              type: node.type,
              errorRate: errRate,
              latencyP95: node.runtimeState?.latencyP95 || 0,
              status: node.runtimeState?.statusClass || "2xx",
              isUnhealthy,
              isDegraded,
              incidentStatus: incidentStatus || null,
            },
          })
        })

        // Add edges
        edges.forEach((edge) => {
          elements.push({
            data: {
              id: `${edge.from}-${edge.to}`,
              source: edge.from,
              target: edge.to,
              successRatio: edge.successRatio ?? 1,
              label: edge.type || "",
            },
          })
        })

        const bg = darkNow ? "#0f172a" : "#f8fafc"
        const textColor = darkNow ? "#e2e8f0" : "#1e293b"
        const borderColor = darkNow ? "#1e293b" : "#ffffff"

        // Stylesheet
        const stylesheet: any[] = [
          {
            selector: "node",
            style: {
              "background-color": (ele: any) => {
                if (ele.data("isUnhealthy")) return "#ef4444"
                if (ele.data("isDegraded")) return "#f59e0b"
                const type = ele.data("type")
                return NODE_COLORS[type] || NODE_COLORS.SERVICE
              },
              label: "data(label)",
              "text-valign": "bottom",
              "text-halign": "center",
              "text-margin-y": "6px",
              "font-size": "10px",
              "font-weight": "600",
              color: textColor,
              "text-background-color": bg,
              "text-background-opacity": 0.8,
              "text-background-padding": "3px",
              "text-background-shape": "roundrectangle",
              width: "52px",
              height: "52px",
              "border-width": "3px",
              "border-color": (ele: any) => {
                if (ele.data("isUnhealthy")) return "#fca5a5"
                if (ele.data("isDegraded")) return "#fcd34d"
                return borderColor
              },
              "background-opacity": 0.9,
              "overlay-padding": "6px",
            },
          },
          {
            selector: "node:selected",
            style: {
              "border-width": "4px",
              "border-color": "#60a5fa",
              "background-opacity": 1,
              "z-index": 10,
            },
          },
          {
            selector: "node:selected",
            style: {
              "background-opacity": 1,
              "border-width": "4px",
              "border-color": "#93c5fd",
            },
          },
          {
            selector: "edge",
            style: {
              "line-color": (ele: any) => {
                const ratio = ele.data("successRatio")
                if (ratio > 0.99) return "#22c55e"
                if (ratio > 0.95) return "#eab308"
                return "#ef4444"
              },
              width: "2px",
              "target-arrow-color": (ele: any) => {
                const ratio = ele.data("successRatio")
                if (ratio > 0.99) return "#22c55e"
                if (ratio > 0.95) return "#eab308"
                return "#ef4444"
              },
              "target-arrow-shape": "triangle",
              "curve-style": "bezier",
              opacity: 0.65,
            },
          },

          {
            selector: "edge:selected",
            style: {
              opacity: 1,
              width: "4px",
            },
          },
        ]

        // Create instance
        if (!containerRef.current || containerRef.current.offsetHeight === 0) {
          console.warn("Container not ready for Cytoscape initialization")
          return
        }

        const cy = Cytoscape({
          container: containerRef.current,
          elements,
          style: stylesheet,
          layout: {
            name: "cose",
            animate: true,
            animationDuration: 600,
            fit: true,
            padding: 30,
            nodeRepulsion: 8000,
            edgeElasticity: 100,
          } as any,
          boxSelectionEnabled: true,
          autounselectify: false,
          minZoom: 0.2,
          maxZoom: 3,
        })

        // Event handlers
        cy.on("tap", "node", (event: any) => {
          setSelectedNode(event.target.id())
          onNodeSelect?.(event.target.id())
        })

        cy.on("tap", (event: any) => {
          if (event.target === cy) {
            setSelectedNode(null)
            onNodeSelect?.(null)
          }
        })

        // Guard: if the component unmounted while we were awaiting, destroy and bail
        if (cancelled) {
          try { cy.stop() } catch {}
          try { cy.destroy() } catch {}
          return
        }
        cyRef.current = cy
      } catch (error) {
        console.error("Failed to initialize Cytoscape:", error)
      }
    }

    initializeCytoscape()

    return () => {
      cancelled = true
      if (cyRef.current) {
        try { cyRef.current.stop() } catch {}
        try { cyRef.current.destroy() } catch {}
        cyRef.current = null
      }
    }
  // IMPORTANT: Only depend on mounted. Nodes/edges changes are handled by the topology fingerprint check inside.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nodes, edges, mounted])

  // Update node health colors when incidentMap changes — without destroying the graph
  useEffect(() => {
    const cy = cyRef.current
    if (!cy || !cy.container()) return
    cy.nodes().forEach((ele: any) => {
      const nodeId = ele.id()
      const node = nodes.find((n) => n.id === nodeId)
      if (!node) return
      const errRate = node.runtimeState?.errorRate || 0
      const incidentEntry = incidentMap[node.name] || incidentMap[node.id]
      const incidentStatus = incidentEntry?.status
      const hasOpenIncident = incidentStatus === "OPEN"
      const hasAcknowledgedIncident = incidentStatus === "ACKNOWLEDGED"
      const isUnhealthy = hasOpenIncident || errRate > 0.5 || node.runtimeState?.statusClass === "5xx" || node.runtimeState?.statusClass === "timeout" || node.runtimeState?.statusClass === "connection_error" || node.runtimeState?.statusClass === "down"
      const isDegraded = hasAcknowledgedIncident || (errRate > 0.1 && !isUnhealthy)
      ele.data("isUnhealthy", isUnhealthy)
      ele.data("isDegraded", isDegraded)
      ele.data("incidentStatus", incidentStatus || null)
      ele.data("errorRate", errRate)
      ele.data("status", node.runtimeState?.statusClass || "2xx")
    })
    // Also update edge success ratios if they changed
    cy.edges().forEach((ele: any) => {
      const edgeId = ele.id()
      const edge = edges.find((e) => `${e.from}-${e.to}` === edgeId)
      if (edge && edge.successRatio !== undefined) {
        ele.data("successRatio", edge.successRatio)
      }
    })
    try { cy.style().update() } catch {}
  }, [incidentMap, nodes, edges])

  // Handle dark mode theme change — update stylesheet colors without re-layout
  useEffect(() => {
    const cy = cyRef.current
    if (!cy || !cy.container()) return
    // Force style recalculation — Cytoscape mappers re-evaluate on style().update()
    try { cy.style().update() } catch {}
  }, [isDark])

  // Get selected node data for tooltip
  const selectedNodeData = selectedNode ? nodes.find((n) => n.id === selectedNode) : null

  if (!mounted) {
    return <div className="w-full rounded-lg bg-slate-100 dark:bg-slate-800 animate-pulse" style={{ height: "700px" }} />
  }

  if (nodes.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center rounded-lg border-2 border-dashed border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/50 text-center p-12" style={{ height: "700px" }}>
        <div className="text-4xl mb-4">🕸️</div>
        <h3 className="text-lg font-semibold text-slate-700 dark:text-slate-300 mb-2">No Graph Data</h3>
        <p className="text-sm text-slate-500 dark:text-slate-400 max-w-sm">
          Select a project and environment above to load the service dependency graph.
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-3">
      {/* What this graph shows */}
      <div className="px-4 py-3 rounded-lg bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 text-sm text-blue-800 dark:text-blue-200">
        <strong>Service Dependency Graph</strong> — Shows how your microservices call each other.
        Nodes are services/databases/queues. Arrows show call direction. 
        <span className="text-red-600 dark:text-red-400"> Red nodes = actively failing</span>,
        <span className="text-yellow-600 dark:text-yellow-400"> yellow = degraded</span>,
        <span className="text-blue-600 dark:text-blue-400"> blue = healthy</span>.
        Edge color shows success rate. Click any node for details.
      </div>

      {/* Graph canvas with zoom controls */}
      <div className="relative rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden bg-white dark:bg-slate-900">
        <div
          ref={containerRef}
          className="w-full"
          style={{ height: "680px" }}
        />

        {/* Zoom controls overlay */}
        <div className="absolute top-3 right-3 flex flex-col gap-1 z-10">
          <button
            onClick={zoomIn}
            className="w-8 h-8 flex items-center justify-center rounded bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-700 shadow-sm transition-colors"
            title="Zoom in"
          >
            <ZoomIn className="w-4 h-4" />
          </button>
          <button
            onClick={zoomOut}
            className="w-8 h-8 flex items-center justify-center rounded bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-700 shadow-sm transition-colors"
            title="Zoom out"
          >
            <ZoomOut className="w-4 h-4" />
          </button>
          <button
            onClick={fitGraph}
            className="w-8 h-8 flex items-center justify-center rounded bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-700 shadow-sm transition-colors"
            title="Fit to screen"
          >
            <Maximize2 className="w-4 h-4" />
          </button>
          <button
            onClick={resetLayout}
            className="w-8 h-8 flex items-center justify-center rounded bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-700 shadow-sm transition-colors"
            title="Re-run layout"
          >
            <RefreshCw className="w-4 h-4" />
          </button>
        </div>

        {/* Stats overlay (bottom left) */}
        <div className="absolute bottom-3 left-3 z-10 flex gap-2">
          <span className="px-2 py-1 rounded text-xs bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 shadow-sm">
            {nodes.length} nodes
          </span>
          <span className="px-2 py-1 rounded text-xs bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 shadow-sm">
            {edges.length} edges
          </span>
          {incidentStates.filter(s => s.status === "OPEN").length > 0 && (
            <span className="px-2 py-1 rounded text-xs bg-red-100 dark:bg-red-900/30 border border-red-200 dark:border-red-700 text-red-600 dark:text-red-400 shadow-sm font-medium animate-pulse">
              🔴 {incidentStates.filter(s => s.status === "OPEN").length} incident{incidentStates.filter(s => s.status === "OPEN").length !== 1 ? "s" : ""}
            </span>
          )}
          {incidentStates.filter(s => s.status === "ACKNOWLEDGED").length > 0 && (
            <span className="px-2 py-1 rounded text-xs bg-yellow-100 dark:bg-yellow-900/30 border border-yellow-200 dark:border-yellow-700 text-yellow-600 dark:text-yellow-400 shadow-sm font-medium">
              🟡 {incidentStates.filter(s => s.status === "ACKNOWLEDGED").length} acknowledged
            </span>
          )}
          {nodes.filter(n => (n.runtimeState?.errorRate ?? 0) > 0.5).length > 0 && incidentStates.length === 0 && (
            <span className="px-2 py-1 rounded text-xs bg-red-100 dark:bg-red-900/30 border border-red-200 dark:border-red-700 text-red-600 dark:text-red-400 shadow-sm font-medium">
              ⚠ {nodes.filter(n => (n.runtimeState?.errorRate ?? 0) > 0.5).length} failing
            </span>
          )}
        </div>
      </div>

      {/* Legend and node detail side-by-side */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        {/* Legend */}
        <div className="p-4 rounded-lg bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700">
          <h3 className="text-sm font-semibold text-slate-900 dark:text-white mb-3">Legend</h3>
          <div className="grid grid-cols-2 gap-2 mb-3">
            {(Object.entries(NODE_LABELS) as [string, string][]).map(([type, label]) => (
              <div key={type} className="flex items-center gap-2">
                <div className="w-4 h-4 rounded-full flex-shrink-0" style={{ backgroundColor: NODE_COLORS[type] }} />
                <span className="text-xs text-slate-600 dark:text-slate-400">{label}</span>
              </div>
            ))}
          </div>
          <div className="border-t border-slate-200 dark:border-slate-700 pt-2 mt-2">
            <p className="text-xs font-medium text-slate-600 dark:text-slate-400 mb-2">Node health (overrides type color):</p>
            <div className="flex gap-3">
              <span className="flex items-center gap-1 text-xs text-slate-600 dark:text-slate-400">
                <span className="w-3 h-3 rounded-full bg-red-500 flex-shrink-0" /> Failing (&gt;50% error)
              </span>
              <span className="flex items-center gap-1 text-xs text-slate-600 dark:text-slate-400">
                <span className="w-3 h-3 rounded-full bg-yellow-500 flex-shrink-0" /> Degraded
              </span>
            </div>
          </div>
          <div className="border-t border-slate-200 dark:border-slate-700 pt-2 mt-2">
            <p className="text-xs font-medium text-slate-600 dark:text-slate-400 mb-2">Edge success rate:</p>
            <div className="flex gap-3 flex-wrap">
              <span className="flex items-center gap-1 text-xs text-slate-600 dark:text-slate-400">
                <span className="w-6 h-0.5 bg-green-500 inline-block" /> &gt;99%
              </span>
              <span className="flex items-center gap-1 text-xs text-slate-600 dark:text-slate-400">
                <span className="w-6 h-0.5 bg-yellow-500 inline-block" /> 95-99%
              </span>
              <span className="flex items-center gap-1 text-xs text-slate-600 dark:text-slate-400">
                <span className="w-6 h-0.5 bg-red-500 inline-block" /> &lt;95%
              </span>
            </div>
          </div>
        </div>

        {/* Node Details */}
        <div className="p-4 rounded-lg bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700">
          {selectedNodeData ? (
            <>
              <h3 className="text-sm font-semibold text-slate-900 dark:text-white mb-3 flex items-center gap-2">
                <div className="w-3 h-3 rounded-full flex-shrink-0" style={{ backgroundColor: NODE_COLORS[selectedNodeData.type] || NODE_COLORS.SERVICE }} />
                {selectedNodeData.name || selectedNodeData.id}
              </h3>
              {/* Incident badge */}
              {(() => {
                const incEntry = incidentMap[selectedNodeData.name] || incidentMap[selectedNodeData.id]
                if (!incEntry) return null
                const badgeClass = incEntry.status === "OPEN"
                  ? "bg-red-100 border-red-300 text-red-700 dark:bg-red-900/30 dark:border-red-700 dark:text-red-300"
                  : incEntry.status === "ACKNOWLEDGED"
                  ? "bg-yellow-100 border-yellow-300 text-yellow-700 dark:bg-yellow-900/30 dark:border-yellow-700 dark:text-yellow-300"
                  : "bg-green-100 border-green-300 text-green-700 dark:bg-green-900/30 dark:border-green-700 dark:text-green-300"
                return (
                  <div className={`mb-3 px-3 py-2 rounded-lg border text-xs font-semibold flex items-center justify-between gap-2 ${badgeClass}`}>
                    <span className="flex items-center gap-1.5">
                      <span>{incEntry.status === "OPEN" ? "🔴" : incEntry.status === "ACKNOWLEDGED" ? "🟡" : "🟢"}</span>
                      Active incident — {incEntry.status}
                    </span>
                    {incEntry.incidentId && (
                      <a
                        href={`/dashboard/incidents/${incEntry.incidentId}`}
                        className="underline underline-offset-2 font-bold hover:opacity-80"
                        target="_blank"
                        rel="noreferrer"
                      >
                        View RCA →
                      </a>
                    )}
                  </div>
                )
              })()}
              <div className="grid grid-cols-2 gap-2 text-sm">
                <div className="p-2 rounded bg-white dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700">
                  <p className="text-xs text-slate-500 mb-1">Type</p>
                  <p className="font-semibold text-slate-800 dark:text-slate-200">{NODE_LABELS[selectedNodeData.type] || selectedNodeData.type}</p>
                </div>
                <div className={`p-2 rounded border ${
                  (selectedNodeData.runtimeState?.errorRate ?? 0) > 0.5
                    ? "bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-700"
                    : (selectedNodeData.runtimeState?.errorRate ?? 0) > 0.1
                    ? "bg-yellow-50 dark:bg-yellow-900/20 border-yellow-200 dark:border-yellow-700"
                    : "bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-700"
                }`}>
                  <p className="text-xs text-slate-500 mb-1">Error Rate</p>
                  <p className={`font-bold ${
                    (selectedNodeData.runtimeState?.errorRate ?? 0) > 0.5 ? "text-red-600 dark:text-red-400"
                    : (selectedNodeData.runtimeState?.errorRate ?? 0) > 0.1 ? "text-yellow-600 dark:text-yellow-400"
                    : "text-green-600 dark:text-green-400"
                  }`}>
                    {selectedNodeData.runtimeState
                      ? `${((selectedNodeData.runtimeState.errorRate ?? 0) * 100).toFixed(1)}%`
                      : "N/A"}
                  </p>
                </div>
                {selectedNodeData.runtimeState?.latencyP95 !== undefined && (
                  <div className="p-2 rounded bg-white dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700">
                    <p className="text-xs text-slate-500 mb-1">Latency P95</p>
                    <p className="font-semibold text-slate-800 dark:text-slate-200">{selectedNodeData.runtimeState.latencyP95}ms</p>
                  </div>
                )}
                {selectedNodeData.runtimeState?.statusClass && (
                  <div className="p-2 rounded bg-white dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700">
                    <p className="text-xs text-slate-500 mb-1">HTTP Status</p>
                    <p className="font-semibold text-slate-800 dark:text-slate-200">{selectedNodeData.runtimeState.statusClass}</p>
                  </div>
                )}
              </div>
              {selectedNodeData.tags && selectedNodeData.tags.length > 0 && (
                <div className="mt-2 flex flex-wrap gap-1">
                  {selectedNodeData.tags.slice(0, 4).map((tag) => (
                    <span key={tag} className="px-2 py-0.5 text-xs rounded-full bg-slate-200 dark:bg-slate-700 text-slate-600 dark:text-slate-400">{tag}</span>
                  ))}
                </div>
              )}
            </>
          ) : (
            <div className="flex flex-col items-center justify-center h-full py-6 text-center">
              <div className="text-2xl mb-2">👆</div>
              <p className="text-sm font-medium text-slate-700 dark:text-slate-300">Click a node to inspect it</p>
              <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">See error rate, latency, and status details</p>
            </div>
          )}
        </div>
      </div>

      <div className="text-xs text-slate-500 dark:text-slate-400 text-center">
        Scroll to zoom · Drag nodes to rearrange · Click canvas to deselect · Use controls to zoom in/out/fit
      </div>
    </div>
  )
}
