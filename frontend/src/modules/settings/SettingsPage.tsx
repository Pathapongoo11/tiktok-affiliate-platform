import { useEffect, useState } from 'react'
import client from '../../api/client'
import type { User } from '../../types'

interface TikTokAccount {
  id: string
  username: string
  displayName?: string
  followerCount?: number
  isActive?: boolean
}

interface TikTokStatus {
  mockMode?: boolean
  credentialsSet?: boolean
}

export default function SettingsPage() {
  const [accounts, setAccounts] = useState<TikTokAccount[]>([])
  const [tiktokStatus, setTiktokStatus] = useState<TikTokStatus | null>(null)
  const [loadingAccounts, setLoadingAccounts] = useState(true)
  const [connecting, setConnecting] = useState(false)
  const [user, setUser] = useState<User | null>(null)

  useEffect(() => {
    try {
      const stored = localStorage.getItem('user')
      if (stored) setUser(JSON.parse(stored))
    } catch {}

    Promise.all([
      client.get('/tiktok/accounts').catch(() => ({ data: { accounts: [] } })),
      client.get('/tiktok/status').catch(() => ({ data: null })),
    ]).then(([accRes, statusRes]) => {
      const accs = accRes.data?.accounts ?? accRes.data
      setAccounts(Array.isArray(accs) ? accs : [])
      setTiktokStatus(statusRes.data)
    }).finally(() => setLoadingAccounts(false))
  }, [])

  const handleConnectTikTok = async () => {
    setConnecting(true)
    try {
      const res = await client.get('/tiktok/auth-url')
      const url = res.data.url || res.data.authUrl || res.data.auth_url
      if (url) {
        window.location.href = url
      }
    } catch {
      alert('Failed to get TikTok auth URL. Please try again.')
    } finally {
      setConnecting(false)
    }
  }

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-gray-900">Settings</h2>
        <p className="text-gray-500 text-sm mt-1">Manage your account and integrations</p>
      </div>

      {/* Profile */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-4">
        <h3 className="font-semibold text-gray-800 text-lg">Profile</h3>
        {user ? (
          <div className="space-y-3">
            <div className="flex items-center gap-4">
              <div className="w-14 h-14 rounded-full bg-purple-100 flex items-center justify-center text-2xl font-bold text-purple-600">
                {(user.displayName || user.email || 'U')[0].toUpperCase()}
              </div>
              <div>
                <p className="font-semibold text-gray-900">{user.displayName || 'No display name'}</p>
                <p className="text-sm text-gray-500">{user.email}</p>
              </div>
            </div>
            <div className="bg-gray-50 rounded-lg p-4 space-y-2 text-sm">
              <div className="flex items-center justify-between">
                <span className="text-gray-500">User ID</span>
                <span className="font-mono text-xs text-gray-700">{user.id}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-gray-500">Email</span>
                <span className="text-gray-700">{user.email}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-gray-500">Display Name</span>
                <span className="text-gray-700">{user.displayName || '—'}</span>
              </div>
            </div>
          </div>
        ) : (
          <p className="text-gray-400 text-sm">No profile data available.</p>
        )}
      </div>

      {/* TikTok Accounts */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold text-gray-800 text-lg">Connected TikTok Accounts</h3>
          {tiktokStatus?.mockMode && (
            <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-700">
              Mock Mode Active
            </span>
          )}
        </div>

        {tiktokStatus && (
          <div className="flex items-center gap-2 text-sm text-gray-500">
            <div className={`w-2 h-2 rounded-full ${accounts.length > 0 ? 'bg-green-500' : 'bg-gray-300'}`} />
            {accounts.length > 0 ? 'Connected' : 'Not connected'}
            {tiktokStatus.mockMode && (
              <span className="text-yellow-600 font-medium ml-1">(using mock API)</span>
            )}
          </div>
        )}

        {loadingAccounts ? (
          <p className="text-sm text-gray-400">Loading accounts...</p>
        ) : accounts.length === 0 ? (
          <div className="text-center py-6 bg-gray-50 rounded-xl">
            <div className="text-3xl mb-2">📱</div>
            <p className="text-gray-500 text-sm">No TikTok accounts connected yet.</p>
            <p className="text-gray-400 text-xs mt-1">Connect your TikTok account to start posting.</p>
          </div>
        ) : (
          <div className="space-y-3">
            {accounts.map((acc) => (
              <div
                key={acc.id}
                className="flex items-center justify-between bg-gray-50 rounded-lg p-4"
              >
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-full bg-black flex items-center justify-center text-white font-bold text-sm">
                    T
                  </div>
                  <div>
                    <p className="font-medium text-gray-900">@{acc.username}</p>
                    {acc.displayName && (
                      <p className="text-xs text-gray-500">{acc.displayName}</p>
                    )}
                    {acc.followerCount !== undefined && (
                      <p className="text-xs text-gray-400">{acc.followerCount.toLocaleString()} followers</p>
                    )}
                  </div>
                </div>
                <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                  acc.isActive !== false ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-600'
                }`}>
                  {acc.isActive !== false ? 'Active' : 'Inactive'}
                </span>
              </div>
            ))}
          </div>
        )}

        <button
          onClick={handleConnectTikTok}
          disabled={connecting}
          className="w-full py-2.5 border border-gray-300 text-gray-700 rounded-lg text-sm font-medium hover:bg-gray-50 disabled:opacity-60 disabled:cursor-not-allowed transition-colors flex items-center justify-center gap-2"
        >
          📱 {connecting ? 'Connecting...' : 'Connect TikTok Account'}
        </button>
      </div>

      {/* Sign Out */}
      <div className="bg-white rounded-xl border border-red-100 shadow-sm p-6 space-y-3">
        <h3 className="font-semibold text-red-700 text-lg">Account</h3>
        <p className="text-sm text-gray-500">Manage your session and account data.</p>
        <button
          onClick={() => {
            localStorage.removeItem('token')
            localStorage.removeItem('user')
            window.location.href = '/login'
          }}
          className="px-4 py-2 bg-red-50 text-red-600 rounded-lg text-sm font-medium hover:bg-red-100 transition-colors"
        >
          Sign Out
        </button>
      </div>
    </div>
  )
}
