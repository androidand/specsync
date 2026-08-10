package specsync

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CommentCapable is the optional comment-writing capability.
type CommentCapable interface {
	Comment(issueID string, body string) error
}

// SubItemCapable is the optional sub-item capability (e.g. GitHub sub-issues).
type SubItemCapable interface {
	AddSubItem(parentID, childID string) error
	RemoveSubItem(parentID, childID string) error
}

// CustomFieldCapable is the optional custom-field capability.
type CustomFieldCapable interface {
	SetCustomField(issueID string, field string, value string) error
}

// --- Wire protocol: modelcontextprotocol.io/specification/2026-07-28 (modern,
// stateless, per-request _meta) with a documented fallback to legacy
// (pre-2026-07-28, initialize-handshake) servers. See the era detection
// algorithm on the spec's stdio transport page ("Backward Compatibility").

const (
	mcpProtocolVersion    = "2026-07-28"
	legacyProtocolVersion = "2025-06-18"
	mcpClientName         = "specsync"
	mcpClientVersion      = "1.0"

	codeUnsupportedProtocolVersion = -32022
)

type mcpEra int

const (
	eraUnknown mcpEra = iota
	eraModern
	eraLegacy
)

type jsonrpcRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int64          `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type jsonrpcNotification struct {
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type jsonrpcResponseEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"` // present on notifications received from the server
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpRPCError    `json:"error,omitempty"`
}

type mcpRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *mcpRPCError) Error() string { return fmt.Sprintf("mcp error %d: %s", e.Code, e.Message) }

// modernMeta builds the required per-request _meta object for the modern
// (2026-07-28) protocol: protocolVersion and clientCapabilities are
// required on every request; clientInfo is included per the spec's SHOULD.
func modernMeta() map[string]any {
	return map[string]any{
		"io.modelcontextprotocol/protocolVersion":    mcpProtocolVersion,
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		"io.modelcontextprotocol/clientInfo":         map[string]any{"name": mcpClientName, "version": mcpClientVersion},
	}
}

type discoverResult struct {
	ResultType        string   `json:"resultType"`
	SupportedVersions []string `json:"supportedVersions"`
}

type mcpTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type toolsListResult struct {
	ResultType string    `json:"resultType"`
	Tools      []mcpTool `json:"tools"`
	NextCursor string    `json:"nextCursor"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type toolsCallResult struct {
	ResultType        string          `json:"resultType"`
	Content           []mcpContent    `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	IsError           bool            `json:"isError"`
}

// parseToolsCallResult normalizes a missing resultType (legacy servers never
// send one) to "complete", per the spec's backward-compatibility rule.
func parseToolsCallResult(raw json.RawMessage) (toolsCallResult, error) {
	var r toolsCallResult
	if err := json.Unmarshal(raw, &r); err != nil {
		return toolsCallResult{}, err
	}
	if r.ResultType == "" {
		r.ResultType = "complete"
	}
	return r, nil
}

