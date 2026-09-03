"use client"

import { useState, useRef } from "react"
import { useImportServiceMapMutation, useGetOnboardingJobQuery } from "@/store/services"
import { Button } from "@/components/ui/button"
import { Upload, CheckCircle2, Loader2, AlertCircle, Copy, FileText, ArrowRight, Code } from "lucide-react"
import Link from "next/link"

const EXAMPLE_YAML = `version: "1"
# Example: 10-service e-commerce platform
# Replace health URLs with your actual endpoints.
# Add repo: to each service for code-level RCA (shows which commit caused the fault).

projects:
  - id: proj-ecommerce-prod
    name: E-Commerce Platform
    namespace: ecommerce:prod
    tenant: YOUR_TENANT_ID       # from your Keycloak JWT
    environment: prod
    services:

      # ── Entry point ──────────────────────────────────────────────────────
      - id: svc_api_gateway
        name: api-gateway
        type: GATEWAY
        healthUrl: http://api-gateway/health
        repo: https://github.com/your-org/api-gateway
        calls: [svc_user, svc_product, svc_cart, svc_order]

      # ── Domain services ──────────────────────────────────────────────────
      - id: svc_user
        name: user-service
        type: SERVICE
        healthUrl: http://user-service/health
        repo: https://github.com/your-org/user-service
        calls: [db_users]

      - id: svc_product
        name: product-service
        type: SERVICE
        healthUrl: http://product-service/health
        repo: https://github.com/your-org/product-service
        calls: [db_products, cache_redis]

      - id: svc_cart
        name: cart-service
        type: SERVICE
        healthUrl: http://cart-service/health
        repo: https://github.com/your-org/cart-service
        calls: [svc_product, cache_redis]

      - id: svc_order
        name: order-service
        type: SERVICE
        healthUrl: http://order-service/health
        repo: https://github.com/your-org/order-service
        calls: [svc_cart, svc_payment, svc_notification, db_orders]

      - id: svc_payment
        name: payment-service
        type: SERVICE
        healthUrl: http://payment-service/health
        repo: https://github.com/your-org/payment-service
        calls: [db_orders, ext_stripe]

      - id: svc_notification
        name: notification-service
        type: SERVICE
        healthUrl: http://notification-service/health
        repo: https://github.com/your-org/notification-service
        calls: [cache_redis]

      # ── Data stores — use TCP checks (no HTTP endpoint needed) ──────
      - id: db_users
        name: users-db
        type: DATABASE
        healthUrl: tcp://postgres:5432             # TCP port check — no HTTP needed
        timeoutSeconds: 3

      - id: db_products
        name: products-db
        type: DATABASE
        healthUrl: tcp://postgres:5432

      - id: db_orders
        name: orders-db
        type: DATABASE
        healthUrl: tcp://postgres:5432

      - id: cache_redis
        name: redis-cache
        type: DATABASE
        healthUrl: tcp://redis:6379

      # ── External APIs (HTTPS with body check) ─────────────────────
      - id: ext_stripe
        name: stripe-api
        type: EXTERNAL
        healthUrl: https://status.stripe.com/api/v2/status.json
        healthCheck:
          bodyFormat: none                         # trust HTTP 200 only
          expectedStatus: [200]

# ─── Health Check Format Examples ──────────────────────────────────────────
# Copy the pattern that matches your stack:
#
# Standard (TechGraph default — any JSON with "status"):
#   healthUrl: http://my-service/health
#   # Response: {"status":"ok","latency_p95_ms":45,"error_rate":0.001}
#
# Spring Boot Actuator:
#   healthUrl: http://my-service/actuator/health
#   healthCheck:
#     bodyFormat: spring       # parses {"status":"UP"/"DOWN"/"OUT_OF_SERVICE"}
#
# Kubernetes-style:
#   healthUrl: http://my-service/readyz
#   healthCheck:
#     bodyFormat: kubernetes   # parses {"status":"ok"} or plain "ok"
#
# HTTPS with self-signed cert (internal services):
#   healthUrl: https://my-service:8443/health
#   healthCheck:
#     tlsSkipVerify: true      # for internal self-signed certs only
#
# Custom headers (auth-protected health endpoint):
#   healthUrl: http://my-service/health
#   healthCheck:
#     headers:
#       Authorization: "Bearer health-check-token"
#       X-Internal-Check: "true"
#
# Synthetic check (verify real endpoint, not just /health):
#   healthUrl: http://order-service/health
#   healthCheck:
#     syntheticPath: /api/v1/orders?limit=1     # also checks this endpoint
#     # If /health returns 200 but /api/v1/orders fails → service marked degraded
#
# TCP check (databases, Redis, Kafka — no HTTP needed):
#   healthUrl: tcp://postgres:5432`

