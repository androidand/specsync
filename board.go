package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Default stage->Status-name mapping. Both are overridable per stage via
// BoardTarget.StatusMapping; when a board doesn't name its options literally
// this way, resolution falls back positionally (first non-terminal / terminal).
const (
	defaultActiveStatus   = "In progress"
	defaultArchivedStatus = "Done"
)

// boardSchema is a target board resolved once per run: its ProjectV2 node id and
// its single-select Status field (id + options). Ids are discovered at runtime,
// never hard-coded, so the mapping generalizes across boards.
type boardSchema struct {
	projectID   string
	statusField statusField
}

type statusOption struct {
	id   string
	name string
}

// statusField holds the Status field id and its options in board order, plus a
// name->option-id lookup. Option order matters: the last option is treated as
// terminal ("Done"-like) for the positional default of the archived stage.
type statusField struct {
	id      string
	name    string
	options []statusOption
	byName  map[string]string
}

func (f statusField) optionID(name string) (string, bool) {
	id, ok := f.byName[name]
	return id, ok
}

// ProjectOntoBoard satisfies the BoardProjector capability. See the interface
// doc: it ensures ref's issue is on target, maps item.Stage to the board Status,
// and assigns it, idempotently and without clobbering human curation.
func (p *GitHubProvider) ProjectOntoBoard(ctx context.Context, target BoardTarget, ref Ref, item WorkItem, dryRun bool) (BoardPlan, error) {
	if !target.Configured() {
		// Unconfigured target: no board calls at all (backward-compatible).
		return BoardPlan{}, nil
	}

	if dryRun {
		// Zero-API dry-run: preview the intent from config alone, issuing no gh
		// calls (honoring the existing dry-run contract). Membership and current
		// curation are unknown without a query, so the plan states what specsync
		// would ensure rather than what it observed.
		name, _ := target.statusNameFor(item.Stage)
		login := target.Assignee
		if login == "" || strings.EqualFold(login, "me") || login == "@me" {
			login = "me"
		}
		return BoardPlan{
			StatusField:   "Status",
			AddedToBoard:  true,
			StatusName:    name,
			AssigneeLogin: login,
		}, nil
	}

	schema, err := p.resolveBoardSchema(ctx, target)
	if err != nil {
		return BoardPlan{}, err
	}
	plan := BoardPlan{ProjectID: schema.projectID, StatusField: schema.statusField.name}

	// Resolve the target Status name+option before touching the board so an
	// unknown *configured* status fails loud rather than after a partial write.
	wantName, wantOption, err := p.resolveStatus(target, item.Stage, schema.statusField)
	if err != nil {
		return BoardPlan{}, err
	}

	member, err := p.boardMembership(ctx, ref, schema)
	if err != nil {
		return BoardPlan{}, err
	}
	plan.AlreadyOnBoard = member.itemID != ""
	plan.CurrentStatus = member.statusName

	itemID := member.itemID
	if itemID == "" {
		itemID, err = p.addToBoard(ctx, schema.projectID, member.contentID)
		if err != nil {
			return BoardPlan{}, err
		}
		plan.AddedToBoard = true
	}

	// Status: set only when unset or when the current value is one specsync
	// manages; a human-moved card wins.
	if wantName != "" {
		if reason := statusClobberReason(member.statusName, target, schema.statusField); reason != "" {
			plan.StatusSkipped = reason
		} else if member.statusName != wantName {
			if err := p.setStatus(ctx, schema.projectID, itemID, schema.statusField.id, wantOption); err != nil {
				return BoardPlan{}, err
			}
			plan.StatusName = wantName
		}
	}

	// Assignee: add the acting viewer (or a configured login) only when the issue
	// has none; never remove or replace an existing assignee.
	if member.assigneeCount > 0 {
		plan.AssignSkipped = "issue already has an assignee"
	} else {
		login, userID, err := p.resolveAssignee(ctx, target.Assignee)
		if err != nil {
			return BoardPlan{}, err
		}
		if err := p.assign(ctx, member.contentID, userID); err != nil {
			return BoardPlan{}, err
		}
		plan.AssigneeLogin = login
	}

	return plan, nil
}

// statusNameFor returns the Status name for a stage and whether it came from an
// explicit configuration. An explicit name must resolve to a real option (fail
// loud); a default name may fall back positionally.
func (t BoardTarget) statusNameFor(stage Stage) (name string, explicit bool) {
	if t.StatusMapping != nil {
		if n, ok := t.StatusMapping[stage]; ok {
			return n, true
		}
	}
	if stage == StageArchived {
		return defaultArchivedStatus, false
	}
	return defaultActiveStatus, false
}

// resolveStatus maps a stage to the Status name and option id specsync will set.
func (p *GitHubProvider) resolveStatus(target BoardTarget, stage Stage, f statusField) (name, optionID string, err error) {
	wantName, explicit := target.statusNameFor(stage)
	return resolveStatusOption(f, wantName, explicit, stage)
}