func toolsCallResultText(r toolsCallResult) string {
	var parts []string
	for _, c := range r.Content {
		if c.Type == "text" && c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	if len(parts) == 0 {
		return "(no text content in result)"
	}
	return strings.Join(parts, "\n")
}

// mcpClient is a minimal, era-aware MCP JSON-RPC client. It probes the
// server once (server/discover) to learn whether it speaks the modern,
// stateless protocol or requires a legacy initialize handshake, then
// dispatches every subsequent request accordingly. Requests are strictly
// serialized (specsync never needs concurrent MCP calls), which keeps the
// stdio transport's request/response matching trivial.
type mcpClient struct {
	cfg        MCPConfig
	timeout    time.Duration
	httpClient *http.Client

	mu     sync.Mutex
	nextID int64
	era    mcpEra

	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
}

func newMCPClient(cfg MCPConfig) *mcpClient {
	return &mcpClient{
		cfg:        cfg,
		timeout:    20 * time.Second,
		httpClient: &http.Client{},
	}
}

// rpc is the entry point used by every protocol operation except era
// detection itself: it ensures the era is known, adds _meta for modern
// servers, and dispatches.
func (c *mcpClient) rpc(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	if err := c.ensureEra(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	era := c.era
	c.mu.Unlock()
	if era == eraModern {
		if params == nil {
			params = map[string]any{}
		}
		params["_meta"] = modernMeta()
	}
	raw, rpcErr, err := c.send(ctx, method, params)
	if err != nil {
		return nil, err
	}
	if rpcErr != nil {
		return nil, rpcErr
	}
	return raw, nil
}

// ensureEra runs the discover-or-fall-back algorithm exactly once.
func (c *mcpClient) ensureEra(ctx context.Context) error {
	c.mu.Lock()
	if c.era != eraUnknown {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	raw, rpcErr, err := c.send(ctx, "server/discover", map[string]any{"_meta": modernMeta()})
	if err == nil && rpcErr == nil {
		var dr discoverResult
		if perr := json.Unmarshal(raw, &dr); perr == nil && len(dr.SupportedVersions) > 0 {
			if !containsString(dr.SupportedVersions, mcpProtocolVersion) {
				return fmt.Errorf("mcp: server does not support protocol version %s (it supports: %v) — no in-modern-era renegotiation is implemented", mcpProtocolVersion, dr.SupportedVersions)
			}
			c.mu.Lock()
			c.era = eraModern
			c.mu.Unlock()
			return nil
		}
		// A "complete" result that isn't a recognizable DiscoverResult: treat
		// conservatively as legacy rather than guessing.
	}
	if rpcErr != nil && rpcErr.Code == codeUnsupportedProtocolVersion {
		return fmt.Errorf("mcp: server does not support protocol version %s: %s", mcpProtocolVersion, rpcErr.Message)
	}
	// Any other error, or a transport-level failure (timeout, connection
	// refused, process exit): per spec, this identifies a legacy server.
	return c.initializeLegacy(ctx)
}

func (c *mcpClient) initializeLegacy(ctx context.Context) error {
	_, rpcErr, err := c.send(ctx, "initialize", map[string]any{
		"protocolVersion": legacyProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": mcpClientName, "version": mcpClientVersion},
	})
	if err != nil {
		return fmt.Errorf("mcp: server/discover failed and legacy initialize also failed: %w", err)
	}
	if rpcErr != nil {
		return fmt.Errorf("mcp: server/discover failed and legacy initialize also failed: %s", rpcErr.Error())
	}
	if err := c.notify(ctx, "notifications/initialized", nil); err != nil {
		return fmt.Errorf("mcp: legacy initialized notification: %w", err)
	}
	c.mu.Lock()
	c.era = eraLegacy
	c.mu.Unlock()
	return nil
}

func (c *mcpClient) toolsList(ctx context.Context) ([]mcpTool, error) {
	var all []mcpTool
	cursor := ""
	for {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := c.rpc(ctx, "tools/list", params)
		if err != nil {
			return nil, fmt.Errorf("mcp tools/list: %w", err)
		}
		var r toolsListResult
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("mcp tools/list: parse result: %w", err)
		}
		all = append(all, r.Tools...)
		if r.NextCursor == "" {
			break
		}
		cursor = r.NextCursor
	}
	return all, nil
}

func (c *mcpClient) toolsCall(ctx context.Context, name string, args map[string]any) (toolsCallResult, error) {
	raw, err := c.rpc(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return toolsCallResult{}, fmt.Errorf("mcp tools/call %s: %w", name, err)
	}
	r, err := parseToolsCallResult(raw)
	if err != nil {
		return toolsCallResult{}, fmt.Errorf("mcp tools/call %s: parse result: %w", name, err)
	}
	if r.ResultType == "input_required" {
		return toolsCallResult{}, fmt.Errorf("mcp tools/call %s: server requires additional interactive input (multi round-trip requests / elicitation), which specsync cannot supply non-interactively", name)
	}
	if r.IsError {
		return r, fmt.Errorf("mcp tools/call %s: tool execution error: %s", name, toolsCallResultText(r))
	}
	return r, nil
}

// send is the low-level transport dispatcher: assigns a request id and
// routes to stdio or HTTP. Callers needing era-aware _meta go through rpc;
// ensureEra calls send directly since era is what it's establishing.
func (c *mcpClient) send(ctx context.Context, method string, params map[string]any) (json.RawMessage, *mcpRPCError, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nextID++
	req := jsonrpcRequest{JSONRPC: "2.0", ID: c.nextID, Method: method, Params: params}

	switch c.cfg.Transport {
	case "stdio":
		return c.sendStdioLocked(ctx, req)
	case "http":
		return c.sendHTTPLocked(ctx, req)
	default:
		return nil, nil, fmt.Errorf("mcp: unknown transport %q", c.cfg.Transport)
	}
}

// notify sends a one-way JSON-RPC notification (no id, no response wait).
func (c *mcpClient) notify(ctx context.Context, method string, params map[string]any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	notif := jsonrpcNotification{JSONRPC: "2.0", Method: method, Params: params}
	line, err := json.Marshal(notif)
	if err != nil {
		return err
	}
	switch c.cfg.Transport {
	case "stdio":
		if err := c.ensureProcessLocked(); err != nil {
			return err
		}
		_, err := c.stdin.Write(append(line, '\n'))
		return err
	case "http":
		reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
		defer cancel()
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.cfg.URL, bytes.NewReader(line))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range c.cfg.Headers {
			req.Header.Set(k, v)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		return nil
	default:
		return fmt.Errorf("mcp: unknown transport %q", c.cfg.Transport)
	}
}

func (c *mcpClient) sendHTTPLocked(ctx context.Context, req jsonrpcRequest) (json.RawMessage, *mcpRPCError, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
	for k, v := range c.cfg.Headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("mcp http: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("mcp http: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("mcp http %d: %s", resp.StatusCode, string(data))
	}
	var env jsonrpcResponseEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, nil, fmt.Errorf("mcp http: parse response: %w (raw: %s)", err, data)
	}
	return env.Result, env.Error, nil
}

func (c *mcpClient) sendStdioLocked(ctx context.Context, req jsonrpcRequest) (json.RawMessage, *mcpRPCError, error) {
	if err := c.ensureProcessLocked(); err != nil {
		return nil, nil, err
	}
	line, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	if _, err := c.stdin.Write(append(line, '\n')); err != nil {
		return nil, nil, fmt.Errorf("mcp stdio: write request: %w", err)
	}
	for {
		l, err := c.readLineLocked(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("mcp stdio: %w", err)
		}
		var env jsonrpcResponseEnvelope
		if err := json.Unmarshal([]byte(l), &env); err != nil {
			continue // not a valid MCP message; defensively ignore
		}
		if env.Method != "" {
			continue // a notification, not the response we're waiting for
		}
		var envID int64
		_ = json.Unmarshal(env.ID, &envID)
		if envID != req.ID {
			continue // response to an earlier request; shouldn't happen given serialization, but be safe
		}
		return env.Result, env.Error, nil
	}
}

// readLineLocked reads one line from the server's stdout, bounded by the
// client timeout so a wedged server fails fast instead of hanging the CLI.
// On timeout it kills the process so subsequent calls fail immediately
// rather than repeat the same hang.
func (c *mcpClient) readLineLocked(ctx context.Context) (string, error) {
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		if c.scanner.Scan() {
			ch <- result{line: c.scanner.Text()}
			return
		}
		err := c.scanner.Err()
		if err == nil {
			err = io.EOF
		}
		ch <- result{err: err}
	}()
	select {
	case r := <-ch:
		return r.line, r.err
	case <-time.After(c.timeout):
		_ = c.killProcessLocked()
		return "", fmt.Errorf("timed out after %s waiting for a response", c.timeout)
	case <-ctx.Done():
		_ = c.killProcessLocked()
		return "", ctx.Err()
	}
}

