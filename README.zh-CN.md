# LangChainGo MCP Adapter

[English](README.md) | **简体中文**

将 Model Context Protocol (MCP) 的工具无缝接入 [LangChainGo](https://github.com/tmc/langchaingo) 作为可调用工具，帮助你快速构建具备“工具使用能力”的智能体。

## Overview

本适配器充当 MCP 与 LangChainGo 之间的桥梁：

- 从 MCP 服务端发现可用工具并转换为 `langchaingo/tools.Tool`；
- 以统一接口在 LangChainGo 的 Agent/Chain 中调用这些工具；
- 提供超时控制、输入模式描述拼接等易用能力。

代码核心位于 `adapter.go`：

- `New(session, opts...)`：基于一个活跃的 MCP `ClientSession` 创建适配器；
- `Tools(ctx)`：拉取 MCP 服务端的工具并转换为 LangChainGo 的工具切片；
- 工具实现会在 `Description()` 中追加输入模式（schema）说明，`Call(ctx, input)` 支持以 JSON 字符串作为调用参数。

## Features

- 工具发现与转换：基于 MCP `ListTools` 自动发现并生成 `langchaingo/tools.Tool`。
- 统一调用入口：直接在 LangChainGo Agent 中使用 MCP 工具，降低集成复杂度。
- 超时控制：`WithToolTimeout(d)` 配置工具调用超时，避免长时间阻塞。
- 输入模式可见：在工具 `Description()` 中附加 `InputSchema`，提高可用性。
- 多运输层支持：可通过 `Stdio/CommandTransport` 连接本地 MCP 服务器，也可用 `StreamableClientTransport` 连接远端 HTTP 流式 MCP 服务。
- 配套示例：提供最小可运行的 `example/server` 与 `example/agent`。

## Installation

要求：Go 1.21+（模块中声明了更高版本号，建议使用较新 Go 版本）。

安装本模块：

```bash
go get github.com/leondevpt/langchaingo-mcp-adapter
go mod tidy
```

## Usage

以下示例展示如何连接 MCP 服务端、拉取工具并在 LangChainGo Agent 中使用它们（简化自 `example/agent`）：

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

    // 初始化 LLM（以 OpenAI 为例）
    llm, err := openai.New(
        openai.WithToken(os.Getenv("OPENAI_API_KEY")),
        openai.WithBaseURL(os.Getenv("OPENAI_API_BASE_URL")),
        openai.WithModel(os.Getenv("OPENAI_MODEL")),
    )
    if err != nil { log.Fatal(err) }

    // 连接本地 MCP 服务器（使用命令启动示例服务器）
    client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)
    transport := &mcp.CommandTransport{Command: exec.Command("../server/server")}
    session, err := client.Connect(ctx, transport, nil)
    if err != nil { log.Fatal(err) }
    defer session.Close()

    // 创建适配器并拉取工具
    adapter := mcpadapter.New(session, mcpadapter.WithToolTimeout(30*time.Second))
    tools, err := adapter.Tools(ctx)
    if err != nil { log.Fatal(err) }

    // 先直接调用一次 "greet" 工具以验证发现和调用链路
    var greetTool tools.Tool
    for _, t := range tools {
        if t.Name() == "greet" { greetTool = t; break }
    }
    name := os.Getenv("GREETER_NAME")
    if name == "" { name = "leon" }
    payload, _ := json.Marshal(map[string]string{"name": name})
    if greetTool != nil {
        out, err := greetTool.Call(ctx, string(payload))
        if err != nil { log.Fatal(err) }
        fmt.Println("greet tool output:", out)
    }

    // 在 LangChainGo Agent 中使用这些工具（要求以 Final Answer: 前缀返回结果）
    agent := agents.NewOneShotAgent(llm, tools, agents.WithMaxIterations(5))
    executor := agents.NewExecutor(agent)

    prompt := fmt.Sprintf("必须调用工具 greet，并传入参数 %s。完成后以 Final Answer: 为前缀原样返回工具输出，不要添加任何其他内容。", string(payload))
    result, err := chains.Run(ctx, executor, prompt)
    if err != nil { log.Fatal(err) }
    fmt.Println(result)
}
```

连接远端（HTTP 流式）MCP 服务端（例如高德地图 MCP）可替换运输层：

```go
transport := &mcp.StreamableClientTransport{Endpoint: "https://mcp.example.com/mcp?key=YOUR_KEY"}
session, err := client.Connect(ctx, transport, nil)
```

## Examples

仓库提供两个示例：

- `example/server`：一个最小 MCP 服务端（提供 `greet` 工具）。
- `example/agent`：演示如何启动/连接 MCP 服务端并在 LangChainGo Agent 中调用工具。

运行 Agent 示例（会自动构建并通过命令运输层启动示例服务端）：

```bash
export OPENAI_API_KEY=your_key
export OPENAI_API_BASE_URL=https://api.openai.com/v1
export OPENAI_MODEL=gpt-4o-mini

go run ./example/agent
```

也可独立运行示例服务端：

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
