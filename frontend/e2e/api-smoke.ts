/**
 * API Smoke Test Script
 * Usage: npx tsx e2e/api-smoke.ts [base_url] [token]
 *
 * Tests all 16 API endpoints and writes results to playwright-report/api-smoke.json
 * Requires the backend to be running.
 *
 * Design notes
 * ────────────
 * • Register runs BEFORE login so we can login with the same dynamic credentials.
 * • Probe calls (nil-UUID routes, intentionally invalid payloads) use `acceptedStatuses`
 *   to declare the expected non-2xx response.  A 404 on a nil-UUID post is correct
 *   API behaviour, not a failure.
 */

import https from 'https'
import http from 'http'
import fs from 'fs'
import path from 'path'

const BASE_URL = process.argv[2] || 'http://localhost:8080'
const TOKEN = process.argv[3] || process.env.API_TOKEN || ''

interface SmokeResult {
  method: string
  endpoint: string
  status: number | null
  ok: boolean
  error: string | null
  responseExcerpt: string
  durationMs: number
}

const results: SmokeResult[] = []

function request(
  method: string,
  url: string,
  body?: unknown,
  contentType = 'application/json'
): Promise<{ status: number; body: string }> {
  return new Promise((resolve, reject) => {
    const parsedUrl = new URL(url)
    const isHttps = parsedUrl.protocol === 'https:'
    const options: http.RequestOptions = {
      hostname: parsedUrl.hostname,
      port: parsedUrl.port || (isHttps ? 443 : 80),
      path: parsedUrl.pathname + parsedUrl.search,
      method,
      headers: {
        Authorization: `Bearer ${TOKEN}`,
        'Content-Type': contentType,
      },
    }

    const req = (isHttps ? https : http).request(options, (res) => {
      let data = ''
      res.on('data', (chunk) => { data += chunk })
      res.on('end', () => resolve({ status: res.statusCode ?? 0, body: data }))
    })

    req.on('error', reject)

    if (body) {
      if (typeof body === 'string') {
        req.write(body)
      } else {
        req.write(JSON.stringify(body))
      }
    }
    req.end()
  })
}

function excerpt(body: string, maxLen = 200): string {
  const trimmed = body.replace(/\s+/g, ' ').trim()
  return trimmed.length > maxLen ? trimmed.slice(0, maxLen) + '…' : trimmed
}

/**
 * Run a single smoke probe.
 *
 * @param acceptedStatuses  Extra status codes counted as "ok" beyond 2xx.
 *                          Use for probe calls that intentionally target non-2xx
 *                          responses (e.g. a nil UUID → 404, no-file upload → 400).
 */
async function test(
  method: string,
  endpoint: string,
  body?: unknown,
  label?: string,
  acceptedStatuses?: number[]
): Promise<void> {
  const displayLabel = label || `${method} ${endpoint}`
  const url = `${BASE_URL}${endpoint}`
  const start = Date.now()
  let result: SmokeResult

  try {
    const res = await request(method, url, body)
    const durationMs = Date.now() - start
    const ok =
      (res.status >= 200 && res.status < 300) ||
      (acceptedStatuses?.includes(res.status) ?? false)
    const icon = ok ? '✓' : '✗'
    console.log(`  ${icon} [${res.status}] ${displayLabel} (${durationMs}ms)`)
    result = {
      method,
      endpoint,
      status: res.status,
      ok,
      error: ok ? null : `HTTP ${res.status}`,
      responseExcerpt: excerpt(res.body),
      durationMs,
    }
  } catch (err: unknown) {
    const durationMs = Date.now() - start
    const msg = (err as Error).message || String(err)
    console.log(`  ✗ [ERR] ${displayLabel} — ${msg}`)
    result = {
      method,
      endpoint,
      status: null,
      ok: false,
      error: msg,
      responseExcerpt: '',
      durationMs,
    }
  }

  results.push(result)
}