func (c *mcpClient) ensureProcessLocked() error {
	if c.cmd != nil {
		return nil
	}
	cmd := exec.Command(c.cfg.Command, c.cfg.Args...)
	if len(c.cfg.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range c.cfg.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp stdio: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp stdio: stdout pipe: %w", err)
	}
	// The server MAY write free-form logging to stderr; per spec, this MUST
	// NOT be treated as an error condition, so it's neither captured nor
	// wired to our own stderr by default.
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mcp stdio: start %s: %w", c.cfg.Command, err)
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024) // tool schemas/results can be large
	c.cmd = cmd
	c.stdin = stdin
	c.scanner = sc
	return nil
}

func (c *mcpClient) killProcessLocked() error {
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	return c.cmd.Process.Kill()
}

// Close shuts the stdio subprocess down per the spec's guidance: close
// stdin, wait for exit, escalate to a kill if it doesn't exit promptly. A
// no-op for HTTP or a client that never started a process.
func (c *mcpClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd == nil {
		return nil
	}
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	done := make(chan error, 1)
	go func() { done <- c.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = c.cmd.Process.Kill()
		<-done
	}
	return nil
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// --- Tool-name resolution: explicit config wins; a conservative heuristic
// (common naming variants, case/separator-insensitive) is the fallback;
// anything else is a loud error listing what the server actually advertises.

var mcpToolHeuristics = map[string][]string{
	"createIssue":    {"create_issue", "new_issue", "create_ticket", "create"},
	"updateIssue":    {"update_issue", "edit_issue", "update_ticket", "update"},
	"find":           {"search_issues", "find_issue", "search", "query_issues", "list_issues", "find"},
	"comment":        {"add_comment", "create_comment", "comment"},
	"addSubItem":     {"add_sub_issue", "add_subitem", "link_child", "add_child"},
	"removeSubItem":  {"remove_sub_issue", "remove_subitem", "unlink_child", "remove_child"},
	"setCustomField": {"set_custom_field", "set_field", "update_field"},
}

func normalizeToolName(s string) string {
	s = strings.ToLower(s)
	return strings.NewReplacer("-", "", "_", "", ".", "", " ", "").Replace(s)
}

func toolNames(tools []mcpTool) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names
}

