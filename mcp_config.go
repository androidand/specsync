package specsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// MCPConfig configures the MCP provider's connection and tool-name mapping.
// Loaded from a committed .specsync-mcp.json (NOT under .specsync/, which is
// gitignored wholesale) so the connection details and tool mapping are shared
// across a team/agents rather than reconstructed per machine.
type MCPConfig struct {
	// Server optionally names an entry in the project's .mcp.json (the
	// convention Claude Code and other agent harnesses already use to declare
	// MCP servers) to reuse instead of duplicating connection details here.
	Server string `json:"server,omitempty"`

	// Transport is "stdio" or "http". Inferred from the resolved .mcp.json
	// entry when Server is set and Transport is left empty.
	Transport string `json:"transport,omitempty"`

	// stdio transport.
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`

	// http transport.
	URL      string            `json:"url,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	TokenEnv string            `json:"tokenEnv,omitempty"` // env var holding a bearer token; merged into Headers["Authorization"]

	// Tools maps specsync operations (createIssue, updateIssue, find, comment,
	// addSubItem, removeSubItem, setCustomField) to the server's actual tool
	// names. Explicit entries always win over heuristic matching.
	Tools map[string]string `json:"tools,omitempty"`

	// Context holds static arguments merged into every tool call — the
	// project/repo-scoping identifiers a real server's tools require (e.g.
	// GitHub's tools take {owner, repo} on every call; a Jira-shaped server
	// might want {projectKey}). MCP is stateless: there is no "current repo"
	// implied by the connection, so this has to come from config.
	Context map[string]any `json:"context,omitempty"`

	// ToolArgs holds static, per-operation arguments merged in last (highest
	// precedence) — for servers that fold multiple specsync operations into
	// one tool distinguished by a fixed parameter (e.g. GitHub's issue_write
	// takes createIssue and updateIssue alike, selected by {"method": "create"}
	// vs {"method": "update"}).
	ToolArgs map[string]map[string]any `json:"toolArgs,omitempty"`

	// FindQuery templates the query text sent to the "find" tool. "{marker}"
	// expands to the literal identity marker comment, "{slug}" to the bare
	// change slug. Defaults to "{marker}" (the literal marker text) when
	// unset. Real search tools vary: some do literal/substring text search
	// (the default works), others need the tracker's own query syntax (e.g.
	// GitHub's search_issues wants "specsync:change={slug} in:body" — its
	// "query" is natural-language/qualifier search, not a literal match).
	FindQuery string `json:"findQuery,omitempty"`

	// IDField names the argument key an existing item's identifier is sent
	// under for updateIssue (and the id-based operations: comment,
	// addSubItem/removeSubItem's parentId/childId, setCustomField).
	// Defaults to "id". Real trackers vary — GitHub's issue_write wants
	// "issue_number", Jira-shaped tools often want "issueIdOrKey", etc.
	IDField string `json:"idField,omitempty"`

	// IDFieldNumeric sends the identifier as a JSON number instead of a
	// string when true (GitHub's issue_number is typed "number", not
	// "string" — most trackers' opaque string ids don't need this).
	IDFieldNumeric bool `json:"idFieldNumeric,omitempty"`

	// FindIDField names the field read from a "find" tool's result items as
	// the identifier, tried before the default "id"/"number" fallback. Some
	// trackers' read shape and write shape disagree on both the field name
	// AND which id is "the" id — GitHub's search results carry both an
	// internal database "id" and the repo-scoped "number" actually used to
	// address the issue in other calls; "id" is the wrong one.
	FindIDField string `json:"findIdField,omitempty"`
}

// mcpServersFile is the shape of a project's .mcp.json.
type mcpServersFile struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

