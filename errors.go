package rattler

// APICompatError occurs when the process of extracting scraped data was
// unsuccessful. This is most likely the result of Twitter changing its
// internal interfaces or bug in the parser.
type APICompatError struct {
	msg     string
	tweetID *uint64
}

// URLError is an error that can happen while fetching or parsing
// data from the remote server.
type URLError struct {
	msg   string
	url   string
	cause error
}

// MediaDownloadError is an error that happens when downloading embedded
// media in tweet.
type MediaDownloadError struct {
	msg   string
	url   string
	cause error
}

func (e *APICompatError) Error() string {
	return e.msg
}

// TwitterID returns a numeric twitter ID that's associated with the error.
func (e *APICompatError) TwitterID() *uint64 {
	return e.tweetID
}

func (e *URLError) Error() string {
	return e.msg
}

// URL returns URL associated with the error.
func (e *URLError) URL() string {
	return e.url
}

// Cause returns inner error object.
func (e *URLError) Cause() error {
	return e.cause
}

func (t *MediaDownloadError) Error() string {
	return t.msg
}

// URL returns a URL associated with the error.
func (t *MediaDownloadError) URL() string {
	return t.url
}

// Cause returns cause of the error.
func (t *MediaDownloadError) Cause() error {
	return t.cause
}
