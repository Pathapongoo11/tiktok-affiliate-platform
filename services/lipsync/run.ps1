<#
.SYNOPSIS
    Start the SadTalker lip-sync microservice on http://localhost:8001
.NOTES
    Run install.ps1 first. The Go API (in Docker) reaches this via
    host.docker.internal:8001 — set LIPSYNC_URL=http://host.docker.internal:8001
    in the api service environment.
#>

$ErrorActionPreference = "Stop"
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $here

$VenvPython = Join-Path $here ".venv-sadtalker\Scripts\python.exe"
if (-not (Test-Path $VenvPython)) {
    throw "venv not found — run install.ps1 first."
}

$env:SADTALKER_DIR    = Join-Path $here "SadTalker"
$env:LIPSYNC_WORK_DIR = Join-Path $here "work"
$env:LIPSYNC_PORT     = if ($env:LIPSYNC_PORT) { $env:LIPSYNC_PORT } else { "8001" }
# Run the SadTalker subprocess with the venv interpreter (has torch/cuda/etc.).
$env:SADTALKER_PYTHON = $VenvPython
# "crop" is fastest and reliable; "full" gives best quality but is slower.
$env:SADTALKER_PREPROCESS = if ($env:SADTALKER_PREPROCESS) { $env:SADTALKER_PREPROCESS } else { "crop" }

Write-Host "Starting lip-sync service on http://localhost:$($env:LIPSYNC_PORT)" -ForegroundColor Green
& $VenvPython (Join-Path $here "app.py")