func resolveMCPTool(op string, explicit map[string]string, discovered []mcpTool) (string, error) {
	if name, ok := explicit[op]; ok && name != "" {
		if containsString(toolNames(discovered), name) {
			return name, nil
		}
		return "", fmt.Errorf("mcp: configured tool %q for %q not found on server (discovered: %v)", name, op, toolNames(discovered))
	}
	normCandidates := map[string]bool{}
	for _, c := range mcpToolHeuristics[op] {
		normCandidates[normalizeToolName(c)] = true
	}
	for _, t := range discovered {
		if normCandidates[normalizeToolName(t.Name)] {
			return t.Name, nil
		}
	}
	return "", fmt.Errorf("mcp: no tool mapping configured for %q and no heuristic match found (discovered tools: %v) — add \"tools\":{%q:\"<tool-name>\"} to your mcp config", op, toolNames(discovered), op)
}

// --- MCPProvider: the WorkProvider (+ optional capabilities) implementation
// backed by mcpClient. The argument/result contract per operation is
// documented in README.md — specsync can't infer it from an arbitrary
// server's tool schema, so it's an explicit, documented convention the
// mapped tools must follow.

// MCPProvider delegates to an external MCP server that implements a small,
// documented tool contract. It satisfies WorkProvider plus the optional
// IssueReader, IssueMarkerWriter, CommentCapable, SubItemCapable, and
// CustomFieldCapable capabilities.
type MCPProvider struct {
	client *mcpClient
	cfg    MCPConfig
	dryRun bool

	toolsOnce sync.Once
	tools     []mcpTool
	toolsErr  error

	resolvedMu sync.Mutex
	resolved   map[string]string
}

// NewMCPProvider creates an MCP provider that performs real tool calls.
func NewMCPProvider(cfg MCPConfig) *MCPProvider { return newMCPProvider(cfg, false) }

// NewMCPProviderDryRun creates an MCP provider whose mutating operations
// print what they would call instead of calling it, following the same
// swap pattern as dryRunner/beadsDryRunner in cmd/specsync/main.go. Reads
// (tool discovery, Find, Get) still execute for real so the preview is
// accurate.
func NewMCPProviderDryRun(cfg MCPConfig) *MCPProvider { return newMCPProvider(cfg, true) }

func newMCPProvider(cfg MCPConfig, dryRun bool) *MCPProvider {
	return &MCPProvider{cfg: cfg, dryRun: dryRun, client: newMCPClient(cfg)}
}

func (p *MCPProvider) Name() string { return "mcp" }

// Close releases the stdio subprocess, if one was started.
func (p *MCPProvider) Close() error { return p.client.Close() }

func (p *MCPProvider) discoverTools(ctx context.Context) ([]mcpTool, error) {
	p.toolsOnce.Do(func() {
		p.tools, p.toolsErr = p.client.toolsList(ctx)
	})
	return p.tools, p.toolsErr
}

func (p *MCPProvider) resolve(ctx context.Context, op string) (string, error) {
	p.resolvedMu.Lock()
	if p.resolved == nil {
		p.resolved = map[string]string{}
	}
	if name, ok := p.resolved[op]; ok {
		p.resolvedMu.Unlock()
		return name, nil
	}
	p.resolvedMu.Unlock()

	tools, err := p.discoverTools(ctx)
	if err != nil {
		return "", fmt.Errorf("mcp: discover tools: %w", err)
	}
	name, err := resolveMCPTool(op, p.cfg.Tools, tools)
	if err != nil {
		return "", err
	}
	p.resolvedMu.Lock()
	p.resolved[op] = name
	p.resolvedMu.Unlock()
	return name, nil
}

