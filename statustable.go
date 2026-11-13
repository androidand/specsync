package specsync

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// StatusTableHeader is the column order of the status table.
var StatusTableHeader = []string{"Issue", "What", "PR", "State", "Checks", "Review", "Size"}

// StatusTableRows turns work rows into table cells, one line per row. current
// is the RowKey to mark with an arrow; empty marks nothing. Rows are shown in
// the order given. The repository is named in the Issue cell only when the rows
// span more than one.
func StatusTableRows(rows []ChangeTopology, current string) [][]string {
	many := manyRepos(rows)
	out := make([][]string, 0, len(rows))
	for _, r := range rows {
		mark := ""
		if current != "" && RowKey(r) == current {
			mark = "▶ "
		}
		out = append(out, []string{
			mark + tableIssue(r, many), tableWhat(r), tablePR(r), tableState(r),
			tableChecks(r.PR), tableReview(r.PR), tableSize(r),
		})
	}
	return out
}

// RenderStatusTable draws rows as a box table no wider than width columns
// (0 means no limit). Long cells wrap at spaces. With hyperlinks, issue and
// request numbers are written as terminal hyperlinks (OSC 8) to their pages.
func RenderStatusTable(rows []ChangeTopology, current string, width int, hyperlinks bool) string {
	var links [][]tableLink
	if hyperlinks {
		links = statusTableLinks(rows)
	}
	return renderGrid(StatusTableHeader, StatusTableRows(rows, current), width, links)
}

// statusTableLinks is the links of each row, issue cell then request cell.
func statusTableLinks(rows []ChangeTopology) [][]tableLink {
	many := manyRepos(rows)
	out := make([][]tableLink, len(rows))
	for i, r := range rows {
		out[i] = issueLinks(r, many)
		if r.PR != nil && r.PR.URL != "" {
			out[i] = append(out[i], tableLink{fmt.Sprintf("#%d", r.PR.Number), r.PR.URL})
		}
	}
	return out
}

// StatusTableURLs lists the page of every issue and request in the table, one
// "label  url" per line, for pasting where terminal hyperlinks do not survive.
func StatusTableURLs(rows []ChangeTopology) []string {
	var out []string
	seen := map[string]bool{}
	many := manyRepos(rows)
	for i, r := range rows {
		prLabel := ""
		if r.PR != nil {
			prLabel = fmt.Sprintf("PR #%d", r.PR.Number)
		}
		for _, l := range statusTableLinks(rows)[i] {
			label := l.Label
			if prLabel != "" && l.URL == r.PR.URL {
				label = prLabel
				if many && r.Repo != "" {
					label = r.Repo[strings.LastIndex(r.Repo, "/")+1:] + " " + prLabel
				}
			}
			if l.URL == "" || seen[l.URL] {
				continue
			}
			seen[l.URL] = true
			out = append(out, label+"  "+l.URL)
		}
	}
	return out
}

func manyRepos(rows []ChangeTopology) bool {
	repos := map[string]bool{}
	for _, r := range rows {
		repos[r.Repo] = true
	}
	return len(repos) > 1
}

// tableLink is a label in a cell and the page it names.
type tableLink struct{ Label, URL string }

// issueLinks lists the issues a row stands for: its own and those its request
// closes, each with its page when the tracker or the request says where it is.
func issueLinks(r ChangeTopology, withRepo bool) []tableLink {
	var out []tableLink
	add := func(id, url string) {
		if id == "" {
			return
		}
		label := "#" + id
		for _, have := range out {
			if strings.HasSuffix(have.Label, label) {
				return
			}
		}
		if withRepo && r.Repo != "" {
			label = r.Repo[strings.LastIndex(r.Repo, "/")+1:] + label
		}
		out = append(out, tableLink{label, url})
	}
	if r.Issue != nil {
		add(r.Issue.ID, r.Issue.URL)
	}
	if r.PR != nil {
		closes := append([]string(nil), r.PR.Closes...)
		sort.Strings(closes)
		for _, id := range closes {
			add(id, closedIssueURL(r.PR, id))
		}
	}
	return out
}

// closedIssueURL names the page of an issue a request closes: the repository
// the request says when it does, else the request's own.
func closedIssueURL(pr *ChangeRequest, id string) string {
	for _, ref := range pr.ClosesRefs {
		if repo, n, ok := strings.Cut(ref, "#"); ok && n == id {
			if base, _, ok := strings.Cut(pr.URL, "://"); ok {
				host, _, _ := strings.Cut(strings.TrimPrefix(pr.URL, base+"://"), "/")
				return base + "://" + host + "/" + repo + "/issues/" + id
			}
		}
	}
	if i := strings.LastIndex(pr.URL, "/pull/"); i > 0 {
		return pr.URL[:i] + "/issues/" + id
	}
	return ""
}

