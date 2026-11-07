package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Fetcher retrieves details that the Events API no longer includes in event payloads.
// See https://github.blog/changelog/2025-08-08-upcoming-changes-to-github-events-api-payloads/
type Fetcher struct {
	client *api.RESTClient
	mu     sync.Mutex
	pulls  map[string]*pullEntry
}

type pullEntry struct {
	once sync.Once
	pr   *PullRequest
	err  error
}

func NewFetcher(client *api.RESTClient) *Fetcher {
	return &Fetcher{client: client, pulls: map[string]*pullEntry{}}
}

func isNotFound(err error) bool {
	var httpErr *api.HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound
}

// Pull returns the PR with the given number, or nil if it no longer exists
// (e.g. the repo was deleted or made private).
func (f *Fetcher) Pull(repo string, number int) (*PullRequest, error) {
	key := fmt.Sprintf("%s#%d", repo, number)

	f.mu.Lock()
	e, ok := f.pulls[key]
	if !ok {
		e = new(pullEntry)
		f.pulls[key] = e
	}
	f.mu.Unlock()

	e.once.Do(func() {
		pr := new(PullRequest)
		err := f.client.Get(fmt.Sprintf("repos/%s/pulls/%d", repo, number), pr)
		if isNotFound(err) {
			return
		} else if err != nil {
			e.err = fmt.Errorf("fetching PR %s: %w", key, err)
			return
		}
		e.pr = pr
	})

	return e.pr, e.err
}

type Comparison struct {
	Html_Url      string
	Total_Commits int
	Commits       []Commit // the oldest maxCommits commits
	Files         []struct {
		Additions int
		Deletions int
	}
}

// compareFileLimit is the maximum number of files the compare API lists. A
// comparison with this many files may have been truncated.
const compareFileLimit = 300

type Commit struct {
	Sha      string
	Html_Url string
	Commit   struct {
		Message string
	}
}

// Compare returns the commits in before...head, or nil if they can't be
// determined (new branch, or a commit that no longer exists).
func (f *Fetcher) Compare(repo string, before string, head string) (*Comparison, error) {
	if strings.Trim(before, "0") == "" {
		return nil, nil
	}

	cmp := new(Comparison)
	url := fmt.Sprintf("repos/%s/compare/%s...%s?per_page=%d", repo, before, head, maxCommits)
	err := f.client.Get(url, cmp)
	if isNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("comparing %s %s...%s: %w", repo, before, head, err)
	}

	return cmp, nil
}
