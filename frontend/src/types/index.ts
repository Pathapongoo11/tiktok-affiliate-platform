export interface User { id: string; email: string; displayName: string }

export interface Product {
  id: string; name: string; description?: string; price: number
  commissionRate: number; category: string; shopProductId?: string
  imageUrls?: string[]; score: number; isActive: boolean
  flags?: { warnings: string[] }
}

export interface Post {
  id: string; title?: string; caption?: string; hashtags?: string[]
  videoPath?: string; tikTokVideoId?: string
  status: 'draft'|'scheduled'|'posting'|'published'|'failed'
  scheduledAt?: string; publishedAt?: string; productId?: string
}

export interface VideoJob {
  id: string; status: 'pending'|'processing'|'done'|'failed'
  outputPath?: string; errorMessage?: string; durationSeconds?: number
}

// Matches Go models.DashboardStats (snake_case auto-converted to camelCase by axios)
export interface DashboardStats {
  totalPosts: number; totalViews: number; totalLikes: number
  totalRevenue: number; avgEngagementRate: number
}

// Matches Go models.TopPost — nested structure {post, views}
export interface TopPost {
  post: Post; views: number
}
