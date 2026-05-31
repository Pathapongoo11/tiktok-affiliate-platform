import { useCallback, useEffect, useRef, useState } from 'react'
import { useDropzone } from 'react-dropzone'
import client from '../../api/client'
import type { VideoJob } from '../../types'

export default function VideoStudioPage() {
  const [uploadedPaths, setUploadedPaths] = useState<string[]>([])
  const [overlayText, setOverlayText] = useState('')
  const [scenePrompt, setScenePrompt] = useState('')
  const [duration, setDuration] = useState(15)
  const [animationStyle, setAnimationStyle] = useState('ken_burns')
  const [audioPath, setAudioPath] = useState('')
  const [ttsText, setTtsText] = useState('')
  const [uploading, setUploading] = useState(false)
  const [audioUploading, setAudioUploading] = useState(false)
  const [job, setJob] = useState<VideoJob | null>(null)
  const [polling, setPolling] = useState(false)
  const [error, setError] = useState('')
  const [videoError, setVideoError] = useState('')
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    return () => { if (pollRef.current) clearInterval(pollRef.current) }
  }, [])

  // Core generation logic with explicit paths — called from both auto and manual triggers
  const startGeneration = useCallback(async (paths: string[]) => {
    if (paths.length === 0 || polling) return
    setError('')
    setVideoError('')
    setJob(null)
    setPolling(true)
    try {
      const payload: Record<string, unknown> = {
        input_images: paths,
        overlay_text: overlayText,
        duration_seconds: duration,
        animation_style: animationStyle,
      }
      if (scenePrompt) payload.scene_prompt = scenePrompt
      if (audioPath) payload.audio_path = audioPath
      if (!audioPath && ttsText) payload.tts_text = ttsText
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
  }, [overlayText, scenePrompt, duration, animationStyle, audioPath, ttsText, polling]) // eslint-disable-line react-hooks/exhaustive-deps

  // Upload images only — generation is triggered manually via the Generate button.
  // (Auto-generating on drop fired with a stale animationStyle/scenePrompt before
  //  the user finished choosing them — see BUG 1 in PENDING_PLAN.md.)
  const onDropImages = useCallback(async (files: File[]) => {
    setUploading(true)
    setError('')
    try {
      const newPaths: string[] = []
      for (const file of files) {
        const fd = new FormData()
        fd.append('image', file)
        const res = await client.post('/videos/upload', fd, {
          headers: { 'Content-Type': 'multipart/form-data' },
        })
        newPaths.push(res.data.path || res.data.filePath || res.data.url)
      }
      setUploadedPaths((prev) => [...prev, ...newPaths])
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
      fd.append('audio', file)
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

  // Manual "Generate Video" trigger.
  // AI styles generate from the Scene Prompt and ignore the uploaded photo, but
  // the backend still expects a non-empty input_images array — send a placeholder.
  const generateVideo = () => {
    const isAI = animationStyle.startsWith('ai_')
    const paths = uploadedPaths.length > 0 ? uploadedPaths : (isAI ? ['ai-generated'] : [])
    startGeneration(paths)
  }

  const handleDownload = () => {
    if (!job?.outputPath) return
    const a = document.createElement('a')
    a.href = job.outputPath
    a.download = 'generated-video.mp4'
    a.click()
  }

  // AI styles generate a brand-new image from Scene Prompt (they do NOT use the
  // uploaded photo). FFmpeg styles animate the uploaded photo itself.
  const isAIStyle = animationStyle.startsWith('ai_')

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
            <p className="text-sm text-gray-400 mt-1">JPG, PNG, WebP • Then pick a style and press Generate</p>
          </div>

          {isAIStyle && uploadedPaths.length > 0 && (
            <p className="text-[11px] text-green-700 bg-green-50 border border-green-200 rounded-md px-2 py-1.5">
              ✨ AI จะแปลงรูปสินค้าที่อัปโหลด ให้เป็นการ์ตูน/3D โดยคงรูปสินค้าจริงไว้ (img2img)
            </p>
          )}
          {isAIStyle && uploadedPaths.length === 0 && (
            <p className="text-[11px] text-pink-600 bg-pink-50 border border-pink-200 rounded-md px-2 py-1.5">
              💡 อัปโหลดรูปสินค้า → AI จะแปลงเป็นการ์ตูนโดยคงสินค้าจริงไว้
              (ถ้าไม่อัปรูป จะสร้างภาพใหม่ทั้งหมดจาก "Scene Prompt")
            </p>
          )}

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

          {/* Animation Style Picker */}
          <div className="space-y-2">
            <label className="block text-sm font-medium text-gray-700">Animation Style</label>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
              {[
                { value: 'ken_burns', label: '🔍 Ken Burns',  desc: 'Slow zoom-in' },
                { value: 'zoom_out',  label: '🔭 Zoom Out',   desc: 'Slow zoom-out' },
                { value: 'slide',     label: '↔️ Slide',      desc: 'Pan left→right' },
                { value: 'static',   label: '🖼️ Static',     desc: 'No animation' },
              ].map(({ value, label, desc }) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => setAnimationStyle(value)}
                  className={`flex flex-col items-center gap-0.5 px-2 py-2.5 rounded-lg border text-xs font-medium transition-colors ${
                    animationStyle === value
                      ? 'border-purple-500 bg-purple-50 text-purple-700'
                      : 'border-gray-200 bg-white text-gray-600 hover:border-purple-300 hover:bg-purple-50'
                  }`}
                >
                  <span>{label}</span>
                  <span className="text-gray-400 font-normal">{desc}</span>
                </button>
              ))}
            </div>

            {/* AI-powered styles (require HUGGINGFACE_TOKEN on the server) */}
            <div className="flex items-center gap-2 pt-1">
              <span className="text-xs font-semibold text-purple-600">✨ AI-Powered</span>
              <span className="text-[10px] text-gray-400">(generates a brand-new AI scene image, then animates it)</span>
            </div>
            <div className="grid grid-cols-3 gap-2">
              {[
                { value: 'ai_video',   label: '🤖 AI Motion',  desc: '3D render' },
                { value: 'ai_cartoon', label: '🎨 AI Cartoon', desc: 'Character ad' },
                { value: 'ai_talking', label: '🗣️ AI Talking', desc: 'Lip-sync + voice' },
              ].map(({ value, label, desc }) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => setAnimationStyle(value)}
                  className={`flex flex-col items-center gap-0.5 px-2 py-2.5 rounded-lg border text-xs font-medium transition-colors ${
                    animationStyle === value
                      ? 'border-pink-500 bg-pink-50 text-pink-700'
                      : 'border-gray-200 bg-white text-gray-600 hover:border-pink-300 hover:bg-pink-50'
                  }`}
                >
                  <span>{label}</span>
                  <span className="text-gray-400 font-normal">{desc}</span>
                </button>
              ))}
            </div>

            {/* ai_talking: type Thai text → auto TTS, or upload your own audio below */}
            {animationStyle === 'ai_talking' && (
              <div className="space-y-1.5 pt-2 border-t border-dashed border-pink-200">
                <label className="block text-sm font-medium text-gray-700">
                  🎤 คำพูด (Voice Script) <span className="text-pink-600">— พิมพ์ภาษาไทย ระบบจะสร้างเสียงให้</span>
                </label>
                <textarea
                  value={ttsText}
                  onChange={(e) => setTtsText(e.target.value)}
                  rows={2}
                  disabled={!!audioPath}
                  placeholder="เช่น สวัสดีค่ะ สินค้าตัวนี้ดีมากเลยนะคะ กดสั่งที่ตะกร้าด้านล่างได้เลย"
                  className="w-full px-3 py-2 border border-pink-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-pink-500 disabled:bg-gray-100 disabled:text-gray-400"
                />
                <p className="text-[11px] text-gray-400 leading-snug">
                  {audioPath
                    ? '🔊 ใช้ไฟล์เสียงที่อัปโหลดแล้ว (ลบไฟล์เสียงด้านล่างถ้าอยากพิมพ์ข้อความแทน)'
                    : '💡 พิมพ์ข้อความไทย → ตัวละครจะพูดตามนั้น (เสียงผู้หญิงไทย) หรืออัปโหลดไฟล์เสียงเองด้านล่างก็ได้'}
                </p>
              </div>
            )}

            {/* Scene Prompt — shown for all AI styles */}
            {(animationStyle === 'ai_cartoon' || animationStyle === 'ai_video' || animationStyle === 'ai_talking') && (
              <div className="space-y-1.5 pt-2 border-t border-dashed border-pink-200">
                <label className="block text-sm font-medium text-gray-700">
                  🎬 Scene Prompt <span className="text-pink-600">(English — describe the scene/character)</span>
                </label>
                <textarea
                  value={scenePrompt}
                  onChange={(e) => setScenePrompt(e.target.value)}
                  rows={2}
                  placeholder="e.g. an angry germ monster wearing a crown, sitting on a throne inside the human body"
                  className="w-full px-3 py-2 border border-pink-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-pink-500"
                />
                <p className="text-[11px] text-gray-400 leading-snug">
                  💡 บอกเป็นภาษาอังกฤษว่าอยากได้ตัวละคร/ฉากแบบไหน — AI จะวาดภาพใหม่ตามนี้
                  แล้วเอาข้อความ "Text Overlay" (ไทยได้) ไปแปะทับบนวิดีโอ
                </p>
              </div>
            )}
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

        {/* Generate Button — AI styles don't need an uploaded image (FLUX makes one) */}
        <button
          onClick={generateVideo}
          disabled={(!isAIStyle && uploadedPaths.length === 0) || polling}
          className="w-full py-3 bg-purple-600 text-white rounded-xl text-base font-semibold hover:bg-purple-700 disabled:opacity-60 disabled:cursor-not-allowed transition-colors flex items-center justify-center gap-2"
        >
          {polling ? (
            <>
              <div className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin" />
              Generating Video...
            </>
          ) : job?.status === 'done' ? '🔄 Re-generate Video' : '🎬 Generate Video'}
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
                    key={job.outputPath}
                    src={job.outputPath}
                    controls
                    autoPlay
                    playsInline
                    className="w-full max-h-96 object-contain"
                    onError={() => setVideoError(
                      `Cannot load video from ${job.outputPath} — make sure the API server is running and the file was generated successfully.`
                    )}
                    onLoadedData={() => setVideoError('')}
                  />
                </div>
                {videoError && (
                  <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg text-sm">
                    <p className="font-medium">Video load failed</p>
                    <p className="mt-1 text-xs break-all">{videoError}</p>
                    <a
                      href={job.outputPath}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="mt-2 inline-block text-blue-600 underline text-xs"
                    >
                      Try opening directly ↗
                    </a>
                  </div>
                )}
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
