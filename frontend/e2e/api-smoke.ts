/**
 * API Smoke Test Script — Full Happy-Path Chained Version
 *
 * Usage: npx tsx e2e/api-smoke.ts [base_url] [token]
 *
 * Calls all 16 endpoints with real data, chaining results between calls:
 *   register/login → products → suggest-caption → upload image → generate video
 *   → poll job → create post → schedule post → TikTok endpoints
 *
 * Writes results to playwright-report/api-smoke.json
 */

import fs from 'fs'
import path from 'path'
import { deflateSync } from 'zlib'

const BASE_URL = process.argv[2] || 'http://localhost:8080'
const TOKEN = process.argv[3] || process.env.API_TOKEN || ''

// ── PNG generator ─────────────────────────────────────────────────────────────
// Builds a valid PNG at runtime using Node's built-in zlib — avoids hardcoding
// pre-compressed bytes that silently corrupt the inflate stream.

function crc32(data: Buffer): number {
  const table = Array.from({ length: 256 }, (_, n) => {
    let c = n
    for (let k = 0; k < 8; k++) c = c & 1 ? (0xEDB88320 ^ (c >>> 1)) : (c >>> 1)
    return c >>> 0
  })
  let crc = 0xFFFFFFFF
  for (const byte of data) crc = (table[(crc ^ byte) & 0xFF]! ^ (crc >>> 8)) >>> 0
  return (crc ^ 0xFFFFFFFF) >>> 0
}

function pngChunk(type: string, data: Buffer): Buffer {
  const typeBuf = Buffer.from(type, 'ascii')
  const lenBuf = Buffer.allocUnsafe(4)
  lenBuf.writeUInt32BE(data.length)
  const crcBuf = Buffer.allocUnsafe(4)
  crcBuf.writeUInt32BE(crc32(Buffer.concat([typeBuf, data])))
  return Buffer.concat([lenBuf, typeBuf, data, crcBuf])
}

/** Generate a w×h solid-white RGB PNG with correct CRC and valid zlib IDAT. */
function makePNG(w = 10, h = 10): Buffer {
  // IHDR: width, height, bit-depth=8, color-type=2 (RGB), compression=0, filter=0, interlace=0
  const ihdr = Buffer.allocUnsafe(13)
  ihdr.writeUInt32BE(w, 0)
  ihdr.writeUInt32BE(h, 4)
  ihdr[8] = 8; ihdr[9] = 2; ihdr[10] = 0; ihdr[11] = 0; ihdr[12] = 0

  // Raw scanlines: filter byte (None=0) + w*3 bytes of 0xFF (white) per row
  const rowStride = 1 + w * 3
  const raw = Buffer.alloc(h * rowStride, 0xFF)
  for (let y = 0; y < h; y++) raw[y * rowStride] = 0  // filter byte = None

  return Buffer.concat([
    Buffer.from('89504e470d0a1a0a', 'hex'),  // PNG signature
    pngChunk('IHDR', ihdr),
    pngChunk('IDAT', deflateSync(raw)),       // zlib-wrapped deflate, as PNG requires
    pngChunk('IEND', Buffer.alloc(0)),
  ])
}

// 10×10 solid-white RGB PNG — valid, minimal, FFmpeg-safe
const MINIMAL_PNG = makePNG(10, 10)

interface SmokeResult {
  method: string
  endpoint: string
  label: string
  status: number | null
  ok: boolean
  error: string | null
  responseExcerpt: string
  durationMs: number
}

const results: SmokeResult[] = []

// ── HTTP helpers ──────────────────────────────────────────────────────────────

