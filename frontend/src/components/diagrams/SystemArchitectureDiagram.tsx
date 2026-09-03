"use client";

import React, { useState, useEffect, useCallback, useMemo } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  type Node,
  type Edge,
  Position,
  MarkerType,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

// ─── Node Style Factory ─────────────────────────────────────────────────────
const makeNodeStyle = (isError: boolean) => ({
  background: isError ? "#1a0505" : "#0d1117",
  border: `2px solid ${isError ? "#ef4444" : "#22d3ee"}`,
  borderRadius: "8px",
  padding: "10px 16px",
  color: isError ? "#fca5a5" : "#e2e8f0",
  fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
  fontSize: "11px",
  fontWeight: 600,
  boxShadow: isError
    ? "0 0 15px rgba(239,68,68,0.4)"
    : "0 0 10px rgba(34,211,238,0.2)",
  minWidth: "130px",
  textAlign: "center" as const,
});

// ─── Initial Nodes (≈20 nodes across 8 tiers) ──────────────────────────────
const TIER_Y = {
  client: 0,
  ingress: 120,
  gateway: 240,
  core: 380,
  workers: 530,
  message: 680,
  data: 820,
  downstream: 970,
};

const initialNodes: Node[] = [
  // Tier 1: Client Edge
  { id: "web-app", position: { x: 250, y: TIER_Y.client }, data: { label: "🌐 Web App" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },
  { id: "mobile-app", position: { x: 550, y: TIER_Y.client }, data: { label: "📱 Mobile App" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },

  // Tier 2: Ingress
  { id: "waf-cdn", position: { x: 150, y: TIER_Y.ingress }, data: { label: "🛡️ WAF / CDN" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },
  { id: "load-balancer", position: { x: 550, y: TIER_Y.ingress }, data: { label: "⚖️ Load Balancer" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },

  // Tier 3: API Gateway
  { id: "api-gateway", position: { x: 370, y: TIER_Y.gateway }, data: { label: "🚪 API Gateway" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },

  // Tier 4: Core Microservices
  { id: "auth-svc", position: { x: 50, y: TIER_Y.core }, data: { label: "🔐 Auth Service" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },
  { id: "user-svc", position: { x: 250, y: TIER_Y.core }, data: { label: "👤 User Service" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },
  { id: "product-svc", position: { x: 470, y: TIER_Y.core }, data: { label: "📦 Product Service" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },
  { id: "order-svc", position: { x: 700, y: TIER_Y.core }, data: { label: "🛒 Order Service" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },

  // Tier 5: Workers & AI
  { id: "payment-worker", position: { x: 30, y: TIER_Y.workers }, data: { label: "💳 Payment Worker" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },
  { id: "inventory-worker", position: { x: 230, y: TIER_Y.workers }, data: { label: "📋 Inventory Worker" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },
  { id: "langgraph", position: { x: 470, y: TIER_Y.workers }, data: { label: "🧠 LangGraph Orchestrator" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },
  { id: "crewai", position: { x: 720, y: TIER_Y.workers }, data: { label: "🤖 CrewAI Agents" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },

  // Tier 6: Message Broker
  { id: "kafka", position: { x: 370, y: TIER_Y.message }, data: { label: "📨 Apache Kafka" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },

  // Tier 7: Data Layer
  { id: "redis", position: { x: 50, y: TIER_Y.data }, data: { label: "⚡ Redis Cache" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },
  { id: "postgres", position: { x: 250, y: TIER_Y.data }, data: { label: "🐘 PostgreSQL" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },
  { id: "mongodb", position: { x: 480, y: TIER_Y.data }, data: { label: "🍃 MongoDB" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },
  { id: "vector-db", position: { x: 700, y: TIER_Y.data }, data: { label: "🧮 Vector DB" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },

  // Tier 8: Downstream & Analytics
  { id: "notification", position: { x: 200, y: TIER_Y.downstream }, data: { label: "🔔 Notification Svc" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },
  { id: "elasticsearch", position: { x: 520, y: TIER_Y.downstream }, data: { label: "🔍 ElasticSearch" }, sourcePosition: Position.Bottom, targetPosition: Position.Top },
];

// ─── Initial Edges ──────────────────────────────────────────────────────────
const EDGE_STYLE_HEALTHY = { stroke: "#22c55e", strokeWidth: 2 };
const MARKER_END = { type: MarkerType.ArrowClosed, color: "#22c55e", width: 16, height: 16 };

const initialEdges: Edge[] = [
  // Client → Ingress
  { id: "e-web-waf", source: "web-app", target: "waf-cdn", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-mobile-lb", source: "mobile-app", target: "load-balancer", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },

  // Ingress → Gateway
  { id: "e-waf-gw", source: "waf-cdn", target: "api-gateway", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-lb-gw", source: "load-balancer", target: "api-gateway", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },

  // Gateway → Core
  { id: "e-gw-auth", source: "api-gateway", target: "auth-svc", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-gw-user", source: "api-gateway", target: "user-svc", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-gw-product", source: "api-gateway", target: "product-svc", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-gw-order", source: "api-gateway", target: "order-svc", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },

  // Core → Workers
  { id: "e-order-payment", source: "order-svc", target: "payment-worker", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-product-inventory", source: "product-svc", target: "inventory-worker", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-user-langgraph", source: "user-svc", target: "langgraph", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-auth-crewai", source: "auth-svc", target: "crewai", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },

  // Workers → Kafka
  { id: "e-payment-kafka", source: "payment-worker", target: "kafka", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-inventory-kafka", source: "inventory-worker", target: "kafka", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-langgraph-kafka", source: "langgraph", target: "kafka", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-crewai-kafka", source: "crewai", target: "kafka", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },

  // Kafka → Data
  { id: "e-kafka-redis", source: "kafka", target: "redis", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-kafka-pg", source: "kafka", target: "postgres", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-kafka-mongo", source: "kafka", target: "mongodb", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-kafka-vector", source: "kafka", target: "vector-db", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },

  // Data → Downstream
  { id: "e-redis-notify", source: "redis", target: "notification", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-pg-notify", source: "postgres", target: "notification", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-mongo-elastic", source: "mongodb", target: "elasticsearch", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
  { id: "e-vector-elastic", source: "vector-db", target: "elasticsearch", animated: true, style: EDGE_STYLE_HEALTHY, markerEnd: MARKER_END },
];

// ─── Nodes eligible for chaos monkey failure ────────────────────────────────
const CHAOS_TARGETS = [
  "auth-svc",
  "user-svc",
  "product-svc",
  "order-svc",
  "payment-worker",
  "inventory-worker",
  "langgraph",
  "crewai",
  "kafka",
  "redis",
  "postgres",
  "mongodb",
  "vector-db",
];

// ─── Component ──────────────────────────────────────────────────────────────
export default function SystemArchitectureDiagram() {
  const [nodes, setNodes] = useState<Node[]>(() =>
    initialNodes.map((n) => ({ ...n, style: makeNodeStyle(false) }))
  );
  const [edges, setEdges] = useState<Edge[]>(initialEdges);
  const [failedNode, setFailedNode] = useState<string | null>(null);

  // Apply style to all nodes with healthy default
  const styledNodes = useMemo(
    () =>
      nodes.map((n) => ({
        ...n,
        style: makeNodeStyle(n.id === failedNode),
        data: {
          ...n.data,
          label:
            n.id === failedNode
              ? `❌ [ERR] ${String(n.data.label).replace(/^❌ \[ERR\] /, "")}`
              : String(n.data.label).replace(/^❌ \[ERR\] /, ""),
        },
      })),
    [nodes, failedNode]
  );

  // Apply error state to edges connected to the failed node
  const styledEdges = useMemo(
    () =>
      edges.map((e) => {
        const isAffected =
          failedNode && (e.source === failedNode || e.target === failedNode);
        return {
          ...e,
          animated: !isAffected,
          style: isAffected
            ? { stroke: "#ef4444", strokeWidth: 2.5 }
            : EDGE_STYLE_HEALTHY,
          markerEnd: isAffected
            ? { type: MarkerType.ArrowClosed, color: "#ef4444", width: 16, height: 16 }
            : MARKER_END,
        };
      }),
    [edges, failedNode]
  );

  // ─── Chaos Monkey Loop ──────────────────────────────────────────────────
  const triggerFailure = useCallback(() => {
    const target =
      CHAOS_TARGETS[Math.floor(Math.random() * CHAOS_TARGETS.length)];
    setFailedNode(target);

    // Auto-heal after 3 seconds
    setTimeout(() => {
      setFailedNode((current) => (current === target ? null : current));
    }, 3000);
  }, []);

  useEffect(() => {
    // Random interval between 5-7 seconds
    const scheduleNext = () => {
      const delay = 5000 + Math.random() * 2000;
      return setTimeout(() => {
        triggerFailure();
        timerRef = scheduleNext();
      }, delay);
    };

    let timerRef = scheduleNext();
    return () => clearTimeout(timerRef);
  }, [triggerFailure]);

  return (
    <div
      style={{
        width: "100%",
        height: "100%",
        background: "#0a0a0f",
        borderRadius: "16px",
        overflow: "hidden",
        border: "1px solid rgba(34,211,238,0.15)",
      }}
    >
      {/* Status Bar */}
      <div
        style={{
          position: "absolute",
          top: 12,
          left: 16,
          zIndex: 10,
          display: "flex",
          alignItems: "center",
          gap: "8px",
          padding: "6px 12px",
          background: "rgba(10,10,15,0.85)",
          borderRadius: "8px",
          border: "1px solid rgba(34,211,238,0.2)",
          backdropFilter: "blur(8px)",
        }}
      >
        <span
          style={{
            width: 8,
            height: 8,
            borderRadius: "50%",
            background: failedNode ? "#ef4444" : "#22c55e",
            boxShadow: failedNode
              ? "0 0 8px #ef4444"
              : "0 0 8px #22c55e",
            animation: "pulse 2s ease-in-out infinite",
          }}
        />
        <span
          style={{
            fontFamily: "monospace",
            fontSize: "11px",
            color: failedNode ? "#fca5a5" : "#86efac",
            fontWeight: 600,
          }}
        >
          {failedNode
            ? `INCIDENT DETECTED — ${failedNode}`
            : "ALL SYSTEMS OPERATIONAL"}
        </span>
      </div>

      {/* Legend */}
      <div
        style={{
          position: "absolute",
          top: 12,
          right: 16,
          zIndex: 10,
          display: "flex",
          alignItems: "center",
          gap: "16px",
          padding: "6px 12px",
          background: "rgba(10,10,15,0.85)",
          borderRadius: "8px",
          border: "1px solid rgba(34,211,238,0.2)",
          backdropFilter: "blur(8px)",
          fontFamily: "monospace",
          fontSize: "10px",
        }}
      >
        <span style={{ display: "flex", alignItems: "center", gap: 4, color: "#86efac" }}>
          <span style={{ width: 12, height: 3, background: "#22c55e", borderRadius: 2 }} />
          Healthy Flow
        </span>
        <span style={{ display: "flex", alignItems: "center", gap: 4, color: "#fca5a5" }}>
          <span style={{ width: 12, height: 3, background: "#ef4444", borderRadius: 2 }} />
          Failure
        </span>
        <span style={{ display: "flex", alignItems: "center", gap: 4, color: "#93c5fd" }}>
          <span style={{ width: 8, height: 8, border: "2px solid #22d3ee", borderRadius: 2 }} />
          Node
        </span>
      </div>

      <ReactFlow
        nodes={styledNodes}
        edges={styledEdges}
        fitView
        fitViewOptions={{ padding: 0.15 }}
        proOptions={{ hideAttribution: true }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        panOnDrag={true}
        zoomOnScroll={true}
        minZoom={0.4}
        maxZoom={1.5}
      >
        <Background color="#1e293b" gap={24} size={1} />
        <Controls
          showInteractive={false}
          style={{
            background: "#0d1117",
            border: "1px solid rgba(34,211,238,0.2)",
            borderRadius: "8px",
          }}
        />
      </ReactFlow>

      {/* Pulse animation keyframes */}
      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.5; }
        }
      `}</style>
    </div>
  );
}
