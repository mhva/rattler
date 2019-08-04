package rattler

import (
	"fmt"
	"net/url"
)

// FeedFilter enum represents a feed that is a target for scraping (regular or
// media feed).
type FeedFilter int

const (
	// FeedTypeRegular is a regular Twitter feed (contains all tweets).
	FeedTypeRegular FeedFilter = 0
	// FeedTypeMedia is a media-only feed (contains only image/video/postcard
	// tweets).
	FeedTypeMedia FeedFilter = 1
)

// FeedCursor is an interface for navigating a paginated Twitter feed.
type FeedCursor interface {
	RetrievePage() (FeedPageReader, error)
	Seek(string) bool
}

// GenericFeedCursor is used for traversing any paginated feed that is not
// a search feed.
//
// This cursor has a limit to how many pages it can navigate. This limit is
// imposed by Twitter and if it's important to retrieve every possible tweet
// then SearchFeedCursor should be used instead.
type GenericFeedCursor struct {
	client         *TwitterHTTP
	username       string
	feedType       FeedFilter
	nextPageAnchor string
}

// SearchFeedCursor is used for traversing search feeds.
type SearchFeedCursor struct {
	client         *TwitterHTTP
	query          string
	nextPageAnchor string
}

// NewGenericFeedCursor creates a generic feed cursor for traversing single
// user's Twitter feed.
func NewGenericFeedCursor(
	username string,
	ttype FeedFilter, resumeAt ...string,
) *GenericFeedCursor {
	var anchor string
	if len(resumeAt) == 1 {
		anchor = resumeAt[0]
	} else if len(resumeAt) > 1 {
		panic("Too many arguments")
	}

	return &GenericFeedCursor{
		client:         NewTwitterHTTP(),
		username:       username,
		feedType:       ttype,
		nextPageAnchor: anchor,
	}
}

// NewSearchFeedCursor creates a cursor for traversing search results
// returned from given query.
func NewSearchFeedCursor(query string, resumeAt ...string) *SearchFeedCursor {
	var anchor string
	if len(resumeAt) == 1 {
		anchor = resumeAt[0]
	} else if len(resumeAt) > 1 {
		panic("Too many arguments")
	}
	return &SearchFeedCursor{
		client:         NewTwitterHTTP(),
		query:          query,
		nextPageAnchor: anchor,
	}
}

// RetrievePage downloads page at the current cursor position.
//
// Does not advance the cursor.
func (t *GenericFeedCursor) RetrievePage() (FeedPageReader, error) {
	path := "/i/profiles/show/%s/%s"
	if t.feedType == FeedTypeRegular {
		path = fmt.Sprintf(path, t.username, "timeline")
	} else if t.feedType == FeedTypeMedia {
		path = fmt.Sprintf(path, t.username, "media_timeline")
	} else {
		panic("Unknown timeline type!")
	}

	params := make(url.Values)
	params.Add("include_available_features", "1")
	params.Add("include_entities", "1")
	if len(t.nextPageAnchor) > 0 {
		params.Add("max_position", t.nextPageAnchor)
	}
	params.Add("reset_error_state", "false")

	aURL := url.URL{
		Scheme:   "https",
		Host:     "twitter.com",
		Path:     path,
		RawQuery: params.Encode(),
	}

	request, err := t.client.newRequest(aURL)
	if err != nil {
		return nil, err
	}

	var referrer string
	if t.feedType == FeedTypeMedia {
		referrer = fmt.Sprintf("https://twitter.com/%s/media", t.username)
	} else {
		referrer = fmt.Sprintf("https://twitter.com/%s", t.username)
	}

	request.Header.Set("Referer", referrer)
	request.Header.Set("Accept", "application/json,text/javascript,*/*;q=0.01")
	request.Header.Set("X-Requested-With", "XMLHttpRequest")

	structuredJSON, err := t.client.jsonRequest(request)
	if err != nil {
		return nil, err
	}
	page := NewFeedPage(structuredJSON)
	if page == nil {
		return nil, &URLError{"Failed to create GenericTimelinePage", aURL.String(), nil}
	}
	return page, nil
}

// RetrievePage downloads page at the current cursor position.
//
// Does not advance the cursor.
func (t *SearchFeedCursor) RetrievePage() (FeedPageReader, error) {
	params := make(url.Values)
	params.Add("vertical", "default")
	params.Add("q", t.query)
	params.Add("include_available_features", "1")
	params.Add("include_entities", "1")
	if len(t.nextPageAnchor) > 0 {
		params.Add("max_position", t.nextPageAnchor)
	}
	params.Add("reset_error_state", "false")
	aURL := url.URL{
		Scheme:   "https",
		Host:     "twitter.com",
		Path:     "/i/search/timeline",
		RawQuery: params.Encode(),
	}

	request, err := t.client.newRequest(aURL)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Referer", fmt.Sprintf("https://twitter.com/search?q=%s", t.query))
	request.Header.Add("Accept", "application/json,text/javascript,*/*;q=0.01")
	structuredJSON, err := t.client.jsonRequest(request)
	if err != nil {
		return nil, err
	}
	page := NewFeedPage(structuredJSON)
	if page == nil {
		return nil, &URLError{"Failed to create GenericTimelinePage", aURL.String(), nil}
	}
	return page, nil
}

// Seek positions cursor at given position within feed.
func (t *GenericFeedCursor) Seek(position string) bool {
	if len(position) == 0 {
		return false
	}
	t.nextPageAnchor = position
	return true
}

// Seek positions cursor at given position within feed.
func (t *SearchFeedCursor) Seek(position string) bool {
	if len(position) == 0 {
		return false
	}
	t.nextPageAnchor = position
	return true
}
