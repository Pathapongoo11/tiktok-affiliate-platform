package videos

// CreateConcatList exposes createImageListFile for testing.
// Returns the path to the temporary concat list file.
func CreateConcatList(images []string, totalDuration int) (string, error) {
	return createImageListFile(images, totalDuration)
}
