package rattler

import log "github.com/sirupsen/logrus"

// FeedIterResult is the result of calling FeedIterResult() to retrieve a single tweet
// from feed.
type FeedIterResult struct {
	Tweet *Tweet
	Error error
}

// FeedIter returns a channel which can be used to read all available
// feed tweets.
//
// Using FeedIter() is the recommended way for scraping tweets in this package.
//
// Depending on cursor used, not all available tweets may be retrieved by the
// iterator. Twitter puts a hard limit on a maximum number tweets in a feed.
// So far, the one and the only known way to retrieve every possible tweet from
// the feed is done by searching feed with a time range that extends past the
// last tweet's date in a regular (limited) feed.
func (t *TwitterSession) FeedIter(singlePage ...bool) <-chan (FeedIterResult) {
	type pageIter struct {
		page FeedPageReader
		err  error
	}
	tweetChan := make(chan (FeedIterResult), 5)
	pageChan := make(chan (pageIter), 1)
	pageOut := make(chan (interface{}))

	// Stop download after 1 page if requested by the caller.
	onlyOnePage := len(singlePage) == 1 && singlePage[0]

	// Start goroutine to handle downloading Twitter feed in the background.
	go func() {
		// Helper function that writes out the page to consumer or bails out
		// if it detects that the consumer side has been shut down.
		send := func(page FeedPageReader, err error) bool {
			select {
			case pageChan <- pageIter{page, err}:
				return true
			case <-pageOut:
				return false
			}
		}

		defer close(pageChan)
		for {
			page, err := t.cursor.RetrievePage()
			if !send(page, err) || err != nil || onlyOnePage {
				return
			}

			if minPosition, err := page.GetMinPosition(); err == nil {
				if !t.cursor.Seek(minPosition) {
					return
				}
				continue
			} else {
				send(nil, err)
				return
			}
		}
	}()

	// Consume pages produced by the above goroutine by parsing them and
	// sending the individual tweets into the user channel.
	go func() {
		defer close(pageOut)
		defer close(tweetChan)
		for result := range pageChan {
			if result.err != nil {
				tweetChan <- FeedIterResult{nil, result.err}
				return
			}
			tweets, err := result.page.GetTweets()
			if err != nil {
				tweetChan <- FeedIterResult{nil, err}
				return
			}
			if len(tweets) == 0 {
				return
			}
			for _, tweet := range tweets {
				// XXX: No duplicate tweets has been encountered out there. Is it
				// really neccessary to check tweet IDs against hash table?
				if _, seenAlready := t.seenTweets[tweet.ID]; !seenAlready {
					tweetChan <- FeedIterResult{tweet, nil}
					t.seenTweets[tweet.ID] = struct{}{}
				} else {
					log.WithFields(log.Fields{
						"tweet-id":   tweet.ID,
						"tweet-date": tweet.Timestamp,
					}).Debugf("Duplicate tweet")
				}
			}
		}
	}()
	return tweetChan
}
