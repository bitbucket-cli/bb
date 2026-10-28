package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/bitbucket-cli/bb/internal/output"
	"github.com/spf13/cobra"
)

var prActivityCmd = &cobra.Command{
	Use:   "activity [workspace/repo] <pr-number>",
	Short: "Show a pull request's activity history",
	Long: `List a pull request's activity oldest first: approvals, changes requested,
updates and comments, each with its actor and timestamp.

COMMIT is the source (head) commit. For an update it is the head that update
recorded, and HEAD_MOVED marks updates that changed it. For an approval or
changes-requested event it is the head in effect when the review was given,
and STALE marks reviews followed by a head move, so they predate the current
commit. Both are blank when the history holds no earlier head to compare.

The activity API documents no un-approval entry. Entry types this command
does not recognise are listed under their API name, with date and actor when
present, so an un-approval or other new event is shown if Bitbucket reports
one.

Read-only: this command never changes the pull request.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runPRActivity,
}

func init() {
	prCmd.AddCommand(prActivityCmd)
}

type prActivityUser struct {
	DisplayName string `json:"display_name"`
	UUID        string `json:"uuid"`
}

type prActivityReview struct {
	Date string         `json:"date"`
	User prActivityUser `json:"user"`
}

type prActivityEntry struct {
	Approval         *prActivityReview `json:"approval"`
	ChangesRequested *prActivityReview `json:"changes_requested"`
	Update           *struct {
		Date   string         `json:"date"`
		State  string         `json:"state"`
		Author prActivityUser `json:"author"`
		Source struct {
			Commit *struct {
				Hash string `json:"hash"`
			} `json:"commit"`
		} `json:"source"`
	} `json:"update"`
	Comment *struct {
		ID        int            `json:"id"`
		CreatedOn string         `json:"created_on"`
		User      prActivityUser `json:"user"`
		Content   struct {
			Raw string `json:"raw"`
		} `json:"content"`
	} `json:"comment"`
	// Raw keeps unrecognised entry types visible under their API name.
	Raw map[string]json.RawMessage `json:"-"`
}

func (e *prActivityEntry) UnmarshalJSON(data []byte) error {
	type plain prActivityEntry
	if err := json.Unmarshal(data, (*plain)(e)); err != nil {
		return err
	}
	return json.Unmarshal(data, &e.Raw)
}

// prActivityEvent is the normalised, machine-readable form of one entry.
type prActivityEvent struct {
	Date      string `json:"date"`
	Event     string `json:"event"`
	Actor     string `json:"actor"`
	ActorUUID string `json:"actor_uuid,omitempty"`
	Commit    string `json:"commit,omitempty"`
	HeadMoved *bool  `json:"head_moved,omitempty"`
	Stale     *bool  `json:"stale,omitempty"`
	State     string `json:"state,omitempty"`
	CommentID int    `json:"comment_id,omitempty"`
	Detail    string `json:"detail,omitempty"`

	at time.Time
}

func runPRActivity(cmd *cobra.Command, args []string) error {
	ctx := cmdCtx(cmd)

	client, err := prNewAPIClient()
	if err != nil {
		return err
	}
	repoCtx, rest, err := repoAndArgs(args, 1)
	if err != nil {
		return err
	}
	prID, err := strconv.Atoi(rest[0])
	if err != nil {
		return fmt.Errorf("invalid pr-number %q", rest[0])
	}

	resp, err := client.GetRepositoriesWorkspaceRepoSlugPullrequestsPullRequestIdActivity(ctx, repoCtx.Workspace, repoCtx.RepoSlug, prID)
	if err != nil {
		return fmt.Errorf("failed to get activity: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	entries, err := fetchAllPages[prActivityEntry](ctx, client, resp)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	events := buildPRActivity(entries)

	headers := []string{"DATE", "EVENT", "ACTOR", "COMMIT", "HEAD_MOVED", "STALE", "DETAIL"}
	if len(events) == 0 && output.CurrentFormat == output.FormatTable {
		output.Info("No activity found")
		return nil
	}
	var rows [][]string
	for _, e := range events {
		rows = append(rows, []string{e.Date, e.Event, e.Actor, e.Commit, optBool(e.HeadMoved), optBool(e.Stale), e.Detail + e.State})
	}
	return output.Print(headers, rows, events)
}

// buildPRActivity normalises entries, orders them oldest first and derives
// head movement and review staleness from the update history.
func buildPRActivity(entries []prActivityEntry) []prActivityEvent {
	events := make([]prActivityEvent, 0, len(entries))
	for _, entry := range entries {
		var e prActivityEvent
		switch {
		case entry.Approval != nil:
			e = reviewEvent("approval", entry.Approval)
		case entry.ChangesRequested != nil:
			e = reviewEvent("changes_requested", entry.ChangesRequested)
		case entry.Update != nil:
			u := entry.Update
			e = prActivityEvent{Date: u.Date, Event: "update", Actor: u.Author.DisplayName, ActorUUID: u.Author.UUID, State: u.State}
			if u.Source.Commit != nil {
				e.Commit = u.Source.Commit.Hash
			}
		case entry.Comment != nil:
			c := entry.Comment
			e = prActivityEvent{Date: c.CreatedOn, Event: "comment", Actor: c.User.DisplayName, ActorUUID: c.User.UUID, CommentID: c.ID, Detail: firstLine(c.Content.Raw)}
		default:
			e = unknownActivityEvent(entry.Raw)
		}
		e.at = parseActivityTime(e.Date)
		events = append(events, e)
	}
	// Bitbucket lists newest first; reverse so equal dates stay chronological.
	// Unparseable dates sort first.
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].at.Before(events[j].at) })

	head := ""
	var reviews []int
	for i := range events {
		e := &events[i]
		switch e.Event {
		case "update":
			if e.Commit == "" {
				continue
			}
			if head != "" {
				moved := e.Commit != head
				e.HeadMoved = &moved
				if moved {
					for _, r := range reviews {
						stale := true
						events[r].Stale = &stale
					}
					reviews = nil
				}
			}
			head = e.Commit
		case "approval", "changes_requested":
			if head == "" {
				continue
			}
			e.Commit = head
			fresh := false
			e.Stale = &fresh
			reviews = append(reviews, i)
		}
	}
	return events
}

func reviewEvent(kind string, r *prActivityReview) prActivityEvent {
	return prActivityEvent{Date: r.Date, Event: kind, Actor: r.User.DisplayName, ActorUUID: r.User.UUID}
}

func unknownActivityEvent(raw map[string]json.RawMessage) prActivityEvent {
	keys := make([]string, 0, len(raw))
	for k := range raw {
		if k != "pull_request" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return prActivityEvent{Event: "unknown"}
	}
	e := prActivityEvent{Event: keys[0]}
	var body prActivityReview
	if json.Unmarshal(raw[keys[0]], &body) == nil {
		e.Date, e.Actor, e.ActorUUID = body.Date, body.User.DisplayName, body.User.UUID
	}
	return e
}

func parseActivityTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func optBool(b *bool) string {
	if b == nil {
		return ""
	}
	return strconv.FormatBool(*b)
}
