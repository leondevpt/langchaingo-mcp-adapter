package main

import (
	"context"
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

	// Create a agent with the tools
	agent := agents.NewOneShotAgent(
		llm,
		mcpTools,
		agents.WithMaxIterations(3),
	)
	executor := agents.NewExecutor(agent)

	// Use the agent
	question := "welcome leon"
	result, err := chains.Run(
		ctx,
		executor,
		question,
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
	executableName := getExecutableName("mcp-server")
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
