import { describe, it, expect } from 'vitest'

// ─────────────────────────────────────────────────────────────────────────────
// Schedule datetime fix — regression tests
//
// Bug: <input type="datetime-local"> returns "YYYY-MM-DDTHH:MM" (no timezone,
// no seconds).  Go's time.Time JSON decoder requires RFC 3339 and rejects the
// bare datetime-local string, causing "failed to schedule post" (HTTP 500).
//
// Fix: convert with new Date(scheduledAt).toISOString() before sending.
// ─────────────────────────────────────────────────────────────────────────────

describe('schedule datetime-local → RFC3339 conversion', () => {
  it('datetime-local value is NOT valid RFC3339 on its own', () => {
    const datetimeLocalValue = '2026-05-26T19:00'
    // Must have a timezone suffix (Z or +HH:MM) to be RFC3339
    expect(datetimeLocalValue).not.toMatch(/Z$/)
    expect(datetimeLocalValue).not.toMatch(/[+-]\d{2}:\d{2}$/)
  })

  it('new Date(datetimeLocal).toISOString() produces valid RFC3339 Z-string', () => {
    const datetimeLocalValue = '2026-05-26T19:00'
    const iso = new Date(datetimeLocalValue).toISOString()
    // Must match full RFC3339 UTC format: YYYY-MM-DDTHH:MM:SS.mmmZ
    expect(iso).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/)
  })

  it('conversion is stable for all TikTok suggested times', () => {
    const suggestedTimes = ['07:00', '12:00', '19:00', '21:00']
    for (const t of suggestedTimes) {
      const datetimeLocal = `2026-05-26T${t}`
      const iso = new Date(datetimeLocal).toISOString()
      expect(iso).toMatch(/T\d{2}:\d{2}:\d{2}\.\d{3}Z$/)
      expect(() => new Date(iso)).not.toThrow()
    }
  })

  it('setQuickTime pattern: toISOString().slice(0,16) also needs conversion before sending', () => {
    // setQuickTime produces a datetime-local string (slice removes timezone)
    const today = new Date('2026-05-26T19:00:00.000Z')
    const datetimeLocalFromQuickTime = today.toISOString().slice(0, 16)
    // Result is "2026-05-26T19:00" — NOT RFC3339
    expect(datetimeLocalFromQuickTime).not.toMatch(/Z$/)
    // After the fix (re-converting)
    const fixedISO = new Date(datetimeLocalFromQuickTime).toISOString()
    expect(fixedISO).toMatch(/Z$/)
  })

  it('"Post Now" path already uses toISOString() and is valid RFC3339', () => {
    const before = Date.now()
    const iso = new Date().toISOString()
    const after = Date.now()
    expect(iso).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/)
    const ts = new Date(iso).getTime()
    expect(ts).toBeGreaterThanOrEqual(before)
    expect(ts).toBeLessThanOrEqual(after)
  })

  it('converted ISO string parses back to a valid Date without NaN', () => {
    const inputs = [
      '2026-05-26T07:00',
      '2026-06-15T12:30',
      '2026-12-31T21:00',
      '2026-01-01T00:00',
    ]
    for (const raw of inputs) {
      const iso = new Date(raw).toISOString()
      const roundTrip = new Date(iso)
      expect(roundTrip.getTime()).not.toBeNaN()
    }
  })

  it('schedule payload key must be snake_case scheduled_at (not scheduledAt)', () => {
    // Documents the API contract: Go JSON tag is `json:"scheduled_at"`
    const scheduledAtISO = new Date('2026-05-26T19:00').toISOString()
    const payload = { scheduled_at: scheduledAtISO }

    expect(payload).toHaveProperty('scheduled_at')
    expect((payload as Record<string, unknown>)['scheduledAt']).toBeUndefined()
    // Value is a proper ISO string, not the raw datetime-local
    expect(payload.scheduled_at).toMatch(/Z$/)
    expect(payload.scheduled_at).not.toBe('2026-05-26T19:00')
  })
})
