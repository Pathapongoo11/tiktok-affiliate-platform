import { useCallback, useEffect, useRef, useState } from 'react'
import { useDropzone } from 'react-dropzone'
import client from '../../api/client'
import type { VideoJob } from '../../types'

export default function VideoStudioPage() {
  const [uploadedPaths, setUploadedPaths] = useState<string[]>([])
  const [overlayText, setOverlayText] = useState('')
  const [duration, setDuration] = useState(15)
  const [audioPath, setAudioPath] = useState('')
  const [uploading, setUploading] = useState(false)
  const [audioUploading, setAudioUploading] = useState(false)
  const [job, setJob] = useState<VideoJob | null>(null)
  const [polling, setPolling] = useState(false)
  const [error, setError] = useState('')
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    return () => { if (pollRef.current) clearInterval(pollRef.current) }
  }, [])

  const onDropImages = useCallback(async (files: File[]) => {
    setUploading(true)
    setError('')
    try {
      const paths: string[] = []
      for (const file of files) {
        const fd = new FormData()
        fd.append('file', file)
        const res = await client.post('/videos/upload', fd, {
          headers: { 'Content-Type': 'multipart/form-data' },
        })
        paths.push(res.data.path || res.data.filePath || res.data.url)
      }
      setUploadedPaths((prev) => [...prev, ...paths])
    } catch {
      setError('Image upload failed.')
    } finally {
      setUploading(false)
    }
  }, [])

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop: onDropImages,
    accept: { 'image/*': [] },
    multiple: true,
  })

  const handleAudioUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setAudioUploading(true)
    setError('')
    try {
      const fd = new FormData()
      fd.append('file', file)
      const res = await client.post('/videos/upload', fd, {
        headers: { 'Content-Type': 'multipart/form-data' },
      })
      setAudioPath(res.data.path || res.data.filePath || res.data.url)
    } catch {
      setError('Audio upload failed.')
    } finally {
      setAudioUploading(false)
    }
  }

  const generateVideo = async () => {
    if (uploadedPaths.length === 0) {
      setError('Please upload at least one image.')
      return
    }
    setError('')
    setJob(null)
    setPolling(true)
    try {
      const payload: Record<string, unknown> = {
        imagePaths: uploadedPaths,
        overlayText,
        durationSeconds: duration,
      }
      if (audioPath) payload.audioPath = audioPath
      const res = await client.post('/videos/generate', payload)
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
          setError('Polling failed.')
        }
      }, 3000)
    } catch {
      setPolling(false)
      setError('Failed to start video generation.')
    }
  }

  const handleDownload = () => {
    if (!job?.outputPath) return
    const a = document.createElement('a')
    a.href = job.outputPath
    a.download = 'generated-video.mp4'
    a.click()
  }

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-gray-900">Video Studio</h2>
        <p className="text-gray-500 text-sm mt-1">Create product showcase videos for TikTok</p>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg text-sm">
          {error}
        </div>
      )}

      <div className="grid gap-6">
        {/* Image Upload */}
        <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-4">
          <h3 className="font-semibold text-gray-800">Product Images</h3>
          <div
            {...getRootProps()}
            className={`border-2 border-dashed rounded-xl p-10 text-center cursor-pointer transition-colors ${
              isDragActive ? 'border-purple-400 bg-purple-50' : 'border-gray-300 hover:border-purple-300 hover:bg-gray-50'
            }`}
          >
            <input {...getInputProps()} />
            <div className="text-5xl mb-3">🖼️</div>
            <p className="text-gray-600 font-medium">
              {isDragActive ? 'Drop images here...' : 'Drag & drop images'}
            </p>
            <p className="text-sm text-gray-400 mt-1">JPG, PNG, WebP • Multiple files supported</p>
          </div>

          {uploading && (
            <p className="text-sm text-purple-600 text-center animate-pulse">Uploading...</p>
          )}

          {uploadedPaths.length > 0 && (
            <div className="flex items-start justify-between bg-green-50 border border-green-200 rounded-lg p-3">
              <div>
                <p className="text-sm font-medium text-green-700">{uploadedPaths.length} image(s) ready</p>
                {uploadedPaths.map((p, i) => (
                  <p key={i} className="text-xs text-green-600 truncate max-w-xs">{p}</p>
                ))}
              </div>
              <button
                onClick={() => setUploadedPaths([])}
                className="text-xs text-red-400 hover:text-red-600 shrink-0 ml-3"
              >
                Clear
              </button>
            </div>
          )}
        </div>

        {/* Settings */}
        <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-5">
          <h3 className="font-semibold text-gray-800">Video Settings</h3>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Text Overlay</label>
            <input
              type="text"
              value={overlayText}
              onChange={(e) => setOverlayText(e.target.value)}
              placeholder="e.g. Product Name — ฿299"
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
            />
          </div>

          <div className="space-y-2">
            <label className="block text-sm font-medium text-gray-700">
              Duration: <span className="text-purple-600 font-semibold">{duration}s</span>
            </label>
            <input
              type="range"
              min={15}
              max={60}
              step={5}
              value={duration}
              onChange={(e) => setDuration(Number(e.target.value))}
              className="w-full accent-purple-600"
            />
            <div className="flex justify-between text-xs text-gray-400">
              <span>15s</span>
              <span>30s</span>
              <span>45s</span>
              <span>60s</span>
            </div>
          </div>

          <div className="space-y-2">
            <label className="block text-sm font-medium text-gray-700">Background Audio (optional)</label>
            <div className="flex items-center gap-3">
              <label className="cursor-pointer">
                <span className="px-4 py-2 bg-gray-100 hover:bg-gray-200 text-gray-700 text-sm rounded-lg transition-colors inline-block">
                  {audioUploading ? 'Uploading...' : 'Upload Audio'}
                </span>
                <input
                  type="file"
                  accept="audio/*"
                  className="hidden"
                  onChange={handleAudioUpload}
                  disabled={audioUploading}
                />
              </label>
              {audioPath && (
                <span className="text-xs text-green-600">✅ Audio ready</span>
              )}
            </div>
          </div>
        </div>

        {/* Generate Button */}
        <button
          onClick={generateVideo}
          disabled={uploadedPaths.length === 0 || polling}
          className="w-full py-3 bg-purple-600 text-white rounded-xl text-base font-semibold hover:bg-purple-700 disabled:opacity-60 disabled:cursor-not-allowed transition-colors flex items-center justify-center gap-2"
        >
          {polling ? (
            <>
              <div className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin" />
              Generating Video...
            </>
          ) : '🎬 Generate Video'}
        </button>

        {/* Job Status */}
        {job && (
          <div className={`bg-white rounded-xl border shadow-sm p-6 space-y-4 ${
            job.status === 'done' ? 'border-green-200' :
            job.status === 'failed' ? 'border-red-200' :
            'border-blue-200'
          }`}>
            <div className="flex items-center justify-between">
              <h3 className="font-semibold text-gray-800">
                {job.status === 'done' ? '✅ Video Ready' :
                 job.status === 'failed' ? '❌ Generation Failed' :
                 '⏳ Processing...'}
              </h3>
              {(job.status === 'pending' || job.status === 'processing') && (
                <div className="w-5 h-5 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
              )}
            </div>

            {job.status === 'done' && job.outputPath && (
              <>
                <div className="bg-gray-900 rounded-xl overflow-hidden">
                  <video
                    src={job.outputPath}
                    controls
                    className="w-full max-h-80 object-contain"
                  />
                </div>
                <button
                  onClick={handleDownload}
                  className="w-full py-2.5 bg-green-600 text-white rounded-lg text-sm font-medium hover:bg-green-700 transition-colors"
                >
                  ⬇️ Download MP4
                </button>
              </>
            )}

            {job.status === 'failed' && job.errorMessage && (
              <p className="text-sm text-red-600">{job.errorMessage}</p>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
