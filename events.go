package main

import (
	"encoding/json"
	"fmt"
)

// Helper types

type Repository struct {
	Name string
}

type Issue struct {
	Number       int
	Title        string
	Pull_Request *PullRequestURLs
	Html_Url      string
}

type PullRequest struct {
	Number   int
	Title    string
	Html_Url string
}

type Comment struct {
	Body string
}

type PullRequestURLs struct {
	Url string
}

type Forkee struct {
	Full_Name string
}

type Release struct {
	Name     string
	Tag_Name string
}

type PullRequestReview struct {
	State string
}

type Event struct {
	Type       string
	Created_At string
	Repo       Repository
	Payload    json.RawMessage
}

type Formatter interface {
	Format() string
}

func formatLink(text string, url string) string {
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}

func formatIssueOrPRLink(title string, number int, url string) string {
	return formatLink(fmt.Sprintf("\"%s\" (#%d)", title, number), url)
}

// Event payload types

type PushPayload struct {
	Ref           string
	Size          int
	Distinct_Size int
}

func (p *PushPayload) Format() string {
	return fmt.Sprintf("pushed %d commits to %s", p.Distinct_Size, p.Ref)
}

type CreatePayload struct {
	Ref      string
	Ref_Type string
}

func (p *CreatePayload) Format() string {
	return fmt.Sprintf("created %s %s", p.Ref_Type, p.Ref)
}

type DeletePayload struct {
	Ref      string
	Ref_Type string
}

func (p *DeletePayload) Format() string {
	return fmt.Sprintf("deleted %s %s", p.Ref_Type, p.Ref)
}

type ForkPayload struct {
	Forkee Forkee
}

func (p *ForkPayload) Format() string {
	return fmt.Sprintf("forked repository (creating %s)", p.Forkee.Full_Name)
}

type IssuePayload struct {
	Action string
	Issue  Issue
}

func (p *IssuePayload) Format() string {
	return fmt.Sprintf("%s issue %s", 
		p.Action,
		formatIssueOrPRLink(p.Issue.Title, p.Issue.Number, p.Issue.Html_Url))
}

type IssueCommentPayload struct {
	Action string
	Issue  Issue
}

func (p *IssueCommentPayload) Format() string {
	kind := "issue"
	if p.Issue.Pull_Request != nil {
		// this "issue" is actually a PR
		kind = "PR"
	}
	return fmt.Sprintf("commented on %s %s",
		kind,
		formatIssueOrPRLink(p.Issue.Title, p.Issue.Number, p.Issue.Html_Url))
}

type PullRequestPayload struct {
	Action       string
	Pull_Request PullRequest
}

func (p *PullRequestPayload) Format() string {
	return fmt.Sprintf("%s PR %s",
		p.Action,
		formatIssueOrPRLink(p.Pull_Request.Title, p.Pull_Request.Number, p.Pull_Request.Html_Url))
}

type PullRequestReviewPayload struct {
	Action       string
	Pull_Request PullRequest
	Review       PullRequestReview
}

func (p *PullRequestReviewPayload) Format() string {
	return fmt.Sprintf("reviewed PR %s (%s)",
		formatIssueOrPRLink(p.Pull_Request.Title, p.Pull_Request.Number, p.Pull_Request.Html_Url),
		p.Review.State)
}

type PullRequestReviewCommentPayload struct {
	Action       string
	Pull_Request PullRequest
}

func (p *PullRequestReviewCommentPayload) Format() string {
	return fmt.Sprintf("left review comment on PR %s",
		formatIssueOrPRLink(p.Pull_Request.Title, p.Pull_Request.Number, p.Pull_Request.Html_Url))
}

type ReleasePayload struct {
	Release Release
}

func (p *ReleasePayload) Format() string {
	return fmt.Sprintf("released %s", p.Release.Name)
}

func formatEvent(eventType string, payload json.RawMessage) (string, error) {
	var formatter Formatter

	switch eventType {
	case "PushEvent":
		formatter = new(PushPayload)
	case "CreateEvent":
		formatter = new(CreatePayload)
	case "DeleteEvent":
		formatter = new(DeletePayload)
	case "ForkEvent":
		formatter = new(ForkPayload)
	case "IssuesEvent":
		formatter = new(IssuePayload)
	case "IssueCommentEvent":
		formatter = new(IssueCommentPayload)
	case "PullRequestEvent":
		formatter = new(PullRequestPayload)
	case "PullRequestReviewEvent":
		formatter = new(PullRequestReviewPayload)
	case "PullRequestReviewCommentEvent":
		formatter = new(PullRequestReviewCommentPayload)
	case "ReleaseEvent":
		formatter = new(ReleasePayload)
	case "PublicEvent":
		return "made repository public", nil
	case "WatchEvent":
		return "starred repository", nil
	default:
		return eventType, nil
	}

	err := json.Unmarshal(payload, formatter)
	if err != nil {
		return "", err
	}

	return formatter.Format(), nil
}
