import { baseApi } from "@/store/api"
import type {
  Tenant,
  CreateTenantInput,
  Project,
  ProjectListItem,
  CreateProjectInput,
  ProjectSettings,
  Graph,
  GraphVersion,
  OnboardingJob,
  StartOnboardingInput,
  Incident,
  IncidentListItem,
  IncidentFeedback,
  AnalyzeIncidentInput,
  SignalBatch,
  SignalBatchResponse,
  PlatformHealth,
  SOPPlaybook,
  SOPStep,
  CodeContext,
  AuditEvent,
  CodeIndexStatus,
  Deployment,
  CreateDeploymentInput,
  RequestPath,
  MaintenanceWindow,
} from "@/types/api"

export const faultiqApi = baseApi.injectEndpoints({
  endpoints: (build) => ({
    // ── Health ──────────────────────────────────────────────────────────
    getHealth: build.query<PlatformHealth, void>({
      query: () => "/health",
    }),

    // ── Tenants ─────────────────────────────────────────────────────────
    getTenant: build.query<Tenant, string>({
      query: (tenantId) => `/tenants/${tenantId}`,
      providesTags: (_r, _e, id) => [{ type: "Tenant", id }],
    }),
    createTenant: build.mutation<Tenant, CreateTenantInput>({
      query: (body) => ({ url: "/tenants", method: "POST", body }),
      invalidatesTags: ["Tenant"],
    }),

    // ── Projects ────────────────────────────────────────────────────────
    listProjects: build.query<{ projects: ProjectListItem[]; total: number }, string>({
      query: (tenantId) => `/tenants/${tenantId}/projects`,
      providesTags: (result) => {
        if (!result || !Array.isArray((result as any).projects)) {
          return [{ type: "Project", id: "LIST" }]
        }
        return [
          ...result.projects.map((p) => ({ type: "Project" as const, id: p.id })),
          { type: "Project", id: "LIST" },
        ]
      },
    }),
    getProject: build.query<Project, string>({
      query: (projectId) => `/projects/${projectId}`,
      providesTags: (_r, _e, id) => [{ type: "Project", id }],
    }),
    createProject: build.mutation<Project, { tenantId: string; body: CreateProjectInput }>({
      query: ({ tenantId, body }) => ({
        url: `/tenants/${tenantId}/projects`,
        method: "POST",
        body,
      }),
      invalidatesTags: [{ type: "Project", id: "LIST" }],
    }),
    updateProject: build.mutation<Project, { tenantId: string; projectId: string; body: Partial<CreateProjectInput> }>({
      query: ({ tenantId, projectId, body }) => ({
        url: `/tenants/${tenantId}/projects/${projectId}`,
        method: "PUT",
        body,
      }),
      invalidatesTags: (_r, _e, { projectId }) => [{ type: "Project", id: projectId }, { type: "Project", id: "LIST" }],
    }),
    deleteProject: build.mutation<void, { tenantId: string; projectId: string }>({
      query: ({ tenantId, projectId }) => ({
        url: `/tenants/${tenantId}/projects/${projectId}`,
        method: "DELETE",
      }),
      invalidatesTags: [{ type: "Project", id: "LIST" }],
    }),

    // ── Onboarding ──────────────────────────────────────────────────────
    startOnboarding: build.mutation<OnboardingJob, { projectId: string; body: StartOnboardingInput }>({
      query: ({ projectId, body }) => ({
        url: `/projects/${projectId}/onboarding/import`,
        method: "POST",
        body,
      }),
    }),
    getOnboardingJob: build.query<OnboardingJob, { projectId: string; jobId: string }>({
      query: ({ projectId, jobId }) =>
        `/projects/${projectId}/onboarding/jobs/${jobId}`,
    }),

    // ── Graph ───────────────────────────────────────────────────────────
    getCurrentGraph: build.query<Graph, { projectId: string; environment: string }>({
      query: ({ projectId, environment }) =>
        `/projects/${projectId}/graphs/current?environment=${environment}`,
      providesTags: (_r, _e, { projectId }) => [{ type: "Graph", id: projectId }],
    }),
    listGraphVersions: build.query<{ versions: GraphVersion[] }, { projectId: string; environment: string }>({
      query: ({ projectId, environment }) =>
        `/projects/${projectId}/graphs/versions?environment=${environment}`,
    }),
    publishGraph: build.mutation<GraphVersion, { projectId: string; environment: string; notes?: string }>({
      query: ({ projectId, ...body }) => ({
        url: `/projects/${projectId}/graphs/publish`,
        method: "POST",
        body,
      }),
      invalidatesTags: (_r, _e, { projectId }) => [{ type: "Graph", id: projectId }],
    }),

    // ── Incidents ───────────────────────────────────────────────────────
    listIncidents: build.query<
      { incidents: IncidentListItem[]; total: number; hasMore?: boolean; nextCursor?: string },
      { projectId: string; environment?: string; status?: string; limit?: number; offset?: number; q?: string; cursor?: string }
    >({
      query: ({ projectId, ...params }) => ({
        url: `/projects/${projectId}/incidents`,
        params,
      }),
      providesTags: (result) => {
        if (!result || !Array.isArray((result as any).incidents)) {
          return [{ type: "Incident", id: "LIST" }]
        }
        return [
          ...result.incidents.map((i) => ({ type: "Incident" as const, id: i.id })),
          { type: "Incident", id: "LIST" },
        ]
      },
    }),
    getIncident: build.query<Incident, string>({
      query: (incidentId) => `/incidents/${incidentId}`,
      providesTags: (_r, _e, id) => [{ type: "Incident", id }],
    }),
    analyzeIncident: build.mutation<Incident, { projectId: string; body: AnalyzeIncidentInput }>({
      query: ({ projectId, body }) => ({
        url: `/projects/${projectId}/incidents/analyze`,
        method: "POST",
        body,
      }),
      invalidatesTags: [{ type: "Incident", id: "LIST" }],
    }),
    submitFeedback: build.mutation<{ incidentId: string; status: string; feedbackRecorded: boolean }, { incidentId: string; body: IncidentFeedback }>({
      query: ({ incidentId, body }) => ({
        url: `/incidents/${incidentId}/feedback`,
        method: "POST",
        body,
      }),
      invalidatesTags: (_r, _e, { incidentId }) => [{ type: "Incident", id: incidentId }],
    }),
    updateIncidentStatus: build.mutation<Incident, { incidentId: string; status: string }>({
      query: ({ incidentId, status }) => ({
        url: `/incidents/${incidentId}/status`,
        method: "PUT",
        body: { status },
      }),
      invalidatesTags: (_r, _e, { incidentId }) => [
        { type: "Incident", id: incidentId },
        { type: "Incident", id: "LIST" },
      ],
    }),

    // ── Signals ─────────────────────────────────────────────────────────
    submitSignals: build.mutation<SignalBatchResponse, SignalBatch>({
      query: (body) => ({ url: "/signals", method: "POST", body }),
    }),

    // ── Project Settings (repo / code-awareness config) ─────────────────
    getProjectSettings: build.query<ProjectSettings, string>({
      query: (projectId) => `/projects/${projectId}/settings`,
      providesTags: (_r, _e, id) => [{ type: "Project", id: `settings-${id}` }],
    }),
    updateProjectSettings: build.mutation<ProjectSettings, { projectId: string; body: Partial<ProjectSettings> }>({
      query: ({ projectId, body }) => ({
        url: `/projects/${projectId}/settings`,
        method: "PUT",
        body,
      }),
      invalidatesTags: (_r, _e, { projectId }) => [
        { type: "Project", id: `settings-${projectId}` },
        { type: "Project", id: projectId },
      ],
    }),
    // Import service-map YAML — triggers onboarding worker
    importServiceMap: build.mutation<OnboardingJob, { projectId: string; yaml: string }>({
      query: ({ projectId, yaml }) => ({
        url: `/projects/${projectId}/onboarding/import`,
        method: "POST",
        body: { mode: "service-map", sources: { serviceMapYaml: yaml } },
      }),
    }),
    // Code context for a specific service (recent commits from code-indexer)
    getCodeContext: build.query<CodeContext, { projectId: string; serviceId: string }>({
      query: ({ projectId, serviceId }) =>
        `/projects/${projectId}/services/${serviceId}/code-context`,
    }),

    // ── SOP ─────────────────────────────────────────────────────────────
    getSOPPlaybook: build.query<SOPPlaybook, string>({
      query: (incidentId) => `/incidents/${incidentId}/sop`,
      providesTags: (_r, _e, id) => [{ type: "Incident", id: `sop-${id}` }],
    }),
    updateSOPStep: build.mutation<SOPPlaybook, { incidentId: string; stepOrder: number; status: "DONE" | "FAILED" }>({
      query: ({ incidentId, stepOrder, status }) => ({
        url: `/incidents/${incidentId}/sop/steps/${stepOrder}`,
        method: "PUT",
        body: { status },
      }),
      invalidatesTags: (_r, _e, { incidentId }) => [
        { type: "Incident", id: `sop-${incidentId}` },
        { type: "Incident", id: incidentId },
      ],
    }),
    getIncidentPhase: build.query<{ incidentId: string; phase: string; status: string; detectedAt: string; updatedAt: string }, string>({
      query: (incidentId) => `/incidents/${incidentId}/phase`,
      providesTags: (_r, _e, id) => [{ type: "Incident", id: `phase-${id}` }],
    }),

    // ── Tenant-level incidents ────────────────────────────────────────────
    listTenantIncidents: build.query<
      { incidents: IncidentListItem[]; total: number },
      { tenantId: string; status?: string; limit?: number }
    >({
      query: ({ tenantId, ...params }) => ({
        url: `/tenants/${tenantId}/incidents`,
        params,
      }),
      providesTags: [{ type: "Incident", id: "TENANT_LIST" }],
    }),

    // ── Audit Trail ──────────────────────────────────────────────────────────
    getIncidentAudit: build.query<{ events: AuditEvent[] }, string>({
      query: (incidentId) => `/incidents/${incidentId}/audit`,
      providesTags: (_r, _e, id) => [{ type: "Incident", id: `audit-${id}` }],
    }),

    // ── Code Index Status ─────────────────────────────────────────────────────
    getCodeIndexStatus: build.query<{ services: CodeIndexStatus[] }, string>({
      query: (projectId) => `/projects/${projectId}/code-index/status`,
      providesTags: (_r, _e, id) => [{ type: "Project", id: `ci-status-${id}` }],
    }),

    // ── Deployments ───────────────────────────────────────────────────────────
    listDeployments: build.query<{ deployments: Deployment[]; total: number }, { projectId: string; serviceId?: string }>({
      query: ({ projectId, serviceId }) => ({
        url: `/deployments`,
        params: { projectId, ...(serviceId ? { serviceId } : {}) },
      }),
      providesTags: [{ type: "Project", id: "DEPLOYMENTS" }],
    }),
    createDeployment: build.mutation<{ deploymentId: string; healthGateUntil: string; message: string }, CreateDeploymentInput>({
      query: (body) => ({ url: `/deployments`, method: "POST", body }),
      invalidatesTags: [{ type: "Project", id: "DEPLOYMENTS" }],
    }),
    updateDeployment: build.mutation<{ id: string; status: string }, { deploymentId: string; status: string; healthStatus?: string }>({
      query: ({ deploymentId, ...body }) => ({
        url: `/deployments/${deploymentId}`,
        method: "PUT",
        body,
      }),
      invalidatesTags: [{ type: "Project", id: "DEPLOYMENTS" }],
    }),

    // ── Request Path ──────────────────────────────────────────────────────────
    getRequestPath: build.query<RequestPath, { projectId: string; serviceId: string }>({
      query: ({ projectId, serviceId }) =>
        `/projects/${projectId}/services/${serviceId}/request-path`,
    }),

    // ── Service Deployments (project-scoped) ──────────────────────────────────
    listServiceDeployments: build.query<{ deployments: Deployment[] }, { projectId: string; serviceId: string }>({
      query: ({ projectId, serviceId }) =>
        `/projects/${projectId}/services/${serviceId}/deployments`,
      providesTags: [{ type: "Project", id: "DEPLOYMENTS" }],
    }),

    // ── Playbook Learning ─────────────────────────────────────────────────────
    getPlaybookLearning: build.query<
      { steps: { stepOrder: number; successRate: number; totalCount: number }[] },
      { projectId: string; serviceId: string; faultType: string }
    >({
      query: ({ projectId, serviceId, faultType }) =>
        `/projects/${projectId}/services/${serviceId}/playbook-learning?faultType=${faultType}`,
    }),

    // ── SOP Playbook Editor ───────────────────────────────────────────────────
    updateSOPPlaybook: build.mutation<{ steps: SOPStep[] }, { projectId: string; serviceId: string; steps: SOPStep[] }>({
      query: ({ projectId, serviceId, steps }) => ({
        url: `/projects/${projectId}/services/${serviceId}/sop-playbook`,
        method: "PUT",
        body: { steps },
      }),
    }),
    getSOPPlaybookHistory: build.query<
      { versions: { version: number; steps: SOPStep[]; createdAt: string; createdBy: string }[] },
      { projectId: string; serviceId: string }
    >({
      query: ({ projectId, serviceId }) =>
        `/projects/${projectId}/services/${serviceId}/sop-playbook/history`,
    }),
    revertSOPPlaybook: build.mutation<{ steps: SOPStep[] }, { projectId: string; serviceId: string; version: number }>({
      query: ({ projectId, serviceId, version }) => ({
        url: `/projects/${projectId}/services/${serviceId}/sop-playbook/revert?version=${version}`,
        method: "POST",
      }),
    }),

    // ── Graph Diff ────────────────────────────────────────────────────────────
    takeGraphSnapshot: build.mutation<{ snapshotAt: string }, string>({
      query: (projectId) => ({ url: `/projects/${projectId}/graphs/snapshot`, method: "POST" }),
      invalidatesTags: (_r, _e, projectId) => [{ type: "Graph", id: `diff-${projectId}` }],
    }),
    getGraphDiff: build.query<
      {
        added: { id: string; name: string; type: string }[]
        removed: { id: string; name: string }[]
        changed: { id: string; name: string; changes: string[] }[]
        addedEdges: { from: string; to: string }[]
        removedEdges: { from: string; to: string }[]
        snapshotAge: string
      },
      string
    >({
      query: (projectId) => `/projects/${projectId}/graphs/diff`,
      providesTags: (_r, _e, id) => [{ type: "Graph", id: `diff-${id}` }],
    }),

    // ── Signal History ────────────────────────────────────────────────────────
    getSignalHistory: build.query<
      {
        errorRateTimeline: { time: string; errorRate: number }[]
        avgLatency: number
        faultCount: number
        uptime: number
      },
      { projectId: string; serviceId: string; hours?: number }
    >({
      query: ({ projectId, serviceId, hours = 24 }) =>
        `/projects/${projectId}/services/${serviceId}/signal-history?hours=${hours}`,
    }),

    // ── Batch Signal History ──────────────────────────────────────────────────
    getSignalHistoryBatch: build.query<
      Record<string, {
        errorRateTimeline: { time: string; errorRate: number }[]
        avgLatency: number
        faultCount: number
        uptime: number
        sloTarget?: number
        sloStatus?: string
        p95LatencyMs?: number
      }>,
      { projectId: string; services: string[]; hours?: number }
    >({
      query: ({ projectId, services, hours = 1 }) =>
        `/projects/${projectId}/signal-history/batch?services=${services.join(",")}&hours=${hours}`,
    }),

    // ── Service SLO ───────────────────────────────────────────────────────────
    updateServiceSLO: build.mutation<
      void,
      {
        projectId: string
        serviceId: string
        env?: string
        body: { errorRateThreshold: number; latencyThresholdMs: number; minSignalCount: number }
      }
    >({
      query: ({ projectId, serviceId, env, body }) => ({
        url: `/projects/${projectId}/services/${serviceId}/slo${env ? `?env=${env}` : ""}`,
        method: "PUT",
        body,
      }),
    }),

    // ── Tenant Members ────────────────────────────────────────────────────────
    listTenantMembers: build.query<{ members: { email: string; role: string; joinedAt: string }[] }, string>({
      query: (tenantId) => `/tenants/${tenantId}/members`,
    }),

    // ── Related Incidents ─────────────────────────────────────────────────────
    getRelatedIncidents: build.query<{ incidents: { id: string; service: string; status: string; phase?: string; confidence: number }[] }, string>({
      query: (incidentId) => `/incidents/${incidentId}/related`,
      providesTags: (_r, _e, id) => [{ type: "Incident", id: `related-${id}` }],
    }),

    // ── Maintenance Windows ───────────────────────────────────────────────────
    createMaintenance: build.mutation<MaintenanceWindow, { projectId: string; serviceId?: string; durationMinutes: number; reason?: string }>({
      query: ({ projectId, ...body }) => ({ url: `/projects/${projectId}/maintenance`, method: "POST", body }),
    }),
    endMaintenance: build.mutation<void, string>({
      query: (projectId) => ({ url: `/projects/${projectId}/maintenance`, method: "DELETE" }),
    }),

    // ── Incident Assignment ───────────────────────────────────────────────────
    assignIncident: build.mutation<void, { incidentId: string; assignTo: string | null }>({
      query: ({ incidentId, assignTo }) => ({
        url: `/incidents/${incidentId}/assign`,
        method: "PATCH",
        body: { assignTo },
      }),
      invalidatesTags: (_r, _e, { incidentId }) => [{ type: "Incident", id: incidentId }, { type: "Incident", id: "LIST" }],
    }),

    // ── RCA Feedback ─────────────────────────────────────────────────────────
    submitRCAFeedback: build.mutation<void, { incidentId: string; wasCorrect: boolean; actualRoot?: string; note?: string }>({
      query: ({ incidentId, ...body }) => ({
        url: `/incidents/${incidentId}/rca-feedback`,
        method: "POST",
        body,
      }),
      invalidatesTags: (_r, _e, { incidentId }) => [{ type: "Incident", id: incidentId }],
    }),
  }),
  overrideExisting: false,
})

