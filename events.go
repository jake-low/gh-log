package main

import (
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"strings"
	"time"
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
	Head             PullRequestRef
	Base             PullRequestRef
	Additions        int
	Deletions        int
	Changed_Files    int
}

type PullRequestRef struct {
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

type CommitComment struct {
	Commit_Id string
	Html_Url  string
}

type User struct {
	Login    string
	Html_Url string
}

type Discussion struct {
	Number   int
	Title    string
	Html_Url string
}

type Release struct {
	Name     string
	Tag_Name string
	Html_Url string
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

func withComments(n int) string {
	switch n {
	case 0:
		return ""
	case 1:
		return " with comment"
	default:
		return fmt.Sprintf(" with %d comments", n)
	}
}

func formatStats(additions int, deletions int) string {
	return fmt.Sprintf(" (%s %s)",
		color.GreenString("+%d", additions),
		color.RedString("-%d", deletions))
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

// the number of commits listed for a push when the --commits option is used
const maxCommits = 5

type PushPayload struct {
	Ref        string
	Head       string
	Before     string
	count      int // -1 if unknown
	commits    []Commit
	compareUrl string
	additions  int
	deletions  int
	truncated  bool // too many files changed to count additions and deletions
	pushes     int  // number of pushes collapsed into this one, if more than one
}

func (p *PushPayload) Hydrate(f *Fetcher, repo string) error {
	cmp, err := f.Compare(repo, p.Before, p.Head)
	if err != nil {
		return err
	}
	if cmp == nil {
		p.count = -1
		return nil
	}
	p.count = cmp.Total_Commits
	p.commits = cmp.Commits
	p.compareUrl = cmp.Html_Url
	for _, file := range cmp.Files {
		p.additions += file.Additions
		p.deletions += file.Deletions
	}
	p.truncated = len(cmp.Files) >= compareFileLimit
	return nil
}

func (p *PushPayload) Format() string {
	ref := strings.TrimPrefix(p.Ref, "refs/heads/")
	pushes := ""
	if p.pushes > 1 {
		pushes = fmt.Sprintf(" (%d pushes)", p.pushes)
	}
	if p.count < 0 {
		return fmt.Sprintf("pushed to %s%s", ref, pushes)
	}
	noun := "commits"
	if p.count == 1 {
		noun = "commit"
	}
	stats := formatStats(p.additions, p.deletions)
	if p.truncated {
		stats = fmt.Sprintf(" (%d+ files)", compareFileLimit)
	}
	return fmt.Sprintf("pushed %d %s to %s%s%s", p.count, noun, ref, stats, pushes)
}

// FormatCommits returns one line for each of the push's first few commits,
// plus a line counting the rest if there are more.
func (p *PushPayload) FormatCommits() []string {
	var lines []string
	for _, c := range p.commits[:min(len(p.commits), maxCommits)] {
		subject, _, _ := strings.Cut(c.Commit.Message, "\n")
		lines = append(lines, fmt.Sprintf("%s %s", formatLink(color.HiBlackString(c.Sha[:7]), c.Html_Url), subject))
	}
	if rest := p.count - len(lines); p.count >= 0 && rest > 0 {
		more := fmt.Sprintf("... and %d more", rest)
		if p.compareUrl != "" {
			more = formatLink(more, p.compareUrl)
		}
		lines = append(lines, more)
	}
	return lines
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
	Action   string
	Issue    Issue
	comments int
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

	return fmt.Sprintf("%s issue %s%s%s",
		action,
		formatIssueOrPRLink(p.Issue.Title, p.Issue.Number, p.Issue.Html_Url),
		status,
		withComments(p.comments))
}

type IssueCommentPayload struct {
	Action   string
	Issue    Issue
	comments int // number of comments collapsed into this one, if more than one
}

func (p *IssueCommentPayload) Format() string {
	kind := "issue"
	if p.Issue.Pull_Request != nil {
		// this "issue" is actually a PR
		kind = "PR"
	}
	link := formatIssueOrPRLink(p.Issue.Title, p.Issue.Number, p.Issue.Html_Url)
	if p.comments > 1 {
		return fmt.Sprintf("left %d comments on %s %s", p.comments, kind, link)
	}
	return fmt.Sprintf("commented on %s %s", kind, link)
}

type PullRequestPayload struct {
	Action        string
	Pull_Request  PullRequest
	comments      int
	createdBranch bool // the head branch was created just before opening
	deletedBranch bool // the head branch was deleted just after merging
}

func (p *PullRequestPayload) Hydrate(f *Fetcher, repo string) error {
	return hydratePull(f, repo, &p.Pull_Request)
}

func (p *PullRequestPayload) isMerge() bool {
	return p.Action == "merged" || (p.Action == "closed" && p.Pull_Request.Merged)
}

func (p *PullRequestPayload) Format() string {
	link := formatIssueOrPRLink(p.Pull_Request.Title, p.Pull_Request.Number, p.Pull_Request.Html_Url)

	// Stats reflect the PR's current state, not its state at the time of the
	// event. Changed_Files is zero if the PR couldn't be fetched.
	stats := ""
	if p.Pull_Request.Changed_Files > 0 && (p.isMerge() || p.Action == "opened") {
		stats = formatStats(p.Pull_Request.Additions, p.Pull_Request.Deletions)
	}

	if p.isMerge() {
		deleted := ""
		if p.deletedBranch {
			deleted = fmt.Sprintf(" (deleted branch %s)", p.Pull_Request.Head.Ref)
		}
		return fmt.Sprintf("%s PR %s to %s%s%s%s",
			color.MagentaString("merged"), link, p.Pull_Request.Base.Ref, stats, deleted, withComments(p.comments))
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

	from := ""
	if p.createdBranch {
		from = fmt.Sprintf(" from %s", p.Pull_Request.Head.Ref)
	}

	return fmt.Sprintf("%s PR %s%s%s%s", action, link, from, stats, withComments(p.comments))
}

type PullRequestReviewPayload struct {
	Action       string
	Pull_Request PullRequest
	Review       PullRequestReview
	comments     int
}

func (p *PullRequestReviewPayload) Hydrate(f *Fetcher, repo string) error {
	return hydratePull(f, repo, &p.Pull_Request)
}

func (p *PullRequestReviewPayload) Format() string {
	verb := "reviewed PR"
	if p.Action == "updated" {
		verb = "updated review on PR"
	}
	return fmt.Sprintf("%s %s (%s)%s",
		verb,
		formatIssueOrPRLink(p.Pull_Request.Title, p.Pull_Request.Number, p.Pull_Request.Html_Url),
		strings.ReplaceAll(p.Review.State, "_", " "),
		withComments(p.comments))
}

type PullRequestReviewCommentPayload struct {
	Action       string
	Pull_Request PullRequest
	comments     int // number of comments collapsed into this one, if more than one
}

func (p *PullRequestReviewCommentPayload) Hydrate(f *Fetcher, repo string) error {
	return hydratePull(f, repo, &p.Pull_Request)
}

func (p *PullRequestReviewCommentPayload) Format() string {
	link := formatIssueOrPRLink(p.Pull_Request.Title, p.Pull_Request.Number, p.Pull_Request.Html_Url)
	if p.comments > 1 {
		return fmt.Sprintf("left %d review comments on PR %s", p.comments, link)
	}
	return fmt.Sprintf("left review comment on PR %s", link)
}

type ReleasePayload struct {
	Release Release
}

func (p *ReleasePayload) Format() string {
	name := p.Release.Name
	if name == "" {
		name = p.Release.Tag_Name
	}
	return fmt.Sprintf("released %s", formatLink(name, p.Release.Html_Url))
}

type CommitCommentPayload struct {
	Comment CommitComment
}

func (p *CommitCommentPayload) Format() string {
	sha := p.Comment.Commit_Id
	if len(sha) > 7 {
		sha = sha[:7]
	}
	return fmt.Sprintf("commented on commit %s", formatLink(sha, p.Comment.Html_Url))
}

type MemberPayload struct {
	Member User
}

// FIXME: it's unclear to me from the docs whether the 'actor' of a MemberEvent
// is the user who sent the invitation or the one who accepted it. Until I can
// test this I'm phrasing the message so it'll kind of make sense either way.
func (p *MemberPayload) Format() string {
	return fmt.Sprintf("%s added as collaborator", formatLink(p.Member.Login, p.Member.Html_Url))
}

type DiscussionPayload struct {
	Action     string
	Discussion Discussion
}

func (p *DiscussionPayload) Format() string {
	action := p.Action
	if action == "created" {
		action = color.GreenString("created")
	}
	return fmt.Sprintf("%s discussion %s",
		action,
		formatIssueOrPRLink(p.Discussion.Title, p.Discussion.Number, p.Discussion.Html_Url))
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
	case "CommitCommentEvent":
		formatter = new(CommitCommentPayload)
	case "MemberEvent":
		formatter = new(MemberPayload)
	case "DiscussionEvent":
		formatter = new(DiscussionPayload)
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

// collapse merges runs of closely related events into a single event, such as
// several comments on the same issue, or review comments followed by the
// review they belong to. Only adjacent events in the same repo on the same
// (local) day and less than an hour apart are merged. The limit applies between
// consecutive events, so a long run of frequent events still becomes one line.
func collapse(events []Event) []Event {
	var out []Event
	var last time.Time
	for _, e := range events {
		t, _ := time.Parse(time.RFC3339, e.Created_At)
		t = t.In(time.Local)
		n := len(out)
		if n > 0 && t.Sub(last) <= time.Hour && t.Format(time.DateOnly) == last.Format(time.DateOnly) && out[n-1].Repo.Name == e.Repo.Name {
			if merged, ok := combine(out[n-1], e); ok {
				out[n-1] = merged
				last = t
				continue
			}
		}
		out = append(out, e)
		last = t
	}
	return out
}

// combine tries to merge event b into the event a that directly precedes it,
// returning the merged event. The result is a (if b adds detail to it) or b
// (if a adds detail to b, e.g. comments that preceded closing an issue).
func combine(a, b Event) (Event, bool) {
	switch p := a.parsed.(type) {
	case *IssueCommentPayload:
		n := max(p.comments, 1)
		switch q := b.parsed.(type) {
		case *IssueCommentPayload:
			if q.Issue.Number == p.Issue.Number {
				p.comments = n + 1
				return a, true
			}
		case *IssuePayload:
			if q.Action == "closed" && q.Issue.Number == p.Issue.Number {
				q.comments = n
				return b, true
			}
		case *PullRequestPayload:
			if q.Action == "closed" && !q.isMerge() && q.Pull_Request.Number == p.Issue.Number {
				q.comments = n
				return b, true
			}
		}

	case *PullRequestReviewCommentPayload:
		n := max(p.comments, 1)
		switch q := b.parsed.(type) {
		case *PullRequestReviewCommentPayload:
			if q.Pull_Request.Number == p.Pull_Request.Number {
				p.comments = n + 1
				return a, true
			}
		case *PullRequestReviewPayload:
			if q.Pull_Request.Number == p.Pull_Request.Number {
				q.comments = n
				return b, true
			}
		}

	case *PushPayload:
		if q, ok := b.parsed.(*PushPayload); ok && q.Ref == p.Ref {
			if p.count < 0 || q.count < 0 {
				p.count = -1
			} else {
				p.count += q.count
			}
			p.commits = append(p.commits, q.commits...)
			p.additions += q.additions
			p.deletions += q.deletions
			p.truncated = p.truncated || q.truncated
			// The compare URL ends with the head SHA we requested; point it at
			// the new head so it covers all of the collapsed pushes
			if prefix, ok := strings.CutSuffix(p.compareUrl, p.Head); ok && q.compareUrl != "" {
				p.compareUrl = prefix + q.Head
			} else {
				p.compareUrl = ""
			}
			p.pushes = max(p.pushes, 1) + 1
			p.Head = q.Head
			return a, true
		}

	case *PullRequestPayload:
		q, ok := b.parsed.(*DeletePayload)
		if ok && p.isMerge() && !p.deletedBranch && q.Ref_Type == "branch" && q.Ref == p.Pull_Request.Head.Ref {
			p.deletedBranch = true
			return a, true
		}

	case *CreatePayload:
		switch q := b.parsed.(type) {
		case *PullRequestPayload:
			if p.Ref_Type == "branch" && q.Action == "opened" && q.Pull_Request.Head.Ref == p.Ref {
				q.createdBranch = true
				return b, true
			}
		case *ReleasePayload:
			if p.Ref_Type == "tag" && q.Release.Tag_Name == p.Ref {
				return b, true
			}
		}
	}

	return Event{}, false
}
