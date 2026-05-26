import { describe, it, expect } from 'vitest'
import { toCamel, camelizeKeys } from './camelizeKeys'

// ─────────────────────────────────────────────────────────────
// toCamel — single-string conversion
// ─────────────────────────────────────────────────────────────

describe('toCamel', () => {
  it('converts snake_case to camelCase', () => {
    expect(toCamel('total_posts')).toBe('totalPosts')
    expect(toCamel('avg_engagement_rate')).toBe('avgEngagementRate')
    expect(toCamel('commission_rate')).toBe('commissionRate')
    expect(toCamel('video_path')).toBe('videoPath')
    expect(toCamel('product_id')).toBe('productId')
    expect(toCamel('scheduled_at')).toBe('scheduledAt')
    expect(toCamel('mock_mode')).toBe('mockMode')
    expect(toCamel('credentials_set')).toBe('credentialsSet')
  })

  it('leaves camelCase unchanged', () => {
    expect(toCamel('totalPosts')).toBe('totalPosts')
    expect(toCamel('displayName')).toBe('displayName')
  })

  it('leaves plain lowercase unchanged', () => {
    expect(toCamel('status')).toBe('status')
    expect(toCamel('email')).toBe('email')
    expect(toCamel('title')).toBe('title')
  })

  it('handles single-segment keys', () => {
    expect(toCamel('id')).toBe('id')
    expect(toCamel('name')).toBe('name')
  })

  it('handles consecutive underscores — only _[a-z] sequences are converted', () => {
    // 'foo__bar': first underscore is followed by another underscore (not [a-z]),
    // second underscore IS followed by 'b' → converts to 'B'.
    // Result: 'foo_Bar'
    expect(toCamel('foo__bar')).toBe('foo_Bar')
  })
})

// ─────────────────────────────────────────────────────────────
// camelizeKeys — recursive object transformation
// ─────────────────────────────────────────────────────────────

