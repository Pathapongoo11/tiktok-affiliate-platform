package videos_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/videos"
)

// ---------------------------------------------------------------------------
// Concat list tests (no FFmpeg binary required)
// ---------------------------------------------------------------------------

func TestCreateConcatList_SingleImage(t *testing.T) {
	images := []string{"image1.jpg"}
	totalDuration := 10

	listPath, err := videos.CreateConcatList(images, totalDuration)
	require.NoError(t, err)
	defer os.Remove(listPath)

	content, err := os.ReadFile(listPath)
	require.NoError(t, err)

	text := string(content)
	// Should contain the file reference
	assert.Contains(t, text, "file '")
	assert.Contains(t, text, "image1.jpg")
	// Should contain a duration line
	assert.Contains(t, text, "duration")
	// For 1 image with 10 seconds, duration should be 10.00
	assert.Contains(t, text, "duration 10.00")
}

func TestCreateConcatList_MultipleImages_DurationPerImage(t *testing.T) {
	images := []string{"a.jpg", "b.jpg", "c.jpg"}
	totalDuration := 30

	listPath, err := videos.CreateConcatList(images, totalDuration)
	require.NoError(t, err)
	defer os.Remove(listPath)

	content, err := os.ReadFile(listPath)
	require.NoError(t, err)

	text := string(content)

	// Each image should get totalDuration/count = 30/3 = 10 seconds
	expectedDuration := fmt.Sprintf("duration %.2f", float64(totalDuration)/float64(len(images)))
	assert.Contains(t, text, expectedDuration)

	// All image paths should appear
	for _, img := range images {
		assert.Contains(t, text, img, "concat list should contain path for %s", img)
	}
}

func TestCreateConcatList_LastImageRepeated(t *testing.T) {
	// FFmpeg concat demuxer requires the last entry to appear without a duration
	images := []string{"first.jpg", "last.jpg"}
	totalDuration := 20

	listPath, err := videos.CreateConcatList(images, totalDuration)
	require.NoError(t, err)
	defer os.Remove(listPath)

	content, err := os.ReadFile(listPath)
	require.NoError(t, err)

	text := string(content)

	// "last.jpg" should appear at least twice (once with duration, once without)
	count := strings.Count(text, "last.jpg")
	assert.GreaterOrEqual(t, count, 2, "last image should be repeated in concat list")
}

func TestCreateConcatList_ValidFormat(t *testing.T) {
	images := []string{"img1.jpg", "img2.jpg"}
	listPath, err := videos.CreateConcatList(images, 10)
	require.NoError(t, err)
	defer os.Remove(listPath)

	content, err := os.ReadFile(listPath)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	// Each "file" line should be followed by a "duration" line (except the last repeat)
	fileLines := 0
	durationLines := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "file '") {
			fileLines++
		}
		if strings.HasPrefix(trimmed, "duration ") {
			durationLines++
		}
	}

	// Should have N+1 file entries (N images + repeated last) and N duration entries
	assert.Equal(t, len(images)+1, fileLines, "should have N+1 file entries")
	assert.Equal(t, len(images), durationLines, "should have N duration entries")
}

// ---------------------------------------------------------------------------
// GenerateVideo edge case tests (no FFmpeg binary execution needed for these)
// ---------------------------------------------------------------------------