export const {
  useGetHealthQuery,
  useGetTenantQuery,
  useCreateTenantMutation,
  useListProjectsQuery,
  useGetProjectQuery,
  useCreateProjectMutation,
  useUpdateProjectMutation,
  useDeleteProjectMutation,
  useStartOnboardingMutation,
  useGetOnboardingJobQuery,
  useGetCurrentGraphQuery,
  useListGraphVersionsQuery,
  usePublishGraphMutation,
  useListIncidentsQuery,
  useGetIncidentQuery,
  useAnalyzeIncidentMutation,
  useSubmitFeedbackMutation,
  useUpdateIncidentStatusMutation,
  useSubmitSignalsMutation,
  useGetProjectSettingsQuery,
  useUpdateProjectSettingsMutation,
  useImportServiceMapMutation,
  useGetCodeContextQuery,
  useGetSOPPlaybookQuery,
  useUpdateSOPStepMutation,
  useGetIncidentPhaseQuery,
  useListTenantIncidentsQuery,
  useGetIncidentAuditQuery,
  useGetCodeIndexStatusQuery,
  useListDeploymentsQuery,
  useCreateDeploymentMutation,
  useUpdateDeploymentMutation,
  useGetRequestPathQuery,
  useListServiceDeploymentsQuery,
  useGetPlaybookLearningQuery,
  useUpdateSOPPlaybookMutation,
  useGetSOPPlaybookHistoryQuery,
  useRevertSOPPlaybookMutation,
  useTakeGraphSnapshotMutation,
  useGetGraphDiffQuery,
  useGetSignalHistoryQuery,
  useGetSignalHistoryBatchQuery,
  useUpdateServiceSLOMutation,
  useListTenantMembersQuery,
  useGetRelatedIncidentsQuery,
  useCreateMaintenanceMutation,
  useEndMaintenanceMutation,
  useAssignIncidentMutation,
  useSubmitRCAFeedbackMutation,
} = faultiqApi
