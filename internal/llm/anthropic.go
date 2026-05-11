// AnthropicProvider implements Provider using the Anthropic Claude API.

package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
)

// DefaultMaxOutputTokens is the maximum output tokens requested per call.
// Claude Sonnet/Opus support up to 16384 for non-streaming requests.
const DefaultMaxOutputTokens int64 = 16384

// Compile-time assertion that AnthropicProvider implements Provider.
var _ Provider = (*AnthropicProvider)(nil)

// AnthropicProvider implements Provider using the official Anthropic Go SDK.
// It translates between the provider-agnostic types in this package and the
// SDK's API types.
type AnthropicProvider struct {
	client anthropic.Client
}

// NewAnthropicProvider creates a provider that calls the Anthropic API with the given key.
// Additional SDK options (e.g., option.WithBaseURL for testing) can be passed.
func NewAnthropicProvider(apiKey string, opts ...option.RequestOption) *AnthropicProvider {
	allOpts := append([]option.RequestOption{
		option.WithAPIKey(apiKey),
	}, opts...)
	return &AnthropicProvider{client: anthropic.NewClient(allOpts...)}
}

// Complete implements Provider.
// @lattice:boundary anthropic
func (p *AnthropicProvider) Complete(ctx context.Context, params CompleteParams) (*CompleteResponse, error) {
	maxTokens := int64(params.MaxTokens)
	if maxTokens == 0 {
		maxTokens = DefaultMaxOutputTokens
	}

	apiParams := anthropic.MessageNewParams{
		Model:     params.Model,
		MaxTokens: maxTokens,
		Messages:  toAnthropicMessages(params.Messages),
	}

	if params.Temperature != nil {
		apiParams.Temperature = anthropic.Float(*params.Temperature)
	}

	if params.SystemPrompt != "" {
		apiParams.System = []anthropic.TextBlockParam{{
			Text:         params.SystemPrompt,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}}
	}

	if len(params.Tools) > 0 {
		apiParams.Tools = toAnthropicTools(params.Tools)
	}

	resp, err := p.client.Messages.New(ctx, apiParams)
	if err != nil {
		return nil, translateAnthropicError(err)
	}

	return fromAnthropicResponse(resp), nil
}

// toAnthropicMessages converts our Message slice to Anthropic SDK MessageParams.
// Consecutive RoleTool messages are coalesced into a single user message
// with multiple tool_result content blocks (Anthropic API requirement).
func toAnthropicMessages(messages []Message) []anthropic.MessageParam {
	var result []anthropic.MessageParam
	var pendingToolResults []anthropic.ContentBlockParamUnion

	for _, msg := range messages {
		if msg.Role != RoleTool && len(pendingToolResults) > 0 {
			result = append(result, anthropic.NewUserMessage(pendingToolResults...))
			pendingToolResults = nil
		}

		switch msg.Role {
		case RoleUser:
			result = append(result, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))

		case RoleAssistant:
			var blocks []anthropic.ContentBlockParamUnion
			if msg.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
			}
			for _, tc := range msg.ToolCalls {
				var rawInput any
				_ = json.Unmarshal(tc.Input, &rawInput)
				blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, rawInput, tc.Name))
			}
			result = append(result, anthropic.NewAssistantMessage(blocks...))

		case RoleTool:
			isError := len(msg.Content) > 7 && msg.Content[:7] == "ERROR: "
			content := msg.Content
			if isError {
				content = msg.Content[7:]
			}
			pendingToolResults = append(pendingToolResults,
				anthropic.NewToolResultBlock(msg.ToolCallID, content, isError))
		}
	}

	if len(pendingToolResults) > 0 {
		result = append(result, anthropic.NewUserMessage(pendingToolResults...))
	}

	return result
}

// toAnthropicTools converts our ToolDefinition slice to Anthropic SDK tool params.
func toAnthropicTools(tools []ToolDefinition) []anthropic.ToolUnionParam {
	result := make([]anthropic.ToolUnionParam, len(tools))
	for i, t := range tools {
		var schema struct {
			Properties any      `json:"properties"`
			Required   []string `json:"required"`
		}
		_ = json.Unmarshal(t.Schema, &schema)

		result[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: schema.Properties,
					Required:   schema.Required,
				},
			},
		}
	}
	return result
}

// fromAnthropicResponse converts an Anthropic SDK Message to our CompleteResponse.
func fromAnthropicResponse(resp *anthropic.Message) *CompleteResponse {
	msg := Message{Role: RoleAssistant}

	for _, block := range resp.Content {
		switch v := block.AsAny().(type) {
		case anthropic.TextBlock:
			if msg.Content != "" {
				msg.Content += "\n"
			}
			msg.Content += v.Text
		case anthropic.ToolUseBlock:
			inputJSON, _ := json.Marshal(v.Input)
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:    block.ID,
				Name:  v.Name,
				Input: inputJSON,
			})
		}
	}

	var stopReason StopReason
	switch resp.StopReason {
	case anthropic.StopReasonToolUse:
		stopReason = StopReasonToolUse
	case anthropic.StopReasonMaxTokens:
		stopReason = StopReasonMaxTokens
	case anthropic.StopReasonEndTurn, anthropic.StopReasonStopSequence,
		anthropic.StopReasonPauseTurn, anthropic.StopReasonRefusal:
		stopReason = StopReasonEndTurn
	}

	return &CompleteResponse{
		Message:    msg,
		StopReason: stopReason,
		Usage: Usage{
			InputTokens:         int(resp.Usage.InputTokens),
			OutputTokens:        int(resp.Usage.OutputTokens),
			CacheCreationTokens: int(resp.Usage.CacheCreationInputTokens),
			CacheReadTokens:     int(resp.Usage.CacheReadInputTokens),
		},
	}
}

// translateAnthropicError converts SDK errors to our error types.
func translateAnthropicError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

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
			return jerrerr.New(jerrerr.CodeLLMCallFailed,
				fmt.Sprintf("Anthropic API error (HTTP 400): %s", errMsg))
		case http.StatusUnauthorized:
			return jerrerr.New(jerrerr.CodeLLMAuthFailed,
				"Anthropic API authentication failed — check ANTHROPIC_API_KEY")
		case http.StatusTooManyRequests:
			return jerrerr.New(jerrerr.CodeLLMRateLimited,
				"Anthropic API rate limited after SDK retries exhausted")
		case http.StatusInternalServerError, http.StatusBadGateway,
			http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return jerrerr.New(jerrerr.CodeLLMServerError,
				fmt.Sprintf("Anthropic API server error (HTTP %d)", apiErr.StatusCode))
		default:
			return jerrerr.New(jerrerr.CodeLLMCallFailed,
				fmt.Sprintf("Anthropic API error (HTTP %d): %s", apiErr.StatusCode, err.Error()))
		}
	}

	return jerrerr.New(jerrerr.CodeLLMCallFailed,
		fmt.Sprintf("LLM call failed: %s", err.Error()))
}