type mcpServerEntry struct {
	Type    string            `json:"type,omitempty"` // "stdio" (default) or "http"/"sse"
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// LoadMCPConfig reads path (typically ".specsync-mcp.json"), resolves an
// optional "server" reference against the project's .mcp.json (same
// directory), expands ${VAR} references in connection fields, and folds
// TokenEnv into a Bearer Authorization header. It fails loudly when neither a
// usable stdio command nor an http URL can be resolved.
func LoadMCPConfig(path string) (MCPConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MCPConfig{}, fmt.Errorf("read mcp config %s: %w", path, err)
	}
	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return MCPConfig{}, fmt.Errorf("parse mcp config %s: %w", path, err)
	}

	if cfg.Server != "" {
		entry, err := loadMCPServerEntry(filepath.Dir(path), cfg.Server)
		if err != nil {
			return MCPConfig{}, err
		}
		mergeMCPServerEntry(&cfg, entry)
	}

	cfg.Command = expandEnvRefs(cfg.Command)
	for i, a := range cfg.Args {
		cfg.Args[i] = expandEnvRefs(a)
	}
	for k, v := range cfg.Env {
		cfg.Env[k] = expandEnvRefs(v)
	}
	cfg.URL = expandEnvRefs(cfg.URL)
	for k, v := range cfg.Headers {
		cfg.Headers[k] = expandEnvRefs(v)
	}

	if cfg.TokenEnv != "" {
		if cfg.Headers == nil {
			cfg.Headers = map[string]string{}
		}
		if _, ok := cfg.Headers["Authorization"]; !ok {
			cfg.Headers["Authorization"] = "Bearer " + os.Getenv(cfg.TokenEnv)
		}
	}

	switch cfg.Transport {
	case "stdio":
		if cfg.Command == "" {
			return MCPConfig{}, fmt.Errorf("mcp config %s: transport is \"stdio\" but no command is set", path)
		}
	case "http":
		if cfg.URL == "" {
			return MCPConfig{}, fmt.Errorf("mcp config %s: transport is \"http\" but no url is set", path)
		}
	case "":
		return MCPConfig{}, fmt.Errorf("mcp config %s: transport must be \"stdio\" or \"http\" (set directly, or inferred from \"server\")", path)
	default:
		return MCPConfig{}, fmt.Errorf("mcp config %s: unknown transport %q, use \"stdio\" or \"http\"", path, cfg.Transport)
	}

	return cfg, nil
}

// loadMCPServerEntry reads dir/.mcp.json and returns the named server entry.
func loadMCPServerEntry(dir, name string) (mcpServerEntry, error) {
	path := filepath.Join(dir, ".mcp.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return mcpServerEntry{}, fmt.Errorf("mcp config references server %q but %s is unreadable: %w", name, path, err)
	}
	var f mcpServersFile
	if err := json.Unmarshal(data, &f); err != nil {
		return mcpServerEntry{}, fmt.Errorf("parse %s: %w", path, err)
	}
	entry, ok := f.MCPServers[name]
	if !ok {
		var names []string
		for n := range f.MCPServers {
			names = append(names, n)
		}
		return mcpServerEntry{}, fmt.Errorf("mcp config references server %q, not found in %s (has: %v)", name, path, names)
	}
	return entry, nil
}

// mergeMCPServerEntry fills unset fields on cfg from a resolved .mcp.json
// entry. Fields already set directly in .specsync-mcp.json act as overrides.
func mergeMCPServerEntry(cfg *MCPConfig, entry mcpServerEntry) {
	if cfg.Transport == "" {
		switch entry.Type {
		case "http", "sse":
			cfg.Transport = "http"
		default:
			if entry.URL != "" {
				cfg.Transport = "http"
			} else {
				cfg.Transport = "stdio"
			}
		}
	}
	if cfg.Command == "" {
		cfg.Command = entry.Command
	}
	if len(cfg.Args) == 0 {
		cfg.Args = entry.Args
	}
	if cfg.URL == "" {
		cfg.URL = entry.URL
	}
	if len(cfg.Env) == 0 {
		cfg.Env = entry.Env
	}
	if len(cfg.Headers) == 0 {
		cfg.Headers = entry.Headers
	}
}

var envRefPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandEnvRefs replaces ${VAR} with the environment variable's value (empty
// if unset), matching the ${VAR} convention .mcp.json already uses.
func expandEnvRefs(s string) string {
	return envRefPattern.ReplaceAllStringFunc(s, func(m string) string {
		name := envRefPattern.FindStringSubmatch(m)[1]
		return os.Getenv(name)
	})
}
