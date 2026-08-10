package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
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

// MCPEndpoint holds connection info for an MCP server.
type MCPEndpoint struct {
	Transport MCPTransport
	Command   string
	Args      []string
	HTTPURL   string
	HTTPToken string `json:"-"`
}

// MCPTransport defines the transport for MCP communication.
type MCPTransport string

const (
	MCPTransportStdio MCPTransport = "stdio"
	MCPTransportHTTP  MCPTransport = "http"
)

// MCPProvider delegates to an external MCP server that implements the WorkProvider
// protocol. It satisfies WorkProvider plus optional capability interfaces.
type MCPProvider struct {
	endpoint MCPEndpoint
}

// NewMCPProvider creates an MCP provider.
func NewMCPProvider(ep MCPEndpoint) *MCPProvider {
	return &MCPProvider{endpoint: ep}
}

// Name returns the provider name.
func (p *MCPProvider) Name() string {
	return "mcp"
}

// Push delegates to the MCP server's "push" method.
func (p *MCPProvider) Push(ctx context.Context, item WorkItem, existing *Ref) (Ref, error) {
	params := map[string]any{
		"item": map[string]any{
			"slug":         item.Slug,
			"title":        item.Title,
			"body":         item.Body,
			"stage":        string(item.Stage),
			"priority":     item.Priority,
			"closed":       item.Closed,
			"manageClosed": item.ManageClosed,
			"labels":       item.Labels,
		},
	}
	if existing != nil {
		params["existing"] = map[string]any{
			"provider": existing.Provider,
			"id":       existing.ID,
			"url":      existing.URL,
		}
	}
	resp, err := p.call(ctx, "push", params)
	if err != nil {
		return Ref{}, fmt.Errorf("mcp push: %w", err)
	}
	return p.parseRef(resp)
}

// Find delegates to the MCP server's "find" method.
func (p *MCPProvider) Find(ctx context.Context, slug string) (*Ref, error) {
	resp, err := p.call(ctx, "find", map[string]any{"slug": slug})
	if err != nil {
		return nil, fmt.Errorf("mcp find: %w", err)
	}
	if resp == nil || len(resp) == 0 || string(resp) == "null" {
		return nil, nil
	}
	ref, err := p.parseRef(resp)
	if err != nil {
		return nil, err
	}
	return &ref, nil
}

// Comment posts a comment on an issue via the MCP server.
func (p *MCPProvider) Comment(issueID string, body string) error {
	ctx := context.Background()
	_, err := p.call(ctx, "comment", map[string]any{
		"issueId": issueID,
		"body":    body,
	})
	return err
}

// AddSubItem adds a sub-item via the MCP server.
func (p *MCPProvider) AddSubItem(parentID, childID string) error {
	ctx := context.Background()
	_, err := p.call(ctx, "addSubItem", map[string]any{
		"parentId": parentID,
		"childId":  childID,
	})
	return err
}

// RemoveSubItem removes a sub-item via the MCP server.
func (p *MCPProvider) RemoveSubItem(parentID, childID string) error {
	ctx := context.Background()
	_, err := p.call(ctx, "removeSubItem", map[string]any{
		"parentId": parentID,
		"childId":  childID,
	})
	return err
}

// SetCustomField sets a custom field on an issue via the MCP server.
func (p *MCPProvider) SetCustomField(issueID string, field string, value string) error {
	ctx := context.Background()
	_, err := p.call(ctx, "setCustomField", map[string]any{
		"issueId": issueID,
		"field":   field,
		"value":   value,
	})
	return err
}

// call makes a JSON-RPC call to the MCP server.
func (p *MCPProvider) call(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  method,
		"params":  params,
	}

	if p.endpoint.Transport == MCPTransportHTTP {
		return p.callHTTP(ctx, req)
	}
	return p.callStdio(ctx, req)
}

func (p *MCPProvider) callHTTP(ctx context.Context, req map[string]any) (json.RawMessage, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	reqURL := p.endpoint.HTTPURL
	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.endpoint.HTTPToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.endpoint.HTTPToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mcp http %d: %s", resp.StatusCode, slurp)
	}

	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int64           `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", envelope.Error.Code, envelope.Error.Message)
	}
	return envelope.Result, nil
}

func (p *MCPProvider) callStdio(ctx context.Context, req map[string]any) (json.RawMessage, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, p.endpoint.Command, p.endpoint.Args...)
	cmd.Stdin = strings.NewReader(string(body) + "\n")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("mcp stdio %s: %w (output: %s)", p.endpoint.Command, err, out)
	}

	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int64           `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		return nil, fmt.Errorf("parse mcp response: %w (raw: %s)", err, out)
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", envelope.Error.Code, envelope.Error.Message)
	}
	return envelope.Result, nil
}

func (p *MCPProvider) parseRef(data json.RawMessage) (Ref, error) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return Ref{}, err
	}
	var ref Ref
	if v, ok := m["provider"]; ok {
		ref.Provider = v.(string)
	}
	if v, ok := m["id"]; ok {
		ref.ID = v.(string)
	}
	if v, ok := m["url"]; ok {
		ref.URL = v.(string)
	}
	return ref, nil
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
