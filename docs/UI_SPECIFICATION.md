# FaultIQ — UI Specification & Screen Reference

## Design System

- **Framework**: Next.js 16 + React 19 + TypeScript
- **Styling**: Tailwind CSS v4, class-based dark mode (`class="dark"` on `<html>`)
- **Animations**: Framer Motion (page transitions, card entrances)
- **Charts**: Recharts (bar, pie, line charts)
- **Graph**: Cytoscape.js (interactive node-edge visualization)
- **Icons**: Lucide React
- **Theme**: Dark by default, toggle via Settings or header button

---

## Page Map

### Public Pages (No Auth Required)

| Page | Route | Purpose |
|------|-------|---------|
| Landing | `/` | Hero, feature grid, architecture diagram, CTA |
| About | `/about` | Mission, values, community |
| Contact | `/contact` | Contact form + info |
| Services Status | `/services-status` | Live service health (6 backend services) |
| Login | `/login` | Keycloak SSO redirect |

### Dashboard Pages (Auth Required)

| Page | Route | Purpose |
|------|-------|---------|
| Overview | `/dashboard` | KPI cards, project list, recent incidents |
| Projects | `/dashboard/projects` | CRUD projects, environment management |
| New Project | `/dashboard/onboarding` | 4-step wizard: create → sources → import → done |
| Service Graph | `/dashboard/graph` | Interactive graph visualization with node detail panel |
| Incidents | `/dashboard/incidents` | Filtered incident list with status chips |
| Incident Detail | `/dashboard/incidents/[id]` | RCA candidates, recommendations, feedback form |
| Signals | `/dashboard/signals` | Inject error signals per service |
| Analytics | `/dashboard/reports` | Charts: daily trends, status pie, confidence histogram |
| Settings | `/dashboard/settings` | Profile, notifications, theme, security |

---

## Screen Specifications

### 1. Dashboard Overview (`/dashboard`)

**Layout**: 4-column KPI cards + 2-column grid (projects, incidents)

**KPI Cards** (top row):
- Projects count (blue icon)
- Open Incidents (red if >0, green if 0)
- Resolved Incidents (green)
- Published Graphs (violet)

**Left Column**: Project list (max 5, links to `/dashboard/projects`)
**Right Column**: Recent incidents (max 5, status dots, links to detail)

**Data Source**: `useListProjectsQuery`, `useListIncidentsQuery`

---

### 2. Service Graph (`/dashboard/graph`)

**Layout**: Full-width graph canvas with floating panels

**Components**:
- **Project/Environment selector** (top bar)
- **Stats bar**: node count, edge count, namespace, published date
- **Cytoscape graph** (main area): force-directed layout with colored nodes
- **Real-time indicator**: "Live" (green) or "Polling" (gray) badge
- **Node Detail Panel** (right overlay, appears on node click):
  - Health status badge (Healthy/Degraded/Unhealthy)
  - Metrics: status class, error rate %, P95 latency
  - Fix recommendations (fault-type-specific steps)
  - Active incidents linked to this node
  - Tags

**Node Colors**:
- Green border: healthy (2xx, errorRate < 10%)
- Yellow border: degraded (acknowledged incident OR errorRate 10-50%)
- Red border + pulse: unhealthy (open incident OR errorRate > 50% OR non-2xx status)

**Edge Colors**:
- Green: success ratio > 99%
- Yellow: 95-99%
- Red: < 95%

**Real-time Updates**: SSE via `useIncidentEvents` hook → instant refetch on incident_created/resolved

---

### 3. Incidents List (`/dashboard/incidents`)

**Layout**: Filter bar + scrollable card list

**Filters**: Project selector, status chips (All, OPEN, ACKNOWLEDGED, RESOLVED)

**Incident Card**:
- Status badge (red/yellow/green)
- Detected timestamp
- Service name (with Target icon)
- Root cause candidate (orange text)
- Confidence bar (colored by percentage)
- Evidence count

**Polling**: 5s interval for live updates

---

### 4. Incident Detail (`/dashboard/incidents/[id]`)

**Layout**: 2-column grid on desktop

**Left Column**: RCA Candidates
- Ranked list with confidence bars
- Fault type label
- Impacted callers list
- Blast radius badges

