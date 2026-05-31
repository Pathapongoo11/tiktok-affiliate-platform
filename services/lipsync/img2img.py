"""
img2img — stylize a product photo into a cartoon/3D render while preserving its
shape, using a local Stable Diffusion XL img2img pipeline on the GPU.

Why local: the free Hugging Face hf-inference provider no longer serves
image-to-image models, so for true product-likeness stylization (GOAL 1) we run
SDXL img2img on the same RTX 4060 the lip-sync service already uses.

The pipeline is lazy-loaded on first use and kept in memory afterwards.

Model choice: SD 1.5 img2img (fp16). SDXL was ~3.5 min/image on an RTX 4060 8GB
(too slow); SD 1.5 renders in ~10-20s and uses far less VRAM, which matters since
SadTalker shares the GPU. Quality is plenty for cartoon/stylized product shots.
Override with the SD_IMG2IMG_MODEL env var.
"""

from __future__ import annotations

import os
import threading
from pathlib import Path

# The diffusers pipeline is heavy; import lazily so the service still starts (and
# /tts, /generate keep working) even if diffusers/torch aren't installed.
_pipe = None
_pipe_lock = threading.Lock()

_MODEL_ID = os.getenv("SD_IMG2IMG_MODEL", "runwayml/stable-diffusion-v1-5")


def _load_pipeline():
    """Lazy-load the SD 1.5 img2img pipeline (fp16, CUDA). Thread-safe singleton."""
    global _pipe
    if _pipe is not None:
        return _pipe
    with _pipe_lock:
        if _pipe is not None:
            return _pipe
        import torch
        from diffusers import StableDiffusionImg2ImgPipeline

        pipe = StableDiffusionImg2ImgPipeline.from_pretrained(
            _MODEL_ID,
            torch_dtype=torch.float16,
            safety_checker=None,  # product images; skip to save VRAM/time
        )
        pipe = pipe.to("cuda")
        pipe.enable_attention_slicing()
        _pipe = pipe
        return _pipe


def stylize_image(
    src_path: str,
    dest_path: str,
    prompt: str,
    *,
    strength: float = 0.55,
    guidance_scale: float = 7.0,
    steps: int = 30,
) -> None:
    """Stylize the source image with the prompt and write the result to dest_path.

    strength controls how much the output deviates from the source:
      ~0.4 keeps the product very faithful, ~0.7 is more stylized.
    0.55 is a good default that preserves product shape while applying a cartoon look.
    """
    from PIL import Image

    pipe = _load_pipeline()

    init = Image.open(src_path).convert("RGB")
    # SD 1.5 is trained at 512; keep aspect, cap the long edge to avoid OOM/artifacts.
    init.thumbnail((768, 768))

    result = pipe(
        prompt=prompt,
        image=init,
        strength=strength,
        guidance_scale=guidance_scale,
        num_inference_steps=steps,
    ).images[0]

    Path(dest_path).parent.mkdir(parents=True, exist_ok=True)
    result.save(dest_path)


def is_available() -> bool:
    """Report whether diffusers + a CUDA torch are importable (without loading the model)."""
    try:
        import torch  # noqa: F401
        import diffusers  # noqa: F401

        return torch.cuda.is_available()
    except Exception:  # noqa: BLE001
        return False
