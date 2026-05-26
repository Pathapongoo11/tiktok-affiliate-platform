import { useEffect, useState } from 'react'
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts'
import client from '../../api/client'
import type { DashboardStats, TopPost } from '../../types'

const MOCK_TREND = [
  { day: 'Mon', views: 1200 },
  { day: 'Tue', views: 2100 },
  { day: 'Wed', views: 1800 },
  { day: 'Thu', views: 3400 },
  { day: 'Fri', views: 2900 },
  { day: 'Sat', views: 4500 },
  { day: 'Sun', views: 3800 },
]

function StatCard({ label, value, icon }: { label: string; value: string | number; icon: string }) {
  return (
    <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-5 flex items-center gap-4">
      <div className="text-3xl">{icon}</div>
      <div>
        <p className="text-sm text-gray-500">{label}</p>
        <p className="text-2xl font-bold text-gray-900">{value}</p>
      </div>
    </div>
  )
}

type StatusChip = 'published' | 'scheduled' | 'failed' | 'posting' | 'draft'

function StatusBadge({ status }: { status: StatusChip }) {
  const styles: Record<StatusChip, string> = {
    published: 'bg-green-100 text-green-700',
    scheduled: 'bg-blue-100 text-blue-700',
    failed: 'bg-red-100 text-red-700',
    posting: 'bg-orange-100 text-orange-700',
    draft: 'bg-gray-100 text-gray-600',
  }
  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${styles[status] || styles.draft}`}>
      {status}
    </span>
  )
}

export default function DashboardPage() {
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [topPosts, setTopPosts] = useState<TopPost[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    Promise.all([
      client.get('/dashboard/stats').catch(() => ({ data: null })),
      client.get('/dashboard/top-posts').catch(() => ({ data: [] })),
    ]).then(([statsRes, postsRes]) => {
      setStats(statsRes.data)
      setTopPosts(Array.isArray(postsRes.data) ? postsRes.data : [])
    }).finally(() => setLoading(false))
  }, [])

  const s = stats || { totalPosts: 0, totalViews: 0, totalLikes: 0, totalRevenue: 0, avgEngagementRate: 0 }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-gray-900">Dashboard</h2>
        <p className="text-gray-500 text-sm mt-1">Overview of your TikTok affiliate performance</p>
      </div>

      {/* Stat Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
        <StatCard label="Total Posts" value={loading ? '...' : s.totalPosts} icon="📝" />
        <StatCard label="Total Views" value={loading ? '...' : (s.totalViews ?? 0).toLocaleString()} icon="👁️" />
        <StatCard label="Avg Engagement" value={loading ? '...' : `${Number(s.avgEngagementRate ?? 0).toFixed(1)}%`} icon="💬" />
        <StatCard label="Total Likes" value={loading ? '...' : (s.totalLikes ?? 0).toLocaleString()} icon="❤️" />
      </div>

      {/* Views Trend Chart */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6">
        <h3 className="text-lg font-semibold text-gray-800 mb-4">Views Trend (Last 7 Days)</h3>
        <ResponsiveContainer width="100%" height={240}>
          <LineChart data={MOCK_TREND}>
            <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
            <XAxis dataKey="day" tick={{ fontSize: 12 }} />
            <YAxis tick={{ fontSize: 12 }} />
            <Tooltip />
            <Line type="monotone" dataKey="views" stroke="#7c3aed" strokeWidth={2} dot={{ r: 4 }} />
          </LineChart>
        </ResponsiveContainer>
      </div>

      {/* Top Posts Table */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6">
        <h3 className="text-lg font-semibold text-gray-800 mb-4">Top Performing Posts</h3>
        {topPosts.length === 0 ? (
          <p className="text-gray-400 text-sm text-center py-8">No posts yet. Create your first post!</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-100 text-left">
                  <th className="pb-3 text-gray-500 font-medium">Title</th>
                  <th className="pb-3 text-gray-500 font-medium">Views</th>
                  <th className="pb-3 text-gray-500 font-medium">Likes</th>
                  <th className="pb-3 text-gray-500 font-medium">Engagement</th>
                  <th className="pb-3 text-gray-500 font-medium">Status</th>
                </tr>
              </thead>
              <tbody>
                {topPosts.map((tp) => (
                  <tr key={tp.post?.id} className="border-b border-gray-50 hover:bg-gray-50">
                    <td className="py-3 font-medium text-gray-800">{tp.post?.title || tp.post?.caption || '(untitled)'}</td>
                    <td className="py-3 text-gray-600">{(tp.views ?? 0).toLocaleString()}</td>
                    <td className="py-3 text-gray-600">—</td>
                    <td className="py-3 text-gray-600">—</td>
                    <td className="py-3">
                      <StatusBadge status={tp.post?.status ?? 'draft'} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