// buildArgs assembles the final tools/call arguments for operation op:
// config Context first (project/repo-scoping defaults), then the
// operation's own payload (overwrites on key collision), then config
// ToolArgs[op] last (always wins — the escape hatch for servers that fold
// several specsync operations into one tool distinguished by a fixed
// parameter, e.g. GitHub's issue_write + {"method": "create"|"update"}).
func (p *MCPProvider) buildArgs(op string, payload map[string]any) map[string]any {
	args := map[string]any{}
	maps.Copy(args, p.cfg.Context)
	maps.Copy(args, payload)
	maps.Copy(args, p.cfg.ToolArgs[op])
	return args
}

// idField returns the argument key an existing item's identifier is sent
// under, honoring MCPConfig.IDField ("id" when unset).
func (p *MCPProvider) idField() string {
	if p.cfg.IDField != "" {
		return p.cfg.IDField
	}
	return "id"
}

// idValue coerces id to a JSON number when MCPConfig.IDFieldNumeric is set
// (falling back to the raw string if it isn't actually numeric), otherwise
// returns it as-is.
func (p *MCPProvider) idValue(id string) any {
	if p.cfg.IDFieldNumeric {
		if n, err := strconv.Atoi(id); err == nil {
			return n
		}
	}
	return id
}

func mustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%+v", v)
	}
	return string(b)
}

// Push creates or updates the projection via the mapped createIssue/updateIssue
// tool. The tool's result MUST carry structuredContent (or a JSON text
// fallback) with at least {id, url}.
func (p *MCPProvider) Push(ctx context.Context, item WorkItem, existing *Ref) (Ref, error) {
	// Defend against duplicates: if we have no cached ref, look one up by
	// marker first, mirroring GitHubProvider.Push exactly, retrying briefly
	// in case a very-recently-created item hasn't reached the server's
	// search index yet (see findWithRetry).
	if existing == nil {
		found, err := findWithRetry(ctx, func(ctx context.Context) (*Ref, error) { return p.Find(ctx, item.Slug) })
		if err != nil {
			return Ref{}, err
		}
		existing = found
	}

	op := "createIssue"
	if existing != nil {
		op = "updateIssue"
	}
	toolName, err := p.resolve(ctx, op)
	if err != nil {
		return Ref{}, err
	}

	// The identity marker goes in every body specsync writes, not just on
	// EnsureMarker's explicit upsert path — this is what makes Find work at
	// all, mirroring GitHubProvider.renderBody.
	body := marker(item.Slug) + "\n\n" + item.Body

	payload := map[string]any{
		"title": item.Title,
		"body":  body,
		"stage": string(item.Stage),
	}
	if existing != nil {
		payload[p.idField()] = p.idValue(existing.ID)
	}
	if item.Priority != 0 {
		payload["priority"] = item.Priority
	}
	if len(item.Labels) > 0 {
		payload["labels"] = item.Labels
	}
	if item.ManageClosed {
		payload["closed"] = item.Closed
	}
	args := p.buildArgs(op, payload)

	if p.dryRun {
		fmt.Printf("mcp (dry run): would call %s with %s\n", toolName, mustJSON(args))
		ref := Ref{Provider: p.Name()}
		if existing != nil {
			ref.ID, ref.URL = existing.ID, existing.URL
		}
		return ref, nil
	}

	res, err := p.client.toolsCall(ctx, toolName, args)
	if err != nil {
		return Ref{}, err
	}
	return parseRefFromToolResult(p.Name(), res)
}

// findQuery builds the query text sent to the mapped "find" tool. See
// MCPConfig.FindQuery: real search tools vary between literal/substring
// matching (the default, the marker text itself) and a tracker-specific
// query syntax that has to be configured (e.g. GitHub's search_issues wants
// "specsync:change=<slug> in:body", not the raw marker comment).
func (p *MCPProvider) findQuery(slug string) string {
	tmpl := p.cfg.FindQuery
	if tmpl == "" {
		tmpl = "{marker}"
	}
	tmpl = strings.ReplaceAll(tmpl, "{marker}", marker(slug))
	tmpl = strings.ReplaceAll(tmpl, "{slug}", slug)
	return tmpl
}

