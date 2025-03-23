package main

import (
	"encoding/json"
	"fmt"
	"github.com/cli/go-gh/v2/pkg/api"
	"log"
	"strings"
	"time"
)

type Repository struct {
	Name string
}

type Event struct {
	Type       string
	Created_At string
	Repo       Repository
	Payload    json.RawMessage
}

type PushPayload struct {
	Ref           string
	Size          int
	Distinct_Size int
}

type CreatePayload struct {
	Ref      string
	Ref_Type string
}

type DeletePayload struct {
	Ref      string
	Ref_Type string
}

type ForkPayload struct {
	Forkee Forkee
}

type Forkee struct {
	Full_Name string
}

type IssuePayload struct {
	Action string
	Issue  Issue
}

type IssueCommentPayload struct {
	Action string
	Issue  Issue
	// Comment Comment
}

type Issue struct {
	Number       int
	Title        string
	Pull_Request *PullRequestURLs
}

type PullRequestPayload struct {
	Action       string
	Pull_Request PullRequest
}

type PullRequest struct {
	Number int
	Title  string
}

type PullRequestURLs struct {
	Url string
}

type PullRequestReviewPayload struct {
	Action       string
	Pull_Request PullRequest
	Review       PullRequestReview
}

type PullRequestReviewCommentPayload struct {
	Action       string
	Pull_Request PullRequest
	// Comment      Comment
}

type Comment struct {
	Body string
}

type PullRequestReview struct {
	State string
}

type ReleasePayload struct {
	Release Release
}

type Release struct {
	Name     string
	Tag_Name string
}

const TIME_FORMAT = "15:04"
const DATE_FORMAT = "Monday, January 2"

func main() {
	client, err := api.DefaultRESTClient()
	if err != nil {
		log.Fatal(err)
	}

	userResponse := struct{ Login string }{}
	err = client.Get("user", &userResponse)
	if err != nil {
		log.Fatal(err)
	}
	username := userResponse.Login

	eventsResponse := []Event{}
	err = client.Get(fmt.Sprintf("users/%s/events?per_page=100", username), &eventsResponse)
	if err != nil {
		log.Fatal(err)
	}

	var currentDay time.Time
	var currentRepo string
	for _, event := range eventsResponse {
		t, err := time.Parse(time.RFC3339, event.Created_At)
		if err != nil {
			log.Fatal(err)
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

		switch event.Type {
		case "PushEvent":
			payload := new(PushPayload)
			err = json.Unmarshal(event.Payload, payload)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("pushed %d commits to %s\n", payload.Distinct_Size, payload.Ref)
		case "CreateEvent":
			payload := new(CreatePayload)
			err = json.Unmarshal(event.Payload, payload)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("created %s %s\n", payload.Ref_Type, payload.Ref)
		case "DeleteEvent":
			payload := new(DeletePayload)
			err = json.Unmarshal(event.Payload, payload)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("deleted %s %s\n", payload.Ref_Type, payload.Ref)
		case "ForkEvent":
			payload := new(ForkPayload)
			err = json.Unmarshal(event.Payload, payload)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("forked repository (creating %s)\n", payload.Forkee.Full_Name)
		case "IssuesEvent":
			payload := new(IssuePayload)
			err = json.Unmarshal(event.Payload, payload)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("%s issue \"%s\" (#%d)\n", payload.Action, payload.Issue.Title, payload.Issue.Number)
		case "IssueCommentEvent":
			payload := new(IssueCommentPayload)
			err = json.Unmarshal(event.Payload, payload)
			if err != nil {
				log.Fatal(err)
			}
			kind := "issue"
			if payload.Issue.Pull_Request != nil {
				// this "issue" is actually a PR
				kind = "PR"
			}
			fmt.Printf("commented on %s \"%s\" (#%d)\n", kind, payload.Issue.Title, payload.Issue.Number)
		case "PublicEvent":
			fmt.Printf("made repository public\n")
		case "PullRequestEvent":
			payload := new(PullRequestPayload)
			err = json.Unmarshal(event.Payload, payload)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("%s PR \"%s\" (#%d)\n", payload.Action, payload.Pull_Request.Title, payload.Pull_Request.Number)
		case "PullRequestReviewEvent":
			payload := new(PullRequestReviewPayload)
			err = json.Unmarshal(event.Payload, payload)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("reviewed PR \"%s\" (#%d) (%s)\n", payload.Pull_Request.Title, payload.Pull_Request.Number, payload.Review.State)
		case "PullRequestReviewCommentEvent":
			payload := new(PullRequestReviewCommentPayload)
			err = json.Unmarshal(event.Payload, payload)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("left review comment on PR \"%s\" (#%d)\n", payload.Pull_Request.Title, payload.Pull_Request.Number)
		case "ReleaseEvent":
			payload := new(ReleasePayload)
			err = json.Unmarshal(event.Payload, payload)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("released %s\n", payload.Release.Name)
		case "WatchEvent":
			fmt.Printf("starred repository\n")
		default:
			fmt.Println(event.Type)
		}
	}
}

// For more examples of using go-gh, see:
// https://github.com/cli/go-gh/blob/trunk/example_gh_test.go
