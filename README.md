# Rattler

Rattler is a Go library for scraping web version of Twitter.

## Sample Usage

An example that prints information about first 30 tweets in the feed from github's Twitter:

```golang
// Create a cursor and session for scraping unfiltered timeline from @github.
cursor := rattler.NewGenericFeedCursor("github", rattler.FeedTypeRegular)
session := rattler.NewTwitterSession(cursor)

counter := 0

// Print information about first 30 tweets.
for result := range session.FeedIter() {
	if result.Error != nil {
		fmt.Printf("Oops: %s", result.Error)
		return
	}

	fmt.Printf("Tweet from %s with ID %d: %s\n",
		result.Tweet.Timestamp.String(),
		result.Tweet.ID,
		result.Tweet.Text,
	)

	counter++
	if counter >= 30 {
		break
	}
}
```

Note that `session.FeedIter()` will keep fetching the feed until it hits Twitter's hard limit (which is about 3000 tweets per feed) or until the feed ends and that generates *a lot* of HTTP requests. It's roughly 1 request per 20 tweets. So please rate-limit your requests, if you need to scrape lots of data! In the above example, it can be achieved by inserting `time.Sleep(...)` into the loop.