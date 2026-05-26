/**
 * Recursively converts snake_case object keys to camelCase.
 * Used by the axios response interceptor to normalise API responses.
 * This only applies to RESPONSES — request bodies must use snake_case
 * to match the Go API's JSON tags.
 */

export function toCamel(s: string): string {
  return s.replace(/_([a-z])/g, (_, c: string) => c.toUpperCase())
}

export function camelizeKeys(val: unknown): unknown {
  if (Array.isArray(val)) return val.map(camelizeKeys)
  if (val !== null && typeof val === 'object') {
    return Object.fromEntries(
      Object.entries(val as Record<string, unknown>).map(([k, v]) => [toCamel(k), camelizeKeys(v)])
    )
  }
  return val
}
