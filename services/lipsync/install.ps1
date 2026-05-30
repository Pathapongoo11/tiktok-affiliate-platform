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
# We install the deps explicitly rather than via SadTalker's requirements.txt:
# its pins (numpy 1.23.4, librosa 0.9.2, ...) are too old for Python 3.10/3.11.
Write-Host "Installing SadTalker requirements..." -ForegroundColor Yellow
& $VenvPython -m pip install `
    face_alignment==1.3.5 imageio imageio-ffmpeg librosa==0.10.1 numba resampy `
    pydub scipy kornia tqdm yacs pyyaml joblib scikit-image basicsr==1.4.2 `
    facexlib gfpgan av safetensors

# numba/opencv pull in numpy 2.x, but torch 2.2.2 + SadTalker need numpy < 2.
# Re-pin AFTER the bulk install so it sticks. (opencv warns but works fine.)
Write-Host "Pinning numpy to 1.26.4 (required by torch 2.2.2 + SadTalker)..." -ForegroundColor Yellow
& $VenvPython -m pip install "numpy==1.26.4"

# 4b. Patch SadTalker for modern numpy / torchvision ---------------------------
# These are well-known breakages when running SadTalker on current deps.
Write-Host "Patching SadTalker for modern numpy/torchvision..." -ForegroundColor Yellow

# (i) basicsr imports a torchvision path that was removed in newer torchvision.
$degFile = Join-Path $VenvDir "Lib\site-packages\basicsr\data\degradations.py"
if (Test-Path $degFile) {
    (Get-Content $degFile) `
        -replace 'from torchvision\.transforms\.functional_tensor import rgb_to_grayscale', `
                 'from torchvision.transforms.functional import rgb_to_grayscale' |
        Set-Content $degFile
}

# (ii) np.float was removed in numpy >= 1.24.
$awing = Join-Path $SadTalkerDir "src\face3d\util\my_awing_arch.py"
if (Test-Path $awing) {
    (Get-Content $awing) -replace 'np\.float\b(?!\d|32|64)', 'float' | Set-Content $awing
}

# (iii) numpy >= 1.24 rejects inhomogeneous array construction in align_img.
$pre = Join-Path $SadTalkerDir "src\face3d\util\preprocess.py"
if (Test-Path $pre) {
    (Get-Content $pre) `
        -replace 'trans_params = np\.array\(\[w0, h0, s, t\[0\], t\[1\]\]\)', `
                 'trans_params = np.array([w0, h0, s, float(np.squeeze(t)[0]), float(np.squeeze(t)[1])])' |
        Set-Content $pre
}

# 5. Download model checkpoints -------------------------------------------------
# SadTalker's download_models.sh uses wget/unzip which aren't on Windows; download
# the release assets directly with Invoke-WebRequest instead.
$checkpointsDir = Join-Path $SadTalkerDir "checkpoints"
$gfpganWeights  = Join-Path $SadTalkerDir "gfpgan\weights"
New-Item -ItemType Directory -Force $checkpointsDir, $gfpganWeights | Out-Null

$rc = "https://github.com/OpenTalker/SadTalker/releases/download/v0.0.2-rc"
$fx = "https://github.com/xinntao/facexlib/releases/download/v0.1.0"
$models = @(
    @{u = "$rc/mapping_00109-model.pth.tar";        o = (Join-Path $checkpointsDir "mapping_00109-model.pth.tar") },
    @{u = "$rc/mapping_00229-model.pth.tar";        o = (Join-Path $checkpointsDir "mapping_00229-model.pth.tar") },
    @{u = "$rc/SadTalker_V0.0.2_256.safetensors";   o = (Join-Path $checkpointsDir "SadTalker_V0.0.2_256.safetensors") },
    @{u = "$rc/SadTalker_V0.0.2_512.safetensors";   o = (Join-Path $checkpointsDir "SadTalker_V0.0.2_512.safetensors") },
    @{u = "$fx/alignment_WFLW_4HG.pth";             o = (Join-Path $gfpganWeights "alignment_WFLW_4HG.pth") },
    @{u = "$fx/detection_Resnet50_Final.pth";       o = (Join-Path $gfpganWeights "detection_Resnet50_Final.pth") },
    @{u = "https://github.com/TencentARC/GFPGAN/releases/download/v1.3.0/GFPGANv1.4.pth"; o = (Join-Path $gfpganWeights "GFPGANv1.4.pth") },
    @{u = "https://github.com/xinntao/facexlib/releases/download/v0.2.2/parsing_parsenet.pth"; o = (Join-Path $gfpganWeights "parsing_parsenet.pth") }
)
$ProgressPreference = "SilentlyContinue"
foreach ($m in $models) {
    if (Test-Path $m.o) { Write-Host "  skip (exists): $([IO.Path]::GetFileName($m.o))" -ForegroundColor DarkGray; continue }
    Write-Host "  downloading: $([IO.Path]::GetFileName($m.o))" -ForegroundColor Yellow
    Invoke-WebRequest -Uri $m.u -OutFile $m.o -TimeoutSec 600
}

# 6. Install microservice wrapper deps into the SAME venv -----------------------
Write-Host "Installing microservice (FastAPI/uvicorn)..." -ForegroundColor Yellow
& $VenvPython -m pip install -r (Join-Path $here "requirements.txt")

Write-Host ""
Write-Host "=== Install complete ===" -ForegroundColor Green
Write-Host "Start the service with:" -ForegroundColor Cyan
Write-Host "  services\lipsync\run.ps1" -ForegroundColor White
