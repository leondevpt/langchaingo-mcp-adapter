# LangChainGo MCP Adapter

**English** | [简体中文](README.zh-CN.md)

Integrate tools from the Model Context Protocol (MCP) into [LangChainGo](https://github.com/tmc/langchaingo) as callable tools, enabling you to build agents with robust tool-usage capabilities.

## Overview

This adapter bridges MCP and LangChainGo:
- Discovers available tools from an MCP server and converts them to `langchaingo/tools.Tool`.
- Provides a unified interface to call these tools within LangChainGo Agents/Chains.
- Adds convenience features like call timeouts and input schema exposition.

Core APIs in `adapter.go`:
- `New(session, opts...)`: create an adapter from an active MCP `ClientSession`.
- `Tools(ctx)`: fetch tools from MCP and convert them to a slice of LangChainGo tools.
- Each tool includes `Description()` with appended input schema and `Call(ctx, input)` where `input` is a JSON string.

## Features

- Tool discovery and conversion via MCP `ListTools`.
- Unified invocation entry for LangChainGo Agents.
- Timeout control using `WithToolTimeout(d)`.
- Input schema visibility via tool `Description()`.
- Multiple transports supported: `Stdio/CommandTransport` for local servers, `StreamableClientTransport` for remote HTTP streaming MCP.
- Ready-to-run examples: `example/server` and `example/agent`.

## Installation

Requirements: Go 1.21+.

Install:

```bash
go get github.com/leondevpt/langchaingo-mcp-adapter
go mod tidy
```

## Usage

Below shows how to connect to an MCP server, fetch tools, and use them in a LangChainGo Agent (simplified from `example/agent`):

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "os/exec"
    "time"

    mcpadapter "github.com/leondevpt/langchaingo-mcp-adapter"
    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/tmc/langchaingo/agents"
    "github.com/tmc/langchaingo/chains"
    "github.com/tmc/langchaingo/llms/openai"
    "github.com/tmc/langchaingo/tools"
)

func main() {
    ctx := context.Background()

    // Initialize LLM (OpenAI example)
    llm, err := openai.New(
        openai.WithToken(os.Getenv("OPENAI_API_KEY")),
        openai.WithBaseURL(os.Getenv("OPENAI_API_BASE_URL")),
        openai.WithModel(os.Getenv("OPENAI_MODEL")),
    )
    if err != nil { log.Fatal(err) }

    // Connect to a local MCP server (start example server via command)
    client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)
    transport := &mcp.CommandTransport{Command: exec.Command("../server/server")}
    session, err := client.Connect(ctx, transport, nil)
    if err != nil { log.Fatal(err) }
    defer session.Close()

    // Create adapter and fetch tools
    adapter := mcpadapter.New(session, mcpadapter.WithToolTimeout(30*time.Second))
    tools, err := adapter.Tools(ctx)
    if err != nil { log.Fatal(err) }

    // Verify discovery by directly calling the "greet" tool
    var greetTool tools.Tool
    for _, t := range tools {
        if t.Name() == "greet" { greetTool = t; break }
    }
    name := "leon" // dynamic variable, can be from env or user input
    payload, _ := json.Marshal(map[string]string{"name": name})
    if greetTool != nil {
        out, err := greetTool.Call(ctx, string(payload))
        if err != nil { log.Fatal(err) }
        fmt.Println("greet tool output:", out)
    }

    // Use tools in a LangChainGo Agent (require Final Answer prefix)
    agent := agents.NewOneShotAgent(llm, tools, agents.WithMaxIterations(5))
    executor := agents.NewExecutor(agent)

    prompt := fmt.Sprintf("Must call tool greet with params %s. Return only the tool output prefixed by 'Final Answer:'", string(payload))
    result, err := chains.Run(ctx, executor, prompt)
    if err != nil { log.Fatal(err) }
    fmt.Println(result)
}
```

To connect to a remote (HTTP streaming) MCP server, swap the transport:

```go
transport := &mcp.StreamableClientTransport{Endpoint: "https://mcp.example.com/mcp?key=YOUR_KEY"}
session, err := client.Connect(ctx, transport, nil)
```

## Examples

This repo ships two examples:

- `example/server`: a minimal MCP server (provides `greet`).
- `example/agent`: shows how to start/connect to the MCP server and call tools in an Agent.

Run the Agent example (it will build and start the example server via command transport):

```bash
export OPENAI_API_KEY=your_key
export OPENAI_API_BASE_URL=https://api.openai.com/v1
export OPENAI_MODEL=gpt-4o-mini

go run ./example/agent
```

Run the example server independently:

```bash
go run ./example/server
```

## License

MIT License

Copyright (c) 2024 leondevpt

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.