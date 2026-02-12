package tooling

import (
	"context"
	"encoding/json"
	"focalors-go/slogger"

	"github.com/openai/openai-go"
)

var logger = slogger.New("tooling")

// Context keys for tool execution
type ctxKey string

const targetKey ctxKey = "target"

// WithTarget adds a message target to the context
func WithTarget(ctx context.Context, target string) context.Context {
	return context.WithValue(ctx, targetKey, target)
}

// GetTarget retrieves the message target from context
func GetTarget(ctx context.Context) string {
	if v, ok := ctx.Value(targetKey).(string); ok {
		return v
	}
	return ""
}

// ContentType represents the type of content in a tool result
type ContentType int

const (
	ContentText ContentType = iota
	ContentImage
)

// Content represents a piece of content (text or image) in a tool result
type Content struct {
	Type     ContentType
	ToolName string // name of the tool that produced this content
	Text     string // for ContentText: markdown text
	Image    string // for ContentImage: base64 encoded image
	AltText  string // for ContentImage: alt text
}

// ToolResult contains the result of a tool execution
type ToolResult struct {
	// Text result to send back to OpenAI (summary for LLM)
	Text string
	// Contents is a sequence of text/images for the response card
	Contents []Content
}

// AddText adds a text content to the result
func (r *ToolResult) AddText(text string) *ToolResult {
	r.Contents = append(r.Contents, Content{Type: ContentText, Text: text})
	return r
}

// AddImage adds an image content to the result
func (r *ToolResult) AddImage(base64Image, altText string) *ToolResult {
	r.Contents = append(r.Contents, Content{Type: ContentImage, Image: base64Image, AltText: altText})
	return r
}

// NewToolResult creates a new tool result with LLM-facing text
func NewToolResult(text string) *ToolResult {
	return &ToolResult{Text: text}
}

// Tool represents a callable function tool for OpenAI
type Tool interface {
	// Name returns the tool name
	Name() string
	// Definition returns the OpenAI function definition
	Definition() openai.FunctionDefinitionParam
	// Execute runs the tool with the given arguments JSON and returns the result
	Execute(ctx context.Context, argsJSON string) (*ToolResult, error)
}

// Registry holds all registered tools
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Get returns a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// Definitions returns all tool definitions for OpenAI
func (r *Registry) Definitions() []openai.ChatCompletionToolParam {
	var defs []openai.ChatCompletionToolParam
	for _, tool := range r.tools {
		defs = append(defs, openai.ChatCompletionToolParam{
			Function: tool.Definition(),
		})
	}
	return defs
}

// Execute runs a tool by name with the given arguments
func (r *Registry) Execute(ctx context.Context, name string, argsJSON string) (*ToolResult, error) {
	tool, ok := r.tools[name]
	if !ok {
		return &ToolResult{Text: "Unknown tool"}, nil
	}
	result, err := tool.Execute(ctx, argsJSON)
	if err != nil {
		return result, err
	}
	// Tag all contents with the tool name
	for i := range result.Contents {
		result.Contents[i].ToolName = name
	}
	return result, nil
}

// ParseArgs is a helper to unmarshal tool arguments
func ParseArgs[T any](argsJSON string) (T, error) {
	var args T
	err := json.Unmarshal([]byte(argsJSON), &args)
	return args, err
}
