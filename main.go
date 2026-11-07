package main

import (
	"flag"
	"fmt"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/config"
	"github.com/fatih/color"
	"github.com/markusmobius/go-dateparser"
	"golang.org/x/sync/errgroup"
	"log"
	"sort"
	"time"
)

const TIME_FORMAT = "15:04"
const DATE_FORMAT = "Monday, January 2"

func parseDateString(input string) (time.Time, error) {
	cfg := &dateparser.Configuration{
		CurrentTime: time.Now(),
	}
	parsed, err := dateparser.Parse(cfg, input)
	if err != nil {
		return time.Time{}, err
	}

	since := parsed.Time

	if !parsed.Period.IsTime() {
		year, month, day := since.Date()
		since = time.Date(year, month, day, 0, 0, 0, 0, since.Location())
	}

	return since, nil
}

func run() error {
	sinceStr := flag.String("since", "7 days ago", "Show events since this time (accepts absolute or relative times)")
	flag.Parse()

	since, err := parseDateString(*sinceStr)
	if err != nil {
		return fmt.Errorf("parsing --since date: %w", err)
	}

	client, err := api.DefaultRESTClient()
	if err != nil {
		return err
	}

	cfg, err := config.Read(nil)
	if err != nil {
		return err
	}

	username, err := cfg.Get([]string{"hosts", "github.com", "user"})
	if err != nil {
		return err
	}

	var events []Event
	var oldest time.Time
	page := 1
	pageSize := 100

	// Events are only roughly ordered by time, so keep fetching pages until one
	// contains no events after the --since time.
	for {
		var batch []Event
		url := fmt.Sprintf("users/%s/events?per_page=%d&page=%d", username, pageSize, page)
		err := client.Get(url, &batch)
		if err != nil {
			return fmt.Errorf("fetching events page %d: %w", page, err)
		}

		found := false
		for _, event := range batch {
			dt, err := time.Parse(time.RFC3339, event.Created_At)
			if err != nil {
				return fmt.Errorf("parsing event time: %w", err)
			}

			if oldest.IsZero() || dt.Before(oldest) {
				oldest = dt
			}

			if !dt.Before(since) {
				events = append(events, event)
				found = true
			}
		}

		if !found {
			break
		}

		if len(batch) < pageSize {
			if oldest.After(since) {
				fmt.Println("Warning: GitHub API did not return any events before", oldest.Format(time.RFC3339))
			}
			break
		}

		page++
	}

	// Sort events by timestamp (lexicographical sort is fine, since timestamps
	// are RFC 3339 and in UTC)
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Created_At < events[j].Created_At
	})

	// Parsing may require API requests, so do it concurrently
	fetcher := NewFetcher(client)
	var g errgroup.Group
	g.SetLimit(10)
	for i := range events {
		g.Go(func() error {
			if err := parseEvent(fetcher, &events[i]); err != nil {
				return fmt.Errorf("parsing %s payload: %w", events[i].Type, err)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	events = dropRedundant(events)

	var currentDay time.Time
	var currentRepo string

	for _, event := range events {
		eventTime, _ := time.Parse(time.RFC3339, event.Created_At)
		localTime := eventTime.In(time.Local)

		// if this is a new day, print the date header
		if currentDay.IsZero() || localTime.YearDay() != currentDay.YearDay() {
			if !currentDay.IsZero() {
				fmt.Println()
			}
			fmt.Printf("%s\n", color.YellowString(localTime.Format(DATE_FORMAT)))
			currentDay = localTime
			currentRepo = ""
		}

		// if this is a new repository, print the repo header
		if currentRepo != event.Repo.Name {
			if currentRepo != "" {
				fmt.Println()
			}
			fmt.Printf("%s\n", color.HiWhiteString(event.Repo.Name))
			currentRepo = event.Repo.Name
		}

		fmt.Printf("  %s  %s\n", localTime.Format(TIME_FORMAT), event.parsed.Format())
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
