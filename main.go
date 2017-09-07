package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/caarlos0/env"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/fatih/color"
	"github.com/hako/durafmt"
)

type config struct {
	TwitterConsumerKey       string   `env:"TRAIN_TRACKER_TWITTER_CONSUMER_KEY"`
	TwitterConsumerSecretKey string   `env:"TRAIN_TRACKER_TWITTER_CONSUMER_SECRET_KEY"`
	TwitterToken             string   `env:"TRAIN_TRACKER_TWITTER_TOKEN"`
	TwitterTokenSecret       string   `env:"TRAIN_TRACKER_TWITTER_TOKEN_SECRET"`
	Lines                    []string `env:"TRAIN_TRACKER_LINES"` // a comma separated list of train lines
}

var (
	fetchSleepDuration = time.Duration(10) * time.Second
	red                = color.New(color.FgRed).SprintFunc()
)

func main() {
	fmt.Println("loading configs")
	cfg := config{}
	err := env.Parse(&cfg)
	if err != nil {
		fmt.Errorf("failed to load configs: %s", err)
		return
	}
	fmt.Printf("configs loaded: %#v\n", cfg)

	config := oauth1.NewConfig(cfg.TwitterConsumerKey, cfg.TwitterConsumerSecretKey)
	token := oauth1.NewToken(cfg.TwitterToken, cfg.TwitterTokenSecret)
	httpClient := config.Client(oauth1.NoContext, token)

	// setup twitter fetcher
	client := twitter.NewClient(httpClient)
	fetch := &twitterFetch{
		twitterClient: client,
		lines:         cfg.Lines,
	}

	for {
		err := fetch.fetch()
		if err != nil {
			fmt.Printf("failed to fetch: %s", err)
		}
		time.Sleep(fetchSleepDuration)
	}
}

type twitterFetch struct {
	twitterClient *twitter.Client
	sinceID       int64
	lines         []string
}

func (f *twitterFetch) fetch() error {
	sinceDate := time.Now().Format("2006-01-02")

	// Search Tweets
	search, resp, err := f.twitterClient.Search.Tweets(&twitter.SearchTweetParams{
		Query:      fmt.Sprintf("from:metrolink since:%s %s", sinceDate, strings.Join(f.lines, " OR ")),
		ResultType: "recent",
		SinceID:    f.sinceID,
	})
	if err != nil {
		return fmt.Errorf("error when searching: %s", err)
	}

	if len(search.Statuses) == 0 {
		fmt.Println("no tweets")
	}

	for _, tweet := range search.Statuses {
		fmt.Printf("@%s tweeted %s ago:\n%s\n\n", tweet.User.Name, timeAgo(tweet), f.highlightLines(tweet.Text))
	}

	f.sinceID = search.Metadata.MaxID
	fmt.Println(rateLimitInfo(resp))

	return nil
}

func (f *twitterFetch) highlightLines(s string) string {
	var result = s
	for _, line := range f.lines {
		result = strings.Replace(result, line, red(line), -1)
	}
	return result
}

func rateLimitInfo(resp *http.Response) string {
	// get rate limit reset duration
	resetTimeInt, _ := strconv.ParseInt(resp.Header.Get("X-Rate-Limit-Reset"), 10, 64)
	resetTime := time.Unix(resetTimeInt, 0)
	resetDuration := time.Until(resetTime).Round(time.Second)
	resetDurationPretty := durafmt.Parse(resetDuration)

	// get rate limit
	resetLimitRemaining, _ := strconv.ParseInt(resp.Header.Get("X-Rate-Limit-Remaining"), 10, 64)
	resetLimitLimit, _ := strconv.ParseInt(resp.Header.Get("X-Rate-Limit-Limit"), 10, 64)

	return fmt.Sprintf("rate limit: %d/%d api calls remaining for the next %s", resetLimitRemaining, resetLimitLimit, resetDurationPretty)
}

func timeAgo(tweet twitter.Tweet) string {
	tweetTime, _ := tweet.CreatedAtTime()
	tweetTimeAgoRouding := time.Minute
	if time.Now().After(tweetTime.Add(time.Duration(-3) * time.Hour)) {
		tweetTimeAgoRouding = time.Hour
	}
	return durafmt.Parse(time.Since(tweetTime).Round(tweetTimeAgoRouding)).String()
}