func TestGenerateVideo_NoImages_ReturnsError(t *testing.T) {
	cfg := videos.VideoConfig{
		InputImages: []string{},
		OutputPath:  "/tmp/test_output.mp4",
		DurationSec: 10,
	}

	err := videos.GenerateVideo(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no input images provided")
}

func TestGenerateVideo_NilImages_ReturnsError(t *testing.T) {
	cfg := videos.VideoConfig{
		InputImages: nil,
		OutputPath:  "/tmp/test_output.mp4",
		DurationSec: 10,
	}

	err := videos.GenerateVideo(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no input images provided")
}

func TestGenerateVideo_InvalidOutputPath_HandledGracefully(t *testing.T) {
	// Use images slice with one entry so we get past the no-images check.
	// The output path is under a deeply nested invalid dir; directory creation may fail
	// or FFmpeg will fail (binary not present in test env) — either is a valid error.
	cfg := videos.VideoConfig{
		InputImages: []string{"nonexistent.jpg"},
		OutputPath:  "/no_such_root_dir/a/b/c/out.mp4",
		DurationSec: 5,
	}

	err := videos.GenerateVideo(cfg)

	// We expect an error (either mkdir failure or ffmpeg failure)
	assert.Error(t, err)
}

func TestGenerateVideo_DefaultsFPSAndDuration(t *testing.T) {
	// GenerateVideo applies defaults before reaching FFmpeg.
	// We can't easily inspect the applied values without running FFmpeg,
	// but we can verify the function proceeds past the guard clauses and
	// attempts execution (returns an error due to missing binary/files, not a panic).
	cfg := videos.VideoConfig{
		InputImages: []string{"img.jpg"},
		OutputPath:  t.TempDir() + "/out.mp4",
		// FPS and DurationSec are 0 — should be defaulted inside GenerateVideo
	}

	err := videos.GenerateVideo(cfg)

	// Expected: error from FFmpeg not found or file not found — not a nil-panic
	assert.Error(t, err)
}

func TestGenerateVideo_ErrorContainsFfmpegPrefix(t *testing.T) {
	// Regression guard: error message must include "ffmpeg failed" prefix and
	// NOT be just the raw exit code string. This ensures the caller (service.go)
	// receives diagnostic information including FFmpeg's stderr output.
	cfg := videos.VideoConfig{
		InputImages: []string{"nonexistent_img.jpg"},
		OutputPath:  t.TempDir() + "/out.mp4",
		DurationSec: 5,
	}

	err := videos.GenerateVideo(cfg)

	require.Error(t, err)
	// Must start with our wrapper prefix, NOT the bare "exit status N"
	assert.True(t,
		strings.HasPrefix(err.Error(), "ffmpeg failed") ||
			strings.HasPrefix(err.Error(), "failed to create image list file"),
		"error must start with diagnostic prefix, got: %q", err.Error(),
	)
}

// ---------------------------------------------------------------------------
// buildPerImageFilter tests (no FFmpeg required)
// ---------------------------------------------------------------------------

func TestBuildPerImageFilter_KenBurns_ContainsZoompan(t *testing.T) {
	f := videos.BuildPerImageFilter(0, 150, videos.StyleKenBurns)
	assert.Contains(t, f, "[0:v]")
	assert.Contains(t, f, "scale=1080:1920")
	assert.Contains(t, f, "zoompan")
	assert.Contains(t, f, "zoom+0.0015")
	assert.Contains(t, f, "s=1080x1920")
}

func TestBuildPerImageFilter_ZoomOut_ContainsZoompan(t *testing.T) {
	f := videos.BuildPerImageFilter(1, 150, videos.StyleZoomOut)
	assert.Contains(t, f, "[1:v]")
	assert.Contains(t, f, "zoompan")
	assert.Contains(t, f, "zoom-0.0015")
}

func TestBuildPerImageFilter_Slide_ContainsZoompan(t *testing.T) {
	f := videos.BuildPerImageFilter(2, 150, videos.StyleSlide)
	assert.Contains(t, f, "[2:v]")
	assert.Contains(t, f, "zoompan")
	assert.Contains(t, f, "z=1.05")
}

func TestBuildPerImageFilter_Static_NoZoompan(t *testing.T) {
	f := videos.BuildPerImageFilter(0, 150, videos.StyleStatic)
	assert.Contains(t, f, "[0:v]")
	assert.Contains(t, f, "scale=1080:1920")
	assert.NotContains(t, f, "zoompan")
}

func TestBuildPerImageFilter_UnknownStyle_DefaultsToKenBurns(t *testing.T) {
	f := videos.BuildPerImageFilter(0, 150, "unknown_style")
	assert.Contains(t, f, "zoom+0.0015", "unknown style should fall back to ken_burns")
}

// ---------------------------------------------------------------------------
// buildAnimatedFilterComplex tests (no FFmpeg required)
// ---------------------------------------------------------------------------

func TestBuildAnimatedFilterComplex_SingleImage_NoXfade(t *testing.T) {
	inputArgs, fc, label := videos.BuildAnimatedFilterComplex(
		[]string{"img.jpg"}, 5, videos.StyleKenBurns, "", "",
	)

	// 6 input flags per image: -loop 1 -t X.XXX -i path
	assert.Len(t, inputArgs, 6)
	assert.Equal(t, "-loop", inputArgs[0])
	assert.Equal(t, "-i", inputArgs[4])

	// Single image → output label is [v0], no xfade
	assert.Equal(t, "[v0]", label)
	assert.Contains(t, fc, "[v0]")
	assert.Contains(t, fc, "zoompan")
	assert.NotContains(t, fc, "xfade")
}

func TestBuildAnimatedFilterComplex_TwoImages_HasXfade(t *testing.T) {
	inputArgs, fc, label := videos.BuildAnimatedFilterComplex(
		[]string{"a.jpg", "b.jpg"}, 10, videos.StyleKenBurns, "", "",
	)

	// 12 input flags for 2 images (6 per image)
	assert.Len(t, inputArgs, 12)

	assert.Contains(t, fc, "[v0]")
	assert.Contains(t, fc, "[v1]")
	assert.Contains(t, fc, "xfade")
	assert.Contains(t, fc, "transition=fade")
	assert.Equal(t, "[vxfinal]", label)
}

func TestBuildAnimatedFilterComplex_ThreeImages_XfadeOffsets(t *testing.T) {
	// 3 images × 5s each = 15s total, xfade duration 0.5s
	// offset[1] = 1 × (5 - 0.5) = 4.5
	// offset[2] = 2 × (5 - 0.5) = 9.0
	_, fc, _ := videos.BuildAnimatedFilterComplex(
		[]string{"a.jpg", "b.jpg", "c.jpg"}, 15, videos.StyleKenBurns, "", "",
	)

	assert.Contains(t, fc, "offset=4.500")
	assert.Contains(t, fc, "offset=9.000")
	assert.Contains(t, fc, "[vxfinal]")
}

func TestBuildAnimatedFilterComplex_WithOverlayText_AddsDrawtext(t *testing.T) {
	_, fc, label := videos.BuildAnimatedFilterComplex(
		[]string{"img.jpg"}, 5, videos.StyleKenBurns,
		"Test Product ฿399", "/usr/share/fonts/test.ttf",
	)

	assert.Contains(t, fc, "drawtext")
	assert.Contains(t, fc, "Test Product")
	assert.Equal(t, "[vout]", label)
}

func TestBuildAnimatedFilterComplex_NoOverlayText_NoDrawtext(t *testing.T) {
	_, fc, _ := videos.BuildAnimatedFilterComplex(
		[]string{"img.jpg"}, 5, videos.StyleKenBurns, "", "",
	)
	assert.NotContains(t, fc, "drawtext")
}

func TestBuildAnimatedFilterComplex_NoFontPath_NoDrawtext(t *testing.T) {
	// overlay text provided but no font path → no drawtext
	_, fc, _ := videos.BuildAnimatedFilterComplex(
		[]string{"img.jpg"}, 5, videos.StyleKenBurns, "Some Text", "",
	)
	assert.NotContains(t, fc, "drawtext")
}

func TestBuildAnimatedFilterComplex_InputArgsCount(t *testing.T) {
	// N images → N×6 input args: -loop 1 -t X.XXX -i path
	for _, n := range []int{1, 2, 3, 5} {
		images := make([]string, n)
		for i := range images {
			images[i] = fmt.Sprintf("img%d.jpg", i)
		}
		inputArgs, _, _ := videos.BuildAnimatedFilterComplex(images, 15, videos.StyleKenBurns, "", "")
		assert.Len(t, inputArgs, n*6, "expected %d×6 input flags for %d images", n, n)
	}
}

func TestBuildAnimatedFilterComplex_StaticStyle_NoZoompan(t *testing.T) {
	_, fc, _ := videos.BuildAnimatedFilterComplex(
		[]string{"a.jpg", "b.jpg"}, 10, videos.StyleStatic, "", "",
	)
	assert.NotContains(t, fc, "zoompan")
	assert.Contains(t, fc, "scale=1080:1920")
}
