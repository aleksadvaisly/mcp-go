package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestStdioWorkerPanicRecovery proves that a panicking tool handler does not
// kill a stdio worker goroutine permanently. Before the fix the worker died on
// panic, shrinking the pool until the server stopped responding. After the fix
// the panicking call returns an INTERNAL_ERROR and a subsequent call to a
// healthy tool still succeeds.
func TestStdioWorkerPanicRecovery(t *testing.T) {
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	mcpServer := NewMCPServer("test", "1.0.0")
	mcpServer.AddTool(mcp.NewTool("boom"), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		panic("handler exploded")
	})
	mcpServer.AddTool(mcp.NewTool("ok"), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("still alive"), nil
	})

	stdioServer := NewStdioServer(mcpServer)
	stdioServer.SetErrorLogger(log.New(io.Discard, "", 0))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = stdioServer.Listen(ctx, stdinReader, stdoutWriter)
		stdoutWriter.Close()
	}()

	scanner := bufio.NewScanner(stdoutReader)

	write := func(v any) {
		b, _ := json.Marshal(v)
		if _, err := stdinWriter.Write(append(b, '\n')); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	readResp := func() map[string]any {
		if !scanner.Scan() {
			t.Fatal("expected a response line, got EOF")
		}
		var m map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		return m
	}

	// init
	write(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"clientInfo":      map[string]any{"name": "c", "version": "1.0.0"},
		},
	})
	readResp()

	// panicking call -> must come back as an error, not a hang/crash
	done := make(chan map[string]any, 1)
	go func() { done <- readResp() }()
	write(map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]any{"name": "boom"},
	})

	select {
	case resp := <-done:
		if resp["error"] == nil {
			t.Fatalf("expected error response from panicking handler, got: %v", resp)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: worker did not respond to panicking call (likely died)")
	}

	// healthy call after the panic -> proves the worker pool survived
	go func() { done <- readResp() }()
	write(map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "tools/call",
		"params": map[string]any{"name": "ok"},
	})

	select {
	case resp := <-done:
		if resp["error"] != nil {
			t.Fatalf("healthy call after panic failed: %v", resp["error"])
		}
		if resp["result"] == nil {
			t.Fatalf("expected result for healthy call, got: %v", resp)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: server did not process healthy call after a panic")
	}
}
