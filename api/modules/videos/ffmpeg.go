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

// animFPS is the output frame rate used by the animated pipeline.
// The static (concat demuxer) pipeline keeps its original 1 fps.
const animFPS = 30

// xfadeDur is the cross-fade duration in seconds between consecutive clips.
const xfadeDur = 0.5

// target dimensions for TikTok 9:16 vertical video.
const (
	targetW = 1080
	targetH = 1920
)

// GenerateVideo creates a TikTok-format (9:16, 1080×1920) MP4 from product images.
//
// Two rendering pipelines are available, selected by cfg.AnimationStyle:
//   - "" or "static"  → concat-demuxer slideshow (fast, original behaviour)
//   - "ken_burns"     → zoompan zoom-in + xfade cross-fades   (default animated)
//   - "zoom_out"      → zoompan zoom-out + xfade cross-fades
//   - "slide"         → zoompan slow pan  + xfade cross-fades
func GenerateVideo(cfg VideoConfig) error {
	if len(cfg.InputImages) == 0 {
		return fmt.Errorf("no input images provided")
	}
	if cfg.DurationSec == 0 {
		cfg.DurationSec = 15
	}
	if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	if cfg.AnimationStyle != "" && cfg.AnimationStyle != StyleStatic {
		return generateAnimated(cfg)
	}
	return generateStatic(cfg)
}

// generateStatic is the original concat-demuxer pipeline — unchanged behaviour.
func generateStatic(cfg VideoConfig) error {
	if cfg.FPS == 0 {
		cfg.FPS = 1
	}

	listFile, err := createImageListFile(cfg.InputImages, cfg.DurationSec)
	if err != nil {
		return fmt.Errorf("failed to create image list file: %w", err)
	}
	defer os.Remove(listFile)

	vf := "scale=1080:1920:force_original_aspect_ratio=decrease,pad=1080:1920:(ow-iw)/2:(oh-ih)/2"
	if cfg.OverlayText != "" {
		if fontPath := findFont(); fontPath != "" {
			vf += fmt.Sprintf(
				",drawtext=fontfile='%s':text='%s':fontsize=52:fontcolor=white"+
					":x=(w-text_w)/2:y=h-200:box=1:boxcolor=black@0.5:boxborderw=10",
				fontPath, escapeFFmpegText(cfg.OverlayText),
			)
		}
	}

	args := []string{"-f", "concat", "-safe", "0", "-i", listFile}
	if cfg.AudioPath != "" {
		args = append(args, "-i", cfg.AudioPath)
	}
	args = append(args, "-vf", vf, "-c:v", "libx264", "-preset", "fast",
		"-pix_fmt", "yuv420p", "-movflags", "+faststart",
		"-t", fmt.Sprintf("%d", cfg.DurationSec))
	if cfg.AudioPath != "" {
		args = append(args, "-c:a", "aac", "-b:a", "128k", "-shortest")
	} else {
		args = append(args, "-an")
	}
	args = append(args, "-y", cfg.OutputPath)

	return runFFmpeg("static", args)
}

// generateAnimated runs the filter_complex pipeline with zoompan + xfade.
func generateAnimated(cfg VideoConfig) error {
	fontPath := ""
	if cfg.OverlayText != "" {
		fontPath = findFont() // empty string → overlay silently skipped
	}

	inputArgs, filterComplex, outputLabel := buildAnimatedFilterComplex(
		cfg.InputImages, cfg.DurationSec, cfg.AnimationStyle, cfg.OverlayText, fontPath,
	)

	// Assemble: image inputs → optional audio → filter_complex → output options
	args := make([]string, 0, len(inputArgs)+20)
	args = append(args, inputArgs...)
	if cfg.AudioPath != "" {
		args = append(args, "-i", cfg.AudioPath)
	}
	args = append(args,
		"-filter_complex", filterComplex,
		"-map", outputLabel,
		"-c:v", "libx264", "-preset", "fast",
		"-pix_fmt", "yuv420p", "-movflags", "+faststart",
	)
	if cfg.AudioPath != "" {
		audioIdx := len(cfg.InputImages)
		args = append(args, "-map", fmt.Sprintf("%d:a", audioIdx),
			"-c:a", "aac", "-b:a", "128k", "-shortest")
	} else {
		args = append(args, "-an")
	}
	args = append(args, "-t", fmt.Sprintf("%d", cfg.DurationSec), "-y", cfg.OutputPath)

	return runFFmpeg("animated/"+cfg.AnimationStyle, args)
}

