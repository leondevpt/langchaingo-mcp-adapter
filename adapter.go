package langchaingo_mcp_adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tmc/langchaingo/tools"
)

// Adapter is a bridge between MCP and LangChainGo, allowing MCP tools to be used as tools in LangChainGo.
type Adapter struct {
	session *mcp.ClientSession // MCP client session
	timeout time.Duration  //  timeout for tool calls
}


// Option defines a functional option type for configuring Adapter.
type Option func(*Adapter)

// WithToolTimeout sets the timeout for tool calls.
func WithToolTimeout(timeout time.Duration) Option {
	return func(a *Adapter) {
		a.timeout = timeout
	}
}


// New create a new Adapter instance
// param session is a active MCP client session
func New(session *mcp.ClientSession, opts... Option) *Adapter { 
	adapter := &Adapter{
		session: session,
		timeout: 30 *time.Second,
	}

	for _, opt := range opts {
		opt(adapter)
	}
	return adapter
}

// Tools  get all MCP server available tools and convert them to langchaingo/tools.Tool slice
func (a *Adapter)Tools(ctx context.Context) ([]tools.Tool, error) {
	if a.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.timeout)
		defer cancel()
	}

	// Discover available tools from MCP server
	mcpTools, err := a.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("ListTools error: %w", err)
	}

	// convert each MCP tool to langchaingo/tools.Tool
	var langchainTools []tools.Tool
	for _, tool := range mcpTools.Tools {
		langchainTools = append(langchainTools, &mcpTool{
			mcpTool: tool,
			session: a.session,
			timeout: a.timeout,
		})
	}

	return langchainTools, nil
}


// mcpTool implements langchaingo/tools.Tool interface 
// It wraps a MCP tool and provides a way to call it using the langchaingo/tools.Tool interface.
type mcpTool struct {
	// mcpTool is the MCP tool definition
	mcpTool *mcp.Tool
	// session is a active MCP client session, used to call the tool
	session *mcp.ClientSession 
	timeout time.Duration
}

// langchaingo/tools.Tool  interface methods implementation

// Name returns the name of the MCP tool.
func (t *mcpTool) Name() string {
	return t.mcpTool.Name
}

// Description returns the description and input schema of the MCP tool.
func (t *mcpTool) Description() string {
	description := t.mcpTool.Description
	if t.mcpTool.InputSchema != nil {
		schema, err := toString(t.mcpTool.InputSchema)
		if err == nil {
			description += "\n The input schema is: " + schema
		}
	}

	return description
}

// Call accepts a JSON string as input, which will be decoded into the arguments required by the tool.
// It returns the result of the tool call as a string.
func (t *mcpTool) Call(ctx context.Context, input string) (string, error) {
	if t.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.timeout)
		defer cancel()
	}
	var args map[string]any
	// decode the JSON input string from langchaingo
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("invalid JSON input: %w", err)
	}

	// prepare the call parameters for MCP tool
	callParams := &mcp.CallToolParams{
		Name:      t.mcpTool.Name,
		Arguments: args,
	}

	// call the MCP tool using the session
	result, err := t.session.CallTool(ctx, callParams)
	if err != nil {
		return "", fmt.Errorf("CallTool MCP tool '%s' error: %w", t.mcpTool.Name, err)
	}

	if result == nil || result.Content == nil {
		return "", fmt.Errorf("MCP tool '%s' returned nil result", t.mcpTool.Name)
	}

	// concatenate the result content into a single string
	var contentParts []string
	for _, content := range result.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			contentParts = append(contentParts, textContent.Text)
		}
	}

	return strings.Join(contentParts, "\n"), nil
}


func toString(v any) (string, error) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("error marshaling to JSON: %w", err)
	}
	return string(jsonBytes),nil
}