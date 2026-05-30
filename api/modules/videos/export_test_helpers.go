package videos

// CreateConcatList exposes createImageListFile for testing.
// Returns the path to the temporary concat list file.
func CreateConcatList(images []string, totalDuration int) (string, error) {
	return createImageListFile(images, totalDuration)
}

// BuildAnimatedFilterComplex exposes buildAnimatedFilterComplex for testing.
func BuildAnimatedFilterComplex(images []string, durationSec int, style, overlayText, fontPath string) (inputArgs []string, filterComplex string, outputLabel string) {
	return buildAnimatedFilterComplex(images, durationSec, style, overlayText, fontPath)
}

// BuildPerImageFilter exposes buildPerImageFilter for testing.
func BuildPerImageFilter(idx, framesPerImg int, style string) string {
	return buildPerImageFilter(idx, framesPerImg, style)
}