**Right Column**: Recommendations
- Ranked playbook cards
- Category badge
- Step-by-step instructions
- Reason explanation

**Header**: Status badge, detected time, environment, graph version
**Actions**: Submit Feedback, Mark Resolved

**Trigger Signals Section**: Shows what signals triggered this incident

---

### 5. Projects (`/dashboard/projects`)

**Layout**: Explainer cards (top) + project grid

**Explainer Row** (3 cards):
- Dependency Graph: what it is
- Signal Injection: how to test
- RCA Analysis: what it produces

**Project Card**:
- Name + slug
- Graph status badge (PUBLISHED/DRAFT)
- Incident count badge
- Stats grid: open incidents, graph status, active status
- Action buttons: View Graph, Inject Signal, Incidents

**Modals**: New Project, Edit Project, Delete Confirmation

---

### 6. Onboarding Wizard (`/dashboard/onboarding`)

**Layout**: Step indicator + content card

**Steps**:
1. **Create Project**: name, slug, domain, environments, services (optional)
2. **Configure Sources**: environment, mode, OpenAPI/k8s/traces checkboxes
3. **Import Monitor**: Real-time job progress (connectors → catalog → graph → validation)
4. **Done**: Success screen with links to Graph and Signals

**Key Feature**: If services are provided in step 1, graph auto-builds and skips to step 4.

---

### 7. Signals (`/dashboard/signals`)

**Layout**: Project/environment selector + signal cards grid

**Signal Card** (per service in the graph):
- Service name + type badge
- Metrics preview: status class, error rate, latency
- "Send Signal" button (red, with success animation)
- "Copy as JSON" button

**Empty State**: If no services in graph, directs to Service Graph page

---

### 8. Analytics (`/dashboard/reports`)

**Layout**: KPI row + 2x2 chart grid

**KPI Cards**: Total incidents, Open, Resolved, Avg Confidence
**Charts**:
- Daily Incidents (bar chart, last 14 days)
- Status Distribution (pie chart: OPEN/ACK/RESOLVED)
- RCA Confidence Histogram (bar chart: 0-20%, 21-40%, ..., 81-100%)

---

### 9. Settings (`/dashboard/settings`)

**Layout**: Stacked cards

**Sections**:
- **Profile**: First/last name, email (read-only from Keycloak)
- **Notifications**: Email alerts, weekly report, incident notifications (toggles)
- **Security**: Link to Keycloak account management
- **Appearance**: 3-button theme selector (Light/Dark/System) — uses `next-themes`

---

## Interaction Patterns

### Graph Node Click Flow
1. User clicks a node → `onNodeSelect(nodeId)` fires
2. `NodeDetailPanel` appears (absolute positioned, right side)
3. Shows health status, metrics, fix recommendations, linked incidents
4. Click outside or X button → panel closes

### Signal → Incident → Graph Update Flow
1. User sends signal from `/dashboard/signals`
2. Signal hits API Gateway → Redis pub/sub → Detection Engine
3. Detection engine publishes `incident_created` to Redis
4. SSE hub broadcasts to all connected clients
5. `useIncidentEvents` hook receives event → triggers `refetch()`
6. Graph node turns red, incident appears in list
7. If user is on graph page with node panel open → panel updates

### Auto-Recovery Flow
1. Service sends healthy signal (2xx, low error rate)
2. Detection engine detects recovery → publishes `incident_resolved`
3. Frontend receives event → node turns green
4. Incident moves to RESOLVED status

---

## Accessibility

- All interactive elements have `aria-label` or visible text
- Focus rings on all buttons/inputs (`focus-visible:ring-2`)
- Color is never the only indicator (status text + icons accompany colors)
- Keyboard navigable: Tab through all controls
- Reduced motion: Framer Motion respects `prefers-reduced-motion`

---

## Performance

- **Code splitting**: Next.js automatic per-page bundles
- **Lazy loading**: Cytoscape loaded via `dynamic(import(...), { ssr: false })`
- **Polling**: RTK Query with `pollingInterval` (3-5s for incidents, 120s for graph)
- **SSE**: Single EventSource connection, shared across components
- **Graph cache**: Stale-while-revalidate pattern in detection engine
