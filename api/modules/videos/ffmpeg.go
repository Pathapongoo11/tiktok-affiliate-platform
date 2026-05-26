package videos

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	ffmpeg "github.com/u2takey/ffmpeg-go"
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

	// Base video stream: read images via concat demuxer
	videoInput := ffmpeg.Input(listFile,
		ffmpeg.KwArgs{
			"f":    "concat",
			"safe": "0",
			"r":    fmt.Sprintf("%d", cfg.FPS),
		},
	)

	// Scale to 1080x1920 (9:16 TikTok vertical) then pad to exact dimensions
	videoStream := videoInput.
		Filter("scale", ffmpeg.Args{"1080:1920"},
			ffmpeg.KwArgs{"force_original_aspect_ratio": "decrease"},
		).
		Filter("pad", ffmpeg.Args{"1080:1920:(ow-iw)/2:(oh-ih)/2"})

	// Optional text overlay burned at bottom-centre.
	// drawtext requires a font file; skip the filter if none is found to
	// avoid exit-status-254 failures on minimal Docker images.
	if cfg.OverlayText != "" {
		if fontPath := findFont(); fontPath != "" {
			videoStream = videoStream.Filter("drawtext", ffmpeg.Args{},
				ffmpeg.KwArgs{
					"fontfile":   fontPath,
					"text":       cfg.OverlayText,
					"fontsize":   "52",
					"fontcolor":  "white",
					"x":          "(w-text_w)/2",
					"y":          "h-200",
					"box":        "1",
					"boxcolor":   "black@0.5",
					"boxborderw": "10",
				},
			)
		}
		// If no font found, text overlay is silently skipped rather than crashing.
	}

	var output *ffmpeg.Stream

	if cfg.AudioPath != "" {
		// Mix video + audio; -shortest stops at whichever track ends first
		audioInput := ffmpeg.Input(cfg.AudioPath)
		output = ffmpeg.Output(
			[]*ffmpeg.Stream{videoStream, audioInput},
			cfg.OutputPath,
			ffmpeg.KwArgs{
				"c:v":      "libx264",
				"pix_fmt":  "yuv420p",
				"movflags": "+faststart", // MOOV atom at start → browser can play without full download
				"c:a":      "aac",
				"b:a":      "128k",
				"shortest": "",
				"t":        fmt.Sprintf("%d", cfg.DurationSec),
			},
		)
	} else {
		output = videoStream.Output(
			cfg.OutputPath,
			ffmpeg.KwArgs{
				"c:v":       "libx264",
				"pix_fmt":   "yuv420p",
				"movflags":  "+faststart", // MOOV atom at start → browser can play without full download
				"t":         fmt.Sprintf("%d", cfg.DurationSec),
			},
		)
	}

	// Compile the ffmpeg command, capture stderr for diagnostics.
	cmd := output.OverWriteOutput().Compile()

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nstderr: %s", err, stderrBuf.String())
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
