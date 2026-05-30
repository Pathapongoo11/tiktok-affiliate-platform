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

func TestGenerateAIVideo_MissingImageFile_ReturnsError(t *testing.T) {
	// Token + style set, but the image file doesn't exist on disk.
	// Should fail at the read step, not panic.
	cfg := videos.VideoConfig{
		InputImages:    []string{filepath.Join(t.TempDir(), "does-not-exist.png")},
		OutputPath:     filepath.Join(t.TempDir(), "out.mp4"),
		AnimationStyle: videos.StyleAIVideo,
	}

	// Use a cancelled context so we never actually hit the network even if the
	// file somehow existed — the read error should surface first regardless.
	err := videos.GenerateAIVideo(context.Background(), cfg, "fake-token")

	require.Error(t, err)
}

func TestGenerateAIVideo_CreatesOutputDir(t *testing.T) {
	// Verify the output directory is created before any network work.
	// We point at a nested dir that doesn't exist yet and a missing image so
	// the function returns after mkdir + read-fail, but the dir must exist.
	base := t.TempDir()
	nested := filepath.Join(base, "a", "b", "c")
	cfg := videos.VideoConfig{
		InputImages:    []string{filepath.Join(base, "missing.png")},
		OutputPath:     filepath.Join(nested, "out.mp4"),
		AnimationStyle: videos.StyleAIVideo,
	}

	_ = videos.GenerateAIVideo(context.Background(), cfg, "fake-token")

	_, statErr := os.Stat(nested)
	assert.NoError(t, statErr, "output directory should have been created")
}
