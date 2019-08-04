package rattler

import (
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TwitterSession represents a single scraping session.
type TwitterSession struct {
	cursor     FeedCursor
	seenTweets map[uint64]struct{}
}

// TwitterHTTP is a session parameters that can be shared across multiple
// TwitterSession`s.
type TwitterHTTP struct {
	httpClient *http.Client
}

// NewTwitterHTTP creates new session parameters.
func NewTwitterHTTP() *TwitterHTTP {
	return &TwitterHTTP{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewTwitterSession creates new TwitterSession based on given cursor.
func NewTwitterSession(cursor FeedCursor) *TwitterSession {
	session := &TwitterSession{
		cursor:     cursor,
		seenTweets: make(map[uint64]struct{}),
	}
	return session
}

func (t *TwitterHTTP) newRequest(aURL url.URL) (*http.Request, error) {
	return t.newRequestS(aURL.String())
}

func (t *TwitterHTTP) newRequestS(aURL string) (*http.Request, error) {
	request, err := http.NewRequest("GET", aURL, nil)
	if err != nil {
		return nil, &URLError{"Unable to create request object", aURL, err}
	}
	configureRequest(request)
	return request, nil
}

func (t *TwitterHTTP) httpRequest(request *http.Request) (io.ReadCloser, error) {
	response, err := t.httpClient.Do(request)
	if err != nil {
		return nil, &URLError{"Failed to execute HTTP request", request.URL.String(), err}
	}

	if response.StatusCode != http.StatusOK {
		io.Copy(ioutil.Discard, response.Body)
		response.Body.Close()
		statusText := http.StatusText(response.StatusCode)
		return nil, &URLError{"HTTP error", request.URL.String(), fmt.Errorf(statusText)}
	}

	// Twitter does not respect Accept-Encoding (which is set to 'gzip' by Go) and
	// returns response compressed with zlib.
	//
	// https://github.com/golang/go/issues/18779
	if strings.ToLower(response.Header.Get("Content-Encoding")) == "deflate" {
		reader, zlibErr := zlib.NewReader(response.Body)
		if zlibErr != nil {
			return nil, &URLError{"Corrupt ZLIB stream", request.URL.String(), zlibErr}
		}
		return reader, nil
	}

	return response.Body, nil
}

func (t *TwitterHTTP) jsonRequest(request *http.Request) (interface{}, error) {
	bodyReader, err := t.httpRequest(request)
	if err != nil {
		return nil, err
	}
	defer bodyReader.Close()

	var structuredJSON interface{}
	decoder := json.NewDecoder(bodyReader)
	err = decoder.Decode(&structuredJSON)
	if err != nil {
		// Drain the reader to allow reuse of current connection.
		io.Copy(ioutil.Discard, bodyReader)
		return nil, &URLError{"Failed to decode JSON response", request.URL.String(), err}
	}
	return structuredJSON, nil
}

func configureRequest(request *http.Request) {
	request.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml,*/*;q=0.8")
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:67.0) "+
		"Gecko/20100101 Firefox/67.0")
}

func extractFileExtFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	extOffset := strings.LastIndexAny(u.Path, "/.")
	if extOffset != -1 && u.Path[extOffset] == '.' && extOffset < len(u.Path)-1 {
		return u.Path[extOffset+1:]
	}
	return ""
}