// findIDKeys returns the item-field names tried, in order, when extracting
// an identifier from a "find" tool's result. MCPConfig.FindIDField, when
// set, is tried first (ahead of the "id"/"number" default), since some
// trackers' result items carry more than one id-shaped field and the wrong
// one by default (see FindIDField's doc).
func (p *MCPProvider) findIDKeys() []string {
	if p.cfg.FindIDField != "" {
		return []string{p.cfg.FindIDField, "id", "number"}
	}
	return []string{"id", "number"}
}

// Find locates an existing projection via the mapped "find" tool. The first
// result whose body (when the tool returns one) actually contains the
// identity marker wins — a defensive check against a search tool that does
// fuzzy/keyword matching rather than an exact one, since the query itself
// may not be a literal substring match (see findQuery).
func (p *MCPProvider) Find(ctx context.Context, slug string) (*Ref, error) {
	toolName, err := p.resolve(ctx, "find")
	if err != nil {
		return nil, err
	}
	res, err := p.client.toolsCall(ctx, toolName, p.buildArgs("find", map[string]any{"query": p.findQuery(slug)}))
	if err != nil {
		return nil, err
	}
	items, err := extractResultItems(res)
	if err != nil {
		return nil, fmt.Errorf("mcp: find tool result: %w", err)
	}
	m := marker(slug)
	for _, it := range items {
		if body, ok := it["body"].(string); ok && !strings.Contains(body, m) {
			continue
		}
		id := itemString(it, p.findIDKeys()...)
		if id == "" {
			continue
		}
		return &Ref{Provider: p.Name(), ID: id, URL: itemString(it, "url", "html_url")}, nil
	}
	return nil, nil
}

// Get fetches an existing item by id via the mapped "find" tool (reusing the
// same tool: a query of the literal id, taking the matching item).
func (p *MCPProvider) Get(ctx context.Context, id string) (FetchedItem, error) {
	toolName, err := p.resolve(ctx, "find")
	if err != nil {
		return FetchedItem{}, err
	}
	res, err := p.client.toolsCall(ctx, toolName, p.buildArgs("find", map[string]any{"query": id}))
	if err != nil {
		return FetchedItem{}, err
	}
	items, err := extractResultItems(res)
	if err != nil {
		return FetchedItem{}, fmt.Errorf("mcp: find tool result: %w", err)
	}
	for _, it := range items {
		if itemString(it, p.findIDKeys()...) == id {
			return FetchedItem{
				ID:     id,
				URL:    itemString(it, "url", "html_url"),
				Title:  itemString(it, "title"),
				Body:   itemString(it, "body"),
				Closed: itemBool(it, "closed", "state"),
			}, nil
		}
	}
	return FetchedItem{}, fmt.Errorf("mcp: no item found for id %q", id)
}

// EnsureMarker upserts the identity marker into an existing item's body via
// updateIssue, mirroring the GitHub provider's rediscoverability guarantee
// for pull. It is idempotent: a body already carrying the marker is a no-op.
func (p *MCPProvider) EnsureMarker(ctx context.Context, id, slug, body string) (bool, error) {
	m := marker(slug)
	if strings.Contains(body, m) {
		return false, nil
	}
	toolName, err := p.resolve(ctx, "updateIssue")
	if err != nil {
		return false, err
	}
	newBody := body + "\n\n" + m
	if p.dryRun {
		fmt.Printf("mcp (dry run): would call %s to add identity marker to %s\n", toolName, id)
		return true, nil
	}
	if _, err := p.client.toolsCall(ctx, toolName, p.buildArgs("updateIssue", map[string]any{p.idField(): p.idValue(id), "body": newBody})); err != nil {
		return false, err
	}
	return true, nil
}

// Comment posts a comment via the mapped "comment" tool.
func (p *MCPProvider) Comment(issueID string, body string) error {
	ctx := context.Background()
	toolName, err := p.resolve(ctx, "comment")
	if err != nil {
		return err
	}
	if p.dryRun {
		fmt.Printf("mcp (dry run): would call %s on %s\n", toolName, issueID)
		return nil
	}
	_, err = p.client.toolsCall(ctx, toolName, p.buildArgs("comment", map[string]any{p.idField(): p.idValue(issueID), "body": body}))
	return err
}