const ECOMMERCE_YAML = `version: "1"
# E-Commerce Platform — 12 services
# Replace YOUR_TENANT_ID with your tenant slug.
# Replace demo-services healthUrls with your real endpoints.

projects:
  - id: proj-ecommerce-prod
    name: E-Commerce Platform
    namespace: ecommerce:prod
    tenant: YOUR_TENANT_ID
    environment: prod
    services:

      - id: svc_api_gateway
        name: api-gateway
        type: GATEWAY
        healthUrl: http://demo-services:8091/services/api-gateway/health
        # repo: https://github.com/your-org/api-gateway
        calls: [svc_user, svc_product, svc_cart, svc_order, svc_search]
        tags: [public-facing, ingress]

      - id: svc_user
        name: user-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/user-service/health
        # repo: https://github.com/your-org/user-service
        calls: [db_users, cache_redis]

      - id: svc_product
        name: product-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/product-service/health
        calls: [db_products, cache_redis]

      - id: svc_cart
        name: cart-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/cart-service/health
        calls: [svc_product, cache_redis]

      - id: svc_order
        name: order-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/order-service/health
        calls: [svc_cart, svc_payment, svc_inventory, svc_notification, db_orders]
        thresholds:
          errorRateThreshold: 0.002
          latencyThresholdMs: 800

      - id: svc_payment
        name: payment-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/payment-service/health
        calls: [db_orders, ext_stripe]
        thresholds:
          errorRateThreshold: 0.001
          latencyThresholdMs: 1200
          minSignalCount: 2

      - id: svc_inventory
        name: inventory-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/inventory-service/health
        calls: [db_products, cache_redis]

      - id: svc_notification
        name: notification-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/notification-service/health
        calls: [cache_redis]

      - id: svc_search
        name: search-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/search-service/health
        calls: [db_products, cache_redis]

      - id: db_users
        name: users-db
        type: DATABASE
        healthUrl: http://demo-services:8091/services/users-db/health

      - id: db_products
        name: products-db
        type: DATABASE
        healthUrl: http://demo-services:8091/services/products-db/health

      - id: db_orders
        name: orders-db
        type: DATABASE
        healthUrl: http://demo-services:8091/services/orders-db/health

      - id: cache_redis
        name: redis-cache
        type: DATABASE
        healthUrl: http://demo-services:8091/services/redis-cache/health

      - id: ext_stripe
        name: stripe-api
        type: EXTERNAL
        healthUrl: http://demo-services:8091/services/stripe-api/health`

