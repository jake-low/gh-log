package main

import (
	"fmt"
	"github.com/cli/go-gh/v2/pkg/api"
	"log"
	"strings"
	"time"
)

const TIME_FORMAT = "15:04"
const DATE_FORMAT = "Monday, January 2"

func run() error {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return err
	}

	userResponse := struct{ Login string }{}
	err = client.Get("user", &userResponse)
	if err != nil {
		return err
	}
	username := userResponse.Login

	eventsResponse := []Event{}
	err = client.Get(fmt.Sprintf("users/%s/events?per_page=100", username), &eventsResponse)
	if err != nil {
		return err
	}

	var currentDay time.Time
	var currentRepo string
	for _, event := range eventsResponse {
		t, err := time.Parse(time.RFC3339, event.Created_At)
		if err != nil {
			return err
		}

		localTime := t.In(time.Local)

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
			return err
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

// For more examples of using go-gh, see:
// https://github.com/cli/go-gh/blob/trunk/example_gh_test.go
