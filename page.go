package rattler

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	gq "github.com/PuerkitoBio/goquery"
	log "github.com/sirupsen/logrus"
)

// FeedPageReader interface defines means of accessing paginated feed's tweets
// and each page's position within feed. Twitter uses pagination to sub-divide
// feeds that contain large number of tweets.
type FeedPageReader interface {
	GetTweets() ([]*Tweet, error)
	GetMinPosition() (string, error)
}

// FeedPage stores a single page from Twitter feed.
//
// Tweets and additional page data can be retrieved through FeedPage interface,
// which is implemented by this type.
type FeedPage struct {
	json map[string]interface{}
}

// NewFeedPage creates a page parser.
func NewFeedPage(structuredJSON interface{}) *FeedPage {
	jsonDict, ok := structuredJSON.(map[string]interface{})
	if !ok {
		return nil
	}
	return &FeedPage{
		json: jsonDict,
	}
}

// GetTweets returns a list of tweets in page.
func (t *FeedPage) GetTweets() ([]*Tweet, error) {
	html, err := t.lookupString("items_html")
	if err != nil {
		return []*Tweet{}, err
	}
	return t.extractTweets(html)
}

// GetMinPosition returns a position of this page within feed.
func (t *FeedPage) GetMinPosition() (string, error) {
	pos, err := t.lookupString("min_position")
	if err != nil {
		// Return an empty string, if min_position is null.
		if _, exists := t.json["min_position"]; exists && t.json["min_position"] == nil {
			return "", nil
		} else if !exists {
			log.Debug("No 'min_position' attribute is present, trying to extract manually")
			minPosition, err := t.extractMinPosition()
			if err != nil {
				return "", err
			}
			if len(minPosition) > 0 {
				log.Debugf("Successfully extracted 'min_position' (= '%s')", minPosition)
				return minPosition, nil
			}
			log.Debugf("Coudln't extract min_position")
			return "", nil
		}
		return "", err
	}
	return pos, nil
}

func (t *FeedPage) extractMinPosition() (string, error) {
	pageHTML, err := t.lookupString("items_html")
	if err != nil {
		return "", err
	}

	doc, err := gq.NewDocumentFromReader(strings.NewReader(pageHTML))
	if err != nil {
		return "", err
	}

	if tweetSel := doc.Find("li[data-item-type=tweet]"); tweetSel.Length() > 0 {
		if value, exists := tweetSel.Last().Attr("data-item-id"); exists {
			return value, nil
		}
		msg := "Can't extract tweet ID, because HTML attribute data-item-id does not exist"
		return "", errors.New(msg)
	}
	return "", nil
}

func (t *FeedPage) extractEmbeddedTweetImages(sel *gq.Selection) (*TweetEmbeddedGallery, error) {
	var imageURLs []string
	sel.Find("div[data-image-url]").Each(func(_ int, imgSel *gq.Selection) {
		url, exists := imgSel.Attr("data-image-url")
		if exists {
			imageURLs = append(imageURLs, url)
		} else {
			panic("Selected node is missing expected attribute")
		}
	})
	if len(imageURLs) > 0 {
		return &TweetEmbeddedGallery{imageURLs}, nil
	}
	return nil, nil
}

func (t *FeedPage) extractEmbeddedTweetCard(sel *gq.Selection) (*TweetEmbeddedCard, error) {
	if cardSel := sel.Find("*[data-card-url]"); cardSel.Length() > 0 {
		if cardSel.Length() == 1 {
			url, exists := cardSel.Attr("data-card-url")
			if exists {
				return &TweetEmbeddedCard{url}, nil
			}

			// Shouldn't reach here normally, otherwise it would mean that
			// there's a bug in goquery.
			panic("Selected node is missing expected attribute")
		} else {
			return nil, &APICompatError{"Found more than a single card embeddable", nil}
		}
	}
	return nil, nil
}

func (t *FeedPage) extractEmbeddedTweetQuote(sel *gq.Selection) (*TweetEmbeddedQuote, error) {
	switch quoteSel := sel.Find("div.QuoteTweet-link"); quoteSel.Length() {
	case 0:
		// No `quote' node.
		return nil, nil
	case 1:
		// Found the node.
		href, exists := quoteSel.Attr("href")
		if exists {
			return &TweetEmbeddedQuote{"https://twitter.com" + href}, nil
		}
		return nil, &APICompatError{"Quote HTML node is missing URL", nil}
	default:
		// Stumbling in here indicates that something's changed in Twitter's
		// HTML.
		return nil, &APICompatError{"Found more than a single quote embeddable", nil}
	}
}

