package specsync

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// StatusTableHeader is the column order of the status table.
var StatusTableHeader = []string{"Issue", "What", "PR", "State", "Checks", "Review", "Size"}

// StatusTableRows turns work rows into table cells, one line per row. current
// is the RowKey to mark with an arrow; empty marks nothing. Rows are shown in
// the order given. The repository is named in the Issue cell only when the rows
// span more than one. requestsKnown is false when pull requests were not looked
// up, so a row without one reads "Unknown" instead of "Not opened".
func StatusTableRows(rows []ChangeTopology, current string, requestsKnown bool) [][]string {
	many := manyRepos(rows)
	out := make([][]string, 0, len(rows))
	for _, r := range rows {
		mark := ""
		if current != "" && RowKey(r) == current {
			mark = "▶ "
		}
		cells := []string{
			mark + tableIssue(r, many), tableWhat(r), tablePR(r, requestsKnown), tableState(r),
			tableChecks(r.PR), tableReview(r.PR), tableSize(r),
		}
		for i := range cells {
			cells[i] = plainCell(cells[i])
		}
		out = append(out, cells)
	}
	return out
}

// RenderStatusTable draws rows as a box table no wider than width columns
// (0 means no limit). Long cells wrap at spaces. With hyperlinks, issue and
// request numbers are written as terminal hyperlinks (OSC 8) to their pages.
func RenderStatusTable(rows []ChangeTopology, current string, width int, hyperlinks, requestsKnown bool) string {
	var links []rowLinks
	if hyperlinks {
		links = statusTableLinks(rows)
	}
	return renderGrid(StatusTableHeader, StatusTableRows(rows, current, requestsKnown), width, links)
}

// tableLink is one number in a cell and the page it names.
type tableLink struct {
	Label    string // as the cell shows it: "#42" or "api#42"
	URL      string
	Kind     string // "issue" or "PR"
	Num      string
	Repo     string // short name of the repository the page is in
	ShowRepo bool   // the repository is not the obvious one
}

// rowLinks are the links of one row: those of its Issue cell and of its PR cell.
type rowLinks struct {
	Issues []tableLink
	PR     *tableLink
}

func statusTableLinks(rows []ChangeTopology) []rowLinks {
	many := manyRepos(rows)
	out := make([]rowLinks, len(rows))
	for i, r := range rows {
		out[i].Issues = issueLinks(r, many)
		if r.PR != nil && r.PR.URL != "" {
			out[i].PR = &tableLink{Label: fmt.Sprintf("#%d", r.PR.Number), URL: r.PR.URL, Kind: "PR",
				Num: strconv.Itoa(r.PR.Number), Repo: shortRepo(r.Repo), ShowRepo: many}
		}
	}
	return out
}

// StatusTableURLs lists the page of every issue and request in the table, one
// "label  url" per line with the labels aligned, for pasting where terminal
// hyperlinks do not survive. The label names the repository when the table
// spans several or the page is in another one.
func StatusTableURLs(rows []ChangeTopology) []string {
	var labels, urls []string
	seen := map[string]bool{}
	add := func(l tableLink) {
		if l.URL == "" || seen[l.URL] {
			return
		}
		seen[l.URL] = true
		label := l.Kind + " #" + l.Num
		if l.ShowRepo && l.Repo != "" {
			label = l.Repo + " " + label
		}
		labels, urls = append(labels, label), append(urls, l.URL)
	}
	for _, rl := range statusTableLinks(rows) {
		for _, l := range rl.Issues {
			add(l)
		}
		if rl.PR != nil {
			add(*rl.PR)
		}
	}
	width := 0
	for _, l := range labels {
		if n := utf8.RuneCountInString(l); n > width {
			width = n
		}
	}
	out := make([]string, len(labels))
	for i := range labels {
		out[i] = labels[i] + strings.Repeat(" ", width-utf8.RuneCountInString(labels[i])) + "  " + urls[i]
	}
	return out
}

