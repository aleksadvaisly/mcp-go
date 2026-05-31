# mcp-go

[![Build](https://github.com/mark3labs/mcp-go/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/mark3labs/mcp-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/mark3labs/mcp-go?cache)](https://goreportcard.com/report/github.com/mark3labs/mcp-go)
[![GoDoc](https://pkg.go.dev/badge/github.com/mark3labs/mcp-go.svg)](https://pkg.go.dev/github.com/mark3labs/mcp-go)


<p align="center"><img src="docs/golang_mcp.png" width="520"></p>

You want to give an LLM access to your data and your code - a database, an internal API, a file tree - without hand-rolling a JSON-RPC server and wiring up every protocol detail. mcp-go is a Go library for building [Model Context Protocol](https://modelcontextprotocol.io) servers (and clients) where you write the tool and the library handles the wire.

It implements MCP version 2025-11-25, with backward compatibility for 2025-06-18, 2025-03-26 and 2024-11-05.

## Getting Started

```bash
go get github.com/mark3labs/mcp-go
```

A server is a name, a set of tools and a transport. Here is one that greets people over stdio - the transport AI coding agents launch:

```go
package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer("Demo", "1.0.0", server.WithToolCapabilities(true))

	tool := mcp.NewTool("hello_world",
		mcp.WithDescription("Say hello to someone"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Name to greet")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := req.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Hello, %s!", name)), nil
	})

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
```

Build it, point your MCP client at the binary and the `hello_world` tool shows up. That is the whole loop - everything below is variations on it.

## Tools

A tool is a function the model can call. You declare its parameters with typed options so the model gets a real schema, then read them back inside the handler. The `Require*` and `Get*` accessors coerce and validate for you, so a handler stays focused on logic:

```go
calc := mcp.NewTool("calculate",
	mcp.WithDescription("Perform basic arithmetic"),
	mcp.WithString("operation", mcp.Required(),
		mcp.Enum("add", "subtract", "multiply", "divide")),
	mcp.WithNumber("x", mcp.Required()),
	mcp.WithNumber("y", mcp.Required()),
)

s.AddTool(calc, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	op, _ := req.RequireString("operation")
	x, _ := req.RequireFloat("x")
	y, _ := req.RequireFloat("y")

	switch op {
	case "add":
		return mcp.NewToolResultText(fmt.Sprintf("%.2f", x+y)), nil
	case "divide":
		if y == 0 {
			return mcp.NewToolResultError("cannot divide by zero"), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("%.2f", x/y)), nil
	}
	return mcp.NewToolResultError("unknown operation"), nil
})
```

Return a recoverable problem with `NewToolResultError` so the model sees it and can retry. Reserve a real `error` return for protocol-level failures.

## Resources and Prompts

Tools execute. Resources expose data the model can pull into context - a file, an API response, a query result - addressed by URI. A static one:

```go
res := mcp.NewResource("docs://readme", "Project README",
	mcp.WithMIMEType("text/markdown"))

s.AddResource(res, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	content, err := os.ReadFile("README.md")
	if err != nil {
		return nil, err
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: "docs://readme", MIMEType: "text/markdown", Text: string(content)},
	}, nil
})
```

Prompts are reusable message templates the client can offer to users - encode your "ask it this way" patterns once:

```go
s.AddPrompt(mcp.NewPrompt("greeting",
	mcp.WithPromptDescription("A friendly greeting"),
	mcp.WithArgument("name", mcp.ArgumentDescription("Who to greet")),
), func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	name := req.Params.Arguments["name"]
	if name == "" {
		name = "friend"
	}
	return mcp.NewGetPromptResult("A friendly greeting", []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleAssistant,
			mcp.NewTextContent("Hello, "+name+"! How can I help?")),
	}), nil
})
```

## Transports

The same server runs over three transports. Stdio is what local agents launch. SSE and streamable-HTTP expose it over the network:

```go
server.ServeStdio(s)                       // stdio
server.NewSSEServer(s).Start(":8080")      // SSE
server.NewStreamableHTTPServer(s).Start(":8080") // streamable HTTP
```

For SSE, `SetConnectionLostHandler()` lets you detect drops and drive reconnection.

## Stdio Lifecycle Management

When an agent launches a stdio server but dies without cleaning up, the server can hang forever as an orphan. Two guards prevent that.

The **parent process monitor** (enabled by default) polls the parent PID every 5 seconds and shuts down gracefully once the original launcher is gone. The **idle timeout** (disabled by default) shuts the server down after a quiet period - the timer resets on every message and never fires mid tool-call. It is off by default because most agents do not auto-reconnect, so an early shutdown would strand the endpoint.

```go
server.ServeStdio(s)                                    // defaults: monitor on, no idle timeout
server.ServeStdio(s, server.WithIdleTimeout(30*time.Minute))
server.ServeStdio(s, server.WithParentProcessMonitor(false))
```

## Going Further

Sessions, per-session tools, request hooks, tool-handler middleware and typed handlers are all covered in the [GoDoc](https://pkg.go.dev/github.com/mark3labs/mcp-go) and the runnable programs under [`examples/`](examples/). This is a fork of [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) - import path unchanged - carrying extra stdio lifecycle hardening and structuredContent fixes on top of upstream.
