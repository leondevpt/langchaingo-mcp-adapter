package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"runtime"

	"github.com/joho/godotenv"
	mcpadapter "github.com/leondevpt/langchaingo-mcp-adapter"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/tools"
)

func main() {
	godotenv.Load()
	ctx := context.Background()
	llm, err := initLLM()
	if err != nil {
		panic(err)
	}
	session, err := connectMCPServer(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	adapter := mcpadapter.New(session, mcpadapter.WithToolTimeout(30*time.Second))

    mcpTools, err := adapter.Tools(ctx)
    if err != nil {
        log.Fatal(err)
    }

    // Prepare dynamic input for tool calls
    name := os.Getenv("GREETER_NAME")
    if name == "" {
        name = "leon"
    }
    payload, err := json.Marshal(map[string]string{"name": name})
    if err != nil {
        log.Fatalf("marshal input payload error: %v", err)
    }

	// Verify MCP tool integration by directly calling the "greet" tool
	for _, t := range mcpTools {
		fmt.Println("discovered tool:", t.Name(), "-", t.Description())
	}
	var greetTool tools.Tool
	for _, t := range mcpTools {
		if t.Name() == "greet" {
			greetTool = t
			break
		}
	}
	if greetTool == nil {
		log.Println("greet tool not found; skip direct call verification")
    } else {
        out, err := greetTool.Call(ctx, string(payload))
        if err != nil {
            log.Printf("greet tool call error: %v", err)
        } else {
            fmt.Println("greet tool output:", out)
        }
    }

	// Create a agent with the tools
	agent := agents.NewOneShotAgent(
		llm,
		mcpTools,
		agents.WithMaxIterations(5),
	)
	executor := agents.NewExecutor(agent)

	// Whether the Agent calls a tool depends on the LLM's decision. To ensure it uses the tool more reliably, rephrase the question as an explicit instruction.
	// To ensure the parser functions correctly, require that the tool output is returned with the prefix "Final Answer:".
	// Use the dynamic name variable to construct the JSON input parameters.
    prompt := fmt.Sprintf("Must call tool greet with params %s. Return only the tool output prefixed by 'Final Answer:'", string(payload))
	result, err := chains.Run(
		ctx,
		executor,
		prompt,
	)
	if err != nil {
		log.Fatalf("Agent execution error: %v", err)
	}

	log.Println("agent result:", result)

}

func initLLM() (llms.Model, error) {
	llm, err := openai.New(
		openai.WithToken(os.Getenv("OPENAI_API_KEY")),
		openai.WithBaseURL(os.Getenv("OPENAI_API_BASE_URL")),
		openai.WithModel(os.Getenv("OPENAI_MODEL")),
	)
	if err != nil {
		return nil, fmt.Errorf("initLLM error: %w", err)
	}
	return llm, nil
}

func connectMCPServer(ctx context.Context) (*mcp.ClientSession, error) {
	serverDir := "../server"
	executableName := getExecutableName("server")
	serverBinary := filepath.Join(serverDir, executableName)

	// Check if binary exists
	if _, err := os.Stat(serverBinary); os.IsNotExist(err) {
		fmt.Println("Building MCP server binary...")
		cmd := exec.Command("go", "build", "-o", executableName, "main.go")
		cmd.Dir = serverDir
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("build server: %w\nOutput: %s", err, output)
		}
		fmt.Println("MCP server binary built successfully")
	}
	fmt.Printf("Starting MCP server from: %s\n", serverBinary)

	// Create a new client, with no features.
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)

	// Connect to a server over stdin/stdout.
	transport := &mcp.CommandTransport{Command: exec.Command(serverBinary)}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to mcp server: %w", err)
	}

	return session, nil
}

// getExecutableName returns the executable name with proper extension for the OS
func getExecutableName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}