func tableIssue(r ChangeTopology, withRepo bool) string {
	links := issueLinks(r, withRepo)
	if len(links) == 0 {
		return "–"
	}
	labels := make([]string, len(links))
	for i, l := range links {
		labels[i] = l.Label
	}
	return strings.Join(labels, " + ")
}

func tableWhat(r ChangeTopology) string {
	switch {
	case r.Title != "":
		return r.Title
	case r.PR != nil && r.PR.Title != "":
		return r.PR.Title
	case r.Branch != "":
		return "branch " + r.Branch + " (no change)"
	}
	return "–"
}

func tablePR(r ChangeTopology) string {
	if r.PR == nil {
		return "Not opened"
	}
	return fmt.Sprintf("#%d", r.PR.Number)
}

func tableState(r ChangeTopology) string {
	pr := r.PR
	if pr == nil {
		state := "Local"
		if l := r.Local; l != nil {
			switch {
			case l.Modified+l.Untracked > 0:
				state += ", uncommitted"
			case l.Ahead > 0:
				state += fmt.Sprintf(", %d unpushed", l.Ahead)
			case l.Upstream == "":
				state += ", not pushed"
			default:
				state += ", clean"
			}
		}
		return state
	}
	state := strings.ToUpper(pr.State[:1]) + pr.State[1:]
	if pr.Draft && pr.State == "open" {
		state = "Draft"
	}
	if pr.State == "open" {
		switch pr.Mergeable {
		case "conflicting":
			state += ", conflicts"
		case "mergeable":
			if len(r.Attention) > 0 && containsString(r.Attention, "ready-to-merge") {
				state += ", ready to merge"
			}
		}
	}
	return state
}

func tableChecks(pr *ChangeRequest) string {
	if pr == nil || pr.State != "open" {
		return "–"
	}
	c := pr.Checks
	total := c.Pass + c.Fail + c.Pending
	switch {
	case total == 0:
		return "none"
	case c.Fail == 0 && c.Pending == 0:
		return fmt.Sprintf("all %d pass", c.Pass)
	}
	parts := []string{fmt.Sprintf("%d pass", c.Pass)}
	if c.Fail > 0 {
		f := fmt.Sprintf("%d failing", c.Fail)
		if len(c.Failing) > 0 {
			f += " (" + strings.Join(c.Failing, ", ") + ")"
		}
		parts = append(parts, f)
	}
	if c.Pending > 0 {
		parts = append(parts, fmt.Sprintf("%d pending", c.Pending))
	}
	return strings.Join(parts, ", ")
}

func tableReview(pr *ChangeRequest) string {
	if pr == nil || pr.State != "open" {
		return "–"
	}
	var parts []string
	switch pr.ReviewDecision {
	case "approved":
		parts = append(parts, "Approved")
	case "changes_requested":
		parts = append(parts, "Changes requested")
	case "review_required":
		parts = append(parts, "Needs review")
	}
	var approved, changes []string
	for _, v := range pr.Reviews {
		switch v.State {
		case "approved":
			approved = append(approved, v.Reviewer)
		case "changes_requested":
			changes = append(changes, v.Reviewer)
		}
	}
	// A repository with no required-review rule reports no decision, so the
	// verdicts themselves say it.
	if len(parts) == 0 {
		switch {
		case len(changes) > 0:
			parts = append(parts, "Changes requested")
		case len(approved) > 0:
			parts = append(parts, "Approved")
		case len(pr.ReviewRequests) == 0:
			parts = append(parts, "No review")
		}
	}
	who := func(label string, names []string) {
		if len(names) == 0 {
			return
		}
		if len(parts) > 0 && parts[0] == label {
			parts[0] = label + " by " + strings.Join(names, ", ")
			return
		}
		parts = append(parts, strings.ToLower(label)+" by "+strings.Join(names, ", "))
	}
	who("Approved", approved)
	who("Changes requested", changes)
	if len(pr.ReviewRequests) > 0 {
		parts = append(parts, "requested from "+strings.Join(pr.ReviewRequests, ", "))
	}
	if n := pr.UnresolvedThreads; n != nil && *n > 0 {
		parts = append(parts, fmt.Sprintf("%d unresolved", *n))
	}
	return strings.Join(parts, "; ")
}

func tableSize(r ChangeTopology) string {
	files, add, del := 0, 0, 0
	switch {
	case r.PR != nil && r.PR.Size.Files > 0:
		files, add, del = r.PR.Size.Files, r.PR.Size.Additions, r.PR.Size.Deletions
	case r.Local != nil && r.Local.Size != nil:
		files, add, del = r.Local.Size.Files, r.Local.Size.Additions, r.Local.Size.Deletions
	}
	if files == 0 {
		return "–"
	}
	noun := "files"
	if files == 1 {
		noun = "file"
	}
	return fmt.Sprintf("%d %s, +%d -%d", files, noun, add, del)
}