async function jsonRequest(
  method: string,
  endpoint: string,
  body?: unknown,
  tokenOverride?: string
): Promise<{ status: number; body: string; json: unknown }> {
  const res = await fetch(`${BASE_URL}${endpoint}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${tokenOverride ?? TOKEN}`,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  const text = await res.text()
  let json: unknown = null
  try { json = JSON.parse(text) } catch { /* not JSON */ }
  return { status: res.status, body: text, json }
}

async function multipartRequest(
  method: string,
  endpoint: string,
  fields: Record<string, { data: Buffer; filename: string; mimeType: string } | string>
): Promise<{ status: number; body: string; json: unknown }> {
  const fd = new FormData()
  for (const [key, value] of Object.entries(fields)) {
    if (typeof value === 'string') {
      fd.append(key, value)
    } else {
      fd.append(key, new Blob([value.data], { type: value.mimeType }), value.filename)
    }
  }
  const res = await fetch(`${BASE_URL}${endpoint}`, {
    method,
    headers: { Authorization: `Bearer ${TOKEN}` },
    body: fd,
  })
  const text = await res.text()
  let json: unknown = null
  try { json = JSON.parse(text) } catch { /* not JSON */ }
  return { status: res.status, body: text, json }
}

function excerpt(body: string, maxLen = 200): string {
  return body.replace(/\s+/g, ' ').trim().slice(0, maxLen)
}

// ── Test runner ────────────────────────────────────────────────────────────────

async function run(
  label: string,
  method: string,
  endpoint: string,
  call: () => Promise<{ status: number; body: string; json: unknown }>,
  expectOk: (status: number, json: unknown) => boolean = (s) => s >= 200 && s < 300
): Promise<{ status: number; json: unknown } | null> {
  const start = Date.now()
  try {
    const res = await call()
    const durationMs = Date.now() - start
    const ok = expectOk(res.status, res.json)
    const icon = ok ? '✓' : '✗'
    console.log(`  ${icon} [${res.status}] ${label} (${durationMs}ms)`)
    results.push({
      method,
      endpoint,
      label,
      status: res.status,
      ok,
      error: ok ? null : `HTTP ${res.status}: ${excerpt(res.body, 100)}`,
      responseExcerpt: excerpt(res.body),
      durationMs,
    })
    return ok ? res : null
  } catch (err: unknown) {
    const durationMs = Date.now() - start
    const msg = (err as Error).message || String(err)
    console.log(`  ✗ [ERR] ${label} — ${msg}`)
    results.push({
      method,
      endpoint,
      label,
      status: null,
      ok: false,
      error: msg,
      responseExcerpt: '',
      durationMs,
    })
    return null
  }
}

// ── Poll helper ────────────────────────────────────────────────────────────────

async function pollJobUntilDone(
  jobId: string,
  timeoutMs = 60000
): Promise<{ status: number; json: unknown } | null> {
  const deadline = Date.now() + timeoutMs
  let attempts = 0
  while (Date.now() < deadline) {
    attempts++
    const res = await jsonRequest('GET', `/api/videos/${jobId}`)
    const j = res.json as Record<string, unknown> | null
    const jobStatus = j?.status as string | undefined
    if (jobStatus === 'done' || jobStatus === 'failed') {
      const ok = jobStatus === 'done'
      const elapsed = Date.now() - (deadline - timeoutMs)
      console.log(`  ${ok ? '✓' : '✗'} [${res.status}] GET /api/videos/:jobId — poll done in ${elapsed}ms (${attempts} attempts), job=${jobStatus}`)
      results.push({
        method: 'GET',
        endpoint: `/api/videos/${jobId}`,
        label: `GET /api/videos/:jobId (polled until done, ${attempts} attempts)`,
        status: res.status,
        ok,
        error: ok ? null : `Job failed: ${(j?.errorMessage as string) || 'unknown'}`,
        responseExcerpt: excerpt(res.body),
        durationMs: elapsed,
      })
      return ok ? res : null
    }
    await new Promise((r) => setTimeout(r, 3000))
  }
  // Timed out
  console.log(`  ✗ [TIMEOUT] GET /api/videos/:jobId — exceeded ${timeoutMs}ms (${attempts} attempts)`)
  results.push({
    method: 'GET',
    endpoint: `/api/videos/${jobId}`,
    label: `GET /api/videos/:jobId (polled, timed out after ${timeoutMs}ms)`,
    status: null,
    ok: false,
    error: `Polling timed out after ${timeoutMs}ms`,
    responseExcerpt: '',
    durationMs: timeoutMs,
  })
  return null
}

// ── Main ──────────────────────────────────────────────────────────────────────

async function main() {
  console.log(`\n╔════════════════════════════════════════════╗`)
  console.log(`║     API Smoke Test — TikTok Affiliate      ║`)
  console.log(`╚════════════════════════════════════════════╝`)
  console.log(`  Backend URL : ${BASE_URL}`)
  console.log(`  Token set   : ${TOKEN ? 'YES' : 'NO'}`)
  console.log()

  let activeToken = TOKEN
  let productId: string | null = null
  let postId: string | null = null
  let uploadedImagePath: string | null = null
  let jobId: string | null = null

  // ── Auth ────────────────────────────────────────────────────────────────────
  console.log('[ Auth ]')

  const smokeEmail = `smoke_${Date.now()}@test.com`
  const smokePassword = 'SmokeTest123!'

  const registerRes = await run(
    `POST /api/auth/register (new unique email)`,
    'POST', '/api/auth/register',
    () => jsonRequest('POST', '/api/auth/register', {
      email: smokeEmail,
      password: smokePassword,
      display_name: 'Smoke Tester',
    })
  )
  // Use registered token if no token provided
  if (!activeToken && registerRes) {
    const d = registerRes.json as Record<string, unknown>
    activeToken = (d?.token as string) || activeToken
  }

  // Login with the SAME email just registered
  await run(
    `POST /api/auth/login (same email from register)`,
    'POST', '/api/auth/login',
    () => jsonRequest('POST', '/api/auth/login', {
      email: smokeEmail,
      password: smokePassword,
    })
  )

  // ── Dashboard ────────────────────────────────────────────────────────────────
  console.log('\n[ Dashboard ]')

  await run(
    `GET /api/dashboard/stats`,
    'GET', '/api/dashboard/stats',
    () => jsonRequest('GET', '/api/dashboard/stats')
  )

  await run(
    `GET /api/dashboard/top-posts`,
    'GET', '/api/dashboard/top-posts',
    () => jsonRequest('GET', '/api/dashboard/top-posts')
  )

  // ── Products ─────────────────────────────────────────────────────────────────
  console.log('\n[ Products ]')

  await run(
    `GET /api/products`,
    'GET', '/api/products',
    () => jsonRequest('GET', '/api/products')
  )

  const addProductRes = await run(
    `POST /api/products (real data: name, price, commission)`,
    'POST', '/api/products',
    () => jsonRequest('POST', '/api/products', {
      name: `Smoke Serum ${Date.now()}`,
      price: 399,
      commission_rate: 15,
      category: 'Beauty',
      description: 'Smoke test product — safe to delete',
    })
  )
  if (addProductRes) {
    const d = addProductRes.json as Record<string, unknown>
    productId = (d?.id as string) || (d?.data as Record<string, unknown>)?.id as string || null
    if (productId) console.log(`       → product_id: ${productId}`)
  }

  // ── Posts ────────────────────────────────────────────────────────────────────
  console.log('\n[ Posts ]')

  await run(
    `GET /api/posts`,
    'GET', '/api/posts',
    () => jsonRequest('GET', '/api/posts')
  )

  await run(
    `POST /api/posts/suggest-caption (with real product_id)`,
    'POST', '/api/posts/suggest-caption',
    () => jsonRequest('POST', '/api/posts/suggest-caption', {
      product_id: productId || 'fallback-id',
    })
  )

  // ── Videos ───────────────────────────────────────────────────────────────────
  console.log('\n[ Videos ]')

  // 1. Upload real image via multipart/form-data
  const uploadRes = await run(
    `POST /api/videos/upload (real 1×1 PNG image, multipart/form-data)`,
    'POST', '/api/videos/upload',
    () => multipartRequest('POST', '/api/videos/upload', {
      image: { data: MINIMAL_PNG, filename: 'smoke-test.png', mimeType: 'image/png' },
    })
  )
  if (uploadRes) {
    const d = uploadRes.json as Record<string, unknown>
    uploadedImagePath = (d?.path ?? d?.filePath ?? d?.url) as string | null
    if (uploadedImagePath) console.log(`       → uploaded path: ${uploadedImagePath}`)
  }

  // 2. Generate video with the real uploaded path
  const generateRes = await run(
    `POST /api/videos/generate (real input_images from upload)`,
    'POST', '/api/videos/generate',
    () => jsonRequest('POST', '/api/videos/generate', {
      input_images: uploadedImagePath ? [uploadedImagePath] : [],
      overlay_text: 'Smoke Test — ฿399',
      duration_seconds: 15,
    }),
    (status, json) => {
      // Accept 200/201 with a job ID, OR 400 if empty images
      if (status >= 200 && status < 300) return true
      if (status === 400 && !uploadedImagePath) return true // fallback if upload failed
      return false
    }
  )
  if (generateRes) {
    const d = generateRes.json as Record<string, unknown>
    jobId = (d?.id ?? d?.jobId) as string | null
    if (jobId) console.log(`       → job_id: ${jobId}`)
  }

  // 3. Poll GET /api/videos/:jobId until done (real polling, up to 60s)
  if (jobId) {
    console.log(`       Polling job ${jobId} (up to 60s)...`)
    await pollJobUntilDone(jobId, 60000)
  } else {
    // Fallback: test with nil UUID (proves route exists)
    await run(
      `GET /api/videos/:jobId (nil UUID → route exists check)`,
      'GET', '/api/videos/00000000-0000-0000-0000-000000000000',
      () => jsonRequest('GET', '/api/videos/00000000-0000-0000-0000-000000000000'),
      (status) => status === 404 || (status >= 200 && status < 300)
    )
  }

  // ── Create post (needed before schedule) ─────────────────────────────────────
  console.log('\n[ Posts — Create & Schedule ]')

  const createPostRes = await run(
    `POST /api/posts (real caption, product_id, video_path)`,
    'POST', '/api/posts',
    () => jsonRequest('POST', '/api/posts', {
      title: 'Smoke Test Post',
      caption: 'สินค้าดีมากค่ะ ราคาถูก คุณภาพดี! #smoketest',
      hashtags: ['smoketest', 'beauty'],
      video_path: uploadedImagePath || '',
      product_id: productId || null,
    })
  )
  if (createPostRes) {
    const d = createPostRes.json as Record<string, unknown>
    postId = (d?.id ?? d?.postId) as string | null
    if (postId) console.log(`       → post_id: ${postId}`)
  }

  // Schedule using REAL post ID
  await run(
    `POST /api/posts/:postId/schedule (real post_id, real ISO date)`,
    'POST', `/api/posts/${postId ?? '00000000-0000-0000-0000-000000000000'}/schedule`,
    () => jsonRequest('POST', `/api/posts/${postId ?? '00000000-0000-0000-0000-000000000000'}/schedule`, {
      scheduled_at: new Date(Date.now() + 3600_000).toISOString(), // 1 hour from now
    }),
    (status) => {
      if (postId) return status >= 200 && status < 300
      return status === 404 // nil UUID expects 404
    }
  )

  // ── TikTok ───────────────────────────────────────────────────────────────────
  console.log('\n[ TikTok ]')

  await run(
    `GET /api/tiktok/accounts`,
    'GET', '/api/tiktok/accounts',
    () => jsonRequest('GET', '/api/tiktok/accounts')
  )

  await run(
    `GET /api/tiktok/status`,
    'GET', '/api/tiktok/status',
    () => jsonRequest('GET', '/api/tiktok/status')
  )

  await run(
    `GET /api/tiktok/auth-url`,
    'GET', '/api/tiktok/auth-url',
    () => jsonRequest('GET', '/api/tiktok/auth-url')
  )

  // ── Summary ──────────────────────────────────────────────────────────────────
  const passed = results.filter((r) => r.ok).length
  const failed = results.filter((r) => !r.ok).length

  console.log(`\n────────────────────────────────────────────`)
  console.log(`  Total  : ${results.length}`)
  console.log(`  Passed : ${passed} ✓`)
  console.log(`  Failed : ${failed} ✗`)

  if (failed > 0) {
    console.log('\n  Failed endpoints:')
    results.filter((r) => !r.ok).forEach((r) => {
      console.log(`    ✗ ${r.label}`)
      console.log(`      Status: ${r.status ?? 'CONNECTION ERROR'}`)
      if (r.error) console.log(`      Error : ${r.error}`)
    })
  }

  // Save JSON
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
