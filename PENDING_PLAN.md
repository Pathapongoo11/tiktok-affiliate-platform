# 📋 Pending Plan — Video Studio Fixes

**บันทึกเมื่อ:** 31 พ.ค. 2026
**สถานะ:** ยังไม่แก้ — รอทำต่อ

---

## 🐛 BUG 1 — Auto-gen ใช้ค่า Animation Style เก่า (upload-first)

### อาการ (ที่ user เจอ)
- ถ้า **ใส่รูปก่อน → แล้วค่อยเลือก Animation Style** → มัน auto-generate ทันทีด้วย style เก่า (default `ken_burns`) ตั้งแต่ตอน upload เสร็จ
- พอเลือก style ใหม่แล้วกด **Re-generate** → เหมือนติด cache ได้ค่าเดิมกลับมา
- ถ้า **ใส่ prompt/เลือก style ก่อน → แล้วค่อยใส่รูป** → ทำงานถูก

### Root cause (ยืนยันจากโค้ด)
`VideoStudioPage.tsx` line 86-91 — `onDropImages` เรียก auto-gen ทันทีหลัง upload:
```ts
setUploadedPaths((prev) => {
  const all = [...prev, ...newPaths]
  setTimeout(() => autoGenRef.current(all), 0)  // ← ยิงทันที ด้วย animationStyle ปัจจุบัน
  return all
})
```
ตอน upload เสร็จ `animationStyle` ยังเป็น default `'ken_burns'` (user ยังไม่ทันเลือก) → auto-gen เลยใช้ ken_burns

### วิธีแก้ (เลือก)
- **Option A (แนะนำ):** เอา auto-generate ออกจาก `onDropImages` — ให้ user กดปุ่ม Generate เองหลังเลือก style/ใส่ prompt ครบ
- **Option B:** ถ้าจะคง auto-gen ไว้ → อย่า auto-gen เมื่อ style เป็น AI (`ai_*`) เพราะต้องรอ scene_prompt
- **Option C:** เพิ่ม debounce + เช็คว่า user แตะ style picker แล้วหรือยัง

### ไฟล์ที่ต้องแก้
- `frontend/src/modules/video-studio/VideoStudioPage.tsx` (onDropImages, line 73-97)
- `frontend/src/modules/posts/ContentEditor.tsx` (มี autoGen pattern เดียวกัน — เช็คด้วย)

---

## 🐛 BUG 2 — AI styles ไม่ใช้รูปที่ user อัปโหลด (สับสน)

### อาการ (ที่ user เจอ)
- อัปโหลดรูปสินค้าเข้าไป แต่ตัวการ์ตูนที่ออกมา **ไม่เกี่ยวกับรูปที่อัป** เลย
- เหมือนระบบไม่หยิบรูปนั้นมาใช้

### Root cause (BY DESIGN — แต่ UX สับสน)
สำหรับ `ai_cartoon` / `ai_video` / `ai_talking`:
- FLUX **สร้างภาพใหม่ทั้งหมด** จาก `scene_prompt` (text→image)
- **ไม่ได้ใช้รูปที่ user อัปเลย** — รูปที่อัปใช้แค่กับ ffmpeg styles (ken_burns/zoom/slide/static)

→ ไม่ใช่บั๊ก แต่ user คาดหวังว่ารูปสินค้าจะถูกใช้ → ต้องแก้ UX

### วิธีแก้ (เลือก)
- **Option A:** เพิ่มข้อความใน UI ว่า "AI styles สร้างภาพใหม่จาก Scene Prompt — ไม่ใช้รูปที่อัป"
- **Option B (ดีกว่า):** ใช้ FLUX **image-to-image** — เอารูปสินค้าที่อัปเป็น input ให้ FLUX ดัดแปลงเป็นการ์ตูน (รักษาเค้าโครงสินค้า)
  - ต้องเช็คว่า hf-inference รองรับ img2img ของ FLUX ไหม (ก่อนหน้านี้ img2img ถูกปิด — ต้องเช็คใหม่)
- **Option C:** แยกให้ชัด — AI styles ซ่อนช่อง upload, ffmpeg styles ซ่อนช่อง scene_prompt

### ไฟล์ที่ต้องแก้
- `frontend/.../VideoStudioPage.tsx` — UI logic แยก AI vs ffmpeg
- `api/modules/videos/huggingface.go` — ถ้าทำ img2img

---

## 🐛 BUG 3 — SadTalker หาใบหน้าในภาพ FLUX ไม่เจอ (ai_talking fail)

### อาการ
- `ai_talking` → job failed: `IndexError: bboxes size 0` = 0 faces detected
- FLUX สร้าง "หน้าการ์ตูน" ที่ face detector (ออกแบบมาจับหน้าคนจริง) จับไม่ได้

### Root cause
- SadTalker ใช้ face detector มาตรฐาน (มนุษย์) — การ์ตูน Pixar/3D สัดส่วนหน้าไม่ตรง → detect ไม่เจอ
- ภาพที่ fail อยู่ที่ `Downloads\FLUX_character_that_failed.png`

### วิธีแก้ (เลือก)
- **Option A (เร็วสุด):** แก้ prompt ให้ FLUX สร้าง portrait หน้าคนเหมือนจริงมากขึ้น
  เพิ่ม keyword: `"extreme close-up portrait, photorealistic human face, centered face, looking straight at camera"`
  ลด keyword การ์ตูน 3D ลง
- **Option B:** เปลี่ยน detector ใน SadTalker เป็นตัวรับ anime/cartoon face
- **Option C:** ให้ ai_talking ใช้รูป portrait คนจริงที่ user อัป (ข้าม FLUX) → ส่งเข้า SadTalker ตรงๆ
  (เหมาะกับ use case "พรีเซนเตอร์พูด" มากกว่า)

### ไฟล์ที่ต้องแก้
- `api/modules/videos/huggingface.go` — `buildCartoonPrompt` (ปรับ prompt สำหรับ ai_talking)
- หรือ `services/lipsync/SadTalker/...` — เปลี่ยน detector

---

## ✅ สิ่งที่ทำงานได้แล้ว (อย่าแตะ)
- ai_cartoon / ai_video — FLUX สร้างตัวละคร + Ken Burns ✓ (เทสต์ผ่าน)
- ai_talking — SadTalker lip-sync ทำงานได้กับ **รูปหน้าคนจริง** (art_0.png ผ่าน)
- microservice + fallback logic ✓
- Go API + DB + migration ✓

---

## 🎯 ลำดับแนะนำตอนทำต่อ
1. **BUG 1 ก่อน** (เร็ว, กระทบ UX มากสุด) — เอา auto-gen ออก/แก้เงื่อนไข
2. **BUG 3** — แก้ prompt ai_talking ให้ได้หน้าที่ detect ได้ (Option A)
3. **BUG 2** — แยก UI AI vs ffmpeg ให้ชัด (Option C) หรือทำ img2img (Option B)
4. **แล้วค่อยทำ TTS ไทย** (edge-tts) — พิมพ์ข้อความ→เสียง→ตัวละครพูดไทย

---

## 📝 งานที่ค้างอื่นๆ (ยังไม่ทำ)
- **TTS ไทยฟรี** (edge-tts) — แปลงข้อความไทย → ไฟล์เสียง อัตโนมัติ (ไม่ต้องอัดเอง)
- PR #18 (smoke PNG generator) — ยัง open อยู่ เช็คว่าต้อง merge ไหม
- TikTok real API — ต้องการ credentials จริง (ยัง mock mode)