// resolveStatusOption resolves a Status name to its option id. A configured name
// that the board doesn't define is an error listing the valid options; a default
// name falls back to the board's first non-terminal (or terminal, for archived)
// option so specsync works against boards with different Status labels.
func resolveStatusOption(f statusField, name string, explicit bool, stage Stage) (string, string, error) {
	if id, ok := f.optionID(name); ok {
		return name, id, nil
	}
	if explicit {
		return "", "", unknownStatusErr(name, f)
	}
	if fb, ok := positionalStatus(f, stage); ok {
		return fb.name, fb.id, nil
	}
	return "", "", unknownStatusErr(name, f)
}

// positionalStatus is the default when a board doesn't literally name its Status
// options "In progress"/"Done": archived maps to the terminal (last) option,
// everything else to the first (non-terminal) option.
func positionalStatus(f statusField, stage Stage) (statusOption, bool) {
	if len(f.options) == 0 {
		return statusOption{}, false
	}
	if stage == StageArchived {
		return f.options[len(f.options)-1], true
	}
	return f.options[0], true
}

// managedStatusNames is the set of Status names specsync itself sets across all
// stages. A board value inside this set is treated as specsync-written and thus
// safe to overwrite; a value outside it was set by a human and is left alone.
func managedStatusNames(target BoardTarget, f statusField) map[string]bool {
	managed := map[string]bool{}
	for _, st := range []Stage{StageActive, StageComplete, StageArchived} {
		name, explicit := target.statusNameFor(st)
		if n, _, err := resolveStatusOption(f, name, explicit, st); err == nil && n != "" {
			managed[n] = true
		}
	}
	return managed
}

func statusClobberReason(current string, target BoardTarget, f statusField) string {
	if current == "" {
		return ""
	}
	if managedStatusNames(target, f)[current] {
		return ""
	}
	return fmt.Sprintf("Status %q was set by a human", current)
}

func unknownStatusErr(name string, f statusField) error {
	var valid []string
	for _, o := range f.options {
		valid = append(valid, o.name)
	}
	return fmt.Errorf("unknown Status %q; valid board options: %s", name, strings.Join(valid, ", "))
}

// resolveBoardSchema resolves and caches (per run) the target's node id and
// Status schema so repeated projections in one run don't re-query.
func (p *GitHubProvider) resolveBoardSchema(ctx context.Context, target BoardTarget) (*boardSchema, error) {
	key := fmt.Sprintf("%s/%d", target.Owner, target.Number)
	p.boardMu.Lock()
	defer p.boardMu.Unlock()
	if p.boardCache == nil {
		p.boardCache = map[string]*boardSchema{}
	}
	if s, ok := p.boardCache[key]; ok {
		return s, nil
	}
	projectID, err := p.resolveProjectID(ctx, target.Owner, target.Number)
	if err != nil {
		return nil, err
	}
	field, err := p.resolveStatusField(ctx, projectID)
	if err != nil {
		return nil, err
	}
	s := &boardSchema{projectID: projectID, statusField: field}
	p.boardCache[key] = s
	return s, nil
}

// resolveProjectID resolves owner/number to a ProjectV2 node id, trying the org
// namespace first and falling back to the user namespace (a login is exactly one
// of the two; the org query errors for a user login, which we treat as a miss).
func (p *GitHubProvider) resolveProjectID(ctx context.Context, owner string, number int) (string, error) {
	type projectRef struct {
		ProjectV2 *struct {
			ID string `json:"id"`
		} `json:"projectV2"`
	}
	numVar := "-F"
	num := "number=" + strconv.Itoa(number)

	orgQ := `query($owner: String!, $number: Int!) { organization(login: $owner) { projectV2(number: $number) { id } } }`
	var org struct {
		Organization *projectRef `json:"organization"`
	}
	if err := p.graphql(ctx, "resolve project", orgQ, &org, "-f", "owner="+owner, numVar, num); err == nil {
		if org.Organization != nil && org.Organization.ProjectV2 != nil && org.Organization.ProjectV2.ID != "" {
			return org.Organization.ProjectV2.ID, nil
		}
	}

	userQ := `query($owner: String!, $number: Int!) { user(login: $owner) { projectV2(number: $number) { id } } }`
	var usr struct {
		User *projectRef `json:"user"`
	}
	if err := p.graphql(ctx, "resolve project", userQ, &usr, "-f", "owner="+owner, numVar, num); err != nil {
		return "", err
	}
	if usr.User != nil && usr.User.ProjectV2 != nil && usr.User.ProjectV2.ID != "" {
		return usr.User.ProjectV2.ID, nil
	}
	return "", fmt.Errorf("project %s/%d not found (checked org and user namespaces)", owner, number)
}

