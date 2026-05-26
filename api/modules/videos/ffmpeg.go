package videos

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// fontSearchPaths lists common font locations across OS and Docker images.
// The first file that exists is used for the drawtext filter.
var fontSearchPaths = []string{
	// ttf-freefont on Alpine Linux
	"/usr/share/fonts/freefont/FreeSans.ttf",
	// font-noto on Alpine Linux
	"/usr/share/fonts/noto/NotoSans-Regular.ttf",
	// Debian/Ubuntu
	"/usr/share/fonts/truetype/freefont/FreeSans.ttf",
	"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
	// macOS (for local dev)
	"/System/Library/Fonts/Helvetica.ttc",
	"/Library/Fonts/Arial.ttf",
	// Windows (Docker Desktop / WSL path)
	"/mnt/c/Windows/Fonts/arial.ttf",
}

// findFont returns the first font file that exists, or empty string if none found.
func findFont() string {
	for _, p := range fontSearchPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// GenerateVideo creates a TikTok-format (9:16, 1080x1920) MP4 from product images.
// It builds a slideshow via the FFmpeg concat demuxer, scales/pads to vertical
// format, optionally burns a text overlay, and optionally mixes background audio.
//
// Uses exec.Command("ffmpeg", ...) directly with -vf for a simpler, more reliable
// command than the ffmpeg-go filter_complex builder.
func GenerateVideo(cfg VideoConfig) error {
	if len(cfg.InputImages) == 0 {
		return fmt.Errorf("no input images provided")
	}
	if cfg.FPS == 0 {
		cfg.FPS = 1
	}
	if cfg.DurationSec == 0 {
		cfg.DurationSec = 30
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	// Write a temporary FFmpeg concat list
	listFile, err := createImageListFile(cfg.InputImages, cfg.DurationSec)
	if err != nil {
		return fmt.Errorf("failed to create image list file: %w", err)
	}
	defer os.Remove(listFile)

	// ── Video filter chain ──────────────────────────────────────────────────
	// scale: fit inside 1080×1920 preserving aspect ratio
	// pad:   fill remaining space with black bars (letterbox/pillarbox)
	vf := "scale=1080:1920:force_original_aspect_ratio=decrease,pad=1080:1920:(ow-iw)/2:(oh-ih)/2"

	// Optional drawtext overlay — only if a font file is present on disk.
	// Skipped silently when no font is found so the video still generates.
	if cfg.OverlayText != "" {
		if fontPath := findFont(); fontPath != "" {
			// Escape characters that are special in FFmpeg filter strings
			safeText := strings.ReplaceAll(cfg.OverlayText, "\\", "\\\\")
			safeText = strings.ReplaceAll(safeText, ":", "\\:")
			safeText = strings.ReplaceAll(safeText, "'", "\\'")
			vf += fmt.Sprintf(
				",drawtext=fontfile='%s':text='%s':fontsize=52:fontcolor=white"+
					":x=(w-text_w)/2:y=h-200:box=1:boxcolor=black@0.5:boxborderw=10",
				fontPath, safeText,
			)
		}
		// No font found — text overlay silently skipped rather than crashing.
	}

	// ── Build FFmpeg argument list ──────────────────────────────────────────
	args := []string{
		// Input: image slideshow via concat demuxer
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
	}

	if cfg.AudioPath != "" {
		args = append(args, "-i", cfg.AudioPath)
	}

	args = append(args,
		"-vf", vf,
		"-c:v", "libx264",
		"-preset", "fast",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart", // MOOV at start → browser can play without full download
		"-t", fmt.Sprintf("%d", cfg.DurationSec),
	)

	if cfg.AudioPath != "" {
		args = append(args, "-c:a", "aac", "-b:a", "128k", "-shortest")
	} else {
		args = append(args, "-an") // no audio stream in output
	}

	args = append(args, "-y", cfg.OutputPath) // -y = overwrite

	// ── Execute ─────────────────────────────────────────────────────────────
	log.Printf("[video] ffmpeg %s", strings.Join(args, " "))

	cmd := exec.Command("ffmpeg", args...)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed (exit: %v)\nstderr:\n%s", err, stderrBuf.String())
	}
	return nil
}

// createImageListFile writes a temporary FFmpeg concat format text file.
// Each image occupies an equal share of totalDuration seconds.
// The final image is repeated without a duration tag (FFmpeg concat requirement).
func createImageListFile(images []string, totalDuration int) (string, error) {
	durationPerImage := float64(totalDuration) / float64(len(images))

	tmpFile, err := os.CreateTemp("", "ffmpeg-list-*.txt")
	if err != nil {
		return "", fmt.Errorf("os.CreateTemp: %w", err)
	}
	defer tmpFile.Close()

	for _, img := range images {
		absPath, err := filepath.Abs(img)
		if err != nil {
			absPath = img
		}
		// Forward slashes work reliably with FFmpeg on Windows
		absPath = filepath.ToSlash(absPath)
		fmt.Fprintf(tmpFile, "file '%s'\n", absPath)
		fmt.Fprintf(tmpFile, "duration %.2f\n", durationPerImage)
	}

	// Repeat last entry without duration — required by FFmpeg concat demuxer
	if len(images) > 0 {
		absPath, err := filepath.Abs(images[len(images)-1])
		if err != nil {
			absPath = images[len(images)-1]
		}
		absPath = filepath.ToSlash(absPath)
		fmt.Fprintf(tmpFile, "file '%s'\n", absPath)
	}

	return tmpFile.Name(), nil
}