describe('camelizeKeys', () => {
  it('converts top-level snake_case keys', () => {
    const input = {
      total_posts: 5,
      total_views: 1000,
      avg_engagement_rate: 3.5,
    }
    expect(camelizeKeys(input)).toEqual({
      totalPosts: 5,
      totalViews: 1000,
      avgEngagementRate: 3.5,
    })
  })

  it('converts nested snake_case keys recursively', () => {
    const input = {
      top_post: {
        post_id: 'abc',
        video_path: '/uploads/video.mp4',
      },
    }
    expect(camelizeKeys(input)).toEqual({
      topPost: {
        postId: 'abc',
        videoPath: '/uploads/video.mp4',
      },
    })
  })

  it('handles arrays of objects', () => {
    const input = [
      { commission_rate: 15, is_active: true },
      { commission_rate: 20, is_active: false },
    ]
    expect(camelizeKeys(input)).toEqual([
      { commissionRate: 15, isActive: true },
      { commissionRate: 20, isActive: false },
    ])
  })

  it('handles arrays nested inside objects', () => {
    const input = {
      image_urls: ['/img/a.jpg', '/img/b.jpg'],
      top_posts: [{ post_id: '1', views: 100 }],
    }
    expect(camelizeKeys(input)).toEqual({
      imageUrls: ['/img/a.jpg', '/img/b.jpg'],
      topPosts: [{ postId: '1', views: 100 }],
    })
  })

  it('passes through primitive values unchanged', () => {
    expect(camelizeKeys(42)).toBe(42)
    expect(camelizeKeys('hello')).toBe('hello')
    expect(camelizeKeys(true)).toBe(true)
    expect(camelizeKeys(null)).toBeNull()
    expect(camelizeKeys(undefined)).toBeUndefined()
  })

  it('handles deeply nested structures', () => {
    const input = {
      user_info: {
        display_name: 'Alice',
        tiktok_account: {
          is_active: true,
          token_expires_at: '2025-01-01T00:00:00Z',
        },
      },
    }
    expect(camelizeKeys(input)).toEqual({
      userInfo: {
        displayName: 'Alice',
        tiktokAccount: {
          isActive: true,
          tokenExpiresAt: '2025-01-01T00:00:00Z',
        },
      },
    })
  })

  // ── Real API response shapes ──────────────────────────────

  it('correctly transforms DashboardStats response', () => {
    const apiResponse = {
      total_posts: 10,
      total_views: 45000,
      total_likes: 1200,
      total_revenue: 0,
      avg_engagement_rate: 4.2,
    }
    const result = camelizeKeys(apiResponse) as Record<string, unknown>
    expect(result.totalPosts).toBe(10)
    expect(result.totalViews).toBe(45000)
    expect(result.avgEngagementRate).toBe(4.2)
    // Old bug: these would have been undefined → .toLocaleString() crash
    expect(result.total_posts).toBeUndefined()
    expect(result.total_views).toBeUndefined()
  })

  it('correctly transforms TopPost nested response', () => {
    const apiResponse = {
      post: {
        id: 'uuid-123',
        title: 'Best Serum Review',
        video_path: '/uploads/v.mp4',
        tiktok_video_id: 'tt_456',
        scheduled_at: '2025-06-01T19:00:00Z',
        is_active: true,
      },
      views: 9800,
    }
    const result = camelizeKeys(apiResponse) as Record<string, unknown>
    const post = result.post as Record<string, unknown>
    expect(post.videoPath).toBe('/uploads/v.mp4')
    expect(post.tiktokVideoId).toBe('tt_456')  // tiktok_video_id → tiktokVideoId (no split in "tiktok")
    expect(post.scheduledAt).toBe('2025-06-01T19:00:00Z')
    expect(result.views).toBe(9800)
  })

  it('correctly transforms TikTok status response', () => {
    const apiResponse = {
      mock_mode: true,
      credentials_set: false,
    }
    const result = camelizeKeys(apiResponse) as Record<string, unknown>
    expect(result.mockMode).toBe(true)
    expect(result.credentialsSet).toBe(false)
    // Old bug: these would have been undefined
    expect(result.mock_mode).toBeUndefined()
    expect(result.connected).toBeUndefined()
  })

  it('correctly transforms paginated products response', () => {
    const apiResponse = {
      data: [
        {
          id: 'p1',
          name: 'Vitamin C Serum',
          commission_rate: 15,
          is_active: true,
          image_urls: [],
        },
      ],
      total: 1,
      page: 1,
      limit: 20,
    }
    const result = camelizeKeys(apiResponse) as { data: Array<Record<string, unknown>>; total: number }
    expect(result.total).toBe(1)
    const product = result.data[0]
    expect(product.commissionRate).toBe(15)
    expect(product.isActive).toBe(true)
    expect(product.imageUrls).toEqual([])
  })
})

// ─────────────────────────────────────────────────────────────
// Request payload validation — snake_case must NOT be camelized
// (camelizeKeys only runs on responses, not requests)
// ─────────────────────────────────────────────────────────────

