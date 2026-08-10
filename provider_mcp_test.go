package specsync

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestFakeProvider_Push_Create(t *testing.T) {
	f := NewFakeProvider()
	ctx := context.Background()
	ref, err := f.Push(ctx, WorkItem{
		Slug:  "test",
		Title: "test issue",
		Body:  "test body",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ref.ID == "" {
		t.Error("expected ID")
	}
	if len(f.Created) != 1 {
		t.Errorf("expected 1 created, got %d", len(f.Created))
	}
}

func TestFakeProvider_Push_Create_Closed(t *testing.T) {
	f := NewFakeProvider()
	ctx := context.Background()
	ref, err := f.Push(ctx, WorkItem{
		Slug:   "closed",
		Title:  "closed issue",
		Closed: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ref.BaseClosed == nil || !*ref.BaseClosed {
		t.Error("expected BaseClosed = true")
	}
	if len(f.Closed) != 1 {
		t.Errorf("expected 1 closed, got %d", len(f.Closed))
	}
}

func TestFakeProvider_Push_Update(t *testing.T) {
	f := NewFakeProvider()
	ctx := context.Background()
	ref, _ := f.Push(ctx, WorkItem{Slug: "test", Title: "old"}, nil)
	_, err := f.Push(ctx, WorkItem{Slug: "test", Title: "new"}, &ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Updated) != 1 {
		t.Errorf("expected 1 updated, got %d", len(f.Updated))
	}
}

func TestFakeProvider_Push_Error(t *testing.T) {
	f := NewFakeProvider()
	expectedErr := fmt.Errorf("push failed")
	f.PushErr = expectedErr
	ctx := context.Background()
	_, err := f.Push(ctx, WorkItem{Title: "x"}, nil)
	if err != expectedErr {
		t.Errorf("got %v", err)
	}
}

func TestFakeProvider_Find(t *testing.T) {
	f := NewFakeProvider()
	f.Items["1"] = FetchedItem{ID: "1", URL: "https://example.com/1", Body: "slug:test"}
	ctx := context.Background()
	ref, err := f.Find(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	if ref.ID != "1" {
		t.Errorf("got %q", ref.ID)
	}
}

func TestFakeProvider_Find_NotFound(t *testing.T) {
	f := NewFakeProvider()
	ctx := context.Background()
	ref, err := f.Find(ctx, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if ref != nil {
		t.Error("expected nil")
	}
}

func TestFakeProvider_Comment(t *testing.T) {
	f := NewFakeProvider()
	err := f.Comment("1", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Comments) != 1 || f.Comments[0].Body != "hello" {
		t.Errorf("unexpected comment: %v", f.Comments)
	}
}

func TestFakeProvider_SubItems(t *testing.T) {
	f := NewFakeProvider()
	err := f.AddSubItem("parent", "child")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.SubItems) != 1 || f.SubItems[0].Action != "add" {
		t.Errorf("unexpected subitem: %v", f.SubItems)
	}
	err = f.RemoveSubItem("parent", "child")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.SubItems) != 2 || f.SubItems[1].Action != "remove" {
		t.Errorf("unexpected subitem: %v", f.SubItems)
	}
}

func TestFakeProvider_CustomField(t *testing.T) {
	f := NewFakeProvider()
	err := f.SetCustomField("1", "priority", "high")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.CustomFields) != 1 {
		t.Errorf("expected 1 custom field, got %d", len(f.CustomFields))
	}
}

// --- Pure protocol logic: parsing, tool resolution ---

func TestParseToolsCallResult_MissingResultTypeIsComplete(t *testing.T) {
	r, err := parseToolsCallResult(json.RawMessage(`{"content":[{"type":"text","text":"ok"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.ResultType != "complete" {
		t.Errorf("got resultType %q, want \"complete\" (legacy servers omit it)", r.ResultType)
	}
}

func TestResolveMCPTool_ExplicitWins(t *testing.T) {
	discovered := []mcpTool{{Name: "make_ticket"}, {Name: "create_issue"}}
	name, err := resolveMCPTool("createIssue", map[string]string{"createIssue": "make_ticket"}, discovered)
	if err != nil {
		t.Fatal(err)
	}
	if name != "make_ticket" {
		t.Errorf("got %q, want explicit mapping to win over the heuristic match", name)
	}
}

func TestResolveMCPTool_ExplicitNotFoundOnServer(t *testing.T) {
	_, err := resolveMCPTool("createIssue", map[string]string{"createIssue": "nonexistent_tool"}, []mcpTool{{Name: "create_issue"}})
	if err == nil {
		t.Fatal("expected an error when the configured tool isn't advertised by the server")
	}
	if !strings.Contains(err.Error(), "nonexistent_tool") {
		t.Errorf("error should name the missing tool: %v", err)
	}
}

func TestResolveMCPTool_HeuristicFallback(t *testing.T) {
	name, err := resolveMCPTool("find", nil, []mcpTool{{Name: "search_issues"}, {Name: "unrelated_tool"}})
	if err != nil {
		t.Fatal(err)
	}
	if name != "search_issues" {
		t.Errorf("got %q, want heuristic match search_issues", name)
	}
}

func TestResolveMCPTool_NoMatchFailsLoud(t *testing.T) {
	_, err := resolveMCPTool("find", nil, []mcpTool{{Name: "totally_unrelated"}})
	if err == nil {
		t.Fatal("expected a loud error, not a silent guess")
	}
	if !strings.Contains(err.Error(), "totally_unrelated") {
		t.Errorf("error should list the discovered tools so the operator can configure a mapping: %v", err)
	}
}

// --- HTTP transport: era detection, tools/list, tools/call, dry-run ---

// fakeMCPServer is a minimal, scriptable MCP server for HTTP tests.
type fakeMCPServer struct {
	era     string // "modern" or "legacy"
	tools   []mcpTool
	pages   [][]mcpTool // when set, tools/list paginates through these instead of `tools`
	calls   []map[string]any
	handler func(method string, params map[string]any) map[string]any // per-test override for tools/call
}

func (f *fakeMCPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	_ = json.NewDecoder(r.Body).Decode(&req)
	f.calls = append(f.calls, req)
	method, _ := req["method"].(string)
	params, _ := req["params"].(map[string]any)
	id := req["id"]

	w.Header().Set("Content-Type", "application/json")

	switch method {
	case "server/discover":
		if f.era == "legacy" {
			writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32601, "message": "method not found"}})
			return
		}
		writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{
			"resultType": "complete", "supportedVersions": []string{mcpProtocolVersion},
		}})
	case "initialize":
		writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{
			"protocolVersion": legacyProtocolVersion, "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "fake", "version": "0"},
		}})
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
	case "tools/list":
		if len(f.pages) > 0 {
			cursor, _ := params["cursor"].(string)
			idx := 0
			if cursor != "" {
				fmt.Sscanf(cursor, "%d", &idx)
			}
			page := f.pages[idx]
			result := map[string]any{"resultType": "complete", "tools": page}
			if idx+1 < len(f.pages) {
				result["nextCursor"] = fmt.Sprintf("%d", idx+1)
			}
			writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
			return
		}
		writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"resultType": "complete", "tools": f.tools}})
	case "tools/call":
		if f.handler != nil {
			writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": id, "result": f.handler(method, params)})
			return
		}
		writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"resultType": "complete", "content": []map[string]any{}}})
	default:
		writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32601, "message": "unknown method " + method}})
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	b, _ := json.Marshal(v)
	w.Write(b)
}

func modernFakeServer(tools ...mcpTool) *fakeMCPServer {
	return &fakeMCPServer{era: "modern", tools: tools}
}

func TestMCPProvider_EraDetection_Modern(t *testing.T) {
	srv := modernFakeServer(mcpTool{Name: "create_issue"})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	c := newMCPClient(MCPConfig{Transport: "http", URL: ts.URL})
	if err := c.ensureEra(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.era != eraModern {
		t.Errorf("got era %v, want modern", c.era)
	}
	// The discover request must carry the preferred version in _meta.
	discover := srv.calls[0]
	params := discover["params"].(map[string]any)
	meta := params["_meta"].(map[string]any)
	if meta["io.modelcontextprotocol/protocolVersion"] != mcpProtocolVersion {
		t.Errorf("discover request missing protocolVersion in _meta: %v", meta)
	}
}

func TestMCPProvider_EraDetection_LegacyFallback(t *testing.T) {
	srv := &fakeMCPServer{era: "legacy", tools: []mcpTool{{Name: "create_issue"}}}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	c := newMCPClient(MCPConfig{Transport: "http", URL: ts.URL})
	if err := c.ensureEra(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.era != eraLegacy {
		t.Errorf("got era %v, want legacy (server/discover returned a generic error)", c.era)
	}
	// initialize then notifications/initialized must have been sent.
	var sawInitialize, sawInitialized bool
	for _, call := range srv.calls {
		switch call["method"] {
		case "initialize":
			sawInitialize = true
		case "notifications/initialized":
			sawInitialized = true
		}
	}
	if !sawInitialize || !sawInitialized {
		t.Errorf("expected initialize + notifications/initialized on legacy fallback, calls: %v", srv.calls)
	}
}

func TestMCPProvider_EraDetection_UnsupportedVersionErrorsLoud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		writeJSON(w, map[string]any{"jsonrpc": "2.0", "id": req["id"], "error": map[string]any{
			"code": codeUnsupportedProtocolVersion, "message": "unsupported", "data": map[string]any{"supported": []string{"2025-11-25"}},
		}})
	}))
	defer ts.Close()

	c := newMCPClient(MCPConfig{Transport: "http", URL: ts.URL})
	err := c.ensureEra(context.Background())
	if err == nil {
		t.Fatal("expected an error naming the versions the server supports")
	}
	if !strings.Contains(err.Error(), mcpProtocolVersion) {
		t.Errorf("error should name our requested version: %v", err)
	}
}

func TestMCPClient_ToolsList_Pagination(t *testing.T) {
	srv := &fakeMCPServer{era: "modern", pages: [][]mcpTool{
		{{Name: "create_issue"}},
		{{Name: "search_issues"}},
	}}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	c := newMCPClient(MCPConfig{Transport: "http", URL: ts.URL})
	tools, err := c.toolsList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 || tools[0].Name != "create_issue" || tools[1].Name != "search_issues" {
		t.Errorf("expected both pages to be followed via nextCursor, got %v", tools)
	}
}

func TestMCPClient_ToolsCall_IsError(t *testing.T) {
	srv := modernFakeServer()
	srv.handler = func(method string, params map[string]any) map[string]any {
		return map[string]any{"resultType": "complete", "isError": true, "content": []map[string]any{{"type": "text", "text": "boom"}}}
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	c := newMCPClient(MCPConfig{Transport: "http", URL: ts.URL})
	_, err := c.toolsCall(context.Background(), "create_issue", map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected the tool execution error text surfaced, got %v", err)
	}
}

func TestMCPClient_ToolsCall_InputRequired(t *testing.T) {
	srv := modernFakeServer()
	srv.handler = func(method string, params map[string]any) map[string]any {
		return map[string]any{"resultType": "input_required", "inputRequests": map[string]any{}}
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	c := newMCPClient(MCPConfig{Transport: "http", URL: ts.URL})
	_, err := c.toolsCall(context.Background(), "create_issue", map[string]any{})
	if err == nil {
		t.Fatal("expected input_required to surface as an error, not hang or silently retry")
	}
	if !strings.Contains(err.Error(), "interactive") {
		t.Errorf("error should explain why: %v", err)
	}
}

func TestMCPProvider_Push_CreateAndUpdate(t *testing.T) {
	srv := modernFakeServer(mcpTool{Name: "create_issue"}, mcpTool{Name: "update_issue"}, mcpTool{Name: "search_issues"})
	srv.handler = func(method string, params map[string]any) map[string]any {
		name, _ := params["name"].(string)
		if name == "search_issues" {
			// No cached ref on the first Push: Push looks one up by marker
			// before deciding create vs. update, mirroring GitHubProvider.
			return map[string]any{"resultType": "complete", "content": []map[string]any{}, "structuredContent": []map[string]any{}}
		}
		return map[string]any{"resultType": "complete", "content": []map[string]any{}, "structuredContent": map[string]any{"id": "42", "url": "https://example.com/42"}}
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	p := NewMCPProvider(MCPConfig{Transport: "http", URL: ts.URL})
	ctx := context.Background()

	ref, err := p.Push(ctx, WorkItem{Title: "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ref.ID != "42" || ref.URL != "https://example.com/42" {
		t.Errorf("got ref %+v", ref)
	}

	ref2, err := p.Push(ctx, WorkItem{Title: "updated"}, &ref)
	if err != nil {
		t.Fatal(err)
	}
	if ref2.ID != "42" {
		t.Errorf("got ref %+v", ref2)
	}
}

func TestMCPProvider_Find(t *testing.T) {
	srv := modernFakeServer(mcpTool{Name: "search_issues"})
	srv.handler = func(method string, params map[string]any) map[string]any {
		return map[string]any{"resultType": "complete", "content": []map[string]any{}, "structuredContent": []map[string]any{{"id": "7", "url": "https://example.com/7"}}}
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	p := NewMCPProvider(MCPConfig{Transport: "http", URL: ts.URL})
	ref, err := p.Find(context.Background(), "some-slug")
	if err != nil {
		t.Fatal(err)
	}
	if ref == nil || ref.ID != "7" {
		t.Errorf("got %+v", ref)
	}
}

func TestMCPProvider_Find_NotFound(t *testing.T) {
	srv := modernFakeServer(mcpTool{Name: "search_issues"})
	srv.handler = func(method string, params map[string]any) map[string]any {
		return map[string]any{"resultType": "complete", "content": []map[string]any{}, "structuredContent": []map[string]any{}}
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	p := NewMCPProvider(MCPConfig{Transport: "http", URL: ts.URL})
	ref, err := p.Find(context.Background(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if ref != nil {
		t.Errorf("expected nil ref, got %+v", ref)
	}
}

func TestMCPProvider_DryRun_DoesNotCallMutatingTool(t *testing.T) {
	srv := modernFakeServer(mcpTool{Name: "create_issue"}, mcpTool{Name: "search_issues"})
	srv.handler = func(method string, params map[string]any) map[string]any {
		// Find runs for real even in dry-run (it's a read); only the
		// mutating create/update call is skipped.
		return map[string]any{"resultType": "complete", "content": []map[string]any{}, "structuredContent": []map[string]any{}}
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	p := NewMCPProviderDryRun(MCPConfig{Transport: "http", URL: ts.URL})
	ref, err := p.Push(context.Background(), WorkItem{Title: "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ref.ID != "" {
		t.Errorf("dry-run push should not fabricate an id, got %+v", ref)
	}
	for _, call := range srv.calls {
		if call["method"] != "tools/call" {
			continue
		}
		params, _ := call["params"].(map[string]any)
		name, _ := params["name"].(string)
		if name != "search_issues" { // the internal Find lookup is a read; only creation must be skipped
			t.Errorf("dry-run must not call tools/call for a mutating operation, but got: %v", call)
		}
	}
}

func TestMCPProvider_UnmappedOperationFailsLoud(t *testing.T) {
	srv := modernFakeServer(mcpTool{Name: "totally_unrelated"})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	p := NewMCPProvider(MCPConfig{Transport: "http", URL: ts.URL})
	_, err := p.Push(context.Background(), WorkItem{Title: "test"}, nil)
	if err == nil {
		t.Fatal("expected a loud error when no tool maps to createIssue")
	}
}

// --- Stdio transport: a self-reexec fake server proves the process/framing
// mechanics work end to end (spawn, write request, read matching response,
// shutdown). Protocol logic itself is already covered above over HTTP.

func TestMCPProvider_Stdio_RoundTrip(t *testing.T) {
	cfg := MCPConfig{
		Transport: "stdio",
		Command:   os.Args[0],
		Args:      []string{"-test.run=TestMCPStdioHelperProcess"},
		Env:       map[string]string{"SPECSYNC_MCP_TEST_HELPER": "1"},
	}
	p := NewMCPProvider(cfg)
	defer p.Close()

	ref, err := p.Push(context.Background(), WorkItem{Title: "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ref.ID != "99" {
		t.Errorf("got ref %+v", ref)
	}
}

// TestMCPStdioHelperProcess is not a real test: it's a subprocess entry
// point for TestMCPProvider_Stdio_RoundTrip, following the standard Go
// pattern for testing subprocess protocols (as used by os/exec's own
// TestHelperProcess). `go test` runs it like any other test, but it exits
// immediately unless the env var marking it as the intended subprocess is set.
func TestMCPStdioHelperProcess(t *testing.T) {
	if os.Getenv("SPECSYNC_MCP_TEST_HELPER") != "1" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var req map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}
		method, _ := req["method"].(string)
		id := req["id"]
		var resp map[string]any
		switch method {
		case "server/discover":
			resp = map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"resultType": "complete", "supportedVersions": []string{mcpProtocolVersion}}}
		case "tools/list":
			resp = map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"resultType": "complete", "tools": []mcpTool{{Name: "create_issue"}, {Name: "search_issues"}}}}
		case "tools/call":
			params, _ := req["params"].(map[string]any)
			name, _ := params["name"].(string)
			if name == "search_issues" {
				resp = map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"resultType": "complete", "content": []map[string]any{}, "structuredContent": []map[string]any{}}}
				break
			}
			resp = map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{
				"resultType": "complete", "content": []map[string]any{},
				"structuredContent": map[string]any{"id": "99", "url": "https://example.com/99"},
			}}
		default:
			resp = map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32601, "message": "unknown method"}}
		}
		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
	}
	_ = scanner.Err() // EOF from the parent closing stdin is the normal shutdown path
	os.Exit(0)
}
