"""
compose — realistic review-video compositing (GOAL 4, Phase 1).

Two free, local building blocks the Go API orchestrates over HTTP:

  POST /rembg    {image}              → PNG with the background removed (rembg)
  POST /overlay  {base, overlay, ...} → base video/image with the overlay PNG
                                         composited as picture-in-picture (FFmpeg)

rembg runs on CPU/GPU and is lazy-loaded. The overlay step shells out to the
ffmpeg already required by the lip-sync service.
"""

from __future__ import annotations

import subprocess
import threading
from pathlib import Path

_session = None
_session_lock = threading.Lock()


def is_available() -> bool:
    """True if rembg is importable (without loading the model)."""
    try:
        import rembg  # noqa: F401

        return True
    except Exception:  # noqa: BLE001
        return False


def _get_session():
    """Lazy-load a single rembg session (u2net) — thread-safe singleton."""
    global _session
    if _session is not None:
        return _session
    with _session_lock:
        if _session is None:
            from rembg import new_session

            _session = new_session("u2net")
        return _session


def remove_background(src_path: str, dest_path: str) -> None:
    """Remove the background from src image, writing an RGBA PNG to dest_path."""
    from rembg import remove

    data = Path(src_path).read_bytes()
    out = remove(data, session=_get_session())
    Path(dest_path).parent.mkdir(parents=True, exist_ok=True)
    Path(dest_path).write_bytes(out)


# Picture-in-picture corner positions → FFmpeg overlay x:y expressions.
_CORNERS = {
    "bottom_right": "W-w-m:H-h-m",
    "bottom_left": "m:H-h-m",
    "top_right": "W-w-m:m",
    "top_left": "m:m",
}


def overlay_pip(
    base_path: str,
    overlay_png: str,
    dest_path: str,
    *,
    corner: str = "bottom_right",
    scale: float = 0.3,
    margin: int = 40,
) -> None:
    """Composite overlay_png onto base_path (video or image) as picture-in-picture.

    The overlay is scaled to `scale` of the base width and placed in `corner`.
    Output container matches the base extension (mp4 stays mp4, png/jpg → mp4 is
    not implied — keep the base type). Raises CalledProcessError on ffmpeg failure.
    """
    pos = _CORNERS.get(corner, _CORNERS["bottom_right"])
    x_expr, y_expr = pos.replace("m", str(margin)).split(":")

    # Scale overlay to `scale`*base_width, keep aspect, then overlay at the corner.
    # Force yuv420p output: overlaying an RGBA PNG otherwise yields yuv444p/yuva420p,
    # which Windows Media Player / browsers reject ("unsupported encoding", 0x80004005).
    filter_complex = (
        f"[1:v]scale=iw*{scale}:-1[ov];"
        f"[0:v][ov]overlay={x_expr}:{y_expr}:format=auto,format=yuv420p"
    )

    Path(dest_path).parent.mkdir(parents=True, exist_ok=True)
    cmd = [
        "ffmpeg", "-y",
        "-i", base_path,
        "-i", overlay_png,
        "-filter_complex", filter_complex,
        "-c:v", "libx264", "-preset", "fast", "-pix_fmt", "yuv420p",
        "-movflags", "+faststart",  # web/WMP-friendly: moov atom at the front
        "-c:a", "aac", "-b:a", "128k",  # re-encode audio for broad compatibility
        dest_path,
    ]
    subprocess.run(cmd, check=True, capture_output=True, text=True, timeout=300)
