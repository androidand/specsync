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

// IssueStates reads the state of every GitHub issue in refs with one GraphQL
// call. Refs for another provider, or that are not owner/name and a number,
// are "unknown" and never sent; so is an issue GitHub cannot find.
func (p *GitHubProvider) IssueStates(ctx context.Context, refs []Ref) (map[string]IssueState, error) {
	out := make(map[string]IssueState, len(refs))
	var query strings.Builder
	var aliased []Ref
	for _, r := range refs {
		out[IssueStateKey(r)] = IssueState{State: "unknown"}
		owner, name, ok := strings.Cut(RepoFromProviderKey(r.Provider), "/")
		number, err := strconv.Atoi(r.ID)
		if !ok || err != nil || number <= 0 || !repoPart.MatchString(owner) || !repoPart.MatchString(name) {
			continue
		}
		fmt.Fprintf(&query, "i%d:repository(owner:%q,name:%q){issue(number:%d){state stateReason}} ", len(aliased), owner, name, number)
		aliased = append(aliased, r)
	}
	if len(aliased) == 0 {
		return out, nil
	}

	raw, runErr := p.run(ctx, "api", "graphql", "-f", "query={"+query.String()+"}")
	if raw == "" && runErr != nil {
		raw = jsonObjectIn(runErr.Error())
	}
	var resp struct {
		Data map[string]struct {
			Issue *struct {
				State       string `json:"state"`
				StateReason string `json:"stateReason"`
			} `json:"issue"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil || resp.Data == nil {
		if runErr != nil {
			return nil, runErr
		}
		return nil, fmt.Errorf("parse graphql: %w", err)
	}
	for i, r := range aliased {
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
	return out, nil
}

// jsonObjectIn returns the outermost {...} in s, or "". gh reports a partly
// failed GraphQL call as an error that still carries the response body.
func jsonObjectIn(s string) string {
	i, j := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if i < 0 || j < i {
		return ""
	}
	return s[i : j+1]
}
