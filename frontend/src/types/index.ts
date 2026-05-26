export interface User { id: string; email: string; displayName: string }
export interface Product {
  id: string; name: string; price: number; commissionRate: number
  category: string; score: number; isActive: boolean
  flags?: { warnings: string[] }
}
export interface Post {
  id: string; title: string; caption: string; hashtags: string[]
  videoPath: string; status: 'draft'|'scheduled'|'posting'|'published'|'failed'
  scheduledAt?: string; publishedAt?: string; product?: Product
}
export interface VideoJob {
  id: string; status: 'pending'|'processing'|'done'|'failed'
  outputPath?: string; errorMessage?: string; durationSeconds: number
}
export interface DashboardStats {
  totalPosts: number; totalViews: number; avgEngagementRate: number; totalBasketClicks: number
}
export interface TopPost {
  id: string; title: string; views: number; likes: number; engagementRate: number
}