// plainCell makes text safe for one table line: tracker-supplied titles and
// names may hold newlines or terminal escape sequences, which would break the
// grid or drive the terminal. Control characters become spaces.
func plainCell(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

func manyRepos(rows []ChangeTopology) bool {
	repos := map[string]bool{}
	for _, r := range rows {
		repos[r.Repo] = true
	}
	return len(repos) > 1
}

func shortRepo(repo string) string { return repo[strings.LastIndex(repo, "/")+1:] }

// issueLinks lists the issues a row stands for: its own and those its request
// closes, each in the repository that holds it and with its page when known.
// An issue is the same issue only if repository and number both match.
func issueLinks(r ChangeTopology, many bool) []tableLink {
	type issue struct{ repo, id, url string }
	var found []issue
	seen := map[string]bool{}
	add := func(repo, id, url string) {
		key := strings.ToLower(repo) + "#" + id // GitHub repository names are case-insensitive
		if id == "" || seen[key] {
			return
		}
		seen[key] = true
		found = append(found, issue{repo, id, url})
	}
	if r.Issue != nil {
		// The issue may live in a tracker repository other than the code's.
		repo := r.Repo
		if tracker, ok := strings.CutPrefix(r.Issue.Provider, "github:"); ok && tracker != "" {
			repo = tracker
		}
		add(repo, r.Issue.ID, r.Issue.URL)
	}
	if pr := r.PR; pr != nil {
		var closed []issue
		for _, ref := range pr.ClosesRefs {
			if repo, id, ok := strings.Cut(ref, "#"); ok {
				closed = append(closed, issue{repo, id, ""})
			}
		}
		// Closes and ClosesRefs are not index-aligned: the request names a
		// repository only for some. A number a reference already placed is not
		// placed again in this repository.
		named := map[string]bool{}
		for _, c := range closed {
			named[c.id] = true
		}
		for _, id := range pr.Closes {
			if !named[id] {
				closed = append(closed, issue{r.Repo, id, ""})
			}
		}
		sort.SliceStable(closed, func(i, j int) bool {
			if closed[i].repo != closed[j].repo {
				return closed[i].repo < closed[j].repo
			}
			a, errA := strconv.Atoi(closed[i].id)
			b, errB := strconv.Atoi(closed[j].id)
			if errA != nil || errB != nil {
				return closed[i].id < closed[j].id
			}
			return a < b
		})
		for _, c := range closed {
			add(c.repo, c.id, issuePageURL(pr.URL, c.repo, c.id))
		}
	}
	out := make([]tableLink, len(found))
	for i, f := range found {
		other := f.repo != "" && !strings.EqualFold(f.repo, r.Repo)
		label := "#" + f.id
		if (many || other) && f.repo != "" {
			label = shortRepo(f.repo) + label
		}
		out[i] = tableLink{Label: label, URL: f.url, Kind: "issue", Num: f.id, Repo: shortRepo(f.repo), ShowRepo: many || other}
	}
	return out
}

// issuePageURL names the page of an issue in repo, on the host the request is on.
func issuePageURL(requestURL, repo, id string) string {
	scheme, rest, ok := strings.Cut(requestURL, "://")
	if !ok || repo == "" {
		return ""
	}
	host, _, _ := strings.Cut(rest, "/")
	return scheme + "://" + host + "/" + repo + "/issues/" + id
}

func tableIssue(r ChangeTopology, many bool) string {
	links := issueLinks(r, many)
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

func tablePR(r ChangeTopology, requestsKnown bool) string {
	if r.PR == nil {
		if !requestsKnown {
			return "Unknown"
		}
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
			case l.UpstreamGone:
				state += ", upstream gone"
			case l.StatusUnavailable:
				state += ", state unknown"
			default:
				state += ", clean"
			}
		}
		return state
	}
	state := "Unknown"
	if pr.State != "" {
		state = strings.ToUpper(pr.State[:1]) + pr.State[1:]
	}
	if pr.Draft && pr.State == "open" {
		state = "Draft"
	}
	if pr.State == "open" {
		switch pr.Mergeable {
		case "conflicting":
			state += ", conflicts"
		case "mergeable":
			if containsString(r.Attention, "ready-to-merge") {
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
func renderGrid(header []string, rows [][]string, width int, links []rowLinks) string {
	n := len(header)
	w := make([]int, n)
	all := append([][]string{header}, rows...)
	for _, row := range all {
		for i, c := range row {
			if l := displayWidth(c); l > w[i] {
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
	line := func(row []string, rl *rowLinks) string {
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
				fmt.Fprintf(&b, " %s%s │", linkify(text, cellLinks(rl, i)), strings.Repeat(" ", w[i]-displayWidth(text)))
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
		var rl *rowLinks
		if i < len(links) {
			rl = &links[i]
		}
		b.WriteString(line(row, rl))
	}
	b.WriteString(rule("└", "┴", "┘"))
	return b.String()
}

// wrapCell splits text into lines at most width terminal columns wide, breaking
// at spaces; a word longer than width is cut between characters.
func wrapCell(text string, width int) []string {
	if width <= 0 || displayWidth(text) <= width {
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
		for displayWidth(word) > width {
			flush()
			cut, cols := 0, 0
			for i, r := range word {
				if cols+runeWidth(r) > width && cols > 0 {
					break
				}
				cols += runeWidth(r)
				cut = i + utf8.RuneLen(r)
			}
			lines = append(lines, word[:cut])
			word = word[cut:]
		}
		switch {
		case cur == "":
			cur = word
		case displayWidth(cur)+1+displayWidth(word) <= width:
			cur += " " + word
		default:
			flush()
			cur = word
		}
	}
	flush()
	return lines
}

// displayWidth is how many terminal columns text takes.
func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		n += runeWidth(r)
	}
	return n
}

// runeWidth is the columns one rune takes: 0 for combining marks and format
// characters (joiners, variation selectors), 2 for East Asian wide characters
// and emoji, else 1. It is a compact table, not the whole of Unicode.
func runeWidth(r rune) int {
	switch {
	case unicode.Is(unicode.Mn, r), unicode.Is(unicode.Me, r), unicode.Is(unicode.Cf, r):
		return 0
	case r >= 0x1100 && r <= 0x115F,
		r >= 0x2E80 && r <= 0xA4CF,
		r >= 0xAC00 && r <= 0xD7A3,
		r >= 0xF900 && r <= 0xFAFF,
		r >= 0xFE30 && r <= 0xFE6F,
		r >= 0xFF00 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x1F300 && r <= 0x1F64F,
		r >= 0x1F680 && r <= 0x1F6FF,
		r >= 0x1F900 && r <= 0x1F9FF,
		r >= 0x20000 && r <= 0x3FFFD,
		r == 0x2705, r == 0x2728, r == 0x274C, r == 0x274E, r == 0x2753, r == 0x2757, r == 0x26A1, r == 0x2B50:
		return 2
	}
	return 1
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

// cellLinks are the links that belong in column col of a row: the issue links
// in the Issue column, the request link in the PR column, none elsewhere.
func cellLinks(rl *rowLinks, col int) []tableLink {
	if rl == nil {
		return nil
	}
	switch col {
	case 0:
		return rl.Issues
	case 2:
		if rl.PR != nil {
			return []tableLink{*rl.PR}
		}
	}
	return nil
}

// linkify wraps each label found in text in a terminal hyperlink. Only a whole
// number counts: a label inside a longer word or number ("#7" in "#70" or in
// "api#7") is left alone. Matches are found in the plain text first and never
// overlap, so no hyperlink is wrapped inside another.
func linkify(text string, links []tableLink) string {
	type hit struct {
		start, end int
		l          tableLink
	}
	var hits []hit
	for _, l := range links {
		if l.URL == "" || l.Label == "" {
			continue
		}
		for at := 0; ; {
			i := strings.Index(text[at:], l.Label)
			if i < 0 {
				break
			}
			i += at
			end := i + len(l.Label)
			before, _ := utf8.DecodeLastRuneInString(text[:i])
			after, _ := utf8.DecodeRuneInString(text[end:])
			if (i == 0 || !isLabelRune(before)) && (end == len(text) || !isLabelRune(after)) {
				hits = append(hits, hit{i, end, l})
			}
			at = end
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].start < hits[j].start })
	var b strings.Builder
	from := 0
	for _, h := range hits {
		if h.start < from { // overlaps one already written
			continue
		}
		b.WriteString(text[from:h.start])
		b.WriteString("\x1b]8;;" + h.l.URL + "\x1b\\" + h.l.Label + "\x1b]8;;\x1b\\")
		from = h.end
	}
	b.WriteString(text[from:])
	return b.String()
}

func isLabelRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '#' || r == '_'
}
