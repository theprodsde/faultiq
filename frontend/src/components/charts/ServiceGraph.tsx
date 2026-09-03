'use client'

import React, { useMemo } from 'react'

interface ServiceNode {
  id: string
  name: string
  status: 'healthy' | 'warning' | 'critical'
  errorRate?: number
  latency?: number
}

interface ServiceEdge {
  from: string
  to: string
  successRatio?: number
}

interface ServiceGraphProps {
  nodes: ServiceNode[]
  edges: ServiceEdge[]
  title?: string
}

export default function ServiceGraph({ nodes, edges, title }: ServiceGraphProps) {
  const mermaidDiagram = useMemo(() => {
    if (!nodes || nodes.length === 0) return null

    // Build node definitions with styling
    const nodeDefinitions = nodes.map((node) => {
      const statusColor = {
        healthy: '#10b981',
        warning: '#f59e0b',
        critical: '#ef4444'
      }[node.status]

      const style = `
        style ${node.id} fill:${statusColor}20,stroke:${statusColor},color:#fff,stroke-width:2px
      `

      return `
        ${node.id}["${node.name}<br/>${node.errorRate !== undefined ? `Errors: ${node.errorRate}%` : ''}<br/>${node.latency !== undefined ? `Latency: ${node.latency}ms` : ''}"]
        ${style}
      `
    }).join('\n')

    // Build edge definitions
    const edgeDefinitions = edges.map((edge) => {
      const label = edge.successRatio ? ` |${(edge.successRatio * 100).toFixed(0)}%| ` : ''
      return `${edge.from} -->|${label}| ${edge.to}`
    }).join('\n')

    const diagram = `
      graph LR
        ${nodeDefinitions}
        ${edgeDefinitions}
    `

    return diagram
  }, [nodes, edges])

  if (!mermaidDiagram) {
    return (
      <div className="p-12 text-center text-slate-500">
        <p>No service data available</p>
      </div>
    )
  }

  return (
    <div className="p-6 bg-slate-50 dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-800">
      {title && <h3 className="font-semibold text-lg mb-4 text-slate-900 dark:text-white">{title}</h3>}

      <div className="overflow-x-auto">
        <div className="mermaid min-w-full flex justify-center">
          {mermaidDiagram}
        </div>
      </div>

      {/* Status Legend */}
      <div className="mt-6 grid grid-cols-3 gap-4 text-sm">
        <div className="flex items-center gap-2">
          <div className="w-4 h-4 rounded bg-green-500" />
          <span className="text-slate-700 dark:text-slate-300">Healthy</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-4 h-4 rounded bg-yellow-500" />
          <span className="text-slate-700 dark:text-slate-300">Warning</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-4 h-4 rounded bg-red-500" />
          <span className="text-slate-700 dark:text-slate-300">Critical</span>
        </div>
      </div>

      {/* Load Mermaid script */}
      <script async src="https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js" />
    </div>
  )
}
