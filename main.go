package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/auth"
	"github.com/cli/go-gh/v2/pkg/config"
	"github.com/fatih/color"
	"github.com/markusmobius/go-dateparser"
	"golang.org/x/sync/errgroup"
	"log"
	"sort"
	"strings"
	"time"
)

const TIME_FORMAT = "15:04"
const DATE_FORMAT = "Monday, January 2"

// parses an absolute or relative date/time. If the input specifies a date but
// not a time, the result is the start of that day and isDay is true.
func parseDateString(input string) (t time.Time, isDay bool, err error) {
	cfg := &dateparser.Configuration{
		CurrentTime: time.Now(),
		// without this, inputs like "yesterday 14:00" have Period == Day
		ReturnTimeAsPeriod: true,
	}
	parsed, err := dateparser.Parse(cfg, input)
	if err != nil {
		return time.Time{}, false, err
	}

	t = parsed.Time

	if !parsed.Period.IsTime() {
		year, month, day := t.Date()
		t = time.Date(year, month, day, 0, 0, 0, 0, t.Location())
		isDay = true
	}

	return t, isDay, nil
}

func run() error {
	sinceStr := flag.String("since", "7 days ago", "Show events since this time (accepts absolute or relative times)")
	untilStr := flag.String("until", "", "Show events before this time; a date without a time includes that whole day")
	repos := map[string]bool{}
	flag.Func("repo", "Only show events in this repo (owner/name); may be repeated", func(s string) error {
		if !strings.Contains(s, "/") {
			return errors.New("expected owner/name (use --owner to filter by owner)")
		}
		repos[strings.ToLower(s)] = true
		return nil
	})
	owners := map[string]bool{}
	flag.Func("owner", "Only show events in repos owned by this user or org; may be repeated", func(s string) error {
		owners[strings.ToLower(s)] = true
		return nil
	})
	flag.Parse()

	since, _, err := parseDateString(*sinceStr)
	if err != nil {
		return fmt.Errorf("parsing --since date: %w", err)
	}

	var until time.Time
	if *untilStr != "" {
		var isDay bool
		until, isDay, err = parseDateString(*untilStr)
		if err != nil {
			return fmt.Errorf("parsing --until date: %w", err)
		}
		if isDay {
			until = until.AddDate(0, 0, 1)
		}
	}

	client, err := api.DefaultRESTClient()
	if err != nil {
		return err
	}

	cfg, err := config.Read(nil)
	if err != nil {
		return err
	}

	host, _ := auth.DefaultHost()
	username, err := cfg.Get([]string{"hosts", host, "user"})
	var notFound *config.KeyNotFoundError
	if errors.As(err, &notFound) {
		// if we've authenticated via a GH_TOKEN, we don't know what user we are,
		// but we can look it up from the API
		var user struct{ Login string }
		if err := client.Get("user", &user); err != nil {
			return fmt.Errorf("fetching current user: %w", err)
		}
		username = user.Login
	} else if err != nil {
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

			if dt.Before(since) {
				continue
			}
			found = true

			name := strings.ToLower(event.Repo.Name)
			owner, _, _ := strings.Cut(name, "/")
			if (until.IsZero() || dt.Before(until)) && (len(repos)+len(owners) == 0 || repos[name] || owners[owner]) {
				events = append(events, event)
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