describe('request payload contracts (snake_case keys)', () => {
  /**
   * These tests document what the frontend must send to the API.
   * camelizeKeys is NOT called on requests — so these payloads
   * must already be snake_case to match Go's JSON tags.
   */

  it('video generate payload uses snake_case keys', () => {
    const uploadedPaths = ['/uploads/images/a.jpg']
    const overlayText = 'Test Product ฿299'
    const duration = 30
    const audioPath = '/uploads/audio/bg.mp3'

    // Correct payload shape (fixed version)
    const payload = {
      input_images: uploadedPaths,
      overlay_text: overlayText,
      duration_seconds: duration,
      audio_path: audioPath,
    }

    expect(payload).toHaveProperty('input_images')
    expect(payload).toHaveProperty('overlay_text')
    expect(payload).toHaveProperty('duration_seconds')
    expect(payload).toHaveProperty('audio_path')

    // Must NOT use old camelCase keys
    expect(payload).not.toHaveProperty('imagePaths')
    expect(payload).not.toHaveProperty('overlayText')
    expect(payload).not.toHaveProperty('durationSeconds')
    expect(payload).not.toHaveProperty('audioPath')
  })

  it('create product payload uses snake_case commission_rate', () => {
    const payload = {
      name: 'Test Product',
      price: 299,
      commission_rate: 15,   // ✅ correct
      category: 'Beauty',
      description: 'Test',
    }

    expect(payload.commission_rate).toBe(15)
    // Regression: old bug used commissionRate (camelCase)
    expect((payload as Record<string, unknown>)['commissionRate']).toBeUndefined()
  })

  it('create post payload uses snake_case video_path and product_id', () => {
    const payload = {
      title: 'New Post',
      caption: 'Amazing product!',
      hashtags: ['tiktok', 'beauty'],
      video_path: '/uploads/v.mp4',  // ✅ correct
      product_id: 'uuid-123',         // ✅ correct
    }

    expect(payload.video_path).toBe('/uploads/v.mp4')
    expect(payload.product_id).toBe('uuid-123')
    expect((payload as Record<string, unknown>)['videoPath']).toBeUndefined()
    expect((payload as Record<string, unknown>)['productId']).toBeUndefined()
  })

  it('schedule post payload uses snake_case scheduled_at', () => {
    const scheduledAt = '2025-06-01T19:00:00'
    const payload = { scheduled_at: scheduledAt }  // ✅ correct

    expect(payload.scheduled_at).toBe(scheduledAt)
    expect((payload as Record<string, unknown>)['scheduledAt']).toBeUndefined()
    expect((payload as Record<string, unknown>)['scheduleNow']).toBeUndefined()
  })

  it('register payload uses snake_case display_name', () => {
    const displayName = 'Alice'
    const payload = {
      email: 'alice@example.com',
      password: 'secret123',
      display_name: displayName,  // ✅ correct
    }

    expect(payload.display_name).toBe('Alice')
    expect((payload as Record<string, unknown>)['displayName']).toBeUndefined()
  })

  it('"post now" schedule sends scheduled_at with current time (not scheduleNow flag)', () => {
    const before = Date.now()
    const scheduledAt = new Date().toISOString()
    const after = Date.now()

    const payload = { scheduled_at: scheduledAt }

    expect(payload).toHaveProperty('scheduled_at')
    const ts = new Date(payload.scheduled_at).getTime()
    expect(ts).toBeGreaterThanOrEqual(before)
    expect(ts).toBeLessThanOrEqual(after)
    expect((payload as Record<string, unknown>)['scheduleNow']).toBeUndefined()
  })
})

// ─────────────────────────────────────────────────────────────
// Paginated response handling
// ─────────────────────────────────────────────────────────────

describe('paginated response extraction', () => {
  /**
   * Documents the fix: API returns {data: [], total, page, limit}.
   * Frontend must extract res.data?.data (not res.data directly).
   */

  function extractList(resData: unknown): unknown[] {
    const data = (resData as Record<string, unknown>)
    const list = data?.data ?? resData
    return Array.isArray(list) ? list : []
  }

  it('extracts array from paginated response', () => {
    const apiResponse = {
      data: [{ id: '1' }, { id: '2' }],
      total: 2,
      page: 1,
      limit: 20,
    }
    expect(extractList(apiResponse)).toHaveLength(2)
    expect(extractList(apiResponse)).toEqual([{ id: '1' }, { id: '2' }])
  })

  it('returns empty array when data field is missing', () => {
    expect(extractList({})).toEqual([])
    expect(extractList(null)).toEqual([])
  })

  it('handles direct array response (non-paginated endpoints)', () => {
    const directArray = [{ id: '1' }]
    expect(extractList(directArray)).toHaveLength(1)
  })

  it('old bug: treating paginated response as array returns empty', () => {
    // This documents the bug that was fixed
    const paginatedResponse = {
      data: [{ id: '1' }, { id: '2' }],
      total: 2,
      page: 1,
      limit: 20,
    }
    // Old (buggy) code: Array.isArray(res.data) ? res.data : []
    const oldBugResult = Array.isArray(paginatedResponse) ? paginatedResponse : []
    expect(oldBugResult).toEqual([], 'old code returned empty array — that was the bug')

    // New (fixed) code: res.data?.data
    const fixedResult = extractList(paginatedResponse)
    expect(fixedResult).toHaveLength(2)
  })
})
