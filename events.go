package main

import (
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"strings"
)

// Helper types

type Repository struct {
	Name string
}

type Issue struct {
	Number       int
	Title        string
	Pull_Request *PullRequestURLs
	Html_Url     string
	State        string
	State_Reason string
}

type PullRequest struct {
	Number           int
	Title            string
	Html_Url         string
	State            string
	Merged           bool
	Merge_Commit_Sha string
	Base             PullRequestBase
}

type PullRequestBase struct {
	Ref string
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
	Id    int64
	State string
}

type Page struct {
	Page_Name string
	Title     string
	Summary   *string
	Action    string
	Sha       string
	Html_Url  string
}

type Event struct {
	Type       string
	Created_At string
	Repo       Repository
	Payload    json.RawMessage
	parsed     Formatter
}

type Formatter interface {
	Format() string
}

type staticFormatter string

func (s staticFormatter) Format() string {
	return string(s)
}

// Hydrator is implemented by payloads that need to fetch details missing from
// the Events API response before they can be formatted.
type Hydrator interface {
	Hydrate(f *Fetcher, repo string) error
}

func formatLink(text string, url string) string {
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}

func formatIssueOrPRLink(title string, number int, url string) string {
	if title == "" {
		return formatLink(fmt.Sprintf("#%d", number), url)
	}
	return formatLink(fmt.Sprintf("\"%s\" (#%d)", title, number), url)
}

func hydratePull(f *Fetcher, repo string, pr *PullRequest) error {
	full, err := f.Pull(repo, pr.Number)
	if err != nil {
		return err
	}
	if full != nil {
		*pr = *full
	}
	return nil
}

// Event payload types

type PushPayload struct {
	Ref    string
	Head   string
	Before string
	count  int
}

func (p *PushPayload) Hydrate(f *Fetcher, repo string) error {
	var err error
	p.count, err = f.CommitCount(repo, p.Before, p.Head)
	return err
}

func (p *PushPayload) Format() string {
	ref := strings.TrimPrefix(p.Ref, "refs/heads/")
	if p.count < 0 {
		return fmt.Sprintf("pushed to %s", ref)
	}
	noun := "commits"
	if p.count == 1 {
		noun = "commit"
	}
	return fmt.Sprintf("pushed %d %s to %s", p.count, noun, ref)
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
	var action string
	switch p.Action {
	case "opened":
		action = color.GreenString("created")
	case "closed":
		action = color.MagentaString("closed")
		if p.Issue.State_Reason == "not_planned" {
			action = color.HiBlackString("closed")
		}
	default:
		action = p.Action
	}

	// The payload's issue reflects its current state, not its state at the
	// time of the event, so State_Reason may be stale or refer to a later close.
	status := ""
	if p.Action == "closed" && p.Issue.State == "closed" && p.Issue.State_Reason != "" {
		status = fmt.Sprintf(" (%s)", strings.ReplaceAll(p.Issue.State_Reason, "_", " "))
	}

	return fmt.Sprintf("%s issue %s%s",
		action,
		formatIssueOrPRLink(p.Issue.Title, p.Issue.Number, p.Issue.Html_Url),
		status)
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

func (p *PullRequestPayload) Hydrate(f *Fetcher, repo string) error {
	return hydratePull(f, repo, &p.Pull_Request)
}

func (p *PullRequestPayload) isMerge() bool {
	return p.Action == "merged" || (p.Action == "closed" && p.Pull_Request.Merged)
}

func (p *PullRequestPayload) Format() string {
	link := formatIssueOrPRLink(p.Pull_Request.Title, p.Pull_Request.Number, p.Pull_Request.Html_Url)
	if p.isMerge() {
		return fmt.Sprintf("%s PR %s to %s", color.MagentaString("merged"), link, p.Pull_Request.Base.Ref)
	}

	var action string
	switch p.Action {
	case "opened":
		action = color.GreenString("created")
	case "closed":
		action = color.RedString("closed")
	default:
		action = p.Action
	}

	return fmt.Sprintf("%s PR %s", action, link)
}

type PullRequestReviewPayload struct {
	Action       string
	Pull_Request PullRequest
	Review       PullRequestReview
}

func (p *PullRequestReviewPayload) Hydrate(f *Fetcher, repo string) error {
	return hydratePull(f, repo, &p.Pull_Request)
}

func (p *PullRequestReviewPayload) Format() string {
	verb := "reviewed PR"
	if p.Action == "updated" {
		verb = "updated review on PR"
	}
	return fmt.Sprintf("%s %s (%s)",
		verb,
		formatIssueOrPRLink(p.Pull_Request.Title, p.Pull_Request.Number, p.Pull_Request.Html_Url),
		strings.ReplaceAll(p.Review.State, "_", " "))
}

type PullRequestReviewCommentPayload struct {
	Action       string
	Pull_Request PullRequest
}

func (p *PullRequestReviewCommentPayload) Hydrate(f *Fetcher, repo string) error {
	return hydratePull(f, repo, &p.Pull_Request)
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

type GollumPayload struct {
	Pages []Page
}

func (p *GollumPayload) Format() string {
	if len(p.Pages) == 0 {
		return "updated wiki"
	}

	// Page actions are "created" or "edited"; describe a mix as "updated"
	action := p.Pages[0].Action
	for _, page := range p.Pages[1:] {
		if page.Action != action {
			action = "updated"
			break
		}
	}
	if action == "created" {
		action = color.GreenString("created")
	}

	if len(p.Pages) == 1 {
		page := p.Pages[0]
		pageLink := formatLink(fmt.Sprintf("'%s'", page.Title), page.Html_Url)
		return fmt.Sprintf("%s wiki page %s", action, pageLink)
	}

	return fmt.Sprintf("%s %d wiki pages", action, len(p.Pages))
}

// parseEvent decodes the event's payload into event.parsed, fetching any
// details that are missing from the payload.
func parseEvent(f *Fetcher, event *Event) error {
	var formatter Formatter

	switch event.Type {
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
	case "GollumEvent":
		formatter = new(GollumPayload)
	case "PublicEvent":
		formatter = staticFormatter("made repository public")
	case "WatchEvent":
		formatter = staticFormatter("starred repository")
	default:
		formatter = staticFormatter(event.Type)
	}

	if _, ok := formatter.(staticFormatter); !ok {
		if err := json.Unmarshal(event.Payload, formatter); err != nil {
			return err
		}
	}

	if h, ok := formatter.(Hydrator); ok {
		if err := h.Hydrate(f, event.Repo.Name); err != nil {
			return err
		}
	}

	event.parsed = formatter
	return nil
}

// Removes events that duplicate information shown by other events:
// - pushes created by merging a PR
// - review updated events for reviews that were also created (which happens
//   when a pending review is submitted)
func dropRedundant(events []Event) []Event {
	merges := map[string]bool{}
	reviews := map[int64]bool{}
	for _, e := range events {
		switch p := e.parsed.(type) {
		case *PullRequestPayload:
			if p.isMerge() && p.Pull_Request.Merge_Commit_Sha != "" {
				merges[e.Repo.Name+"@"+p.Pull_Request.Merge_Commit_Sha] = true
			}
		case *PullRequestReviewPayload:
			if p.Action == "created" {
				reviews[p.Review.Id] = true
			}
		}
	}

	var kept []Event
	for _, e := range events {
		switch p := e.parsed.(type) {
		case *PushPayload:
			if merges[e.Repo.Name+"@"+p.Head] {
				continue
			}
		case *PullRequestReviewPayload:
			if p.Action == "updated" && reviews[p.Review.Id] {
				continue
			}
		}
		kept = append(kept, e)
	}
	return kept
}
