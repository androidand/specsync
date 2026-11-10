package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var repoPart = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

const issueStateBatch = 50

// IssueStates reads the state of every GitHub issue in refs, at most 50 per
// GraphQL call. Refs for another provider, or that are not owner/name and a
// number, are "unknown" and never sent; so is an issue GitHub cannot find.
func (p *GitHubProvider) IssueStates(ctx context.Context, refs []Ref) (map[string]IssueState, error) {
	out := make(map[string]IssueState, len(refs))
	var queryable []Ref
	for _, r := range refs {
		key := IssueStateKey(r)
		if _, seen := out[key]; seen {
			continue
		}
		out[key] = IssueState{State: "unknown"}
		owner, name, ok := strings.Cut(RepoFromProviderKey(r.Provider), "/")
		number, err := strconv.Atoi(r.ID)
		if ok && err == nil && number > 0 && repoPart.MatchString(owner) && repoPart.MatchString(name) {
			queryable = append(queryable, r)
		}
	}
	for start := 0; start < len(queryable); start += issueStateBatch {
		end := min(start+issueStateBatch, len(queryable))
		if err := p.issueStateBatch(ctx, queryable[start:end], out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (p *GitHubProvider) issueStateBatch(ctx context.Context, batch []Ref, out map[string]IssueState) error {
	var query strings.Builder
	for i, r := range batch {
		owner, name, _ := strings.Cut(RepoFromProviderKey(r.Provider), "/")
		number, _ := strconv.Atoi(r.ID)
		fmt.Fprintf(&query, "i%d:repository(owner:%q,name:%q){issue(number:%d){state stateReason}} ", i, owner, name, number)
	}

	raw, runErr := p.run(ctx, "api", "graphql", "-f", "query={"+query.String()+"}")
	if raw == "" && runErr != nil {
		raw = ghErrorBody(runErr.Error())
	}
	var resp struct {
		Data map[string]struct {
			Issue *struct {
				State       string `json:"state"`
				StateReason string `json:"stateReason"`
			} `json:"issue"`
		} `json:"data"`
	}
	err := json.NewDecoder(strings.NewReader(raw)).Decode(&resp)
	if err != nil || resp.Data == nil {
		if runErr != nil {
			return runErr
		}
		return fmt.Errorf("parse graphql: %w", err)
	}
	for i, r := range batch {
		repo, ok := resp.Data[fmt.Sprintf("i%d", i)]
		if !ok || repo.Issue == nil {
			continue
		}
		st := IssueState{State: strings.ToLower(repo.Issue.State)}
		if st.State == "closed" {
			st.Reason = strings.ToLower(strings.ReplaceAll(repo.Issue.StateReason, "_", " "))
		}
		out[IssueStateKey(r)] = st
	}
	return nil
}

// ghErrorBody returns the output gh printed, which runGH appends after the
// first line of its error (the echoed command, whose query holds braces),
// possibly followed by an error line, so it is decoded as a prefix.
func ghErrorBody(msg string) string {
	_, body, ok := strings.Cut(msg, "\n")
	if !ok {
		return ""
	}
	if i := strings.Index(body, "{"); i >= 0 {
		return body[i:]
	}
	return ""
}