// buildAnimatedFilterComplex constructs the -filter_complex string and matching
// input arguments (-loop/-t/-i per image) for the animated pipeline.
//
// Returns:
//   - inputArgs:     the per-image FFmpeg input flags
//   - filterComplex: value for the -filter_complex flag
//   - outputLabel:   bracketed label of the final video stream, e.g. "[vxfinal]"
func buildAnimatedFilterComplex(images []string, durationSec int, style, overlayText, fontPath string) (inputArgs []string, filterComplex string, outputLabel string) {
	n := len(images)
	durationPerImage := float64(durationSec) / float64(n)
	framesPerImage := int(durationPerImage * float64(animFPS))
	if framesPerImage < animFPS {
		framesPerImage = animFPS // floor at 1 s worth of frames
	}

	parts := make([]string, 0, n*2+1)

	// ── Per-image: scale/pad → fps → optional zoompan ──────────────────────
	for i, img := range images {
		absPath, err := filepath.Abs(img)
		if err != nil {
			absPath = img
		}
		absPath = filepath.ToSlash(absPath)

		inputArgs = append(inputArgs,
			"-loop", "1",
			"-t", fmt.Sprintf("%.3f", durationPerImage),
			"-i", absPath,
		)
		parts = append(parts, buildPerImageFilter(i, framesPerImage, style)+fmt.Sprintf("[v%d]", i))
	}

	// ── xfade chain ─────────────────────────────────────────────────────────
	currentLabel := "[v0]"
	for i := 1; i < n; i++ {
		offset := float64(i) * (durationPerImage - xfadeDur)
		if offset < 0 {
			offset = 0
		}
		outLabel := fmt.Sprintf("[vx%d]", i)
		if i == n-1 {
			outLabel = "[vxfinal]"
		}
		parts = append(parts, fmt.Sprintf(
			"%s[v%d]xfade=transition=fade:duration=%.3f:offset=%.3f%s",
			currentLabel, i, xfadeDur, offset, outLabel,
		))
		currentLabel = outLabel
	}
	// single-image: no xfade, label stays as [v0]
	if n == 1 {
		currentLabel = "[v0]"
	}

	// ── Optional fade-in text overlay ───────────────────────────────────────
	if overlayText != "" && fontPath != "" {
		parts = append(parts, fmt.Sprintf(
			"%sdrawtext=fontfile='%s':text='%s':fontsize=52:fontcolor=white"+
				":x=(w-text_w)/2:y=h-200:box=1:boxcolor=black@0.5:boxborderw=10"+
				":alpha='if(lt(t\\,1)\\,t\\,1)'[vout]",
			currentLabel, fontPath, escapeFFmpegText(overlayText),
		))
		currentLabel = "[vout]"
	}

	return inputArgs, strings.Join(parts, ";\n"), currentLabel
}

// buildPerImageFilter returns the FFmpeg filter chain for a single input stream.
// Format: [N:v]scale/pad/fps[/zoompan]  — WITHOUT the output label.
func buildPerImageFilter(idx, framesPerImg int, style string) string {
	base := fmt.Sprintf(
		"[%d:v]scale=%d:%d:force_original_aspect_ratio=decrease,"+
			"pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=%d",
		idx, targetW, targetH, targetW, targetH, animFPS,
	)
	size := fmt.Sprintf("s=%dx%d", targetW, targetH)

	switch style {
	case StyleZoomOut:
		return base + fmt.Sprintf(
			",zoompan=z='if(lte(zoom\\,1.001)\\,1.5\\,max(1.001\\,zoom-0.0015))':d=%d"+
				":x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':%s",
			framesPerImg, size)
	case StyleSlide:
		// Pan from left to right across a 5% zoom
		return base + fmt.Sprintf(
			",zoompan=z=1.05:d=%d"+
				":x='min(iw*(1-1/zoom)\\,on*iw/zoom/%d)':y='ih/2-(ih/zoom/2)':%s",
			framesPerImg, framesPerImg, size)
	case StyleStatic:
		// No motion — just scale/pad/fps, no zoompan
		return base
	default: // StyleKenBurns (and any unrecognised value)
		return base + fmt.Sprintf(
			",zoompan=z='min(zoom+0.0015\\,1.5)':d=%d"+
				":x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':%s",
			framesPerImg, size)
	}
}

// escapeFFmpegText escapes characters that are special in FFmpeg filter strings.
func escapeFFmpegText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ":", "\\:")
	s = strings.ReplaceAll(s, "'", "\\'")
	return s
}

// runFFmpeg executes the ffmpeg binary with the given arguments, returning a
// descriptive error that includes stderr output on failure.
func runFFmpeg(label string, args []string) error {
	log.Printf("[video] ffmpeg (%s) %s", label, strings.Join(args, " "))
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
