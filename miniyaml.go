package specsync

import (
	"fmt"
	"strings"
)

// YKind is the shape of a parsed config value.
type YKind int

const (
	YScalar YKind = iota
	YMap
	YList
)

// YNode is a value parsed from the documented config subset. Scalars are
// kept as text; the reader decides what a value means.
type YNode struct {
	Kind    YKind
	Line    int
	Scalar  string
	Keys    []string
	Map     map[string]*YNode
	KeyLine map[string]int
	List    []*YNode
}

// Get returns the child of a map, or nil.
func (n *YNode) Get(key string) *YNode {
	if n == nil || n.Kind != YMap {
		return nil
	}
	return n.Map[key]
}

// Str returns a scalar child's text, or "".
func (n *YNode) Str(key string) string {
	if c := n.Get(key); c != nil && c.Kind == YScalar {
		return c.Scalar
	}
	return ""
}

// YAMLError is a parse error with the 1-based line it was found on.
type YAMLError struct {
	Line int
	Msg  string
}

func (e *YAMLError) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Msg) }

type yline struct {
	no     int
	indent int
	text   string
}

type yparser struct {
	lines []yline
	pos   int
}

// ParseConfigYAML parses the YAML subset documented for openspec/specsync.yml:
// block maps, block lists, inline {k: v} maps and [a, b] lists of scalars,
// quoted and bare scalars, and comments. Any other construct is an error.
func ParseConfigYAML(data []byte) (*YNode, error) {
	p := &yparser{}
	for i, raw := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		no := i + 1
		body, err := stripComment(raw, no)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(body) == "" {
			continue
		}
		indent := len(body) - len(strings.TrimLeft(body, " "))
		if strings.HasPrefix(body[indent:], "\t") {
			return nil, &YAMLError{no, "tabs are not allowed for indentation"}
		}
		text := strings.TrimSpace(body)
		if text == "---" || text == "..." {
			return nil, &YAMLError{no, "document markers are unsupported"}
		}
		p.lines = append(p.lines, yline{no, indent, text})
	}
	if len(p.lines) == 0 {
		return &YNode{Kind: YMap, Line: 1, Map: map[string]*YNode{}, KeyLine: map[string]int{}}, nil
	}
	n, err := p.block(p.lines[0].indent)
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.lines) {
		return nil, &YAMLError{p.lines[p.pos].no, "unexpected indent"}
	}
	return n, nil
}

func stripComment(s string, no int) (string, error) {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' && quote == '"' {
				i++
			} else if c == quote {
				quote = 0
			}
		case (c == '"' || c == '\'') && (i == 0 || strings.ContainsRune(" :-[{,", rune(s[i-1]))):
			quote = c
		case c == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t'):
			return s[:i], nil
		}
	}
	return s, nil
}

func (p *yparser) block(indent int) (*YNode, error) {
	l := p.lines[p.pos]
	if l.text == "-" || strings.HasPrefix(l.text, "- ") {
		return p.list(indent)
	}
	return p.mapping(indent)
}

func (p *yparser) mapping(indent int) (*YNode, error) {
	n := &YNode{Kind: YMap, Line: p.lines[p.pos].no, Map: map[string]*YNode{}, KeyLine: map[string]int{}}
	for p.pos < len(p.lines) {
		l := p.lines[p.pos]
		if l.indent < indent {
			break
		}
		if l.indent > indent {
			return nil, &YAMLError{l.no, "unexpected indent"}
		}
		if l.text == "-" || strings.HasPrefix(l.text, "- ") {
			return nil, &YAMLError{l.no, "a list item cannot follow a key at the same level"}
		}
		key, rest, err := splitKey(l)
		if err != nil {
			return nil, err
		}
		if _, dup := n.Map[key]; dup {
			return nil, &YAMLError{l.no, fmt.Sprintf("duplicate key %q", key)}
		}
		p.pos++
		var val *YNode
		if rest == "" {
			val, err = p.nested(indent, l.no)
		} else {
			val, err = inlineValue(rest, l.no)
		}
		if err != nil {
			return nil, err
		}
		n.Keys = append(n.Keys, key)
		n.Map[key] = val
		n.KeyLine[key] = l.no
	}
	return n, nil
}

func (p *yparser) nested(indent, no int) (*YNode, error) {
	if p.pos >= len(p.lines) {
		return &YNode{Kind: YScalar, Line: no}, nil
	}
	next := p.lines[p.pos]
	switch {
	case next.indent > indent:
		return p.block(next.indent)
	case next.indent == indent && (next.text == "-" || strings.HasPrefix(next.text, "- ")):
		return p.list(indent)
	}
	return &YNode{Kind: YScalar, Line: no}, nil
}