// resolveStatusField discovers the project's single-select Status field and its
// options by name, so field/option ids are never hard-coded.
func (p *GitHubProvider) resolveStatusField(ctx context.Context, projectID string) (statusField, error) {
	q := `query($projectId: ID!) {
  node(id: $projectId) {
    ... on ProjectV2 {
      fields(first: 50) {
        nodes {
          ... on ProjectV2SingleSelectField { id name options { id name } }
        }
      }
    }
  }
}`
	var r struct {
		Node struct {
			Fields struct {
				Nodes []struct {
					ID      string `json:"id"`
					Name    string `json:"name"`
					Options []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"options"`
				} `json:"nodes"`
			} `json:"fields"`
		} `json:"node"`
	}
	if err := p.graphql(ctx, "read project schema", q, &r, "-f", "projectId="+projectID); err != nil {
		return statusField{}, err
	}
	for _, n := range r.Node.Fields.Nodes {
		if n.ID == "" || !strings.EqualFold(n.Name, "Status") {
			continue
		}
		f := statusField{id: n.ID, name: n.Name, byName: map[string]string{}}
		for _, o := range n.Options {
			f.options = append(f.options, statusOption{id: o.ID, name: o.Name})
			f.byName[o.Name] = o.ID
		}
		return f, nil
	}
	return statusField{}, fmt.Errorf("project has no single-select \"Status\" field")
}

// boardMembership reports whether ref's issue is on the target board and, if so,
// its current Status; plus the issue node id (content id) and assignee count.
// This one query is uniform for a freshly-created issue (empty projectItems) and
// an existing one, and is the primitive every board action builds on.
type boardMembership struct {
	contentID     string
	itemID        string
	statusName    string
	assigneeCount int
}

func (p *GitHubProvider) boardMembership(ctx context.Context, ref Ref, schema *boardSchema) (boardMembership, error) {
	owner, repo, number, err := parseIssueURL(ref.URL)
	if err != nil {
		return boardMembership{}, err
	}
	q := `query($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    issue(number: $number) {
      id
      assignees(first: 1) { totalCount }
      projectItems(first: 50) {
        nodes {
          id
          project { id }
          fieldValues(first: 50) {
            nodes {
              __typename
              ... on ProjectV2ItemFieldSingleSelectValue {
                name
                field { ... on ProjectV2FieldCommon { id } }
              }
            }
          }
        }
      }
    }
  }
}`
	var r struct {
		Repository struct {
			Issue struct {
				ID        string `json:"id"`
				Assignees struct {
					TotalCount int `json:"totalCount"`
				} `json:"assignees"`
				ProjectItems struct {
					Nodes []struct {
						ID      string `json:"id"`
						Project struct {
							ID string `json:"id"`
						} `json:"project"`
						FieldValues struct {
							Nodes []struct {
								Typename string `json:"__typename"`
								Name     string `json:"name"`
								Field    struct {
									ID string `json:"id"`
								} `json:"field"`
							} `json:"nodes"`
						} `json:"fieldValues"`
					} `json:"nodes"`
				} `json:"projectItems"`
			} `json:"issue"`
		} `json:"repository"`
	}
	if err := p.graphql(ctx, "read board membership", q, &r,
		"-f", "owner="+owner, "-f", "repo="+repo, "-F", "number="+strconv.Itoa(number)); err != nil {
		return boardMembership{}, err
	}
	issue := r.Repository.Issue
	if issue.ID == "" {
		return boardMembership{}, fmt.Errorf("issue %s not found via GraphQL", ref.URL)
	}
	m := boardMembership{contentID: issue.ID, assigneeCount: issue.Assignees.TotalCount}
	for _, it := range issue.ProjectItems.Nodes {
		if it.Project.ID != schema.projectID {
			continue
		}
		m.itemID = it.ID
		for _, fv := range it.FieldValues.Nodes {
			if fv.Typename == "ProjectV2ItemFieldSingleSelectValue" && fv.Field.ID == schema.statusField.id {
				m.statusName = fv.Name
			}
		}
	}
	return m, nil
}

func (p *GitHubProvider) addToBoard(ctx context.Context, projectID, contentID string) (string, error) {
	q := `mutation($projectId: ID!, $contentId: ID!) {
  addProjectV2ItemById(input: { projectId: $projectId, contentId: $contentId }) { item { id } }
}`
	var r struct {
		AddProjectV2ItemById struct {
			Item struct {
				ID string `json:"id"`
			} `json:"item"`
		} `json:"addProjectV2ItemById"`
	}
	if err := p.graphql(ctx, "add to board", q, &r, "-f", "projectId="+projectID, "-f", "contentId="+contentID); err != nil {
		return "", err
	}
	return r.AddProjectV2ItemById.Item.ID, nil
}

func (p *GitHubProvider) setStatus(ctx context.Context, projectID, itemID, fieldID, optionID string) error {
	q := `mutation($projectId: ID!, $itemId: ID!, $fieldId: ID!, $optionId: String!) {
  updateProjectV2ItemFieldValue(input: {
    projectId: $projectId
    itemId: $itemId
    fieldId: $fieldId
    value: { singleSelectOptionId: $optionId }
  }) { projectV2Item { id } }
}`
	return p.graphql(ctx, "set Status", q, nil,
		"-f", "projectId="+projectID, "-f", "itemId="+itemID, "-f", "fieldId="+fieldID, "-f", "optionId="+optionID)
}

// resolveAssignee resolves the assignee login to a user node id. An empty login
// (or "me"/"@me") resolves the acting viewer.
func (p *GitHubProvider) resolveAssignee(ctx context.Context, login string) (string, string, error) {
	if login == "" || strings.EqualFold(login, "me") || login == "@me" {
		var r struct {
			Viewer struct {
				Login string `json:"login"`
				ID    string `json:"id"`
			} `json:"viewer"`
		}
		if err := p.graphql(ctx, "resolve viewer", `query { viewer { login id } }`, &r); err != nil {
			return "", "", err
		}
		return r.Viewer.Login, r.Viewer.ID, nil
	}
	var r struct {
		User *struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := p.graphql(ctx, "resolve assignee", `query($login: String!) { user(login: $login) { id } }`, &r, "-f", "login="+login); err != nil {
		return "", "", err
	}
	if r.User == nil || r.User.ID == "" {
		return "", "", fmt.Errorf("assignee %q is not a known GitHub user", login)
	}
	return login, r.User.ID, nil
}

func (p *GitHubProvider) assign(ctx context.Context, issueID, userID string) error {
	q := `mutation($assignableId: ID!, $assigneeIds: [ID!]!) {
  addAssigneesToAssignable(input: { assignableId: $assignableId, assigneeIds: $assigneeIds }) {
    assignable { ... on Issue { id } }
  }
}`
	return p.graphql(ctx, "assign", q, nil, "-f", "assignableId="+issueID, "-f", "assigneeIds[]="+userID)
}

// graphql runs a `gh api graphql` operation through the injected runner and
// decodes the response's data envelope into out (nil out = ignore the payload).
// vars are pre-formatted gh field flags: "-f k=v" (string) or "-F k=v" (typed).
// op names the operation for error messages.
func (p *GitHubProvider) graphql(ctx context.Context, op, query string, out any, vars ...string) error {
	args := append([]string{"api", "graphql", "-f", "query=" + query}, vars...)
	raw, err := p.run(ctx, args...)
	if err != nil {
		return classifyBoardError(op, err)
	}
	if out == nil {
		return nil
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return fmt.Errorf("%s: parse graphql response: %w", op, err)
	}
	payload := env.Data
	if len(payload) == 0 {
		// A faked runner may return bare data without the {"data":...} envelope.
		payload = json.RawMessage(raw)
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("%s: decode graphql data: %w", op, err)
	}
	return nil
}

// classifyBoardError turns a raw insufficient-scope failure into an actionable
// message naming the failing op and the required token scope, rather than
// surfacing the opaque GraphQL/permission error.
func classifyBoardError(op string, err error) error {
	low := strings.ToLower(err.Error())
	if strings.Contains(low, "insufficient_scopes") ||
		strings.Contains(low, "requires one of the following scopes") ||
		strings.Contains(low, "not accessible") ||
		(strings.Contains(low, "scope") && strings.Contains(low, "project")) {
		return fmt.Errorf("board op %q was rejected for insufficient token scope: "+
			"GitHub Projects needs the `project` scope (plus `repo` and `read:org`). "+
			"Grant it with `gh auth refresh -s project,read:org,repo` and retry.\nunderlying: %w", op, err)
	}
	return fmt.Errorf("%s: %w", op, err)
}

// parseIssueURL extracts owner, repo, and number from a GitHub issue URL. The
// issue node id is resolved via GraphQL from these coordinates.
func parseIssueURL(url string) (owner, repo string, number int, err error) {
	const prefix = "https://github.com/"
	if !strings.HasPrefix(url, prefix) {
		return "", "", 0, fmt.Errorf("cannot parse issue url %q", url)
	}
	parts := strings.Split(strings.TrimPrefix(url, prefix), "/")
	if len(parts) < 4 || parts[2] != "issues" {
		return "", "", 0, fmt.Errorf("cannot parse issue url %q", url)
	}
	n, err := strconv.Atoi(parts[3])
	if err != nil {
		return "", "", 0, fmt.Errorf("cannot parse issue number from %q: %w", url, err)
	}
	return parts[0], parts[1], n, nil
}
