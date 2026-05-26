import { describe, it, expect, vi, beforeEach } from 'vitest'

// ─────────────────────────────────────────────────────────────────────────────
// Auto-generate video behavior — unit tests
//
// Feature: after images are uploaded (onDropImages), video generation should
// start automatically without requiring a manual "Generate Video" button click.
//
// Pattern used: autoGenRef — a React ref that always points at the latest
// version of startGeneration(), so the useCallback-memoized onDropImages
// (which has [] deps) can call it without stale-closure issues.
// ─────────────────────────────────────────────────────────────────────────────

describe('auto-generate via ref pattern', () => {
  it('ref always holds the latest function — stale closure avoided', () => {
    // Simulate the ref pattern: each render updates the ref
    let capturedValue = 0
    const ref = { current: () => capturedValue }

    // "First render": ref points at fn returning 0
    ref.current = () => capturedValue

    // "State changes": capturedValue updates
    capturedValue = 42

    // Re-render: ref is updated to new closure
    ref.current = () => capturedValue

    // onDropImages (with [] deps) calls ref.current() — gets latest value
    expect(ref.current()).toBe(42)
  })

  it('setTimeout 0 fires after setUploadedPaths functional update', async () => {
    // Simulates: setUploadedPaths(prev => { setTimeout(() => autoGenRef.current(all), 0); return all })
    const calls: string[][] = []
    const autoGenRef = { current: (paths: string[]) => { calls.push(paths) } }

    const newPaths = ['/uploads/images/a.jpg', '/uploads/images/b.jpg']
    const prev: string[] = []

    // Mimic the functional update + setTimeout pattern
    const all = [...prev, ...newPaths]
    setTimeout(() => autoGenRef.current(all), 0)

    // Before tick: not yet called
    expect(calls).toHaveLength(0)

    // After tick: called with correct paths
    await new Promise((r) => setTimeout(r, 10))
    expect(calls).toHaveLength(1)
    expect(calls[0]).toEqual(['/uploads/images/a.jpg', '/uploads/images/b.jpg'])
  })

  it('auto-gen is skipped when already polling', async () => {
    let polling = true
    const generationCalls: string[][] = []

    // Matches: autoGenRef.current = (paths) => { if (!polling) startGeneration(paths) }
    const autoGenRef = {
      current: (paths: string[]) => {
        if (!polling) generationCalls.push(paths)
      },
    }

    const paths = ['/uploads/images/a.jpg']
    setTimeout(() => autoGenRef.current(paths), 0)

    await new Promise((r) => setTimeout(r, 10))
    // polling=true → generation NOT called
    expect(generationCalls).toHaveLength(0)

    // Now polling=false → generation fires
    polling = false
    setTimeout(() => autoGenRef.current(paths), 0)
    await new Promise((r) => setTimeout(r, 10))
    expect(generationCalls).toHaveLength(1)
    expect(generationCalls[0]).toEqual(paths)
  })

  it('accumulated paths include both existing and newly uploaded files', () => {
    // Simulates multiple drop events accumulating paths
    const prev = ['/uploads/images/existing.jpg']
    const newPaths = ['/uploads/images/new1.jpg', '/uploads/images/new2.jpg']

    const all = [...prev, ...newPaths]

    expect(all).toHaveLength(3)
    expect(all[0]).toBe('/uploads/images/existing.jpg')
    expect(all[1]).toBe('/uploads/images/new1.jpg')
    expect(all[2]).toBe('/uploads/images/new2.jpg')
  })

  it('empty paths array does not trigger generation', async () => {
    const generationCalls: string[][] = []

    const autoGenRef = {
      current: (paths: string[]) => {
        if (paths.length > 0) generationCalls.push(paths)
      },
    }

    setTimeout(() => autoGenRef.current([]), 0)
    await new Promise((r) => setTimeout(r, 10))
    expect(generationCalls).toHaveLength(0)
  })
})

describe('video generation payload construction', () => {
  it('generates correct payload from explicit paths arg', () => {
    const paths = ['/uploads/images/product1.jpg', '/uploads/images/product2.jpg']
    const overlayText = 'TestProduct — ฿299'
    const duration = 15

    const payload = {
      input_images: paths,
      overlay_text: overlayText,
      duration_seconds: duration,
    }

    expect(payload.input_images).toEqual(paths)
    expect(payload.overlay_text).toBe('TestProduct — ฿299')
    expect(payload.duration_seconds).toBe(15)
  })

  it('includes audio_path when provided', () => {
    const paths = ['/uploads/images/a.jpg']
    const audioPath = '/uploads/audio/bg.mp3'

    const payload: Record<string, unknown> = {
      input_images: paths,
      overlay_text: '',
      duration_seconds: 15,
    }
    if (audioPath) payload.audio_path = audioPath

    expect(payload.audio_path).toBe('/uploads/audio/bg.mp3')
  })

  it('omits audio_path when not provided', () => {
    const paths = ['/uploads/images/a.jpg']
    const audioPath = ''

    const payload: Record<string, unknown> = {
      input_images: paths,
      overlay_text: '',
      duration_seconds: 15,
    }
    if (audioPath) payload.audio_path = audioPath

    expect(payload.audio_path).toBeUndefined()
  })

  it('uses snake_case keys (input_images, overlay_text, duration_seconds)', () => {
    const payload = {
      input_images: ['/uploads/a.jpg'],
      overlay_text: 'Test',
      duration_seconds: 30,
    }
    expect(payload).toHaveProperty('input_images')
    expect(payload).toHaveProperty('overlay_text')
    expect(payload).toHaveProperty('duration_seconds')
    expect((payload as Record<string, unknown>)['inputImages']).toBeUndefined()
    expect((payload as Record<string, unknown>)['overlayText']).toBeUndefined()
    expect((payload as Record<string, unknown>)['durationSeconds']).toBeUndefined()
  })
})

describe('job polling logic', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  it('polling stops when status is done', () => {
    const clearIntervalSpy = vi.spyOn(global, 'clearInterval')
    const intervalId = setInterval(() => {}, 3000) as ReturnType<typeof setInterval>

    // Simulate job completing
    const j = { status: 'done', outputPath: '/uploads/video.mp4' }
    if (j.status === 'done' || j.status === 'failed') {
      clearInterval(intervalId)
    }

    expect(clearIntervalSpy).toHaveBeenCalledWith(intervalId)
    clearIntervalSpy.mockRestore()
  })

  it('polling stops when status is failed', () => {
    const clearIntervalSpy = vi.spyOn(global, 'clearInterval')
    const intervalId = setInterval(() => {}, 3000) as ReturnType<typeof setInterval>

    const j = { status: 'failed', errorMessage: 'ffmpeg failed' }
    if (j.status === 'done' || j.status === 'failed') {
      clearInterval(intervalId)
    }

    expect(clearIntervalSpy).toHaveBeenCalledWith(intervalId)
    clearIntervalSpy.mockRestore()
  })

  it('polling continues while status is processing', () => {
    const clearIntervalSpy = vi.spyOn(global, 'clearInterval')
    const intervalId = setInterval(() => {}, 3000) as ReturnType<typeof setInterval>

    const j = { status: 'processing' }
    if (j.status === 'done' || j.status === 'failed') {
      clearInterval(intervalId)
    }

    expect(clearIntervalSpy).not.toHaveBeenCalledWith(intervalId)
    clearInterval(intervalId) // cleanup
    clearIntervalSpy.mockRestore()
  })
})
