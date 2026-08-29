package mcp

import (
	"errors"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bulislaw/suuntool/internal/api"
)

// toolResult is an SDK-agnostic representation of a tool call result.
// server.go (Task 4) translates this into the SDK's CallToolResult shape.
type toolResult struct {
	IsError           bool
	TextContent       string
	StructuredContent map[string]any
}

// mapError turns any handler error into a structured tool error result.
// Typed *api.Error preserves Code/Message/Hint/HTTP/Exit for the LLM.
func mapError(err error) *toolResult {
	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		return &toolResult{
			IsError:     true,
			TextContent: apiErr.Message + " (" + apiErr.Code + ")",
			StructuredContent: map[string]any{
				"code":    apiErr.Code,
				"message": apiErr.Message,
				"hint":    apiErr.Hint,
				"http":    apiErr.HTTP,
				"exit":    apiErr.Exit,
			},
		}
	}
	return &toolResult{
		IsError:           true,
		TextContent:       err.Error(),
		StructuredContent: map[string]any{"code": "UNKNOWN", "message": err.Error()},
	}
}

// mapErrorToCallToolResult adapts the SDK-agnostic toolResult into the SDK's
// CallToolResult so handlers can return it directly.
func mapErrorToCallToolResult(err error) *sdkmcp.CallToolResult {
	r := mapError(err)
	return &sdkmcp.CallToolResult{
		IsError:           r.IsError,
		Content:           []sdkmcp.Content{&sdkmcp.TextContent{Text: r.TextContent}},
		StructuredContent: r.StructuredContent,
	}
}
