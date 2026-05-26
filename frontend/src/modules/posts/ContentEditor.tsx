import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useDropzone } from 'react-dropzone'
import client from '../../api/client'
import type { Product, VideoJob } from '../../types'

const STEPS = ['Select Product', 'Caption & Hashtags', 'Generate Video', 'Preview', 'Schedule']
const SUGGESTED_TIMES = ['07:00', '12:00', '19:00', '21:00']

export default function ContentEditor() {
  const navigate = useNavigate()
  const [step, setStep] = useState(0)
  const [products, setProducts] = useState<Product[]>([])
  const [selectedProduct, setSelectedProduct] = useState<Product | null>(null)
  const [caption, setCaption] = useState('')
  const [hashtagInput, setHashtagInput] = useState('')
  const [hashtags, setHashtags] = useState<string[]>([])
  const [uploadedPaths, setUploadedPaths] = useState<string[]>([])
  const [job, setJob] = useState<VideoJob | null>(null)
  const [polling, setPolling] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [scheduledAt, setScheduledAt] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    client.get('/products').then((res) => {
      const list = res.data?.data ?? res.data
      setProducts(Array.isArray(list) ? list : [])
    })
    return () => { if (pollRef.current) clearInterval(pollRef.current) }
  }, [])

  const onDrop = useCallback(async (files: File[]) => {
    setUploading(true)
    setError('')
    try {
      const paths: string[] = []
      for (const file of files) {
        const fd = new FormData()
        fd.append('image', file)
        const res = await client.post('/videos/upload', fd, {
          headers: { 'Content-Type': 'multipart/form-data' },
        })
        paths.push(res.data.path || res.data.filePath || res.data.url)
      }
      setUploadedPaths((prev) => [...prev, ...paths])
    } catch {
      setError('Image upload failed. Please try again.')
    } finally {
      setUploading(false)
    }
  }, [])

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    accept: { 'image/*': [] },
    multiple: true,
  })

  const generateVideo = async () => {
    if (uploadedPaths.length === 0) {
      setError('Please upload at least one image.')
      return
    }
    setError('')
    setPolling(true)
    try {
      const overlayText = selectedProduct
        ? `${selectedProduct.name} — ฿${selectedProduct.price}`
        : ''
      const res = await client.post('/videos/generate', {
        input_images: uploadedPaths,
        overlay_text: overlayText,
        duration_seconds: 15,
      })
      const jobId = res.data.id || res.data.jobId
      pollRef.current = setInterval(async () => {
        try {
          const jobRes = await client.get(`/videos/${jobId}`)
          const j: VideoJob = jobRes.data
          setJob(j)
          if (j.status === 'done' || j.status === 'failed') {
            clearInterval(pollRef.current!)
            setPolling(false)
          }
        } catch {
          clearInterval(pollRef.current!)
          setPolling(false)
          setError('Failed to poll job status.')
        }
      }, 3000)
    } catch {
      setPolling(false)
      setError('Failed to start video generation.')
    }
  }

  const addHashtag = () => {
    const tag = hashtagInput.trim().replace(/^#/, '')
    if (tag && !hashtags.includes(tag)) {
      setHashtags([...hashtags, tag])
    }
    setHashtagInput('')
  }

  const removeHashtag = (tag: string) => setHashtags(hashtags.filter((t) => t !== tag))

  const handleSchedule = async (postNow: boolean) => {
    setError('')
    setSubmitting(true)
    try {
      const postRes = await client.post('/posts', {
        title: selectedProduct?.name || 'New Post',
        caption,
        hashtags,
        video_path: job?.outputPath || '',
        product_id: selectedProduct?.id,
      })
      const postId = postRes.data.id || postRes.data.postId
      if (postNow) {
        await client.post(`/posts/${postId}/schedule`, { scheduled_at: new Date().toISOString() })
      } else if (scheduledAt) {
        await client.post(`/posts/${postId}/schedule`, { scheduled_at: scheduledAt })
      }
      navigate('/posts')
    } catch {
      setError('Failed to save post. Please try again.')
      setSubmitting(false)
    }
  }

  const setQuickTime = (time: string) => {
    const today = new Date()
    const [h, m] = time.split(':')
    today.setHours(parseInt(h), parseInt(m), 0)
    setScheduledAt(today.toISOString().slice(0, 16))
  }

  const canAdvance = () => {
    if (step === 0) return selectedProduct !== null
    if (step === 2) return job !== null || uploadedPaths.length > 0
    return true
  }

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-gray-900">Create New Post</h2>
        <p className="text-gray-500 text-sm mt-1">Follow the steps to create and schedule your content</p>
      </div>

      {/* Step indicator */}
      <div className="flex items-center gap-2 overflow-x-auto pb-2">
        {STEPS.map((label, i) => (
          <div key={i} className="flex items-center gap-2 shrink-0">
            <div className={`flex items-center justify-center w-8 h-8 rounded-full text-sm font-bold ${
              i < step ? 'bg-green-500 text-white' :
              i === step ? 'bg-purple-600 text-white' :
              'bg-gray-200 text-gray-500'
            }`}>
              {i < step ? '✓' : i + 1}
            </div>
            <span className={`text-sm ${i === step ? 'font-semibold text-gray-900' : 'text-gray-400'}`}>
              {label}
            </span>
            {i < STEPS.length - 1 && <div className="w-6 h-px bg-gray-200" />}
          </div>
        ))}
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg text-sm">
          {error}
        </div>
      )}

      <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-5">
        {/* Step 1: Select Product */}
        {step === 0 && (
          <div className="space-y-4">
            <h3 className="font-semibold text-gray-800">Select Product</h3>
            <select
              value={selectedProduct?.id || ''}
              onChange={(e) => {
                const p = products.find((pr) => pr.id === e.target.value) || null
                setSelectedProduct(p)
              }}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
            >
              <option value="">Choose a product...</option>
              {products.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name} — ฿{p.price} ({p.commissionRate}%)
                </option>
              ))}
            </select>
            {selectedProduct && (
              <div className="bg-purple-50 rounded-lg p-4 space-y-1">
                <p className="font-medium text-purple-800">{selectedProduct.name}</p>
                <p className="text-sm text-purple-600">Price: ฿{selectedProduct.price.toLocaleString()}</p>
                <p className="text-sm text-purple-600">Commission: {selectedProduct.commissionRate}%</p>
                <p className="text-sm text-purple-600">Score: {selectedProduct.score}/100</p>
              </div>
            )}
          </div>
        )}

        {/* Step 2: Caption & Hashtags */}
        {step === 1 && (
          <div className="space-y-4">
            <h3 className="font-semibold text-gray-800">Write Caption & Hashtags</h3>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Caption</label>
              <textarea
                value={caption}
                onChange={(e) => setCaption(e.target.value)}
                placeholder="Write your TikTok caption here..."
                rows={5}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 resize-none"
              />
            </div>
            <div className="space-y-2">
              <label className="block text-sm font-medium text-gray-700">Hashtags</label>
              <div className="flex gap-2">
                <input
                  value={hashtagInput}
                  onChange={(e) => setHashtagInput(e.target.value)}
                  placeholder="Add hashtag (without #)"
                  onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addHashtag() } }}
                  className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
                />
                <button
                  onClick={addHashtag}
                  className="px-4 py-2 bg-purple-100 text-purple-700 rounded-lg text-sm font-medium hover:bg-purple-200 transition-colors"
                >
                  Add
                </button>
              </div>
              {hashtags.length > 0 && (
                <div className="flex flex-wrap gap-2 pt-1">
                  {hashtags.map((tag) => (
                    <span
                      key={tag}
                      className="inline-flex items-center gap-1 px-3 py-1 bg-purple-100 text-purple-700 rounded-full text-sm"
                    >
                      #{tag}
                      <button
                        onClick={() => removeHashtag(tag)}
                        className="text-purple-400 hover:text-purple-700 leading-none ml-1"
                      >
                        ×
                      </button>
                    </span>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}

        {/* Step 3: Generate Video */}
        {step === 2 && (
          <div className="space-y-4">
            <h3 className="font-semibold text-gray-800">Generate Video</h3>
            <div
              {...getRootProps()}
              className={`border-2 border-dashed rounded-xl p-8 text-center cursor-pointer transition-colors ${
                isDragActive ? 'border-purple-400 bg-purple-50' : 'border-gray-300 hover:border-purple-300 hover:bg-gray-50'
              }`}
            >
              <input {...getInputProps()} />
              <div className="text-4xl mb-3">🖼️</div>
              <p className="text-gray-600 font-medium">
                {isDragActive ? 'Drop images here...' : 'Drag & drop product images'}
              </p>
              <p className="text-sm text-gray-400 mt-1">or click to browse • Multiple images supported</p>
            </div>

            {uploading && <p className="text-sm text-purple-600 text-center">Uploading images...</p>}

            {uploadedPaths.length > 0 && (
              <div className="bg-green-50 border border-green-200 rounded-lg p-3">
                <p className="text-sm text-green-700 font-medium">{uploadedPaths.length} image(s) uploaded</p>
                {uploadedPaths.map((p, i) => (
                  <p key={i} className="text-xs text-green-600 truncate">{p}</p>
                ))}
              </div>
            )}

            {job && (
              <div className={`rounded-lg p-4 ${
                job.status === 'done' ? 'bg-green-50 border border-green-200' :
                job.status === 'failed' ? 'bg-red-50 border border-red-200' :
                'bg-blue-50 border border-blue-200'
              }`}>
                <div className="flex items-center gap-2">
                  {(job.status === 'pending' || job.status === 'processing') && (
                    <div className="w-4 h-4 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
                  )}
                  <p className={`text-sm font-medium ${
                    job.status === 'done' ? 'text-green-700' :
                    job.status === 'failed' ? 'text-red-700' :
                    'text-blue-700'
                  }`}>
                    Video {job.status === 'done' ? 'ready!' : job.status === 'failed' ? 'failed' : 'processing...'}
                  </p>
                </div>
                {job.errorMessage && <p className="text-xs text-red-600 mt-1">{job.errorMessage}</p>}
              </div>
            )}

            <button
              onClick={generateVideo}
              disabled={uploadedPaths.length === 0 || polling || job?.status === 'done'}
              className="w-full py-2.5 bg-purple-600 text-white rounded-lg text-sm font-medium hover:bg-purple-700 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
            >
              {polling ? 'Generating...' : job?.status === 'done' ? '✅ Video Ready!' : '🎬 Generate Video'}
            </button>
          </div>
        )}

        {/* Step 4: Preview */}
        {step === 3 && (
          <div className="space-y-4">
            <h3 className="font-semibold text-gray-800">Preview</h3>
            {job?.status === 'done' && job.outputPath ? (
              <div className="bg-gray-900 rounded-xl overflow-hidden">
                <video
                  src={job.outputPath}
                  controls
                  className="w-full max-h-96 object-contain"
                />
              </div>
            ) : (
              <div className="bg-gray-100 rounded-xl p-8 text-center text-gray-400">
                <div className="text-4xl mb-2">🎬</div>
                <p>No video generated yet</p>
              </div>
            )}
            <div className="bg-gray-50 rounded-lg p-4 space-y-2">
              <p className="text-sm font-medium text-gray-700">Caption:</p>
              <p className="text-sm text-gray-600">{caption || 'No caption'}</p>
              {hashtags.length > 0 && (
                <div className="flex flex-wrap gap-1 pt-1">
                  {hashtags.map((tag) => (
                    <span key={tag} className="text-sm text-purple-600">#{tag}</span>
                  ))}
                </div>
              )}
            </div>
            {selectedProduct && (
              <div className="bg-purple-50 rounded-lg p-3">
                <p className="text-sm font-medium text-purple-800">Product: {selectedProduct.name}</p>
                <p className="text-xs text-purple-600">฿{selectedProduct.price} • {selectedProduct.commissionRate}% commission</p>
              </div>
            )}
          </div>
        )}

        {/* Step 5: Schedule */}
        {step === 4 && (
          <div className="space-y-4">
            <h3 className="font-semibold text-gray-800">Schedule Post</h3>
            <div>
              <p className="text-sm font-medium text-gray-700 mb-2">Suggested times (ICT — best Thai posting times):</p>
              <div className="flex gap-2 flex-wrap">
                {SUGGESTED_TIMES.map((t) => (
                  <button
                    key={t}
                    onClick={() => setQuickTime(t)}
                    className="px-4 py-2 rounded-lg border border-purple-200 text-purple-700 text-sm font-medium hover:bg-purple-50 transition-colors"
                  >
                    {t}
                  </button>
                ))}
              </div>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Schedule Date & Time</label>
              <input
                type="datetime-local"
                value={scheduledAt}
                onChange={(e) => setScheduledAt(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
              />
            </div>
            <div className="flex gap-3 pt-2">
              <button
                onClick={() => handleSchedule(false)}
                disabled={submitting || !scheduledAt}
                className="flex-1 py-2.5 border border-gray-300 text-gray-700 rounded-lg text-sm font-medium hover:bg-gray-50 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
              >
                {submitting ? 'Saving...' : 'Schedule'}
              </button>
              <button
                onClick={() => handleSchedule(true)}
                disabled={submitting}
                className="flex-1 py-2.5 bg-purple-600 text-white rounded-lg text-sm font-medium hover:bg-purple-700 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
              >
                {submitting ? 'Posting...' : 'Post Now'}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Navigation */}
      <div className="flex justify-between">
        <button
          onClick={() => step === 0 ? navigate('/posts') : setStep(step - 1)}
          className="px-4 py-2 border border-gray-300 text-gray-700 rounded-lg text-sm font-medium hover:bg-gray-50 transition-colors"
        >
          {step === 0 ? 'Cancel' : '← Back'}
        </button>
        {step < STEPS.length - 1 && (
          <button
            onClick={() => { setError(''); setStep(step + 1) }}
            disabled={!canAdvance()}
            className="px-4 py-2 bg-purple-600 text-white rounded-lg text-sm font-medium hover:bg-purple-700 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
          >
            Next →
          </button>
        )}
      </div>
    </div>
  )
}
