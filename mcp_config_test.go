package specsync

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMCPConfig_Standalone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".specsync-mcp.json")
	writeFile(t, path, `{
		"transport": "stdio",
		"command": "npx",
		"args": ["-y", "@some/mcp-server"],
		"tools": {"createIssue": "create_issue"}
	}`)

	cfg, err := LoadMCPConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Transport != "stdio" || cfg.Command != "npx" {
		t.Errorf("got %+v", cfg)
	}
	if cfg.Tools["createIssue"] != "create_issue" {
		t.Errorf("tool mapping not preserved: %+v", cfg.Tools)
	}
}

func TestLoadMCPConfig_MissingFile(t *testing.T) {
	_, err := LoadMCPConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected an error for a missing config file")
	}
}

func TestLoadMCPConfig_MissingTransport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".specsync-mcp.json")
	writeFile(t, path, `{"tools": {}}`)

	_, err := LoadMCPConfig(path)
	if err == nil {
		t.Fatal("expected an error when transport can't be resolved from the config alone")
	}
}

func TestLoadMCPConfig_StdioMissingCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".specsync-mcp.json")
	writeFile(t, path, `{"transport": "stdio"}`)

	_, err := LoadMCPConfig(path)
	if err == nil {
		t.Fatal("expected an error when stdio transport has no command")
	}
}

func TestLoadMCPConfig_HTTPMissingURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".specsync-mcp.json")
	writeFile(t, path, `{"transport": "http"}`)

	_, err := LoadMCPConfig(path)
	if err == nil {
		t.Fatal("expected an error when http transport has no url")
	}
}

func TestLoadMCPConfig_ReusesServerFromMCPJSON_Stdio(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".mcp.json"), `{
		"mcpServers": {
			"linear": {"command": "npx", "args": ["-y", "linear-mcp"], "env": {"TOKEN": "abc"}}
		}
	}`)
	path := filepath.Join(dir, ".specsync-mcp.json")
	writeFile(t, path, `{"server": "linear", "tools": {"createIssue": "create_issue"}}`)

	cfg, err := LoadMCPConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Transport != "stdio" {
		t.Errorf("transport should infer stdio from the .mcp.json entry's command, got %q", cfg.Transport)
	}
	if cfg.Command != "npx" || len(cfg.Args) != 2 || cfg.Args[1] != "linear-mcp" {
		t.Errorf("command/args not reused from .mcp.json: %+v", cfg)
	}
	if cfg.Env["TOKEN"] != "abc" {
		t.Errorf("env not reused from .mcp.json: %+v", cfg.Env)
	}
}

func TestLoadMCPConfig_ReusesServerFromMCPJSON_HTTP(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".mcp.json"), `{
		"mcpServers": {
			"jira": {"type": "http", "url": "https://example.com/mcp", "headers": {"Authorization": "Bearer ${MY_TEST_TOKEN}"}}
		}
	}`)
	path := filepath.Join(dir, ".specsync-mcp.json")
	writeFile(t, path, `{"server": "jira"}`)

	t.Setenv("MY_TEST_TOKEN", "secret-value")
	cfg, err := LoadMCPConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Transport != "http" || cfg.URL != "https://example.com/mcp" {
		t.Errorf("got %+v", cfg)
	}
	if cfg.Headers["Authorization"] != "Bearer secret-value" {
		t.Errorf("${VAR} not expanded in header reused from .mcp.json: %+v", cfg.Headers)
	}
}

func TestLoadMCPConfig_ServerNotFoundInMCPJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".mcp.json"), `{"mcpServers": {"other": {"command": "x"}}}`)
	path := filepath.Join(dir, ".specsync-mcp.json")
	writeFile(t, path, `{"server": "missing"}`)

	_, err := LoadMCPConfig(path)
	if err == nil {
		t.Fatal("expected an error naming the missing server")
	}
}

func TestLoadMCPConfig_TokenEnvBecomesBearerHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".specsync-mcp.json")
	writeFile(t, path, `{"transport": "http", "url": "https://example.com/mcp", "tokenEnv": "MY_MCP_TOKEN"}`)

	t.Setenv("MY_MCP_TOKEN", "tok123")
	cfg, err := LoadMCPConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Headers["Authorization"] != "Bearer tok123" {
		t.Errorf("got headers %+v", cfg.Headers)
	}
}

func TestLoadMCPConfig_DirectFieldsOverrideServerEntry(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".mcp.json"), `{
		"mcpServers": {"linear": {"command": "npx", "args": ["-y", "linear-mcp"]}}
	}`)
	path := filepath.Join(dir, ".specsync-mcp.json")
	writeFile(t, path, `{"server": "linear", "command": "/custom/path/linear-mcp"}`)

	cfg, err := LoadMCPConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Command != "/custom/path/linear-mcp" {
		t.Errorf("a directly-set command should override the .mcp.json entry, got %q", cfg.Command)
	}
}
