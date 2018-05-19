package rattler

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExtractExtension(t *testing.T) {
	var ext string

	ext = extractFileExtFromURL("https://example.com/test.png")
	assert.Equal(t, "png", ext)

	ext = extractFileExtFromURL("https://example.com/test.jpeg?test=1.png")
	assert.Equal(t, "jpeg", ext)
}