// AddSubItem links a child item under a parent via the mapped tool.
func (p *MCPProvider) AddSubItem(parentID, childID string) error {
	ctx := context.Background()
	toolName, err := p.resolve(ctx, "addSubItem")
	if err != nil {
		return err
	}
	if p.dryRun {
		fmt.Printf("mcp (dry run): would call %s (%s -> %s)\n", toolName, parentID, childID)
		return nil
	}
	_, err = p.client.toolsCall(ctx, toolName, p.buildArgs("addSubItem", map[string]any{"parentId": parentID, "childId": childID}))
	return err
}

// RemoveSubItem unlinks a child item from a parent via the mapped tool.
func (p *MCPProvider) RemoveSubItem(parentID, childID string) error {
	ctx := context.Background()
	toolName, err := p.resolve(ctx, "removeSubItem")
	if err != nil {
		return err
	}
	if p.dryRun {
		fmt.Printf("mcp (dry run): would call %s (%s -/-> %s)\n", toolName, parentID, childID)
		return nil
	}
	_, err = p.client.toolsCall(ctx, toolName, p.buildArgs("removeSubItem", map[string]any{"parentId": parentID, "childId": childID}))
	return err
}

// SetCustomField sets a custom field via the mapped tool.
func (p *MCPProvider) SetCustomField(issueID string, field string, value string) error {
	ctx := context.Background()
	toolName, err := p.resolve(ctx, "setCustomField")
	if err != nil {
		return err
	}
	if p.dryRun {
		fmt.Printf("mcp (dry run): would call %s on %s (%s=%s)\n", toolName, issueID, field, value)
		return nil
	}
	_, err = p.client.toolsCall(ctx, toolName, p.buildArgs("setCustomField", map[string]any{p.idField(): p.idValue(issueID), "field": field, "value": value}))
	return err
}