func (p *yparser) list(indent int) (*YNode, error) {
	n := &YNode{Kind: YList, Line: p.lines[p.pos].no}
	for p.pos < len(p.lines) {
		l := p.lines[p.pos]
		if l.indent != indent || !(l.text == "-" || strings.HasPrefix(l.text, "- ")) {
			if l.indent > indent {
				return nil, &YAMLError{l.no, "unexpected indent"}
			}
			break
		}
		rest := strings.TrimSpace(strings.TrimPrefix(l.text, "-"))
		if rest == "" {
			p.pos++
			v, err := p.nested(indent, l.no)
			if err != nil {
				return nil, err
			}
			n.List = append(n.List, v)
			continue
		}
		if key, _, ok := plainKey(rest); ok && key != "" {
			offset := len(l.text) - len(rest)
			p.lines[p.pos] = yline{l.no, indent + offset, rest}
			v, err := p.mapping(indent + offset)
			if err != nil {
				return nil, err
			}
			n.List = append(n.List, v)
			continue
		}
		p.pos++
		v, err := inlineValue(rest, l.no)
		if err != nil {
			return nil, err
		}
		n.List = append(n.List, v)
	}
	return n, nil
}

func plainKey(s string) (key, rest string, ok bool) {
	if s == "" || s[0] == '"' || s[0] == '\'' || s[0] == '{' || s[0] == '[' {
		return "", "", false
	}
	i := strings.Index(s, ":")
	if i <= 0 || (i+1 < len(s) && s[i+1] != ' ') {
		return "", "", false
	}
	return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true
}

func splitKey(l yline) (key, rest string, err error) {
	if l.text[0] == '"' || l.text[0] == '\'' {
		k, after, e := quoted(l.text, l.no)
		if e != nil {
			return "", "", e
		}
		after = strings.TrimSpace(after)
		if !strings.HasPrefix(after, ":") {
			return "", "", &YAMLError{l.no, "expected key: value"}
		}
		return k, strings.TrimSpace(after[1:]), nil
	}
	key, rest, ok := plainKey(l.text)
	if !ok || key == "" {
		return "", "", &YAMLError{l.no, "expected key: value"}
	}
	return key, rest, nil
}

func quoted(s string, no int) (val, rest string, err error) {
	q := s[0]
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case q == '"' && c == '\\' && i+1 < len(s):
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(s[i])
			}
		case q == '\'' && c == '\'' && i+1 < len(s) && s[i+1] == '\'':
			b.WriteByte('\'')
			i++
		case c == q:
			return b.String(), s[i+1:], nil
		default:
			b.WriteByte(c)
		}
	}
	return "", "", &YAMLError{no, "unterminated quote"}
}

func inlineValue(s string, no int) (*YNode, error) {
	switch s[0] {
	case '&':
		return nil, &YAMLError{no, "anchors are unsupported"}
	case '*':
		return nil, &YAMLError{no, "aliases are unsupported"}
	case '!':
		return nil, &YAMLError{no, "tags are unsupported"}
	case '|', '>':
		return nil, &YAMLError{no, "multi-line scalars are unsupported"}
	case '{', '[':
		return flow(s, no)
	}
	return scalar(s, no)
}

func scalar(s string, no int) (*YNode, error) {
	if s[0] == '"' || s[0] == '\'' {
		v, rest, err := quoted(s, no)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(rest) != "" {
			return nil, &YAMLError{no, "unexpected text after a quoted value"}
		}
		return &YNode{Kind: YScalar, Line: no, Scalar: v}, nil
	}
	return &YNode{Kind: YScalar, Line: no, Scalar: s}, nil
}

func flow(s string, no int) (*YNode, error) {
	open, closer := s[0], byte('}')
	kind := YMap
	if open == '[' {
		closer, kind = ']', YList
	}
	if s[len(s)-1] != closer {
		return nil, &YAMLError{no, "unterminated flow collection"}
	}
	n := &YNode{Kind: kind, Line: no, Map: map[string]*YNode{}, KeyLine: map[string]int{}}
	for _, part := range splitFlow(s[1 : len(s)-1]) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part[0] == '{' || part[0] == '[' {
			return nil, &YAMLError{no, "nested flow collections are unsupported"}
		}
		if kind == YList {
			v, err := inlineValue(part, no)
			if err != nil {
				return nil, err
			}
			n.List = append(n.List, v)
			continue
		}
		key, rest, ok := plainKey(part)
		if !ok {
			k, after, err := quotedKey(part, no)
			if err != nil {
				return nil, err
			}
			key, rest = k, after
		}
		if _, dup := n.Map[key]; dup {
			return nil, &YAMLError{no, fmt.Sprintf("duplicate key %q", key)}
		}
		if rest != "" && (rest[0] == '{' || rest[0] == '[') {
			return nil, &YAMLError{no, "nested flow collections are unsupported"}
		}
		v, err := inlineValue(orEmpty(rest), no)
		if err != nil {
			return nil, err
		}
		n.Keys = append(n.Keys, key)
		n.Map[key] = v
		n.KeyLine[key] = no
	}
	return n, nil
}

func orEmpty(s string) string {
	if s == "" {
		return `""`
	}
	return s
}

func quotedKey(s string, no int) (string, string, error) {
	if s[0] != '"' && s[0] != '\'' {
		return "", "", &YAMLError{no, "expected key: value"}
	}
	k, rest, err := quoted(s, no)
	if err != nil {
		return "", "", err
	}
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, ":") {
		return "", "", &YAMLError{no, "expected key: value"}
	}
	return k, strings.TrimSpace(rest[1:]), nil
}

func splitFlow(s string) []string {
	var parts []string
	var quote byte
	start, depth := 0, 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' && quote == '"' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '[' || c == '{':
			depth++
		case c == ']' || c == '}':
			depth--
		case c == ',' && depth == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}
