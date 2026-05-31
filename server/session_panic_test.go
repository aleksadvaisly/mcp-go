package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestSessionHookPanicRecovery proves that a panicking OnError hook invoked
// from the blocked-notification-channel goroutine does not crash the process.
// Before the fix these 9 goroutines in session.go had no recover(), so a
// panicking user hook took down the whole server and every connected session.
func TestSessionHookPanicRecovery(t *testing.T) {
	hooks := &Hooks{}
	var called sync.WaitGroup
	called.Add(1)
	hooks.AddOnError(func(ctx context.Context, id any, method mcp.MCPMethod, message any, err error) {
		defer called.Done()
		panic("hook exploded")
	})

	srv := NewMCPServer("test", "1.0.0", WithHooks(hooks))

	// Session with an unbuffered channel and no reader: the notification send
	// in the select hits the default branch and fires the hook goroutine.
	session := &sessionTestClient{
		sessionID:           "blocked-session",
		notificationChannel: make(chan mcp.JSONRPCNotification),
		initialized:         true,
	}
	srv.sessions.Store(session.SessionID(), session)

	srv.sendNotificationToAllClients(mcp.JSONRPCNotification{
		Notification: mcp.Notification{Method: "notifications/test"},
	})

	// If recover() were missing, the panicking goroutine would crash the test
	// binary. Reaching here after the hook ran proves the panic was contained.
	done := make(chan struct{})
	go func() { called.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: OnError hook was never invoked")
	}
}
