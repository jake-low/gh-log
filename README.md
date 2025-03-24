# gh-log

_a.k.a. "wait, what did I do last week?"_

This is an extension (plugin) for GitHub's `gh` CLI tool. It adds a `gh log` subcommand which is similar to `git log` but for actions you take on GitHub, such as creating or closing issues, reviewing or merging PRs, pushing branches and tags, etc.

## Installation

```
gh extension install jake-low/gh-log
```

## Usage

```
$ gh log --since '3 days ago'
Friday, March 21
work-org/monorepo
  12:03  created branch rewrite-in-rust
  12:05  created PR "Rewrite the whole app in Rust!" (#60)
  12:07  merged PR "Rewrite the whole app in Rust!" (#60)
  12:07  pushed 1 commits to refs/heads/main
  13:18  commented on issue "Uhh was the Rust rewrite discussed anywhere?" (#61)
  13:27  closed PR "Revert 'Rewrite the whole app in Rust!'" (#62) (not_planned)

Saturday, March 22
me/cool-side-project
  21:40  pushed 7 commits to refs/heads/main
  21:43  created tag v0.119.0
  21:43  released v0.119.0
  
me/other-cool-side-project
  23:57  pushed 3 commits to refs/heads/refactor

Sunday, March 23
nodejs/node
  17:10  opened issue "A proposal to combine null and undefined into one value" (#57601)
  17:19  commented on issue "A proposal to combine null and undefined into one value" (#57601)
  17:23  commented on issue "A proposal to combine null and undefined into one value" (#57601)
  17:28  commented on issue "A proposal to combine null and undefined into one value" (#57601)
  17:40  commented on issue "A proposal to combine null and undefined into one value" (#57601)
  
me/resume
  19:26  pushed 1 commits to refs/heads/main
```

## Limitations

GitHub's events API has some limitations that affect `gh log`.
- The events API does not immediately return the newest events. This means `gh log` will not immediately show your latest actions. The availability of recent events seems to vary but sometimes events take many hours before they appear in the API response.
- The events API only permits fetching of the first few pages of events. Trying to fetch older events than some cutoff will result in an HTTP 422 error. I'm not sure if the cutoff happens at a specific page number or event count, or if it is based on the age of the events requested. In any case, the effect is that `gh log` will not be able to display very old events, even if you pass a timestamp that is a long time ago for the `--since` option.

## License

The `gh-log` program is offered under the ISC license. See the LICENSE file for details.
