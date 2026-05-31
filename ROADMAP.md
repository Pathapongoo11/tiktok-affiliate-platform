# 🎯 3 Long-Running Goals — TikTok Affiliate Platform

**Set:** 31 May 2026
**Context:** Basket-pinning TikTok affiliate. Platform generates product videos
(FFmpeg + AI cartoon/talking via FLUX + SadTalker on local RTX 4060, free).
All core features work; now maximize business output.

These 3 goals are ranked by **revenue impact per effort** and each is a long
autonomous track of work.

---

## 🥇 GOAL 1 — Real product likeness (FLUX img2img)
**Why #1:** Right now AI styles invent a brand-new image from a text prompt and
ignore the uploaded product photo (PENDING_PLAN BUG 2). For affiliate sales the
video MUST show the *actual* product. This is the single biggest quality gap.

**Definition of done:**
- AI cartoon/video uses the uploaded product photo as an img2img base (FLUX or
  an SDXL img2img model on the local GPU, since HF free tier dropped img2img)
- Product shape/label is recognizably preserved, stylized into cartoon/3D
- Frontend: AI styles re-enable the image upload as the subject
- Tests + live verification with the beverage photo

**Approach:** run an SDXL/FLUX img2img pipeline inside the existing lipsync
microservice (already has the GPU + Python venv) → new `/img2img` endpoint.

---

## 🥈 GOAL 2 — Autonomous content factory (batch + calendar)
**Why #2:** The platform's whole point is producing volume. A batch pipeline
that turns a product list into a week of scripts + videos compounds over time.

**Definition of done:**
- "Batch generate" — pick N products → auto-produce N videos (script → caption →
  hooks → video) using the existing tiktok-script-gen / hook-writer skills
- 7-day calendar view; each slot a ready-to-post video + caption + hashtags
- Re-runnable on a schedule (the /loop or scheduled-task tooling)

**Approach:** new `batch` module that orchestrates posts + videos services;
reuse caption_gen + the content skills.

---

## 🥉 GOAL 3 — Go live on real TikTok (auto-post + analytics loop)
**Why #3:** Closes the loop — scheduled posts actually publish, analytics sync
back, dashboard shows what sells → feeds product selection. Blocked only on
TikTok dev credentials (code is ready, currently mock mode).

**Definition of done:**
- Real OAuth verified with live TikTok credentials
- Scheduler auto-publishes due posts; publish-status polling captures video_id
- Analytics sync populates the dashboard from real metrics
- Product performance ranking drives next batch's product picks

**Approach:** the TikTok module already has real-API code paths behind mockMode;
needs credentials + GetUserInfo + publish-status polling (noted earlier).

---

## Execution order
1. **GOAL 1** now — biggest quality lift, fully local/free, no external blockers.
2. GOAL 2 — build on working video gen to scale output.
3. GOAL 3 — when TikTok credentials are available.

## Done / foundation (do not redo)
- FFmpeg styles (ken_burns/zoom/slide/static) ✓
- AI cartoon/video via FLUX text→image ✓
- AI talking via SadTalker lip-sync on local GPU ✓
- Thai TTS (edge-tts) ✓
- Schedule fix, smoke tests, NULL fix, scene_prompt ✓
