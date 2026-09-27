#!/usr/bin/env node
/**
 * FaultIQ Automated Demo Capture Script
 *
 * Captures the full fault lifecycle and writes the README screenshots + GIF:
 *   1. Log in via Keycloak SSO
 *   2. Dashboard with the live service graph
 *   3. Demo page showing baseline health
 *   4. Inject a fault and watch detection open an incident
 *   5. Incident detail with ranked RCA candidates + SOP playbook
 *   6. Heal the service and watch the incident auto-resolve
 *   7. Encode the recording to docs/demo.gif
 *
 * Rather than sleeping for fixed durations, the script waits for the
 * detection-engine to actually advance the incident through its phases, so a
 * slow poll interval cannot produce a screenshot of an empty state.
 *
 * Usage: npm install playwright && npx playwright install chromium
 *        node scripts/capture-demo.js
 */

const { chromium } = require('playwright');
const fs = require('fs');
const path = require('path');
const os = require('os');
const { execSync } = require('child_process');

const SCREENSHOT_DIR = 'docs/screenshots';
const DEMO_GIF = 'docs/demo.gif';
const VIDEO_DIR = fs.mkdtempSync(path.join(os.tmpdir(), 'faultiq-capture-'));

// Keycloak login credentials (documented on the login page)
const KC_USERNAME = 'super';
const KC_PASSWORD = 'superpass';

const UI = 'http://localhost:4001';
const DEMO_API = 'http://localhost:8091';
const FAULTY_SERVICE = 'ledger-service';

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/** Query the incident table directly so waiting is deterministic. */
function sql(query) {
  return execSync(
    `docker compose exec -T postgres psql -U postgres -d faultiq -tAc ${JSON.stringify(query)}`,
    { cwd: path.resolve(__dirname, '..'), encoding: 'utf8' }
  ).trim();
}