async function main() {
  console.log(`\n╔════════════════════════════════════════════╗`)
  console.log(`║     API Smoke Test — TikTok Affiliate      ║`)
  console.log(`╚════════════════════════════════════════════╝`)
  console.log(`  Backend URL : ${BASE_URL}`)
  console.log(`  Token set   : ${TOKEN ? 'YES' : 'NO (unauthenticated — most endpoints will 401)'}`)
  console.log()

  // Dynamic credentials shared between register and login tests.
  const smokeEmail = `smoke_${Date.now()}@test.com`
  const smokePassword = 'password123'

  // ── Auth ──────────────────────────────────────────────────────────────
  console.log('[ Auth ]')
  // Register first so login can use the same credentials.
  await test('POST', '/api/auth/register', {
    email: smokeEmail,
    password: smokePassword,
    display_name: 'Smoke Test',
  })
  // Login with the credentials we just registered.
  await test('POST', '/api/auth/login', { email: smokeEmail, password: smokePassword })

  // ── Dashboard ─────────────────────────────────────────────────────────
  console.log('\n[ Dashboard ]')
  await test('GET', '/api/dashboard/stats')
  await test('GET', '/api/dashboard/top-posts')

  // ── Products ──────────────────────────────────────────────────────────
  console.log('\n[ Products ]')
  await test('GET', '/api/products')
  await test('POST', '/api/products', {
    name: `Smoke Product ${Date.now()}`,
    price: 299,
    commission_rate: 10,
    category: 'Health',
    description: 'Smoke test product',
  })

  // ── Posts ─────────────────────────────────────────────────────────────
  console.log('\n[ Posts ]')
  await test('GET', '/api/posts')
  await test('POST', '/api/posts/suggest-caption', { product_id: 'smoke-product-id' })
  await test('POST', '/api/posts', {
    title: 'Smoke Post',
    caption: 'Testing',
    hashtags: ['smoke'],
    video_path: '',
    product_id: null,
  })
  // Probe: nil UUID → 404 is the correct response (post doesn't exist).
  await test(
    'POST',
    '/api/posts/00000000-0000-0000-0000-000000000000/schedule',
    { scheduled_at: new Date().toISOString() },
    'POST /api/posts/:postId/schedule (nil UUID → expect 404)',
    [404]
  )

  // ── Videos ────────────────────────────────────────────────────────────
  console.log('\n[ Videos ]')
  // Probe: no multipart body → 400 is correct validation.
  await test(
    'POST',
    '/api/videos/upload',
    undefined,
    'POST /api/videos/upload (no file → expect 400)',
    [400]
  )
  // Probe: empty input_images → 400 is correct validation.
  await test(
    'POST',
    '/api/videos/generate',
    { input_images: [], overlay_text: 'Smoke test', duration_seconds: 15 },
    'POST /api/videos/generate (empty images → expect 400)',
    [400]
  )
  // Probe: nil UUID → 404 is correct (job doesn't exist).
  await test(
    'GET',
    '/api/videos/00000000-0000-0000-0000-000000000000',
    undefined,
    'GET /api/videos/:jobId (nil UUID → expect 404)',
    [404]
  )

  // ── TikTok ────────────────────────────────────────────────────────────
  console.log('\n[ TikTok ]')
  await test('GET', '/api/tiktok/accounts')
  await test('GET', '/api/tiktok/status')
  await test('GET', '/api/tiktok/auth-url')

  // ── Summary ───────────────────────────────────────────────────────────
  const passed = results.filter((r) => r.ok).length
  const failed = results.filter((r) => !r.ok).length
  console.log(`\n────────────────────────────────────────────`)
  console.log(`  Total  : ${results.length}`)
  console.log(`  Passed : ${passed} ✓`)
  console.log(`  Failed : ${failed} ✗`)

  if (failed > 0) {
    console.log('\n  Failed endpoints:')
    results.filter((r) => !r.ok).forEach((r) => {
      console.log(`    ✗ ${r.method} ${r.endpoint}`)
      console.log(`      Status: ${r.status ?? 'CONNECTION ERROR'}`)
      console.log(`      Error : ${r.error}`)
      if (r.responseExcerpt) console.log(`      Body  : ${r.responseExcerpt}`)
    })
  }

  // Write JSON output for report generator
  const outDir = path.join(process.cwd(), 'playwright-report')
  if (!fs.existsSync(outDir)) fs.mkdirSync(outDir, { recursive: true })

  const output = {
    timestamp: new Date().toISOString(),
    baseUrl: BASE_URL,
    tokenProvided: !!TOKEN,
    total: results.length,
    passed,
    failed,
    results,
  }
  fs.writeFileSync(path.join(outDir, 'api-smoke.json'), JSON.stringify(output, null, 2))
  console.log(`\n  Results saved to playwright-report/api-smoke.json`)

  process.exit(failed > 0 ? 1 : 0)
}

main().catch((err) => {
  console.error('Smoke test runner crashed:', err)
  process.exit(2)
})