const SAAS_ANALYTICS_YAML = `version: "1"
# SaaS Analytics Platform — 8 services
# Data pipeline: collector → processor → aggregation → timeseries-db
# Query layer: reporting-api → timeseries-db, dashboard-ui → reporting-api

projects:
  - id: proj-analytics-prod
    name: SaaS Analytics Platform
    namespace: analytics:prod
    tenant: YOUR_TENANT_ID
    environment: prod
    services:

      - id: svc_api_gateway
        name: api-gateway
        type: GATEWAY
        healthUrl: http://demo-services:8091/services/api-gateway/health
        calls: [svc_data_collector, svc_reporting_api, svc_dashboard_ui]
        tags: [public-facing, ingress]

      - id: svc_data_collector
        name: data-collector
        type: SERVICE
        healthUrl: http://demo-services:8091/services/data-collector/health
        calls: [svc_event_processor, cache_redis]
        thresholds:
          errorRateThreshold: 0.005
          latencyThresholdMs: 200

      - id: svc_event_processor
        name: event-processor
        type: SERVICE
        healthUrl: http://demo-services:8091/services/event-processor/health
        calls: [svc_aggregation, cache_redis]
        thresholds:
          errorRateThreshold: 0.005
          latencyThresholdMs: 500

      - id: svc_aggregation
        name: aggregation-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/aggregation-service/health
        calls: [db_timeseries, cache_redis]

      - id: svc_reporting_api
        name: reporting-api
        type: SERVICE
        healthUrl: http://demo-services:8091/services/reporting-api/health
        calls: [db_timeseries, cache_redis]
        thresholds:
          errorRateThreshold: 0.01
          latencyThresholdMs: 3000

      - id: svc_dashboard_ui
        name: dashboard-ui
        type: SERVICE
        healthUrl: http://demo-services:8091/services/dashboard-ui/health
        calls: [svc_reporting_api]

      - id: db_timeseries
        name: timeseries-db
        type: DATABASE
        healthUrl: http://demo-services:8091/services/timeseries-db/health

      - id: cache_redis
        name: redis-cache
        type: DATABASE
        healthUrl: http://demo-services:8091/services/redis-cache/health`

const FINTECH_YAML = `version: "1"
# Fintech Payments Platform — 11 services
# Every service calls auth-service for JWT validation.
# payment-processor → fraud-detection (sync pre-auth check).
# All mutations are logged to audit-log (compliance).

projects:
  - id: proj-fintech-prod
    name: Fintech Payments Platform
    namespace: payments:prod
    tenant: YOUR_TENANT_ID
    environment: prod
    services:

      - id: svc_api_gateway
        name: api-gateway
        type: GATEWAY
        healthUrl: http://demo-services:8091/services/api-gateway/health
        calls: [svc_auth, svc_payment_processor]
        tags: [public-facing, ingress, pci-scope]

      - id: svc_auth
        name: auth-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/auth-service/health
        calls: [cache_redis]
        thresholds:
          errorRateThreshold: 0.001
          latencyThresholdMs: 100
          minSignalCount: 2

      - id: svc_payment_processor
        name: payment-processor
        type: SERVICE
        healthUrl: http://demo-services:8091/services/payment-api/health
        calls: [svc_auth, svc_fraud_detection, svc_ledger, svc_audit_log, ext_stripe]
        tags: [pci-scope, revenue-critical]
        thresholds:
          errorRateThreshold: 0.001
          latencyThresholdMs: 2000
          minSignalCount: 2

      - id: svc_fraud_detection
        name: fraud-detection
        type: SERVICE
        healthUrl: http://demo-services:8091/services/fraud-detection/health
        calls: [svc_auth, cache_redis, db_postgres]
        tags: [pci-scope]
        thresholds:
          errorRateThreshold: 0.002
          latencyThresholdMs: 500

      - id: svc_ledger
        name: ledger-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/ledger-service/health
        calls: [svc_auth, svc_audit_log, db_postgres]
        tags: [pci-scope]
        thresholds:
          errorRateThreshold: 0.001
          latencyThresholdMs: 400

      - id: svc_settlement_worker
        name: settlement-worker
        type: SERVICE
        healthUrl: http://demo-services:8091/services/settlement-worker/health
        calls: [svc_auth, svc_ledger, svc_notification, svc_audit_log]
        thresholds:
          errorRateThreshold: 0.002
          latencyThresholdMs: 5000

      - id: svc_notification
        name: notification-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/notification-service/health
        calls: [svc_auth, cache_redis]

      - id: svc_audit_log
        name: audit-log
        type: SERVICE
        healthUrl: http://demo-services:8091/services/audit-log/health
        calls: [db_postgres]
        tags: [compliance, immutable]
        thresholds:
          errorRateThreshold: 0.0005
          latencyThresholdMs: 200

      - id: db_postgres
        name: postgres-primary
        type: DATABASE
        healthUrl: http://demo-services:8091/services/postgres-db/health

      - id: cache_redis
        name: redis-cache
        type: DATABASE
        healthUrl: http://demo-services:8091/services/redis-cache/health

      - id: ext_stripe
        name: ext-stripe
        type: EXTERNAL
        healthUrl: http://demo-services:8091/services/stripe-api/health`