/** Set a demo service's simulated health from the host. */
function setServiceHealth(name, body) {
  return fetch(`${DEMO_API}/admin/services/${name}/status`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}

/**
 * Poll until `predicate` holds, or throw once `timeoutMs` elapses.
 * Detection runs on a ~30s poll interval, so the default budget has to cover
 * several poll cycles plus time for RCA scoring to cross the threshold.
 */
async function waitFor(description, predicate, timeoutMs = 240000, intervalMs = 3000) {
  const deadline = Date.now() + timeoutMs;
  let last = null;
  while (Date.now() < deadline) {
    try {
      last = await predicate();
      if (last) {
        console.log(`  ✓ ${description}`);
        return last;
      }
    } catch (e) {
      last = e.message;
    }
    await sleep(intervalMs);
  }
  throw new Error(`Timed out after ${timeoutMs / 1000}s waiting for: ${description} (last: ${last})`);
}

async function capture() {
  console.log('=== FaultIQ Automated Demo Capture ===\n');

  fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });

  console.log('[1/7] Checking that the stack is running...');
  try {
    const res = await fetch(UI);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const health = await fetch(`${DEMO_API}/health`);
    if (!health.ok) throw new Error(`HTTP ${health.status}`);
    console.log('  Frontend and demo-services are ready!\n');
  } catch (e) {
    console.error('  ERROR: stack not running — start it with: docker compose up -d');
    console.error(`  (${e.message})`);
    process.exit(1);
  }

  // Start from a clean, healthy state so the capture is reproducible.
  console.log('  Resetting demo services to healthy defaults...');
  await fetch(`${DEMO_API}/admin/reset`, { method: 'POST' });
  await sleep(2000);

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    recordVideo: { dir: VIDEO_DIR, size: { width: 1440, height: 900 } },
  });
  const page = await context.newPage();

  try {
    // ── 1. Log in ──────────────────────────────────────────────────────────
    console.log('[2/7] Logging in via Keycloak SSO...');
    await page.goto(`${UI}/login`, { waitUntil: 'networkidle' });
    await page.click('button:has-text("Sign in with Single Sign-On")');
    await page.waitForURL('**/realms/**', { timeout: 20000 });
    await page.fill('input[name="username"]', KC_USERNAME);
    await page.fill('input[name="password"]', KC_PASSWORD);
    await page.click('input[type="submit"], button[type="submit"]');
    await page.waitForURL('**/dashboard**', { timeout: 30000 });
    await sleep(2500);
    console.log('  Logged in successfully!\n');

    // ── 2. Dashboard + service graph ───────────────────────────────────────
    console.log('[3/7] Capturing dashboard with live service graph...');
    await page.goto(`${UI}/dashboard`, { waitUntil: 'networkidle' });
    await sleep(4000);
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'dashboard-graph.png') });
    console.log('  → dashboard-graph.png\n');

    // ── 3. Demo page baseline (all healthy) ────────────────────────────────
    console.log('[4/7] Capturing demo page baseline health...');
    await page.goto(`${UI}/dashboard/demo`, { waitUntil: 'networkidle' });
    await sleep(4000);
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'demo-page.png') });
    console.log('  → demo-page.png\n');

    // ── 4. Inject fault and wait for the incident to reach TRIAGING ────────
    console.log('[5/7] Injecting fault and waiting for detection + RCA...');
    const faultedAt = new Date().toISOString();
    await setServiceHealth(FAULTY_SERVICE, {
      statusClass: '5xx',
      errorRate: 0.85,
      latencyP95: 5000,
    });
    console.log(`  Injected 5xx into ${FAULTY_SERVICE} (errorRate=0.85)`);

    // Watch the demo page show detection happening. The page only re-reads
    // health when the active step changes, so hit Refresh to pull the fault in.
    await sleep(4000);
    await page.getByRole('button', { name: /Refresh/i }).first().click();
    await sleep(2500);
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'fault-detected.png') });
    console.log('  → fault-detected.png');

    // Wait for the detection-engine to confirm a root cause and build the SOP.
    // TRIAGING is the phase that means "RCA ranked + playbook generated".
    const incidentId = await waitFor(
      'incident reached TRIAGING (RCA ranked + SOP generated)',
      () => {
        const row = sql(
          `SELECT id FROM incidents WHERE detected_at > '${faultedAt}' AND phase = 'TRIAGING' ORDER BY detected_at DESC LIMIT 1`
        );
        return row || null;
      }
    );
    console.log(`  Incident ${incidentId} is in TRIAGING`);

    // ── 5. Incident detail: RCA candidates + SOP playbook ─────────────────
    console.log('[6/7] Capturing RCA candidates and SOP playbook...');
    await page.goto(`${UI}/dashboard/incidents/${incidentId}`, { waitUntil: 'networkidle' });
    await sleep(5000);
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'incident-sop.png') });

    // The RCA ranking and SOP playbook sit below the fold, inside a scrolling
    // container, so a full-page capture would not include them. Scroll the SOP
    // section into view and shoot that instead.
    const sopHeading = page.getByRole('button', { name: /SOP Playbook/i }).first();
    await sopHeading.scrollIntoViewIfNeeded();
    await sleep(1500);
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'sop-playbook.png') });
    console.log('  → incident-sop.png + sop-playbook.png\n');

    // ── 6. Heal the service and wait for auto-resolution ───────────────────
    console.log('[7/7] Healing service and waiting for auto-resolution...');
    await setServiceHealth(FAULTY_SERVICE, {
      statusClass: '2xx',
      errorRate: 0.002,
      latencyP95: 45,
    });
    console.log(`  Restored ${FAULTY_SERVICE} to 2xx`);

    await waitFor(
      'incident auto-resolved',
      () => {
        const row = sql(
          `SELECT status FROM incidents WHERE id = '${incidentId}' AND status = 'RESOLVED'`
        );
        return row || null;
      }
    );

    await page.reload({ waitUntil: 'networkidle' });
    await sleep(4000);
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'resolved.png') });
    console.log('  → resolved.png\n');
  } catch (error) {
    console.error('\n  ERROR during capture:', error.message);
    process.exitCode = 1;
  } finally {
    // Closing the context flushes the video file to disk.
    await context.close();
    await browser.close();
  }

  // ── Encode the recording to GIF ──────────────────────────────────────────
  try {
    const videos = fs.readdirSync(VIDEO_DIR).filter((f) => f.endsWith('.webm'));
    if (videos.length === 0) throw new Error('no video recorded');
    const videoPath = path.join(VIDEO_DIR, videos[0]);
    execSync(
      `ffmpeg -y -i "${videoPath}" -vf "fps=10,scale=900:-1:flags=lanczos" -loop 0 "${DEMO_GIF}"`,
      { stdio: 'ignore' }
    );
    console.log(`  Demo GIF created: ${DEMO_GIF}`);
  } catch (e) {
    console.log(`  WARNING: could not create GIF (${e.message})`);
    process.exitCode = 1;
  } finally {
    fs.rmSync(VIDEO_DIR, { recursive: true, force: true });
  }

  console.log('\n=== Capture Complete ===');
  console.log(`Screenshots: ${SCREENSHOT_DIR}/`);
  console.log(`Demo GIF:    ${DEMO_GIF}`);
}

capture().catch((e) => {
  console.error(e);
  process.exit(1);
});
