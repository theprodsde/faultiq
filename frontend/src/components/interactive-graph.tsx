"use client"

import React, { useEffect, useRef, useState, useMemo } from "react"
import type { ServiceNode, ServiceEdge } from "@/types/api"

interface ForceNode extends ServiceNode {
  x?: number
  y?: number
  vx?: number
  vy?: number
  statusClass?: string
  errorRate?: number
  latencyP95?: number
}

interface ForceEdge extends ServiceEdge {
  source?: ForceNode
  target?: ForceNode
  successRatio?: number
}

interface Props {
  nodes: ServiceNode[]
  edges: ServiceEdge[]
  nodeMap: Record<string, string>
}

const NODE_COLORS: Record<string, string> = {
  SERVICE: "#3b82f6",     // blue
  DATABASE: "#a78bfa",    // violet
  QUEUE: "#f59e0b",       // amber
  GATEWAY: "#06b6d4",     // cyan
  EXTERNAL: "#64748b",    // slate
}

const NODE_LIGHT_COLORS: Record<string, string> = {
  SERVICE: "#1e40af",
  DATABASE: "#7c3aed",
  QUEUE: "#d97706",
  GATEWAY: "#0891b2",
  EXTERNAL: "#475569",
}

const CANVAS_WIDTH = 1200
const CANVAS_HEIGHT = 700
const NODE_RADIUS = 20