interface TemplateCard {
  name: string
  icon: string
  description: string
  services: number
  yaml: string
}

const TEMPLATES: TemplateCard[] = [
  {
    name: "E-Commerce",
    icon: "🛒",
    description: "12 services: api-gateway, cart, orders, payments, inventory, notifications",
    services: 12,
    yaml: ECOMMERCE_YAML,
  },
  {
    name: "SaaS Analytics",
    icon: "📊",
    description: "8 services: data pipeline, event processing, aggregation, reporting",
    services: 8,
    yaml: SAAS_ANALYTICS_YAML,
  },
  {
    name: "Fintech Payments",
    icon: "💳",
    description: "10 services: payment processor, fraud detection, ledger, settlement",
    services: 10,
    yaml: FINTECH_YAML,
  },
  {
    name: "Custom",
    icon: "⚙️",
    description: "Start from scratch with a blank YAML editor",
    services: 0,
    yaml: "",
  },
]

interface ServiceMapImportProps {
  projectId: string
  onSuccess?: (jobId: string) => void
  compact?: boolean
}

export function ServiceMapImport({ projectId, onSuccess, compact = false }: ServiceMapImportProps) {
  const [yaml, setYaml] = useState("")
  const [mode, setMode] = useState<"paste" | "upload">("paste")
  const [selectedTemplate, setSelectedTemplate] = useState<string | null>(null)
  const [importServiceMap, { isLoading }] = useImportServiceMapMutation()
  const [jobId, setJobId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const fileRef = useRef<HTMLInputElement>(null)

  const { data: job } = useGetOnboardingJobQuery(
    { projectId, jobId: jobId ?? "" },
    { skip: !jobId, pollingInterval: 2000 }
  )

  const handleImport = async () => {
    if (!yaml.trim()) {
      setError("Paste your service-map.yaml content above")
      return
    }
    setError(null)
    const result = await importServiceMap({ projectId, yaml })
    if ("data" in result && result.data) {
      setJobId(result.data.jobId ?? "")
      onSuccess?.(result.data.jobId ?? "")
    } else if ("error" in result) {
      setError("Import failed — check your YAML format")
    }
  }

  const handleFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = (ev) => setYaml(ev.target?.result as string ?? "")
    reader.readAsText(file)
  }

  const loadExample = () => {
    setYaml(EXAMPLE_YAML)
    setSelectedTemplate(null)
  }

  const selectTemplate = (tpl: TemplateCard) => {
    setSelectedTemplate(tpl.name)
    setYaml(tpl.yaml)
    setMode("paste")
  }

  // ── Success screen ───────────────────────────────────────────────────────
  if (job?.status === "COMPLETE") {
    // Count services and dependencies from the yaml we just imported
    const serviceLines = (yaml.match(/^\s+- id: /gm) ?? []).length
    const callLines = (yaml.match(/calls:/g) ?? []).length

    return (
      <div className="space-y-4">
        {/* Success banner */}
        <div className="flex items-start gap-3 p-4 rounded-xl bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800">
          <CheckCircle2 className="w-5 h-5 text-green-600 dark:text-green-400 flex-shrink-0 mt-0.5" />
          <div>
            <p className="text-sm font-semibold text-green-800 dark:text-green-300">
              Graph built successfully!
            </p>
            <p className="text-xs text-green-600 dark:text-green-400 mt-0.5">
              Services are now being monitored. Health poller is active.
            </p>
          </div>
        </div>

        {/* Quick stats */}
        {serviceLines > 0 && (
          <div className="flex gap-4">
            <div className="flex-1 p-3 rounded-lg bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-center">
              <p className="text-2xl font-bold text-slate-900 dark:text-white">{serviceLines}</p>
              <p className="text-xs text-slate-500 mt-0.5">services</p>
            </div>
            <div className="flex-1 p-3 rounded-lg bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-center">
              <p className="text-2xl font-bold text-slate-900 dark:text-white">{callLines}</p>
              <p className="text-xs text-slate-500 mt-0.5">dependencies</p>
            </div>
          </div>
        )}

        {/* Next-step CTAs */}
        <div className="space-y-2">
          <Link
            href={`/dashboard/graph?project=${projectId}`}
            className="flex items-center justify-between gap-2 w-full px-4 py-3 rounded-lg bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium transition-colors"
          >
            <span>View Graph</span>
            <ArrowRight className="w-4 h-4" />
          </Link>
          <Link
            href={`/dashboard/projects/${projectId}?tab=code`}
            className="flex items-center justify-between gap-2 w-full px-4 py-3 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-700 dark:text-slate-300 text-sm font-medium hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors"
          >
            <span className="flex items-center gap-2">
              <Code className="w-4 h-4 text-violet-500" />
              Configure repos for code-level RCA
            </span>
            <ArrowRight className="w-4 h-4 text-slate-400" />
          </Link>
        </div>
      </div>
    )
  }

  if (job && (job.status === "RUNNING" || job.status === "FAILED" || job.status === "VALIDATION_PENDING")) {
    return (
      <div className="flex items-center gap-3 p-4 rounded-xl bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800">
        <Loader2 className="w-5 h-5 text-blue-600 dark:text-blue-400 flex-shrink-0 animate-spin" />
        <div>
          <p className="text-sm font-semibold text-blue-800 dark:text-blue-300">Building graph…</p>
          <p className="text-xs text-blue-600 dark:text-blue-400">
            Job {jobId?.slice(0, 8)} — status: {job.status}
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {!compact && (
        <div>
          <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-100 mb-1">
            Import via service-map.yaml
          </h3>
          <p className="text-xs text-slate-500 dark:text-slate-400">
            Define your service topology in YAML. One file maps all services,
            dependencies, health endpoints, and (optionally) git repos for code awareness.
          </p>
        </div>
      )}

      {/* ── Template picker ────────────────────────────────────────────────── */}
      {!compact && (
        <div>
          <p className="text-xs font-medium text-slate-600 dark:text-slate-400 mb-2">
            Start from a template
          </p>
          <div className="grid grid-cols-2 gap-2">
            {TEMPLATES.map((tpl) => (
              <button
                key={tpl.name}
                onClick={() => selectTemplate(tpl)}
                className={`text-left p-3 rounded-lg border transition-colors ${
                  selectedTemplate === tpl.name
                    ? "border-blue-500 bg-blue-50 dark:bg-blue-900/20"
                    : "border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 hover:border-blue-300 dark:hover:border-blue-600 hover:bg-slate-50 dark:hover:bg-slate-800"
                }`}
              >
                <div className="flex items-center gap-2 mb-1">
                  <span className="text-base">{tpl.icon}</span>
                  <span className="text-xs font-semibold text-slate-800 dark:text-slate-100">
                    {tpl.name}
                  </span>
                  {tpl.services > 0 && (
                    <span className="ml-auto text-[10px] text-slate-400">
                      {tpl.services} svcs
                    </span>
                  )}
                </div>
                <p className="text-[11px] text-slate-500 dark:text-slate-400 leading-snug">
                  {tpl.description}
                </p>
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Mode toggle */}
      <div className="flex gap-1 p-1 bg-slate-100 dark:bg-slate-800 rounded-lg w-fit">
        {(["paste", "upload"] as const).map((m) => (
          <button
            key={m}
            onClick={() => setMode(m)}
            className={`px-3 py-1.5 text-xs font-medium rounded-md transition-colors capitalize ${
              mode === m
                ? "bg-white dark:bg-slate-700 text-slate-900 dark:text-white shadow-sm"
                : "text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-300"
            }`}
          >
            {m === "paste" ? "Paste YAML" : "Upload file"}
          </button>
        ))}
      </div>

      {mode === "upload" ? (
        <div
          className="border-2 border-dashed border-slate-300 dark:border-slate-600 rounded-xl p-8 text-center cursor-pointer hover:border-blue-400 dark:hover:border-blue-500 transition-colors"
          onClick={() => fileRef.current?.click()}
        >
          <Upload className="w-8 h-8 text-slate-400 mx-auto mb-2" />
          <p className="text-sm font-medium text-slate-700 dark:text-slate-300">
            {yaml ? "✓ File loaded" : "Click to upload service-map.yaml"}
          </p>
          <p className="text-xs text-slate-400 mt-1">.yaml or .yml files</p>
          <input
            ref={fileRef}
            type="file"
            accept=".yaml,.yml"
            className="hidden"
            onChange={handleFile}
          />
        </div>
      ) : (
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-xs text-slate-500">service-map.yaml</span>
            <div className="flex gap-2">
              <button
                onClick={loadExample}
                className="text-xs text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1"
              >
                <FileText className="w-3 h-3" /> Load example
              </button>
              {yaml && (
                <button
                  onClick={() => navigator.clipboard.writeText(yaml)}
                  className="text-xs text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 flex items-center gap-1"
                >
                  <Copy className="w-3 h-3" /> Copy
                </button>
              )}
            </div>
          </div>
          <textarea
            value={yaml}
            onChange={(e) => {
              setYaml(e.target.value)
              setSelectedTemplate(null)
            }}
            placeholder={`version: "1"\nprojects:\n  - id: proj-my-app\n    name: My Application\n    services:\n      - id: svc_api\n        name: api\n        type: SERVICE\n        healthUrl: http://api/health\n        calls: [svc_db]`}
            rows={compact ? 8 : 14}
            className="w-full font-mono text-xs p-3 rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900 text-slate-800 dark:text-slate-200 resize-y focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
      )}

      {error && (
        <div className="flex items-center gap-2 text-xs text-red-600 dark:text-red-400">
          <AlertCircle className="w-3.5 h-3.5 flex-shrink-0" />
          {error}
        </div>
      )}

      <div className="flex gap-3">
        <Button
          variant="primary"
          size="sm"
          onClick={handleImport}
          disabled={isLoading || !yaml.trim()}
          className="flex-1"
        >
          {isLoading ? (
            <><Loader2 className="w-3.5 h-3.5 mr-1.5 animate-spin" /> Building graph…</>
          ) : (
            "Import & Build Graph"
          )}
        </Button>
        {yaml && (
          <Button variant="ghost" size="sm" onClick={() => { setYaml(""); setSelectedTemplate(null) }}>
            Clear
          </Button>
        )}
      </div>

      {!compact && (
        <div className="pt-2 border-t border-slate-200 dark:border-slate-700">
          <p className="text-xs text-slate-400 dark:text-slate-500">
            <strong className="text-slate-500 dark:text-slate-400">Tip:</strong> Add{" "}
            <code className="bg-slate-100 dark:bg-slate-800 px-1 rounded">repo:</code> to each
            service for code-level RCA (shows which commit caused the fault).{" "}
            <a href="/docs/SERVICE_MAP.md" className="text-blue-500 hover:underline">
              Format reference →
            </a>
          </p>
        </div>
      )}
    </div>
  )
}
