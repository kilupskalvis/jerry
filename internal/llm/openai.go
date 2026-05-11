// OpenAIProvider implements Provider using the OpenAI Chat Completions API.

package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
)

// Compile-time assertion that OpenAIProvider implements Provider.
var _ Provider = (*OpenAIProvider)(nil)

// OpenAIProvider implements Provider using the official OpenAI Go SDK.
type OpenAIProvider struct {
	client openai.Client
}

// NewOpenAIProvider creates a provider that calls the OpenAI API with the given key.
// Additional SDK options (e.g., option.WithBaseURL for testing) can be passed.
func NewOpenAIProvider(apiKey string, opts ...option.RequestOption) *OpenAIProvider {
	allOpts := append([]option.RequestOption{
		option.WithAPIKey(apiKey),
	}, opts...)
	return &OpenAIProvider{client: openai.NewClient(allOpts...)}
}

// Complete implements Provider.
// @lattice:boundary openai
func (p *OpenAIProvider) Complete(ctx context.Context, params CompleteParams) (*CompleteResponse, error) {
	apiParams := openai.ChatCompletionNewParams{
		Model:    params.Model,
		Messages: toOpenAIMessages(params.Messages, params.SystemPrompt),
	}

	if params.Temperature != nil {
		apiParams.Temperature = openai.Float(*params.Temperature)
	}

	if params.MaxTokens > 0 {
		apiParams.MaxCompletionTokens = openai.Int(int64(params.MaxTokens))
	}

	if len(params.Tools) > 0 {
		apiParams.Tools = toOpenAITools(params.Tools)
	}

	resp, err := p.client.Chat.Completions.New(ctx, apiParams)
	if err != nil {
		return nil, translateOpenAIError(err)
	}

	return fromOpenAIResponse(resp), nil
}

func toOpenAIMessages(messages []Message, systemPrompt string) []openai.ChatCompletionMessageParamUnion {
	var result []openai.ChatCompletionMessageParamUnion

	if systemPrompt != "" {
		result = append(result, openai.SystemMessage(systemPrompt))
	}

	for _, msg := range messages {
		switch msg.Role {
		case RoleUser:
			result = append(result, openai.UserMessage(msg.Content))

		case RoleAssistant:
			if len(msg.ToolCalls) == 0 {
				result = append(result, openai.AssistantMessage(msg.Content))
			} else {
				asst := openai.ChatCompletionAssistantMessageParam{}
				if msg.Content != "" {
					asst.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(msg.Content),
					}
				}
				toolCalls := make([]openai.ChatCompletionMessageToolCallParam, len(msg.ToolCalls))
				for i, tc := range msg.ToolCalls {
					toolCalls[i] = openai.ChatCompletionMessageToolCallParam{
						ID: tc.ID,
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      tc.Name,
							Arguments: string(tc.Input),
						},
					}
				}
				asst.ToolCalls = toolCalls
				result = append(result, openai.ChatCompletionMessageParamUnion{OfAssistant: &asst})
			}

		case RoleTool:
			content := msg.Content
			if len(content) > 7 && content[:7] == "ERROR: " {
				content = content[7:]
			}
			result = append(result, openai.ToolMessage(content, msg.ToolCallID))
		}
	}

	return result
}

func toOpenAITools(tools []ToolDefinition) []openai.ChatCompletionToolParam {
	result := make([]openai.ChatCompletionToolParam, len(tools))
	for i, t := range tools {
		var params shared.FunctionParameters
		_ = json.Unmarshal(t.Schema, &params)

		result[i] = openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name,
				Description: openai.String(t.Description),
				Parameters:  params,
			},
		}
	}
	return result
}

func fromOpenAIResponse(resp *openai.ChatCompletion) *CompleteResponse {
	if len(resp.Choices) == 0 {
		return &CompleteResponse{
			Message:    Message{Role: RoleAssistant},
			StopReason: StopReasonEndTurn,
		}
	}

	choice := resp.Choices[0]
	msg := Message{Role: RoleAssistant, Content: choice.Message.Content}

	for _, tc := range choice.Message.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}

	var stopReason StopReason
	switch choice.FinishReason {
	case "tool_calls":
		stopReason = StopReasonToolUse
	case "length":
		stopReason = StopReasonMaxTokens
	default:
		stopReason = StopReasonEndTurn
	}

	return &CompleteResponse{
		Message:    msg,
		StopReason: stopReason,
		Usage: Usage{
			InputTokens:  int(resp.Usage.PromptTokens),
			OutputTokens: int(resp.Usage.CompletionTokens),
		},
	}
}

func translateOpenAIError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusBadRequest:
			if apiErr.Code == "context_length_exceeded" ||
				strings.Contains(apiErr.Message, "maximum context length") {
				return &ContextTooLongError{
					Message: fmt.Sprintf("OpenAI: context too long: %s", apiErr.Message),
				}
			}
			return jerrerr.New(jerrerr.CodeLLMCallFailed,
				fmt.Sprintf("OpenAI API error (HTTP 400): %s", apiErr.Message))
		case http.StatusUnauthorized:
			return jerrerr.New(jerrerr.CodeLLMAuthFailed,
				"OpenAI API authentication failed — check OPENAI_API_KEY")
		case http.StatusTooManyRequests:
			return jerrerr.New(jerrerr.CodeLLMRateLimited,
				"OpenAI API rate limited after SDK retries exhausted")
		default:
			if apiErr.StatusCode >= 500 {
				return jerrerr.New(jerrerr.CodeLLMServerError,
					fmt.Sprintf("OpenAI API server error (HTTP %d)", apiErr.StatusCode))
			}
			return jerrerr.New(jerrerr.CodeLLMCallFailed,
				fmt.Sprintf("OpenAI API error (HTTP %d): %s", apiErr.StatusCode, apiErr.Message))
		}
	}

	return jerrerr.New(jerrerr.CodeLLMCallFailed,
		fmt.Sprintf("OpenAI call failed: %s", err.Error()))
}
