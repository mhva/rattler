package rattler

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type URLRewriteTransport struct {
	newURL *url.URL
}

func (t *URLRewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request.URL.Scheme = t.newURL.Scheme
	request.URL.Host = t.newURL.Host
	return http.DefaultTransport.RoundTrip(request)
}

func setupClientServer(handlerFunc http.HandlerFunc) (*http.Client, *httptest.Server) {
	server := httptest.NewServer(handlerFunc)
	client := &http.Client{}
	url, err := url.Parse(server.URL)
	if err != nil {
		panic("Failed to parse URL: " + server.URL)
	}
	client.Transport = &URLRewriteTransport{url}
	return client, server
}

func readTextFileOrDie(filename string) string {
	var file *os.File
	var err error

	if file, err = os.Open(filename); err != nil {
		panic("Unable to open test file: " + err.Error())
	}
	defer file.Close()

	if data, err := ioutil.ReadAll(file); err == nil {
		return string(data)
	}
	panic("Unable to read test data: " + err.Error())
}

func checkMaxPosition(t *testing.T, expected uint64, url *url.URL) {
	rawPos, present := url.Query()["max_position"]
	assert.True(t, present, "max_position not present in request")
	assert.Equal(t, 1, len(rawPos), "max_position can only have 1 value")

	pos, err := strconv.ParseUint(rawPos[0], 10, 64)
	assert.Nil(t, err, "Failed to parse max_position: %s", rawPos)
	assert.Equal(t, pos, expected)
}

func TestTweetExraction(t *testing.T) {
	t.Log("Testing extraction of well-formed data ...")
	for i := 1; i <= 3; i++ {
		page := FeedPage{nil}
		filename := fmt.Sprintf("testdata/items%d.html", i)
		t.Logf("Extracting tweets from %s", filename)
		itemsHTML := readTextFileOrDie(filename)
		tweets, err := page.extractTweets(itemsHTML)

		if err != nil {
			t.Errorf("Timeline.extractTweets: %s", err.Error())
			continue
		}

		assert.Equal(t, 20, len(tweets), "Extraction returned unexpected number of tweets")
	}
}

func TestLiveRetrieval(t *testing.T) {
	requestHandlers := []func(http.ResponseWriter, *http.Request){
		func(w http.ResponseWriter, r *http.Request) {
			_, present := r.URL.Query()["max_position"]
			assert.False(t, present, "max_position present in initial request")
			fmt.Fprint(w, readTextFileOrDie("testdata/items1.json"))
		},
		func(w http.ResponseWriter, r *http.Request) {
			checkMaxPosition(t, 608164787940413441, r.URL)
			fmt.Fprint(w, readTextFileOrDie("testdata/items2.json"))
		},
		func(w http.ResponseWriter, r *http.Request) {
			checkMaxPosition(t, 506859703859965952, r.URL)
			fmt.Fprint(w, readTextFileOrDie("testdata/items3.json"))
		},
		func(w http.ResponseWriter, r *http.Request) {
			checkMaxPosition(t, 386615604008194048, r.URL)
			fmt.Fprint(w, readTextFileOrDie("testdata/items4.json"))
		},
	}
	requestHandlerIndex := 0
	client, server := setupClientServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if requestHandlerIndex >= len(requestHandlers) {
				assert.Fail(t, "Unexpected request: %s", r.URL.RequestURI)
			}
			requestHandlers[requestHandlerIndex](w, r)
			requestHandlerIndex++
		}))
	defer server.Close()

	tweets := []*Tweet{}
	session := NewTwitterSession(NewGenericFeedCursor("test", FeedTypeMedia))
	session.cursor.(*GenericFeedCursor).client.httpClient = client
	for result := range session.FeedIter() {
		require.Nil(t, result.Error)
		require.NotNil(t, result.Tweet)
		tweets = append(tweets, result.Tweet)
	}
	assert.Equal(t, 59, len(tweets), "LoadFromLiveTimelineFull: Unexpected number of tweets")
}

func TestLiveRetrievalHTTPError(t *testing.T) {
	client, server := setupClientServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, readTextFileOrDie("testdata/items1.json"))
		}))
	defer server.Close()

	session := NewTwitterSession(NewGenericFeedCursor("test", FeedTypeMedia))
	session.cursor.(*GenericFeedCursor).client.httpClient = client

	iterations := 0
	for result := range session.FeedIter() {
		require.NotNil(t, result.Error)
		require.Nil(t, result.Tweet)
		iterations++
	}

	require.Equal(t, 1, iterations)
}
