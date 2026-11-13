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
	repos := map[string]bool{}
	for _, r := range rows {
		repos[r.Repo] = true
	}
	many := len(repos) > 1
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
// (0 means no limit). Long cells wrap at spaces.
func RenderStatusTable(rows []ChangeTopology, current string, width int) string {
	return renderGrid(StatusTableHeader, StatusTableRows(rows, current), width)
}

func tableIssue(r ChangeTopology, withRepo bool) string {
	var ids []string
	add := func(id string) {
		if id == "" {
			return
		}
		id = "#" + id
		for _, have := range ids {
			if strings.HasSuffix(have, id) {
				return
			}
		}
		if withRepo && r.Repo != "" {
			id = r.Repo[strings.LastIndex(r.Repo, "/")+1:] + id
		}
		ids = append(ids, id)
	}
	if r.Issue != nil {
		add(r.Issue.ID)
	}
	if r.PR != nil {
		closes := append([]string(nil), r.PR.Closes...)
		sort.Strings(closes)
		for _, c := range closes {
			add(c)
		}
	}
	if len(ids) == 0 {
		return "–"
	}
	return strings.Join(ids, " + ")
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
func renderGrid(header []string, rows [][]string, width int) string {
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
	line := func(row []string) string {
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
				fmt.Fprintf(&b, " %s%s │", text, strings.Repeat(" ", w[i]-utf8.RuneCountInString(text)))
			}
			b.WriteString("\n")
		}
		return b.String()
	}
	var b strings.Builder
	b.WriteString(rule("┌", "┬", "┐"))
	b.WriteString(line(header))
	b.WriteString(rule("├", "┼", "┤"))
	for i, row := range rows {
		if i > 0 {
			b.WriteString(rule("├", "┼", "┤"))
		}
		b.WriteString(line(row))
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