func (t *FeedPage) extractEmbeddedTweetVideo(sel *gq.Selection) (*TweetEmbeddedVideo, error) {
	// TODO: implement support for extracting embedded videos.
	if videoSel := sel.Find("div.PlayableMedia-player"); videoSel.Length() > 0 {
		log.Debug("Extracting videos is not implemented yet")
	}
	return nil, nil
}

func (t *FeedPage) extractTweetExtra(sel *gq.Selection) (interface{}, error) {
	var imageExtra *TweetEmbeddedGallery
	var cardExtra *TweetEmbeddedCard
	var quoteExtra *TweetEmbeddedQuote
	var videoExtra *TweetEmbeddedVideo
	var err error
	if imageExtra, err = t.extractEmbeddedTweetImages(sel); imageExtra != nil {
		return imageExtra, nil
	} else if err != nil {
		return nil, err
	}
	if cardExtra, err = t.extractEmbeddedTweetCard(sel); cardExtra != nil {
		return cardExtra, nil
	} else if err != nil {
		return nil, err
	}
	if quoteExtra, err = t.extractEmbeddedTweetQuote(sel); quoteExtra != nil {
		return quoteExtra, nil
	} else if err != nil {
		return nil, err
	}
	if videoExtra, err = t.extractEmbeddedTweetVideo(sel); videoExtra != nil {
		return videoExtra, nil
	} else if err != nil {
		return nil, err
	}
	return nil, nil
}

// extractTweet extracts tweet data from DOM node. The selection `sel` is
// expected to point at the top level HTML node of the tweet.
func (t *FeedPage) extractTweet(sel *gq.Selection) (*Tweet, error) {
	var tweetID uint64
	var date time.Time
	var text string
	var extra interface{}
	var err error

	// Extract tweet ID.
	if val, exists := sel.Attr("data-item-id"); exists {
		if tweetID, err = strconv.ParseUint(val, 10, 64); err != nil {
			msg := fmt.Sprintf("Unable to parse tweet id: %s", err.Error())
			return nil, &APICompatError{msg, nil}
		}
	} else {
		return nil, &APICompatError{"Tweet ID not found", nil}
	}

	// Tweet date.
	dateSel := sel.Find("*[data-time]")
	if dateSel.Length() == 1 {
		if dateStr, exists := dateSel.First().Attr("data-time"); exists {
			if unixTime, err := strconv.ParseInt(dateStr, 10, 64); err == nil {
				date = time.Unix(unixTime, 0)
			} else {
				msg := fmt.Sprintf("Unable to parse tweet id: %s", err.Error())
				return nil, &APICompatError{msg, &tweetID}
			}
		} else {
			panic("Selected node is missing expected attribute")
		}
	}

	// Tweet text.
	textSel := sel.Find("p.tweet-text")
	if textSel.Length() == 1 {
		text = textSel.First().Text()
	} else if textSel.Length() == 0 {
		return nil, &APICompatError{"Tweet text not found", &tweetID}
	} else {
		msg := fmt.Sprintf("Expected a single node containing tweet text, got %d instead",
			textSel.Length())
		return nil, &APICompatError{msg, &tweetID}
	}

	// Embedded elements.
	if extra, err = t.extractTweetExtra(sel); err != nil {
		// The extractTweetExtra() function doesn't get a handle of twitterID,
		// so we have to fill it here.
		err.(*APICompatError).tweetID = &tweetID
		return nil, err
	}

	tweet := &Tweet{
		ID:        tweetID,
		Timestamp: date,
		Text:      text,
		Extra:     extra,
	}
	return tweet, nil
}

func (t *FeedPage) extractTweets(html string) ([]*Tweet, error) {
	var doc *gq.Document
	var err error
	var tweets []*Tweet
	if doc, err = gq.NewDocumentFromReader(strings.NewReader(html)); err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
		}).Fatal("Unable to parse feed HTML content")
	}

	doc.Find("li[data-item-type=\"tweet\"]").EachWithBreak(func(_ int, sel *gq.Selection) bool {
		if tweet, err := t.extractTweet(sel); err == nil {
			tweets = append(tweets, tweet)
			return true
		}
		return false
	})

	return tweets, err
}

func (t *FeedPage) lookupString(name string) (string, error) {
	value, ok := t.json[name].(string)
	if !ok {
		if _, exists := t.json[name]; !exists {
			msg := fmt.Sprintf("Key '%s' does not exist in JSON object", name)
			return "", errors.New(msg)
		}
		msg := "Can't convert '%s' (type: %s) to string"
		msg = fmt.Sprintf(msg, name, reflect.TypeOf(t.json[name]))
		return "", errors.New(msg)
	}
	return value, nil
}
