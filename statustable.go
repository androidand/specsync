package specsync

import (
	"fmt"
	"regexp"
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
// span more than one. A row whose pull request lookup did not happen reads
// "Unknown" in the PR column, not "Not opened".
func StatusTableRows(rows []ChangeTopology, current string) [][]string {
	return newTableCtx(rows).cells(rows, current)
}

// tableCtx is what the whole table decides once and every row then uses: each
// row's issues, whether the rows span several repositories, and which short
// repository names clash.
type tableCtx struct {
	issues [][]rowIssue
	many   bool
	clash  map[string]bool
}

func newTableCtx(rows []ChangeTopology) tableCtx {
	c := tableCtx{issues: make([][]rowIssue, len(rows)), many: manyRepos(rows)}
	for i, r := range rows {
		c.issues[i] = rowIssues(r)
	}
	c.clash = tableClashes(rows, c.issues)
	return c
}

func (c tableCtx) cells(rows []ChangeTopology, current string) [][]string {
	many, clash := c.many, c.clash
	out := make([][]string, 0, len(rows))
	for i, r := range rows {
		mark := ""
		if current != "" && RowKey(r) == current {
			mark = "▶ "
		}
		cells := []string{
			mark + tableIssue(c.issues[i], r, many, clash), tableWhat(r), tablePR(r), tableState(r),
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
func RenderStatusTable(rows []ChangeTopology, current string, width int, hyperlinks bool) string {
	c := newTableCtx(rows)
	var links []rowLinks
	if hyperlinks {
		links = c.links(rows)
	}
	return renderGrid(StatusTableHeader, c.cells(rows, current), width, links)
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

func statusTableLinks(rows []ChangeTopology) []rowLinks { return newTableCtx(rows).links(rows) }

func (c tableCtx) links(rows []ChangeTopology) []rowLinks {
	many, clash := c.many, c.clash
	out := make([]rowLinks, len(rows))
	for i, r := range rows {
		out[i].Issues = issueLinks(c.issues[i], r, many, clash)
		if r.PR != nil && safeURL(r.PR.URL) != "" {
			out[i].PR = &tableLink{Label: fmt.Sprintf("#%d", r.PR.Number), URL: safeURL(r.PR.URL), Kind: "PR",
				Num: strconv.Itoa(r.PR.Number), Repo: shortRepo(r.Repo), ShowRepo: many}
			if clash[strings.ToLower(shortRepo(r.Repo))] {
				out[i].PR.Repo, out[i].PR.ShowRepo = r.Repo, true
			}
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
		labels, urls = append(labels, plainCell(label)), append(urls, l.URL)
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
// grid or drive the terminal. It is SanitizeText: control characters are
// dropped and whitespace runs become one space.
func plainCell(s string) string { return SanitizeText(s, 1<<20) }

// safeURL returns u when it is an http(s) URL made of printable characters,
// else "": a URL goes unquoted into a terminal escape, so one holding ESC, BEL
// or other control characters could end the escape early and run the rest.
func safeURL(u string) string {
	if !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
		return ""
	}
	for _, r := range u {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return ""
		}
	}
	return u
}

func manyRepos(rows []ChangeTopology) bool {
	repos := map[string]bool{}
	for _, r := range rows {
		repos[strings.ToLower(r.Repo)] = true // repository names are case-insensitive
	}
	return len(repos) > 1
}

func shortRepo(repo string) string { return repo[strings.LastIndex(repo, "/")+1:] }

// rowIssue is one issue a row stands for, in the repository that holds it.
// closed is set only for the row's own issue, whose state the tracker reported.
type rowIssue struct {
	repo, id, url string
	closed        bool
}

// rowIssues lists the issues a row stands for: its own and those its request
// closes. An issue is the same issue only if repository and number both match.
func rowIssues(r ChangeTopology) []rowIssue {
	var found []rowIssue
	seen := map[string]bool{}
	add := func(repo, id, url string, closed bool) {
		key := strings.ToLower(repo) + "#" + id // GitHub repository names are case-insensitive
		if id == "" || seen[key] {
			return
		}
		seen[key] = true
		found = append(found, rowIssue{repo: repo, id: id, url: url, closed: closed})
	}
	if r.Issue != nil {
		// The issue may live in a tracker repository other than the code's.
		repo := r.Repo
		if tracker, ok := strings.CutPrefix(r.Issue.Provider, "github:"); ok && tracker != "" {
			repo = tracker
		}
		add(repo, r.Issue.ID, r.Issue.URL, strings.EqualFold(r.Issue.State, "closed"))
	}
	if pr := r.PR; pr != nil {
		var closed []rowIssue
		for _, ref := range pr.ClosesRefs {
			if repo, id, ok := strings.Cut(ref, "#"); ok {
				closed = append(closed, rowIssue{repo: repo, id: id})
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
				closed = append(closed, rowIssue{repo: r.Repo, id: id})
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
			add(c.repo, c.id, issuePageURL(pr.URL, c.repo, c.id), false)
		}
	}
	return found
}

// tableClashes is the set of lower-cased short repository names that more than
// one repository in the whole table answers to ("me/api" and "org/api"): those
// are written with their owner everywhere, in every row and in -urls.
func tableClashes(rows []ChangeTopology, issues [][]rowIssue) map[string]bool {
	owners := map[string]map[string]bool{} // short name -> lower-cased full names
	note := func(repo string) {
		if repo == "" {
			return
		}
		short := strings.ToLower(shortRepo(repo))
		if owners[short] == nil {
			owners[short] = map[string]bool{}
		}
		owners[short][strings.ToLower(repo)] = true
	}
	for i, r := range rows {
		note(r.Repo)
		for _, f := range issues[i] {
			note(f.repo)
		}
	}
	clash := map[string]bool{}
	for short, full := range owners {
		if len(full) > 1 {
			clash[short] = true
		}
	}
	return clash
}

// issueLinks lists a row's issues as cell links. A repository is named when the
// table spans several, when the issue is in another one than the row's, and
// with its owner when its short name clashes with another repository's.
func issueLinks(found []rowIssue, r ChangeTopology, many bool, clash map[string]bool) []tableLink {
	out := make([]tableLink, len(found))
	for i, f := range found {
		other := f.repo != "" && !strings.EqualFold(f.repo, r.Repo)
		label, repo := "#"+f.id, shortRepo(f.repo)
		switch {
		case f.repo != "" && clash[strings.ToLower(shortRepo(f.repo))]:
			label, repo = f.repo+"#"+f.id, f.repo
		case (many || other) && f.repo != "":
			label = shortRepo(f.repo) + label
		}
		out[i] = tableLink{Label: label, URL: safeURL(f.url), Kind: "issue", Num: f.id, Repo: repo, ShowRepo: many || other || repo != shortRepo(f.repo)}
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

func tableIssue(found []rowIssue, r ChangeTopology, many bool, clash map[string]bool) string {
	links := issueLinks(found, r, many, clash)
	if len(links) == 0 {
		return "–"
	}
	labels := make([]string, len(links))
	for i, l := range links {
		labels[i] = l.Label
		if found[i].closed {
			labels[i] += " closed"
		}
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
		if r.RequestsUnknown {
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
		if r.Branch == "" && r.Local == nil {
			return "No branch" // a change nobody has started: nothing local to describe
		}
		if l := r.Local; l != nil {
			switch {
			case l.StatusUnavailable: // git status failed, so the counts below mean nothing
				state += ", state unknown"
			case l.Modified+l.Untracked > 0:
				state += ", uncommitted"
			case l.Ahead > 0:
				state += fmt.Sprintf(", %d unpushed", l.Ahead)
			case l.Upstream == "":
				state += ", not pushed"
			case l.UpstreamGone:
				state += ", upstream gone"
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
		case len(pr.ReviewRequests) == 0 && !pr.ReviewersKnown:
			parts = append(parts, "Unknown") // reviewers were not fetched
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
// at spaces; a word longer than width is cut between characters, never inside
// one (a base character with its marks, variation selector or joiner sequence).
func wrapCell(text string, width int) []string {
	if width <= 0 || displayWidth(text) <= width {
		return []string{text}
	}
	var lines []string
	cur, curW := "", 0
	flush := func() {
		if cur != "" {
			lines = append(lines, cur)
			cur, curW = "", 0
		}
	}
	for _, word := range strings.Fields(text) {
		ww := displayWidth(word)
		if ww > width {
			flush()
			var piece strings.Builder
			pieceW := 0
			for _, cl := range clusters(word) {
				cw := displayWidth(cl)
				if pieceW+cw > width && pieceW > 0 {
					lines = append(lines, piece.String())
					piece.Reset()
					pieceW = 0
				}
				piece.WriteString(cl)
				pieceW += cw
			}
			cur, curW = piece.String(), pieceW
			continue
		}
		switch {
		case cur == "":
			cur, curW = word, ww
		case curW+1+ww <= width:
			cur, curW = cur+" "+word, curW+1+ww
		default:
			flush()
			cur, curW = word, ww
		}
	}
	flush()
	return lines
}

// clusters splits s into characters as a terminal draws them: a base rune with
// the zero-width runes after it (combining marks, variation selectors, joiners),
// a joiner pulling in the rune after it, and a regional indicator pair (a flag).
func clusters(s string) []string {
	var out []string
	rs := []rune(s)
	for i := 0; i < len(rs); {
		j := i + 1
		if isRegionalIndicator(rs[i]) && j < len(rs) && isRegionalIndicator(rs[j]) {
			j++
		}
		for j < len(rs) {
			switch {
			case runeWidth(rs[j]) == 0:
				if rs[j] == 0x200D && j+1 < len(rs) {
					j++ // the joiner pulls in what it joins
				}
				j++
			default:
				goto done
			}
		}
	done:
		out = append(out, string(rs[i:j]))
		i = j
	}
	return out
}

func isRegionalIndicator(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }

// displayWidth is how many terminal columns text takes. A joiner makes what
// follows part of the glyph before it, as in a family emoji. A variation selector
// (U+FE0F) asks for the emoji form of the symbol before it, which is wide;
// after a letter or ASCII it changes nothing.
func displayWidth(s string) int {
	n, prev := 0, rune(0)
	for _, r := range s {
		switch {
		case prev == 0x200D:
			// the rune a joiner joins is drawn as part of the glyph before it
		case r == 0xFE0F && prev > 0x7F && !unicode.IsLetter(prev) && runeWidth(prev) == 1:
			n++ // the emoji form of a narrow symbol takes two columns
		default:
			n += runeWidth(r)
		}
		prev = r
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
		r >= 0x1FA70 && r <= 0x1FAFF,
		r >= 0x1F7E0 && r <= 0x1F7EB, r == 0x1F7F0,
		r >= 0x20000 && r <= 0x3FFFD,
		r == 0x1F004, r == 0x1F0CF, r == 0x1F18E, r >= 0x1F191 && r <= 0x1F19A, r >= 0x1F200 && r <= 0x1F2FF:
		return 2
	}
	if r >= 0x231A && r <= 0x2B55 && wideSymbols[r] {
		return 2
	}
	return 1
}

// wideSymbols are the symbols below U+2B56 that default to the wide emoji form.
var wideSymbols = func() map[rune]bool {
	m := map[rune]bool{}
	for _, span := range [][2]rune{
		{0x231A, 0x231B}, {0x23E9, 0x23EC}, {0x23F0, 0x23F0}, {0x23F3, 0x23F3}, {0x25FD, 0x25FE},
		{0x2614, 0x2615}, {0x2648, 0x2653}, {0x267F, 0x267F}, {0x2693, 0x2693}, {0x26A1, 0x26A1},
		{0x26AA, 0x26AB}, {0x26BD, 0x26BE}, {0x26C4, 0x26C5}, {0x26CE, 0x26CE}, {0x26D4, 0x26D4},
		{0x26EA, 0x26EA}, {0x26F2, 0x26F3}, {0x26F5, 0x26F5}, {0x26FA, 0x26FA}, {0x26FD, 0x26FD},
		{0x2705, 0x2705}, {0x270A, 0x270B}, {0x2728, 0x2728}, {0x274C, 0x274C}, {0x274E, 0x274E},
		{0x2753, 0x2755}, {0x2757, 0x2757}, {0x2795, 0x2797}, {0x27B0, 0x27B0}, {0x27BF, 0x27BF},
		{0x2B1B, 0x2B1C}, {0x2B50, 0x2B50}, {0x2B55, 0x2B55},
	} {
		for r := span[0]; r <= span[1]; r++ {
			m[r] = true
		}
	}
	return m
}()

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

// StatusTableNotes lists what to say under the table, each line starting with
// "status:" or "incomplete:": how many rows have no pull request lookup, the
// related-work reasons that concern the rows shown, related items that resolved
// to no row, and status. A reason that status already carries is not repeated,
// and one about a repository no row belongs to is left out.
func StatusTableNotes(rows []ChangeTopology, status []string, related *RelatedWork) []string {
	var notes []string
	if n := countRequestsUnknown(rows); n > 0 {
		notes = append(notes, fmt.Sprintf("status: pull requests were not looked up for %d of %d rows, shown as Unknown", n, len(rows)))
	}
	inStatus := map[string]bool{}
	for _, s := range status {
		inStatus[s] = true
	}
	shown := map[string]bool{}
	for _, r := range rows {
		shown[strings.ToLower(r.Repo)] = true
	}
	if related != nil {
		for _, reason := range related.Incomplete {
			if inStatus[reason] {
				continue
			}
			if repo := reasonRepoOf(reason); repo != "" && !shown[strings.ToLower(repo)] {
				continue
			}
			notes = append(notes, "incomplete: "+plainCell(reason))
		}
		for _, key := range related.Unresolved() {
			notes = append(notes, "status: "+plainCell(key)+" is related but was not found in the workspace, so it has no row")
		}
	}
	for _, s := range status {
		notes = append(notes, "status: "+plainCell(s))
	}
	return notes
}

// repoReasons are the status reasons that are about one repository, and where
// in them it is named. Only these are matched, never any text with a slash in
// it: "local state unavailable for feat/243-x: ..." names a branch.
var repoReasons = []*regexp.Regexp{
	regexp.MustCompile(`^request state omitted for (\S+): `),
	regexp.MustCompile(`^request listing for (\S+) may be truncated`),
	regexp.MustCompile(`^reviewers unavailable for ([^\s#]+): `),
	regexp.MustCompile(`^reviewers unavailable for ([^\s#]+)#\d+: `),
	regexp.MustCompile(`^reviewers of ([^\s#]+)#\d+ may be incomplete`),
}

// reasonRepoOf returns the repository a known kind of reason is about, else "".
func reasonRepoOf(reason string) string {
	for _, re := range repoReasons {
		if m := re.FindStringSubmatch(reason); m != nil {
			return m[1]
		}
	}
	return ""
}

func countRequestsUnknown(rows []ChangeTopology) int {
	n := 0
	for _, r := range rows {
		if r.RequestsUnknown && r.PR == nil {
			n++
		}
	}
	return n
}
