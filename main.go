package main

import (
	"flag"
	"fmt"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/config"
	"github.com/fatih/color"
	"github.com/markusmobius/go-dateparser"
	"log"
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
	page := 1
	pageSize := 100

	// First, fetch events a page at a time until before the --since time
outer:
	for {
		var batch []Event
		url := fmt.Sprintf("users/%s/events?per_page=%d&page=%d", username, pageSize, page)
		err := client.Get(url, &batch)
		if err != nil {
			return fmt.Errorf("fetching events page %d: %w", page, err)
		}

		for _, event := range batch {
			dt, err := time.Parse(time.RFC3339, event.Created_At)
			if err != nil {
				return fmt.Errorf("parsing event time: %w", err)
			}

			if dt.Before(since) {
				break outer
			}

			events = append(events, event)
		}

		if len(batch) < pageSize {
			fmt.Println("Warning: GitHub API did not return any events before", events[len(events) - 1].Created_At)
			break
		}

		page++
	}

	// Then loop over the events in reverse order (so oldest first) and print them
	var currentDay time.Time
	var currentRepo string

	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
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

		fmt.Printf("  %s  ", localTime.Format(TIME_FORMAT))
		message, err := formatEvent(event.Type, event.Payload)
		if err != nil {
			return fmt.Errorf("formatting event: %w", err)
		}
		fmt.Println(message)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
