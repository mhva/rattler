package rattler

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

// Tweet represents a single tweet.
type Tweet struct {
	ID        uint64      `json:"id,string"`
	Timestamp time.Time   `json:"timestamp"`
	Text      string      `json:"text"`
	Extra     interface{} `json:"embed"`
}

// TweetEmbeddedGallery represents multiple images embedded within tweet.
type TweetEmbeddedGallery struct {
	ImageURLs []string
}

// TweetEmbeddedVideo represents a video embedded within tweet.
type TweetEmbeddedVideo struct {
	VideoURL string
}

// TweetEmbeddedCard represents a postcard embedded within tweet.
type TweetEmbeddedCard struct {
	CardURL string
}

// TweetEmbeddedQuote represents a quote, that references another tweet,
// that is embedded within tweet.
type TweetEmbeddedQuote struct {
	QuoteURL string
}

// GalleryDownloadResult is a result of calling Download() on an embedded
// gallery object.
type GalleryDownloadResult struct {
	FileExt string
	Body    io.ReadCloser
	Error   error
}

// Download initiates a sequental download of all images within a Tweet.
//
// Returned channel can be used to read each image's entire body and file
// extension.
func (t *TweetEmbeddedGallery) Download() <-chan GalleryDownloadResult {
	c := make(chan GalleryDownloadResult)

	go func() {
		defer close(c)

		if len(t.ImageURLs) == 0 {
			c <- GalleryDownloadResult{
				Error: errors.New("Tweet contains no image URLs"),
			}
			return
		}

		twitterHTTP := NewTwitterHTTP()
		for _, rawURL := range t.ImageURLs {
			imageVariantURL := rawURL + ":orig"
			request, err := twitterHTTP.newRequestS(imageVariantURL)
			if err != nil {
				c <- GalleryDownloadResult{
					Error: &MediaDownloadError{
						msg:   "Unable to create HTTP request",
						url:   imageVariantURL,
						cause: err,
					},
				}
				return
			}

			reader, err := twitterHTTP.httpRequest(request)
			if err != nil {
				c <- GalleryDownloadResult{
					Error: &MediaDownloadError{
						msg:   "Failed to execute HTTP request",
						url:   imageVariantURL,
						cause: err,
					},
				}
				return
			}

			// Extract file extension.
			var fileExt string
			{
				cleanURL := strings.TrimSuffix(rawURL, ":large")
				cleanURL = strings.TrimSuffix(cleanURL, ":orig")
				fileExt = extractFileExtFromURL(cleanURL)
				if len(fileExt) == 0 {
					// Fallback to using .png.
					fileExt = "png"
				}
			}

			c <- GalleryDownloadResult{
				FileExt: fileExt,
				Body:    reader,
			}
		}
	}()

	return c
}

// MarshalJSON returns TweetEmbeddedGallery encoded as a JSON bytestring.
func (t *TweetEmbeddedGallery) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Type      string   `json:"type"`
		ImageURLs []string `json:"imageURLs"`
	}{
		"EMBED_TYPE_IMAGE",
		t.ImageURLs,
	})
}

// MarshalJSON returns TweetEmbeddedVideo encoded as a JSON bytestring.
func (t *TweetEmbeddedVideo) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Type     string `json:"type"`
		VideoURL string `json:"videoURL"`
	}{
		"EMBED_TYPE_VIDEO",
		t.VideoURL,
	})
}

// MarshalJSON returns TweetEmbeddedCard encoded as a JSON bytestring.
func (t *TweetEmbeddedCard) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Type    string `json:"type"`
		CardURL string `json:"cardURL"`
	}{
		"EMBED_TYPE_CARD",
		t.CardURL,
	})
}

// MarshalJSON returns TweetEmbeddedQuote encoded as a JSON bytestring.
func (t *TweetEmbeddedQuote) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Type     string `json:"type"`
		QuoteURL string `json:"quoteURL"`
	}{
		"EMBED_TYPE_QUOTE",
		t.QuoteURL,
	})
}
