<#
.SYNOPSIS
    One-time installer for the SadTalker lip-sync microservice (Windows + NVIDIA GPU).

.DESCRIPTION
    1. Clones SadTalker into services/lipsync/SadTalker
    2. Creates a dedicated Python venv (.venv-sadtalker)
    3. Installs PyTorch (CUDA build) + SadTalker requirements
    4. Downloads the pretrained model checkpoints
    5. Installs the microservice wrapper deps (FastAPI/uvicorn)

    Re-runnable: skips steps already completed.

.NOTES
    Requires: git, python (3.10 recommended), an NVIDIA GPU with recent driver.
    Run from the repo root or from services/lipsync.
#>

$ErrorActionPreference = "Stop"
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $here

$SadTalkerDir = Join-Path $here "SadTalker"
$VenvDir      = Join-Path $here ".venv-sadtalker"
$VenvPython   = Join-Path $VenvDir "Scripts\python.exe"

Write-Host "=== Lip-Sync (SadTalker) installer ===" -ForegroundColor Cyan

# 0. Sanity checks --------------------------------------------------------------
function Require-Command($name) {
    if (-not (Get-Command $name -ErrorAction SilentlyContinue)) {
        throw "'$name' not found on PATH. Please install it first."
    }
}
Require-Command git
Require-Command python

Write-Host "Checking GPU..." -ForegroundColor Yellow
try { nvidia-smi --query-gpu=name,memory.total --format=csv,noheader } catch {
    Write-Warning "nvidia-smi failed — SadTalker will be very slow or fail without a CUDA GPU."
}

# 1. Clone SadTalker ------------------------------------------------------------
if (-not (Test-Path $SadTalkerDir)) {
    Write-Host "Cloning SadTalker..." -ForegroundColor Yellow
    git clone https://github.com/OpenTalker/SadTalker.git $SadTalkerDir
} else {
    Write-Host "SadTalker already cloned — skipping." -ForegroundColor DarkGray
}

# 2. Create venv ----------------------------------------------------------------
if (-not (Test-Path $VenvPython)) {
    Write-Host "Creating venv (.venv-sadtalker)..." -ForegroundColor Yellow
    python -m venv $VenvDir
} else {
    Write-Host "venv already exists — skipping." -ForegroundColor DarkGray
}

& $VenvPython -m pip install --upgrade pip setuptools wheel

# 3. Install PyTorch (CUDA 12.1 build works on RTX 4060) ------------------------
Write-Host "Installing PyTorch (CUDA build)..." -ForegroundColor Yellow
& $VenvPython -m pip install torch==2.2.2 torchvision==0.17.2 torchaudio==2.2.2 `
    --index-url https://download.pytorch.org/whl/cu121

# 4. Install SadTalker requirements --------------------------------------------
Write-Host "Installing SadTalker requirements..." -ForegroundColor Yellow
& $VenvPython -m pip install -r (Join-Path $SadTalkerDir "requirements.txt")

# 5. Download model checkpoints -------------------------------------------------
$checkpointsDir = Join-Path $SadTalkerDir "checkpoints"
if (-not (Test-Path (Join-Path $checkpointsDir "SadTalker_V0.0.2_512.safetensors"))) {
    Write-Host "Downloading pretrained models (~5 GB)..." -ForegroundColor Yellow
    Push-Location $SadTalkerDir
    if (Test-Path ".\scripts\download_models.sh") {
        # Use bash if available (git bash ships bash.exe)
        if (Get-Command bash -ErrorAction SilentlyContinue) {
            bash ./scripts/download_models.sh
        } else {
            Write-Warning "bash not found — download models manually per SadTalker README:"
            Write-Warning "  https://github.com/OpenTalker/SadTalker#2-download-models"
        }
    }
    Pop-Location
} else {
    Write-Host "Model checkpoints already present — skipping." -ForegroundColor DarkGray
}

# 6. Install microservice wrapper deps into the SAME venv -----------------------
Write-Host "Installing microservice (FastAPI/uvicorn)..." -ForegroundColor Yellow
& $VenvPython -m pip install -r (Join-Path $here "requirements.txt")

Write-Host ""
Write-Host "=== Install complete ===" -ForegroundColor Green
Write-Host "Start the service with:" -ForegroundColor Cyan
Write-Host "  services\lipsync\run.ps1" -ForegroundColor White
