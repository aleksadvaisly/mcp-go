package server

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestExecuteTaskToolPanicRecovery proves that a panicking task handler is
// recovered and the task is marked failed, instead of crashing the process.
func TestExecuteTaskToolPanicRecovery(t *testing.T) {
	server := NewMCPServer(
		"test-server",
		"1.0.0",
		WithTaskCapabilities(true, true, true),
	)

	ctx := context.Background()
	ttl := int64(60000)
	pollInterval := int64(1000)
	entry, err := server.createTask(ctx, "task-panic", "boom-task", &ttl, &pollInterval)
	if err != nil {
		t.Fatalf("createTask: %v", err)
	}

	taskTool := ServerTaskTool{
		Tool: mcp.NewTool("boom-task"),
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CreateTaskResult, error) {
			panic("task handler exploded")
		},
	}
	request := mcp.CallToolRequest{}
	request.Params.Name = "boom-task"

	// Must not panic out of this call.
	server.executeTaskTool(ctx, entry, taskTool, request)

	task, _, err := server.getTask(ctx, "task-panic")
	if err != nil {
		t.Fatalf("getTask after panic: %v", err)
	}
	if task.Status != mcp.TaskStatusFailed {
		t.Fatalf("expected task status %q, got %q", mcp.TaskStatusFailed, task.Status)
	}
	if entry.resultErr == nil {
		t.Fatal("expected resultErr to be set after panicking handler")
	}
}
