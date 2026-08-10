package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestMCPProvider_HTTP_Push(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"provider":"mcp","id":"42","url":"https://example.com/42"}}`))
	}))
	defer server.Close()

	p := NewMCPProvider(MCPEndpoint{
		Transport: MCPTransportHTTP,
		HTTPURL:   server.URL,
	})

	ctx := context.Background()
	ref, err := p.Push(ctx, WorkItem{Slug: "test", Title: "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ref.ID != "42" {
		t.Errorf("got %q", ref.ID)
	}
}

func TestMCPProvider_HTTP_Find(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"provider":"mcp","id":"42","url":"https://example.com/42"}}`))
	}))
	defer server.Close()

	p := NewMCPProvider(MCPEndpoint{
		Transport: MCPTransportHTTP,
		HTTPURL:   server.URL,
	})

	ctx := context.Background()
	ref, err := p.Find(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	if ref.ID != "42" {
		t.Errorf("got %q", ref.ID)
	}
}

func TestMCPProvider_HTTP_Find_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":null}`))
	}))
	defer server.Close()

	p := NewMCPProvider(MCPEndpoint{
		Transport: MCPTransportHTTP,
		HTTPURL:   server.URL,
	})

	ctx := context.Background()
	ref, err := p.Find(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	if ref != nil {
		t.Error("expected nil ref")
	}
}

func TestMCPProvider_HTTP_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"invalid"}}`))
	}))
	defer server.Close()

	p := NewMCPProvider(MCPEndpoint{
		Transport: MCPTransportHTTP,
		HTTPURL:   server.URL,
	})

	ctx := context.Background()
	_, err := p.Push(ctx, WorkItem{Title: "test"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMCPProvider_HTTP_Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	p := NewMCPProvider(MCPEndpoint{
		Transport: MCPTransportHTTP,
		HTTPURL:   server.URL,
	})

	ctx := context.Background()
	_, err := p.Push(ctx, WorkItem{Title: "test"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMCPProvider_Comment(t *testing.T) {
	var method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		method = req["method"].(string)
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer server.Close()

	p := NewMCPProvider(MCPEndpoint{
		Transport: MCPTransportHTTP,
		HTTPURL:   server.URL,
	})

	err := p.Comment("1", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if method != "comment" {
		t.Errorf("got method %s", method)
	}
}

func TestMCPProvider_AddSubItem(t *testing.T) {
	var method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		method = req["method"].(string)
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer server.Close()

	p := NewMCPProvider(MCPEndpoint{
		Transport: MCPTransportHTTP,
		HTTPURL:   server.URL,
	})

	err := p.AddSubItem("parent", "child")
	if err != nil {
		t.Fatal(err)
	}
	if method != "addSubItem" {
		t.Errorf("got method %s", method)
	}
}

func TestMCPProvider_SetCustomField(t *testing.T) {
	var method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		method = req["method"].(string)
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer server.Close()

	p := NewMCPProvider(MCPEndpoint{
		Transport: MCPTransportHTTP,
		HTTPURL:   server.URL,
	})

	err := p.SetCustomField("1", "priority", "high")
	if err != nil {
		t.Fatal(err)
	}
	if method != "setCustomField" {
		t.Errorf("got method %s", method)
	}
}

func TestMCPEndpoint_EncodeDecode(t *testing.T) {
	ep := MCPEndpoint{
		Transport: MCPTransportHTTP,
		HTTPURL:   "https://example.com/mcp",
		HTTPToken: "secret",
	}
	data, _ := json.Marshal(ep)
	var ep2 MCPEndpoint
	json.Unmarshal(data, &ep2)
	if ep2.Transport != MCPTransportHTTP {
		t.Errorf("got transport %s", ep2.Transport)
	}
	if ep2.HTTPURL != "https://example.com/mcp" {
		t.Errorf("got url %s", ep2.HTTPURL)
	}
	if ep2.HTTPToken != "" {
		t.Error("HTTPToken should not be encoded")
	}
}
