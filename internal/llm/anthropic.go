// Anthropic Claude LLM client, backed by the official anthropic-sdk-go.

package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	motifErrors "github.com/kilupskalvis/motif/internal/errors"
)

// DefaultMaxOutputTokens is the maximum output tokens requested per call.
// Claude Sonnet/Opus support up to 16384 for non-streaming requests.
const DefaultMaxOutputTokens int64 = 16384

// Compile-time assertion that AnthropicClient implements Client.
var _ Client = (*AnthropicClient)(nil)

// AnthropicClient implements Client using the official Anthropic Go SDK.
// It translates between our provider-neutral types and the SDK's API types.
type AnthropicClient struct {
	sdk   *anthropic.Client
	model string
}

// NewAnthropicClient creates an Anthropic client with the given API key.
// model is the default model to use (e.g., "claude-sonnet-4-6").
// Additional SDK options (e.g., option.WithBaseURL for testing) can be passed.
func NewAnthropicClient(apiKey, model string, opts ...option.RequestOption) *AnthropicClient {
	allOpts := append([]option.RequestOption{
		option.WithAPIKey(apiKey),
	}, opts...)
	sdkClient := anthropic.NewClient(allOpts...)
	return &AnthropicClient{
		sdk:   &sdkClient,
		model: model,
	}
}

// Send translates our Message types to Anthropic SDK types, calls the API,
// and translates the response back.
func (c *AnthropicClient) Send(
	requestCtx context.Context,
	system string,
	messages []Message,
	tools []ToolDef,
) (*Response, error) {
	sdkMessages := translateMessages(messages)
	sdkTools := translateToolDefs(tools)

	params := anthropic.MessageNewParams{
		Model:     c.resolveModel(),
		MaxTokens: DefaultMaxOutputTokens,
		Messages:  sdkMessages,
	}

	if system != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: system},
		}
	}

	if len(sdkTools) > 0 {
		params.Tools = sdkTools
	}

	sdkResp, err := c.sdk.Messages.New(requestCtx, params)
	if err != nil {
		return nil, c.translateError(err)
	}

	return translateResponse(sdkResp), nil
}

// resolveModel returns the model to use for API calls.
func (c *AnthropicClient) resolveModel() string {
	if c.model != "" {
		return c.model
	}
	return "claude-sonnet-4-6"
}

// translateMessages converts our Message slice to Anthropic SDK MessageParams.
// Consecutive RoleTool messages are coalesced into a single user message
// with multiple tool_result content blocks (Anthropic API requirement).
func translateMessages(messages []Message) []anthropic.MessageParam {
	var result []anthropic.MessageParam

	i := 0
	for i < len(messages) {
		msg := messages[i]

		switch msg.Role {
		case RoleUser:
			result = append(result, anthropic.NewUserMessage(
				anthropic.NewTextBlock(msg.Content),
			))
			i++

		case RoleAssistant:
			var blocks []anthropic.ContentBlockParamUnion
			if msg.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
			}
			for _, tc := range msg.ToolCalls {
				inputJSON, _ := json.Marshal(tc.Arguments)
				blocks = append(blocks, anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID:    tc.ID,
						Name:  tc.Name,
						Input: json.RawMessage(inputJSON),
					},
				})
			}
			result = append(result, anthropic.NewAssistantMessage(blocks...))
			i++

		case RoleTool:
			// Coalesce consecutive tool result messages into one user message.
			// The Anthropic API requires all tool results for a single assistant
			// turn to be sent as a single user message with multiple tool_result
			// content blocks.
			var toolResults []anthropic.ContentBlockParamUnion
			for i < len(messages) && messages[i].Role == RoleTool {
				toolResults = append(toolResults,
					anthropic.NewToolResultBlock(
						messages[i].ToolID,
						messages[i].Content,
						false,
					),
				)
				i++
			}
			result = append(result, anthropic.NewUserMessage(toolResults...))

		default:
			// Skip unknown roles gracefully.
			i++
		}
	}

	return result
}

// translateToolDefs converts our ToolDef slice to Anthropic SDK tool params.
func translateToolDefs(tools []ToolDef) []anthropic.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}

	result := make([]anthropic.ToolUnionParam, len(tools))
	for i, t := range tools {
		// Extract properties and required from our JSON Schema parameters.
		properties := t.Parameters["properties"]
		required, _ := t.Parameters["required"].([]any)

		var requiredStrs []string
		for _, r := range required {
			if s, ok := r.(string); ok {
				requiredStrs = append(requiredStrs, s)
			}
		}

		result[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.Opt(t.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: properties,
					Required:   requiredStrs,
				},
			},
		}
	}

	return result
}

// translateResponse converts an Anthropic SDK Message to our Response type.
func translateResponse(sdkResp *anthropic.Message) *Response {
	resp := &Response{
		StopReason: string(sdkResp.StopReason),
		Usage: TokenUsage{
			InputTokens:  int(sdkResp.Usage.InputTokens),
			OutputTokens: int(sdkResp.Usage.OutputTokens),
		},
	}

	var textParts []string

	for _, block := range sdkResp.Content {
		switch variant := block.AsAny().(type) {
		case anthropic.TextBlock:
			textParts = append(textParts, variant.Text)
		case anthropic.ToolUseBlock:
			var args map[string]any
			if len(variant.Input) > 0 {
				if unmarshalErr := json.Unmarshal(variant.Input, &args); unmarshalErr != nil {
					fmt.Fprintf(os.Stderr, "motif: warning: failed to parse tool input for %s: %s\n", variant.Name, unmarshalErr)
				}
			}
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:        variant.ID,
				Name:      variant.Name,
				Arguments: args,
			})
		}
	}

	if len(textParts) > 0 {
		resp.Content = strings.Join(textParts, "\n")
	}

	return resp
}

// translateError converts SDK errors to our error types.
func (c *AnthropicClient) translateError(err error) error {
	if err == nil {
		return nil
	}

	// Check for context cancellation first.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	// Check for SDK API errors with status codes.
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusBadRequest:
			errMsg := err.Error()
			if strings.Contains(errMsg, "too many tokens") ||
				strings.Contains(errMsg, "prompt is too long") ||
				strings.Contains(errMsg, "context length") {
				return &ContextTooLongError{
					Message: fmt.Sprintf("Anthropic: context too long: %s", errMsg),
				}
			}
			return motifErrors.New(motifErrors.CodeLLMCallFailed,
				fmt.Sprintf("Anthropic API error (HTTP 400): %s", errMsg))
		case http.StatusUnauthorized:
			return motifErrors.New(motifErrors.CodeLLMAuthFailed,
				"Anthropic API authentication failed — check ANTHROPIC_API_KEY")
		case http.StatusTooManyRequests:
			return motifErrors.New(motifErrors.CodeLLMRateLimited,
				"Anthropic API rate limited after SDK retries exhausted")
		case http.StatusInternalServerError, http.StatusBadGateway,
			http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return motifErrors.New(motifErrors.CodeLLMServerError,
				fmt.Sprintf("Anthropic API server error (HTTP %d)", apiErr.StatusCode))
		default:
			return motifErrors.New(motifErrors.CodeLLMCallFailed,
				fmt.Sprintf("Anthropic API error (HTTP %d): %s", apiErr.StatusCode, err.Error()))
		}
	}

	return motifErrors.New(motifErrors.CodeLLMCallFailed,
		fmt.Sprintf("LLM call failed: %s", err.Error()))
}