// renderGrid draws a box table. Columns shrink, widest first, until the table
// fits width; their cells wrap at spaces.
func renderGrid(header []string, rows [][]string, width int, links [][]tableLink) string {
	n := len(header)
	w := make([]int, n)
	all := append([][]string{header}, rows...)
	for _, row := range all {
		for i, c := range row {
			if l := utf8.RuneCountInString(c); l > w[i] {
				w[i] = l
			}
		}
	}
	const minCol = 10
	total := func() int {
		t := 1
		for _, x := range w {
			t += x + 3
		}
		return t
	}
	for width > 0 && total() > width {
		widest := 0
		for i := range w {
			if w[i] > w[widest] {
				widest = i
			}
		}
		if w[widest] <= minCol {
			break
		}
		w[widest]--
	}
	rule := func(l, m, r string) string {
		segs := make([]string, n)
		for i := range w {
			segs[i] = strings.Repeat("─", w[i]+2)
		}
		return l + strings.Join(segs, m) + r + "\n"
	}
	line := func(row []string, ls []tableLink) string {
		cells := make([][]string, n)
		height := 1
		for i := range row {
			cells[i] = wrapCell(row[i], w[i])
			if len(cells[i]) > height {
				height = len(cells[i])
			}
		}
		var b strings.Builder
		for h := 0; h < height; h++ {
			b.WriteString("│")
			for i := range cells {
				text := ""
				if h < len(cells[i]) {
					text = cells[i][h]
				}
				fmt.Fprintf(&b, " %s%s │", linkify(text, ls), strings.Repeat(" ", w[i]-utf8.RuneCountInString(text)))
			}
			b.WriteString("\n")
		}
		return b.String()
	}
	var b strings.Builder
	b.WriteString(rule("┌", "┬", "┐"))
	b.WriteString(line(header, nil))
	b.WriteString(rule("├", "┼", "┤"))
	for i, row := range rows {
		if i > 0 {
			b.WriteString(rule("├", "┼", "┤"))
		}
		var ls []tableLink
		if i < len(links) {
			ls = links[i]
		}
		b.WriteString(line(row, ls))
	}
	b.WriteString(rule("└", "┴", "┘"))
	return b.String()
}

// wrapCell splits text into lines of at most width runes, breaking at spaces;
// a word longer than width is cut.
func wrapCell(text string, width int) []string {
	if width <= 0 || utf8.RuneCountInString(text) <= width {
		return []string{text}
	}
	var lines []string
	cur := ""
	flush := func() {
		if cur != "" {
			lines = append(lines, cur)
			cur = ""
		}
	}
	for _, word := range strings.Fields(text) {
		for utf8.RuneCountInString(word) > width {
			flush()
			r := []rune(word)
			lines = append(lines, string(r[:width]))
			word = string(r[width:])
		}
		switch {
		case cur == "":
			cur = word
		case utf8.RuneCountInString(cur)+1+utf8.RuneCountInString(word) <= width:
			cur += " " + word
		default:
			flush()
			cur = word
		}
	}
	flush()
	return lines
}

// RelatedRows lists the rows for the status table: the current work first,
// then the epic's children, what blocks it, what it blocks and what is linked,
// each once. Items the workspace did not resolve have no row and are skipped.
func RelatedRows(current ChangeTopology, rel *RelatedWork) []ChangeTopology {
	rows := []ChangeTopology{current}
	if rel == nil {
		return rows
	}
	seen := map[string]bool{RowKey(current): true}
	add := func(items []RelatedItem) {
		for _, it := range items {
			if it.Row == nil || seen[it.Key] {
				continue
			}
			seen[it.Key] = true
			rows = append(rows, *it.Row)
		}
	}
	if rel.Epic != nil {
		add(rel.Epic.Items)
	}
	add(rel.BlockedBy)
	add(rel.Blocks)
	add(rel.Linked)
	return rows
}

// linkify wraps each label found in text in a terminal hyperlink. A label
// followed by a digit is a longer number, so it is left alone.
func linkify(text string, links []tableLink) string {
	for _, l := range links {
		if l.URL == "" {
			continue
		}
		var b strings.Builder
		rest := text
		for {
			i := strings.Index(rest, l.Label)
			if i < 0 {
				b.WriteString(rest)
				break
			}
			end := i + len(l.Label)
			b.WriteString(rest[:i])
			if end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
				b.WriteString(l.Label)
			} else {
				b.WriteString("\x1b]8;;" + l.URL + "\x1b\\" + l.Label + "\x1b]8;;\x1b\\")
			}
			rest = rest[end:]
		}
		text = b.String()
	}
	return text
}
