"""
Lip-Sync Microservice — SadTalker wrapper (FastAPI)

Runs on the Windows host with GPU access (RTX 4060). The Go API (inside Docker)
calls this service over host.docker.internal:8001 to turn a still character image
+ an audio clip into a talking-head MP4.

Pipeline:
    POST /generate  (multipart: image, audio)  →  202 + job_id
    GET  /jobs/{id}                             →  status + output path when done
    GET  /health                                →  readiness + GPU info

SadTalker itself is invoked as a subprocess (inference.py) so we don't have to
import its (heavy, version-sensitive) module graph into this process. The path to
the SadTalker checkout is configured via the SADTALKER_DIR env var.

This service is intentionally dependency-light (FastAPI + uvicorn) so it starts
fast; the GPU-heavy work lives entirely inside the SadTalker subprocess.
"""

from __future__ import annotations

import os
import shutil
import subprocess
import sys
import threading
import uuid
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Literal

from fastapi import FastAPI, File, HTTPException, UploadFile
from fastapi.responses import FileResponse, JSONResponse

# ── Configuration ────────────────────────────────────────────────────────────

SADTALKER_DIR = Path(os.getenv("SADTALKER_DIR", "./SadTalker")).resolve()
WORK_DIR = Path(os.getenv("LIPSYNC_WORK_DIR", "./work")).resolve()
PYTHON_BIN = os.getenv("SADTALKER_PYTHON", sys.executable)
# "full" = full face render (slower, best). "crop"/"resize" are faster.
PREPROCESS = os.getenv("SADTALKER_PREPROCESS", "full")

WORK_DIR.mkdir(parents=True, exist_ok=True)

app = FastAPI(title="Lip-Sync Microservice", version="1.0.0")


# ── Job tracking (in-memory; single-host service) ────────────────────────────

JobStatus = Literal["pending", "processing", "done", "failed"]


@dataclass
class Job:
    id: str
    status: JobStatus = "pending"
    output_path: str | None = None
    error: str | None = None
    created_at: str = field(default_factory=lambda: datetime.now(timezone.utc).isoformat())


_jobs: dict[str, Job] = {}
_jobs_lock = threading.Lock()


def _set_job(job_id: str, **changes) -> None:
    with _jobs_lock:
        job = _jobs[job_id]
        for k, v in changes.items():
            setattr(job, k, v)


# ── SadTalker invocation ─────────────────────────────────────────────────────

def _run_sadtalker(job_id: str, image_path: Path, audio_path: Path, job_dir: Path) -> None:
    """Run SadTalker inference.py as a subprocess and record the resulting MP4."""
    _set_job(job_id, status="processing")
    try:
        result_dir = job_dir / "result"
        result_dir.mkdir(parents=True, exist_ok=True)

        cmd = [
            PYTHON_BIN,
            str(SADTALKER_DIR / "inference.py"),
            "--driven_audio", str(audio_path),
            "--source_image", str(image_path),
            "--result_dir", str(result_dir),
            "--preprocess", PREPROCESS,
            "--still",            # less head motion → more stable for product mascots
            "--enhancer", "gfpgan",
        ]

        proc = subprocess.run(
            cmd,
            cwd=str(SADTALKER_DIR),
            capture_output=True,
            text=True,
            timeout=int(os.getenv("SADTALKER_TIMEOUT", "900")),  # 15 min hard cap
        )
        if proc.returncode != 0:
            tail = (proc.stderr or proc.stdout or "")[-800:]
            _set_job(job_id, status="failed", error=f"sadtalker exited {proc.returncode}: {tail}")
            return

        # SadTalker writes an .mp4 somewhere under result_dir; find the newest one.
        mp4s = sorted(result_dir.rglob("*.mp4"), key=lambda p: p.stat().st_mtime, reverse=True)
        if not mp4s:
            _set_job(job_id, status="failed", error="sadtalker produced no mp4 output")
            return

        final = job_dir / "talking.mp4"
        shutil.copy2(mp4s[0], final)
        _set_job(job_id, status="done", output_path=str(final))

    except subprocess.TimeoutExpired:
        _set_job(job_id, status="failed", error="sadtalker timed out")
    except Exception as exc:  # noqa: BLE001 — surface any failure to the caller
        _set_job(job_id, status="failed", error=f"{type(exc).__name__}: {exc}")


# ── Routes ───────────────────────────────────────────────────────────────────

@app.get("/health")
def health() -> JSONResponse:
    gpu = _gpu_info()
    ready = SADTALKER_DIR.exists() and (SADTALKER_DIR / "inference.py").exists()
    return JSONResponse(
        {
            "status": "ok" if ready else "sadtalker_not_found",
            "sadtalker_dir": str(SADTALKER_DIR),
            "sadtalker_ready": ready,
            "gpu": gpu,
        }
    )


@app.post("/generate", status_code=202)
async def generate(image: UploadFile = File(...), audio: UploadFile = File(...)) -> dict:
    if not SADTALKER_DIR.exists():
        raise HTTPException(503, f"SadTalker not installed at {SADTALKER_DIR}")

    job_id = uuid.uuid4().hex
    job_dir = WORK_DIR / job_id
    job_dir.mkdir(parents=True, exist_ok=True)

    image_path = job_dir / f"source{_suffix(image.filename, '.png')}"
    audio_path = job_dir / f"audio{_suffix(audio.filename, '.wav')}"
    _save(image, image_path)
    _save(audio, audio_path)

    with _jobs_lock:
        _jobs[job_id] = Job(id=job_id)

    threading.Thread(
        target=_run_sadtalker,
        args=(job_id, image_path, audio_path, job_dir),
        daemon=True,
    ).start()

    return {"job_id": job_id, "status": "pending"}


@app.get("/jobs/{job_id}")
def get_job(job_id: str) -> dict:
    with _jobs_lock:
        job = _jobs.get(job_id)
    if job is None:
        raise HTTPException(404, "job not found")
    return {
        "job_id": job.id,
        "status": job.status,
        "output_path": job.output_path,
        "error": job.error,
        "created_at": job.created_at,
    }


@app.get("/jobs/{job_id}/download")
def download(job_id: str) -> FileResponse:
    with _jobs_lock:
        job = _jobs.get(job_id)
    if job is None:
        raise HTTPException(404, "job not found")
    if job.status != "done" or not job.output_path:
        raise HTTPException(409, f"job not ready (status={job.status})")
    return FileResponse(job.output_path, media_type="video/mp4", filename="talking.mp4")


# ── Helpers ──────────────────────────────────────────────────────────────────

def _save(upload: UploadFile, dest: Path) -> None:
    with dest.open("wb") as f:
        shutil.copyfileobj(upload.file, f)


def _suffix(filename: str | None, fallback: str) -> str:
    if filename and "." in filename:
        return "." + filename.rsplit(".", 1)[-1]
    return fallback


def _gpu_info() -> dict:
    try:
        out = subprocess.run(
            ["nvidia-smi", "--query-gpu=name,memory.total,memory.free", "--format=csv,noheader"],
            capture_output=True, text=True, timeout=10,
        )
        if out.returncode == 0:
            return {"available": True, "detail": out.stdout.strip()}
    except Exception:  # noqa: BLE001
        pass
    return {"available": False, "detail": "nvidia-smi not available"}


if __name__ == "__main__":
    import uvicorn

    port = int(os.getenv("LIPSYNC_PORT", "8001"))
    uvicorn.run(app, host="0.0.0.0", port=port)