export function InteractiveGraph({ nodes, edges, nodeMap }: Props) {
  const svgRef = useRef<SVGSVGElement>(null)
  const [isDark, setIsDark] = useState(false)
  const [hoveredNodeId, setHoveredNodeId] = useState<string | null>(null)
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [simulation, setSimulation] = useState<any>(null)

  // Detect dark mode
  useEffect(() => {
    const isDarkMode = document.documentElement.classList.contains('dark')
    setIsDark(isDarkMode)
    
    const observer = new MutationObserver(() => {
      const dark = document.documentElement.classList.contains('dark')
      setIsDark(dark)
    })
    
    observer.observe(document.documentElement, { attributes: true })
    return () => observer.disconnect()
  }, [])

  // Initialize force simulation
  const forceData = useMemo(() => {
    const nodeMap = new Map<string, ForceNode>()
    nodes.forEach((n) => {
      nodeMap.set(n.id, { ...n, x: CANVAS_WIDTH / 2, y: CANVAS_HEIGHT / 2, vx: 0, vy: 0 })
    })

    const edgeList: ForceEdge[] = edges.map((e) => ({
      ...e,
      source: nodeMap.get(e.from),
      target: nodeMap.get(e.to),
    }))

    return { nodes: Array.from(nodeMap.values()), edges: edgeList }
  }, [nodes, edges])

  // Simple force simulation
  useEffect(() => {
    if (!svgRef.current || forceData.nodes.length === 0) return

    const tick = () => {
      // Apply forces
      const nodes = forceData.nodes
      const edges = forceData.edges

      // Repulsive force between nodes
      for (let i = 0; i < nodes.length; i++) {
        for (let j = i + 1; j < nodes.length; j++) {
          const a = nodes[i]
          const b = nodes[j]
          if (!a.x || !a.y || !b.x || !b.y) return

          const dx = b.x - a.x
          const dy = b.y - a.y
          const distance = Math.sqrt(dx * dx + dy * dy) || 1
          const force = (200 * 200) / (distance * distance)

          const fx = (force * dx) / distance
          const fy = (force * dy) / distance

          a.vx = (a.vx || 0) - fx * 0.01
          a.vy = (a.vy || 0) - fy * 0.01
          b.vx = (b.vx || 0) + fx * 0.01
          b.vy = (b.vy || 0) + fy * 0.01
        }
      }

      // Attractive force for edges
      edges.forEach((edge) => {
        const a = edge.source
        const b = edge.target
        if (!a || !b || !a.x || !a.y || !b.x || !b.y) return

        const dx = b.x - a.x
        const dy = b.y - a.y
        const distance = Math.sqrt(dx * dx + dy * dy) || 1
        const desiredDistance = 150
        const force = (distance - desiredDistance) * 0.1

        const fx = (force * dx) / distance
        const fy = (force * dy) / distance

        a.vx = (a.vx || 0) + fx * 0.1
        a.vy = (a.vy || 0) + fy * 0.1
        b.vx = (b.vx || 0) - fx * 0.1
        b.vy = (b.vy || 0) - fy * 0.1
      })

      // Center force
      nodes.forEach((node) => {
        if (!node.x || !node.y) return
        const dx = CANVAS_WIDTH / 2 - node.x
        const dy = CANVAS_HEIGHT / 2 - node.y
        node.vx = (node.vx || 0) + dx * 0.002
        node.vy = (node.vy || 0) + dy * 0.002
      })

      // Apply velocity with damping
      nodes.forEach((node) => {
        if (!node.x || !node.y) return
        node.vx = (node.vx || 0) * 0.99
        node.vy = (node.vy || 0) * 0.99
        node.x += node.vx
        node.y += node.vy

        // Boundary conditions
        node.x = Math.max(NODE_RADIUS, Math.min(CANVAS_WIDTH - NODE_RADIUS, node.x))
        node.y = Math.max(NODE_RADIUS, Math.min(CANVAS_HEIGHT - NODE_RADIUS, node.y))
      })

      // Render
      if (svgRef.current) {
        renderGraph(forceData, isDark, hoveredNodeId, selectedNodeId)
      }
    }

    let animationFrameId: number

    const animate = () => {
      tick()
      animationFrameId = requestAnimationFrame(animate)
    }

    animationFrameId = requestAnimationFrame(animate)
    setSimulation(true)

    return () => cancelAnimationFrame(animationFrameId)
  }, [forceData, isDark, hoveredNodeId, selectedNodeId])

  const renderGraph = (
    data: typeof forceData,
    dark: boolean,
    hoveredId: string | null,
    selectedId: string | null,
  ) => {
    if (!svgRef.current) return

    const svg = svgRef.current
    svg.innerHTML = ""

    // Background
    const bg = document.createElementNS("http://www.w3.org/2000/svg", "rect")
    bg.setAttribute("width", String(CANVAS_WIDTH))
    bg.setAttribute("height", String(CANVAS_HEIGHT))
    bg.setAttribute("fill", dark ? "#0f172a" : "#ffffff")
    svg.appendChild(bg)

    // Edges
    const edgesGroup = document.createElementNS("http://www.w3.org/2000/svg", "g")
    data.edges.forEach((edge) => {
      if (!edge.source || !edge.target || !edge.source.x || !edge.source.y || !edge.target.x || !edge.target.y) return

      const line = document.createElementNS("http://www.w3.org/2000/svg", "line")
      line.setAttribute("x1", String(edge.source.x))
      line.setAttribute("y1", String(edge.source.y))
      line.setAttribute("x2", String(edge.target.x))
      line.setAttribute("y2", String(edge.target.y))
      
      const successColor = !edge.successRatio ? "#94a3b8" : 
        edge.successRatio > 0.99 ? "#22c55e" :
        edge.successRatio > 0.95 ? "#eab308" : "#ef4444"
      
      line.setAttribute("stroke", successColor)
      line.setAttribute("stroke-width", "2")
      line.setAttribute("opacity", hoveredId ? (edge.source.id === hoveredId || edge.target.id === hoveredId ? "0.8" : "0.2") : "0.6")
      
      edgesGroup.appendChild(line)
    })
    svg.appendChild(edgesGroup)

    // Nodes
    const nodesGroup = document.createElementNS("http://www.w3.org/2000/svg", "g")
    data.nodes.forEach((node) => {
      if (!node.x || !node.y) return

      // Status indicator dot
      const statusColor =
        node.statusClass === "2xx" ? "#22c55e" :
        node.statusClass === "timeout" ? "#ef4444" :
        node.statusClass === "5xx" ? "#dc2626" : "#94a3b8"

      // Circle
      const circle = document.createElementNS("http://www.w3.org/2000/svg", "circle")
      circle.setAttribute("cx", String(node.x))
      circle.setAttribute("cy", String(node.y))
      circle.setAttribute("r", String(NODE_RADIUS))
      circle.setAttribute("fill", dark ? NODE_LIGHT_COLORS[node.type] || NODE_LIGHT_COLORS.SERVICE : NODE_COLORS[node.type] || NODE_COLORS.SERVICE)
      circle.setAttribute("opacity", hoveredId ? (hoveredId === node.id ? "1" : "0.3") : "0.85")
      circle.setAttribute("style", "transition: all 0.2s; cursor: pointer;")
      circle.addEventListener("mouseenter", () => setHoveredNodeId(node.id))
      circle.addEventListener("mouseleave", () => setHoveredNodeId(null))
      circle.addEventListener("click", () => setSelectedNodeId(selectedNodeId === node.id ? null : node.id))
      
      nodesGroup.appendChild(circle)

      // Status dot
      const statusDot = document.createElementNS("http://www.w3.org/2000/svg", "circle")
      statusDot.setAttribute("cx", String(node.x + NODE_RADIUS - 4))
      statusDot.setAttribute("cy", String(node.y - NODE_RADIUS + 4))
      statusDot.setAttribute("r", "4")
      statusDot.setAttribute("fill", statusColor)
      statusDot.setAttribute("stroke", dark ? "#0f172a" : "#ffffff")
      statusDot.setAttribute("stroke-width", "1")
      
      nodesGroup.appendChild(statusDot)

      // Label
      const text = document.createElementNS("http://www.w3.org/2000/svg", "text")
      text.setAttribute("x", String(node.x))
      text.setAttribute("y", String(node.y + 4))
      text.setAttribute("text-anchor", "middle")
      text.setAttribute("font-size", "11")
      text.setAttribute("font-weight", "500")
      text.setAttribute("fill", dark ? "#e2e8f0" : "#1e293b")
      text.setAttribute("pointer-events", "none")
      text.textContent = node.name?.substring(0, 12) || node.id.substring(0, 12)
      
      nodesGroup.appendChild(text)
    })
    svg.appendChild(nodesGroup)

    // Tooltip for selected node
    if (selectedNodeId) {
      const node = data.nodes.find((n) => n.id === selectedNodeId)
      if (node && node.x && node.y) {
        const tooltip = document.createElementNS("http://www.w3.org/2000/svg", "g")
        
        const bgRect = document.createElementNS("http://www.w3.org/2000/svg", "rect")
        bgRect.setAttribute("x", String(node.x + 30))
        bgRect.setAttribute("y", String(node.y - 50))
        bgRect.setAttribute("width", "220")
        bgRect.setAttribute("height", "80")
        bgRect.setAttribute("rx", "6")
        bgRect.setAttribute("fill", dark ? "#1e293b" : "#f1f5f9")
        bgRect.setAttribute("stroke", dark ? "#475569" : "#cbd5e1")
        bgRect.setAttribute("stroke-width", "1")
        
        tooltip.appendChild(bgRect)

        const nameText = document.createElementNS("http://www.w3.org/2000/svg", "text")
        nameText.setAttribute("x", String(node.x + 40))
        nameText.setAttribute("y", String(node.y - 35))
        nameText.setAttribute("font-size", "12")
        nameText.setAttribute("font-weight", "600")
        nameText.setAttribute("fill", dark ? "#f1f5f9" : "#0f172a")
        nameText.textContent = node.name || node.id

        tooltip.appendChild(nameText)

        const typeText = document.createElementNS("http://www.w3.org/2000/svg", "text")
        typeText.setAttribute("x", String(node.x + 40))
        typeText.setAttribute("y", String(node.y - 20))
        typeText.setAttribute("font-size", "10")
        typeText.setAttribute("fill", dark ? "#94a3b8" : "#475569")
        typeText.textContent = `Type: ${node.type}`

        tooltip.appendChild(typeText)

        const errText = document.createElementNS("http://www.w3.org/2000/svg", "text")
        errText.setAttribute("x", String(node.x + 40))
        errText.setAttribute("y", String(node.y - 8))
        errText.setAttribute("font-size", "10")
        errText.setAttribute("fill", dark ? "#94a3b8" : "#475569")
        errText.textContent = `Error Rate: ${((node.errorRate || 0) * 100).toFixed(2)}%`

        tooltip.appendChild(errText)

        const latText = document.createElementNS("http://www.w3.org/2000/svg", "text")
        latText.setAttribute("x", String(node.x + 40))
        latText.setAttribute("y", String(node.y + 4))
        latText.setAttribute("font-size", "10")
        latText.setAttribute("fill", dark ? "#94a3b8" : "#475569")
        latText.textContent = `Latency P95: ${node.latencyP95}ms`

        tooltip.appendChild(latText)

        svg.appendChild(tooltip)
      }
    }
  }

  return (
    <div className="relative">
      <svg
        ref={svgRef}
        width={CANVAS_WIDTH}
        height={CANVAS_HEIGHT}
        className="w-full border rounded-lg bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-700"
        style={{ maxWidth: "100%" }}
      />
      <div className="flex gap-2 text-xs text-slate-500 dark:text-slate-400 mt-2">
        <span>● Green: Healthy</span>
        <span>● Yellow: Degraded</span>
        <span>● Red: Failed</span>
        <span className="ml-auto">Click nodes for details</span>
      </div>
    </div>
  )
}
