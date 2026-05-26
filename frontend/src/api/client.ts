import axios from 'axios'

// Convert snake_case keys to camelCase recursively so all API responses
// match the frontend TypeScript types without manual mapping.
function toCamel(s: string): string {
  return s.replace(/_([a-z])/g, (_, c) => c.toUpperCase())
}
function camelizeKeys(val: unknown): unknown {
  if (Array.isArray(val)) return val.map(camelizeKeys)
  if (val !== null && typeof val === 'object') {
    return Object.fromEntries(
      Object.entries(val as Record<string, unknown>).map(([k, v]) => [toCamel(k), camelizeKeys(v)])
    )
  }
  return val
}

const client = axios.create({
  baseURL: '/api',
  headers: { 'Content-Type': 'application/json' },
})

client.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

client.interceptors.response.use(
  (res) => {
    res.data = camelizeKeys(res.data)
    return res
  },
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      window.location.href = '/login'
    }
    return Promise.reject(err)
  }
)

export default client
