package main

import (
	"flag"
	"fmt"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/markusmobius/go-dateparser"
	"log"
	"strings"
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

	userResponse := struct{ Login string }{}
	err = client.Get("user", &userResponse)
	if err != nil {
		return err
	}

	var currentDay time.Time
	var currentRepo string
	page := 1

	for {
		var events []Event
		url := fmt.Sprintf("users/%s/events?per_page=50&page=%d", userResponse.Login, page)
		if err := client.Get(url, &events); err != nil {
			return fmt.Errorf("fetching events page %d: %w", page, err)
		}

		if len(events) == 0 {
			fmt.Println("\nGitHub API returned no more events")
			break
		}

		for _, event := range events {
			// Parse the event time and check if it's before our cutoff
			eventTime, err := time.Parse(time.RFC3339, event.Created_At)
			if err != nil {
				return fmt.Errorf("parsing event time: %w", err)
			}

			if eventTime.Before(since) {
				return nil
			}

			localTime := eventTime.In(time.Local)

			// if this is a new day, print the date header
			if currentDay.IsZero() || localTime.YearDay() != currentDay.YearDay() {
				if !currentDay.IsZero() {
					fmt.Println()
				}
				fmt.Printf("%s\n", localTime.Format(DATE_FORMAT))
				fmt.Println(strings.Repeat("-", 48))
				currentDay = localTime
				currentRepo = ""
			}

			// if this is a new repository, print the repo header
			if currentRepo != event.Repo.Name {
				if currentRepo != "" {
					fmt.Println()
				}
				fmt.Printf("%s\n", event.Repo.Name)
				currentRepo = event.Repo.Name
			}

			fmt.Printf("  %s  ", localTime.Format(TIME_FORMAT))

			message, err := formatEvent(event.Type, event.Payload)
			if err != nil {
				return fmt.Errorf("formatting event: %w", err)
			}
			fmt.Println(message)
		}

		page++
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// For more examples of using go-gh, see:
// https://github.com/cli/go-gh/blob/trunk/example_gh_test.go