func parseRefFromToolResult(provider string, r toolsCallResult) (Ref, error) {
	src, err := resultJSONSource(r)
	if err != nil {
		return Ref{}, err
	}
	var payload struct {
		ID  any    `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(src, &payload); err != nil {
		return Ref{}, fmt.Errorf("mcp: tool result does not match the expected {id,url} contract: %w (raw: %s)", err, src)
	}
	id := fmt.Sprintf("%v", payload.ID)
	if payload.ID == nil || id == "" {
		return Ref{}, fmt.Errorf("mcp: tool result is missing \"id\"")
	}
	return Ref{Provider: provider, ID: id, URL: payload.URL}, nil
}

// resultJSONSource extracts the raw JSON payload from a tool result:
// structuredContent when present, otherwise the first text content block
// parsed as JSON. Real servers vary on which they use — e.g. github-mcp-server
// puts search results in a JSON-stringified text block, not structuredContent.
func resultJSONSource(r toolsCallResult) (json.RawMessage, error) {
	if len(r.StructuredContent) > 0 {
		return r.StructuredContent, nil
	}
	if len(r.Content) > 0 && r.Content[0].Type == "text" {
		return json.RawMessage(r.Content[0].Text), nil
	}
	return nil, fmt.Errorf("mcp: tool result carries no structuredContent and no parseable text content")
}

// commonListWrapperKeys are checked, in order, when a "find"-style result
// wraps its item array in an object instead of returning it bare (e.g.
// GitHub's search_issues returns {"total_count":N,"items":[...]}).
var commonListWrapperKeys = []string{"items", "issues", "results", "data"}

// extractResultItems parses a "find" tool result into a list of raw item
// maps, tolerating both a bare JSON array (the documented default contract)
// and a common wrapper-object shape real search APIs often use.
func extractResultItems(r toolsCallResult) ([]map[string]any, error) {
	src, err := resultJSONSource(r)
	if err != nil {
		return nil, err
	}
	var arr []map[string]any
	if err := json.Unmarshal(src, &arr); err == nil {
		return arr, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(src, &obj); err != nil {
		return nil, fmt.Errorf("result is neither a JSON array nor an object (raw: %s)", src)
	}
	for _, key := range commonListWrapperKeys {
		raw, ok := obj[key]
		if !ok {
			continue
		}
		if err := json.Unmarshal(raw, &arr); err == nil {
			return arr, nil
		}
	}
	return nil, fmt.Errorf("result object has no recognized item-list key (tried %v; raw: %s)", commonListWrapperKeys, src)
}

// itemString reads a string-ish field from a raw item map, trying each key
// in order and coercing a non-string JSON value (e.g. GitHub's numeric
// issue "number") to its string form.
func itemString(item map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := item[k]
		if !ok || v == nil {
			continue
		}
		if s, ok := v.(string); ok {
			if s == "" {
				continue
			}
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// itemBool reads a closed/open-ish field, accepting a real bool or a
// "state"-style string ("open"/"closed").
func itemBool(item map[string]any, keys ...string) bool {
	for _, k := range keys {
		v, ok := item[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case bool:
			return t
		case string:
			return t == "closed" || t == "true"
		}
	}
	return false
}

// FakeProvider is a test provider that records all operations for verification.
// It satisfies WorkProvider plus optional capability interfaces.
type FakeProvider struct {
	NameVal        string
	Items          map[string]FetchedItem
	Created        []WorkItem
	Updated        []WorkItem
	FindSlugs      []string
	Pushed         []Ref
	Closed         []string
	Comments       []FakeComment
	SubItems       []FakeSubItem
	CustomFields   []FakeCustomField
	PushErr        error
	FindErr        error
	CommentErr     error
	SubItemErr     error
	CustomFieldErr error
}

type FakeComment struct {
	IssueID string
	Body    string
}

type FakeSubItem struct {
	Action   string // "add" or "remove"
	ParentID string
	ChildID  string
}

type FakeCustomField struct {
	IssueID string
	Field   string
	Value   string
}

func NewFakeProvider() *FakeProvider {
	return &FakeProvider{
		NameVal: "fake",
		Items:   map[string]FetchedItem{},
	}
}

func (f *FakeProvider) Name() string {
	return f.NameVal
}

func (f *FakeProvider) Push(ctx context.Context, item WorkItem, existing *Ref) (Ref, error) {
	if f.PushErr != nil {
		return Ref{}, f.PushErr
	}

	if existing != nil {
		// Update existing item.
		if fi, ok := f.Items[existing.ID]; ok {
			fi.Title = item.Title
			fi.Body = item.Body
			f.Items[existing.ID] = fi
			f.Updated = append(f.Updated, item)
		}
		ref := *existing
		ref.ID = existing.ID
		ref.URL = existing.URL
		ref.Provider = f.NameVal
		f.Pushed = append(f.Pushed, ref)
		return ref, nil
	}

	// Create new item.
	id := strconv.Itoa(len(f.Items) + 1)
	fi := FetchedItem{
		ID:    id,
		URL:   "https://example.com/" + id,
		Title: item.Title,
		Body:  item.Body,
	}
	f.Items[id] = fi
	f.Created = append(f.Created, item)

	ref := Ref{
		Provider: f.NameVal,
		ID:       id,
		URL:      fi.URL,
	}
	if item.Closed {
		ref.BaseClosed = boolPtr(true)
		f.Closed = append(f.Closed, id)
		fi.Closed = true
		f.Items[id] = fi
	}
	if item.ManageClosed {
		ref.BaseClosed = boolPtr(false)
	}
	f.Pushed = append(f.Pushed, ref)
	return ref, nil
}

func (f *FakeProvider) Find(ctx context.Context, slug string) (*Ref, error) {
	if f.FindErr != nil {
		return nil, f.FindErr
	}
	f.FindSlugs = append(f.FindSlugs, slug)
	for _, fi := range f.Items {
		if strings.Contains(fi.Body, slug) {
			ref := Ref{
				Provider: f.NameVal,
				ID:       fi.ID,
				URL:      fi.URL,
			}
			return &ref, nil
		}
	}
	return nil, nil
}

func (f *FakeProvider) Comment(issueID string, body string) error {
	if f.CommentErr != nil {
		return f.CommentErr
	}
	f.Comments = append(f.Comments, FakeComment{IssueID: issueID, Body: body})
	return nil
}

func (f *FakeProvider) AddSubItem(parentID, childID string) error {
	if f.SubItemErr != nil {
		return f.SubItemErr
	}
	f.SubItems = append(f.SubItems, FakeSubItem{Action: "add", ParentID: parentID, ChildID: childID})
	return nil
}

func (f *FakeProvider) RemoveSubItem(parentID, childID string) error {
	if f.SubItemErr != nil {
		return f.SubItemErr
	}
	f.SubItems = append(f.SubItems, FakeSubItem{Action: "remove", ParentID: parentID, ChildID: childID})
	return nil
}

func (f *FakeProvider) SetCustomField(issueID string, field string, value string) error {
	if f.CustomFieldErr != nil {
		return f.CustomFieldErr
	}
	f.CustomFields = append(f.CustomFields, FakeCustomField{IssueID: issueID, Field: field, Value: value})
	return nil
}
