package videos_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/videos"
)

// ---------------------------------------------------------------------------
// IsAIStyle
// ---------------------------------------------------------------------------

func TestIsAIStyle(t *testing.T) {
	cases := map[string]bool{
		videos.StyleAIVideo:   true,
		videos.StyleAICartoon: true,
		videos.StyleAITalking: true,
		videos.StyleKenBurns:  false,
		videos.StyleZoomOut:   false,
		videos.StyleSlide:     false,
		videos.StyleStatic:    false,
		"":                    false,
		"nonsense":            false,
	}
	for style, want := range cases {
		assert.Equal(t, want, videos.IsAIStyle(style), "IsAIStyle(%q)", style)
	}
}

// ---------------------------------------------------------------------------
// GenerateAIVideo guards (no network — error paths only)
// ---------------------------------------------------------------------------

func TestGenerateAIVideo_EmptyToken_ReturnsError(t *testing.T) {
	cfg := videos.VideoConfig{
		InputImages:    []string{"img.png"},
		OutputPath:     filepath.Join(t.TempDir(), "out.mp4"),
		AnimationStyle: videos.StyleAIVideo,
	}

	err := videos.GenerateAIVideo(context.Background(), cfg, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "token not configured")
}

func TestGenerateAIVideo_NoImages_ReturnsError(t *testing.T) {
	cfg := videos.VideoConfig{
		InputImages:    []string{},
		OutputPath:     filepath.Join(t.TempDir(), "out.mp4"),
		AnimationStyle: videos.StyleAIVideo,
	}

	err := videos.GenerateAIVideo(context.Background(), cfg, "fake-token")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no input images provided")
}

func TestGenerateAIVideo_CancelledContext_ReturnsError(t *testing.T) {
	// With a cancelled context the FLUX request fails fast instead of hitting
	// the network — verifies the error path without external calls.
	cfg := videos.VideoConfig{
		InputImages:    []string{filepath.Join(t.TempDir(), "img.png")},
		OutputPath:     filepath.Join(t.TempDir(), "out.mp4"),
		OverlayText:    "Test Product",
		AnimationStyle: videos.StyleAICartoon,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := videos.GenerateAIVideo(ctx, cfg, "fake-token")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "flux generate step")
}

func TestGenerateAIVideo_CreatesOutputDir(t *testing.T) {
	// Verify the output directory is created before any network work.
	base := t.TempDir()
	nested := filepath.Join(base, "a", "b", "c")
	cfg := videos.VideoConfig{
		InputImages:    []string{filepath.Join(base, "img.png")},
		OutputPath:     filepath.Join(nested, "out.mp4"),
		OverlayText:    "Test Product",
		AnimationStyle: videos.StyleAICartoon,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = videos.GenerateAIVideo(ctx, cfg, "fake-token")

	_, statErr := os.Stat(nested)
	assert.NoError(t, statErr, "output directory should have been created")
}

// ---------------------------------------------------------------------------
// buildCartoonPrompt
// ---------------------------------------------------------------------------

func TestBuildCartoonPrompt_UsesSubject(t *testing.T) {
	p := videos.BuildCartoonPrompt("an angry germ monster wearing a crown", videos.StyleAICartoon)
	assert.Contains(t, p, "an angry germ monster wearing a crown")
	assert.Contains(t, p, "character render")
}

func TestBuildCartoonPrompt_EmptyText_HasFallback(t *testing.T) {
	p := videos.BuildCartoonPrompt("", videos.StyleAICartoon)
	assert.Contains(t, p, "mascot")
	assert.NotEmpty(t, p)
}

func TestBuildCartoonPrompt_AIVideoStyle_UsesProductRender(t *testing.T) {
	p := videos.BuildCartoonPrompt("Energy Drink", videos.StyleAIVideo)
	assert.Contains(t, p, "Energy Drink")
	assert.Contains(t, p, "product render")
}

func TestBuildCartoonPrompt_CartoonVsAIVideo_DifferentStyles(t *testing.T) {
	cartoon := videos.BuildCartoonPrompt("X", videos.StyleAICartoon)
	aivideo := videos.BuildCartoonPrompt("X", videos.StyleAIVideo)
	assert.NotEqual(t, cartoon, aivideo, "the two styles should produce different prompts")
}

func TestBuildCartoonPrompt_Talking_BiasesTowardDetectableFace(t *testing.T) {
	// ai_talking feeds SadTalker, which needs a clear front-facing human face.
	p := videos.BuildCartoonPrompt("a friendly doctor", videos.StyleAITalking)
	assert.Contains(t, p, "a friendly doctor")
	assert.Contains(t, p, "front facing")
	assert.Contains(t, p, "portrait")
	// Should NOT use the dramatic 3D cartoon styling that breaks face detection.
	assert.NotContains(t, p, "Pixar")
}
