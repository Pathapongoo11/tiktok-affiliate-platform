import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import client from '../../api/client'
import type { Post } from '../../types'

type StatusFilter = 'all' | Post['status']

const STATUS_OPTIONS: StatusFilter[] = ['all', 'draft', 'scheduled', 'posting', 'published', 'failed']

type StatusKey = 'published' | 'scheduled' | 'failed' | 'posting' | 'draft'

function statusBadgeClass(status: Post['status']) {
  const map: Record<StatusKey, string> = {
    draft: 'bg-gray-100 text-gray-600',
    scheduled: 'bg-blue-100 text-blue-700',
    posting: 'bg-orange-100 text-orange-700',
    published: 'bg-green-100 text-green-700',
    failed: 'bg-red-100 text-red-700',
  }
  return map[status as StatusKey] || map.draft
}

export default function PostsPage() {
  const navigate = useNavigate()
  const [posts, setPosts] = useState<Post[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState<StatusFilter>('all')

  useEffect(() => {
    client.get('/posts')
      .then((res) => {
        const list = res.data?.data ?? res.data
        setPosts(Array.isArray(list) ? list : [])
      })
      .catch(() => setPosts([]))
      .finally(() => setLoading(false))
  }, [])

  const filtered = filter === 'all' ? posts : posts.filter((p) => p.status === filter)

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gray-900">Posts</h2>
          <p className="text-gray-500 text-sm mt-1">Manage your TikTok content</p>
        </div>
        <button
          onClick={() => navigate('/posts/new')}
          className="px-4 py-2 bg-purple-600 text-white rounded-lg text-sm font-medium hover:bg-purple-700 transition-colors"
        >
          + Create New Post
        </button>
      </div>

      {/* Filter tabs */}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-sm text-gray-500 font-medium">Filter:</span>
        {STATUS_OPTIONS.map((s) => (
          <button
            key={s}
            onClick={() => setFilter(s)}
            className={`px-3 py-1 rounded-full text-sm font-medium capitalize transition-colors ${
              filter === s
                ? 'bg-purple-600 text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            {s}
          </button>
        ))}
      </div>

      {loading ? (
        <div className="text-center py-12 text-gray-400">Loading posts...</div>
      ) : filtered.length === 0 ? (
        <div className="text-center py-12 text-gray-400">
          {filter === 'all'
            ? 'No posts yet. Create your first TikTok post!'
            : `No ${filter} posts found.`}
        </div>
      ) : (
        <div className="space-y-3">
          {filtered.map((post) => (
            <div key={post.id} className="bg-white rounded-xl border border-gray-200 shadow-sm hover:shadow-md transition-shadow p-4">
              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <h3 className="font-semibold text-gray-900 truncate">{post.title}</h3>
                    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${statusBadgeClass(post.status)}`}>
                      {post.status}
                    </span>
                  </div>
                  <p className="text-sm text-gray-500 line-clamp-2">{post.caption}</p>
                  {post.hashtags && post.hashtags.length > 0 && (
                    <div className="flex flex-wrap gap-1 mt-2">
                      {post.hashtags.slice(0, 5).map((tag) => (
                        <span key={tag} className="text-xs text-purple-600">#{tag}</span>
                      ))}
                      {post.hashtags.length > 5 && (
                        <span className="text-xs text-gray-400">+{post.hashtags.length - 5}</span>
                      )}
                    </div>
                  )}
                </div>
                <div className="text-right shrink-0">
                  {post.product && (
                    <p className="text-xs text-gray-400 mb-1">{post.product.name}</p>
                  )}
                  {post.scheduledAt && (
                    <p className="text-xs text-gray-400">
                      Scheduled: {new Date(post.scheduledAt).toLocaleString()}
                    </p>
                  )}
                  {post.publishedAt && (
                    <p className="text-xs text-green-600">
                      Published: {new Date(post.publishedAt).toLocaleString()}
                    </p>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
