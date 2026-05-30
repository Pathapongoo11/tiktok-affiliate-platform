# Lip-Sync Microservice (SadTalker)

Turns a still character image + an audio clip into a **talking-head MP4**, running
on the Windows host with GPU acceleration (tested on RTX 4060 8GB). The Dockerised
Go API calls it over `host.docker.internal:8001`.

```
[Go API in Docker]                         [this service — Windows + GPU]
 FLUX generates character image  ──HTTP──►  SadTalker lip-syncs to audio
 (ai_talking style)                         (subprocess, CUDA)
        ◄────────  talking MP4  ───────────
```

## Why a separate service?

SadTalker needs an NVIDIA GPU + CUDA PyTorch, which the Linux API container does
not have. Running it natively on Windows is the simplest way to use the host GPU.
The Go API stays GPU-free and just makes HTTP calls.

## Prerequisites

- NVIDIA GPU + recent driver (`nvidia-smi` works) — verified on RTX 4060 8GB
- Python 3.10 or 3.11 (tested on 3.11.9)
- `git` on PATH
- ~6 GB free disk (SadTalker + model checkpoints)

`install.ps1` handles everything automatically, including:
- CUDA PyTorch (cu121) + pinning numpy to 1.26.4
- downloading the 8 model checkpoints via `Invoke-WebRequest` (no wget needed)
- patching 3 known SadTalker breakages on modern numpy/torchvision
  (basicsr `functional_tensor`, `np.float`, `align_img` array shape)

## Install (one time)

```powershell
cd services\lipsync
.\install.ps1
```

This clones SadTalker, creates `.venv-sadtalker`, installs CUDA PyTorch + deps,
downloads the model checkpoints, and installs the FastAPI wrapper.

## Run

```powershell
.\run.ps1          # serves http://localhost:8001
```

Then point the API at it (in the repo root `.env`):

```
LIPSYNC_URL=http://host.docker.internal:8001
```

and restart the API container:

```bash
docker-compose up -d api
```

## API

| Method | Path | Body | Result |
|---|---|---|---|
| `GET`  | `/health` | — | readiness + GPU info |
| `POST` | `/generate` | multipart `image`, `audio` | `202 {job_id, status}` |
| `GET`  | `/jobs/{id}` | — | `{status, output_path, error}` |
| `GET`  | `/jobs/{id}/download` | — | MP4 bytes |

## Environment variables

| Var | Default | Meaning |
|---|---|---|
| `SADTALKER_DIR` | `./SadTalker` | SadTalker checkout path |
| `LIPSYNC_WORK_DIR` | `./work` | per-job scratch dir |
| `LIPSYNC_PORT` | `8001` | service port |
| `SADTALKER_PREPROCESS` | `full` | `full` \| `crop` \| `resize` |
| `SADTALKER_TIMEOUT` | `900` | per-job hard cap (seconds) |

## Graceful fallback

If `LIPSYNC_URL` is unset, or no audio is provided, the `ai_talking` style falls
back to `ai_cartoon` (FLUX character + Ken Burns) — so the platform never hard-fails
when the GPU service isn't running.

## Notes

- First request is slow (model load). Subsequent ones are faster.
- `--still` is used to reduce head motion (better for product mascots).
- The SadTalker checkout, venv, and `work/` are git-ignored.
