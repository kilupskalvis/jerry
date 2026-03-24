// OpenAI Chat Completions LLM client, backed by the official openai-go SDK.

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

	motifErrors "github.com/kilupskalvis/motif/internal/errors"
)

// Compile-time assertion that OpenAIClient implements Client.
var _ Client = (*OpenAIClient)(nil)

// OpenAIClient implements Client using the official OpenAI Go SDK.
type OpenAIClient struct {
	sdk   *openai.Client
	model string
}

// NewOpenAIClient creates an OpenAI client with the given API key and model.
// Additional SDK options (e.g., option.WithBaseURL for testing) can be passed.
func NewOpenAIClient(apiKey, model string, opts ...option.RequestOption) *OpenAIClient {
	allOpts := append([]option.RequestOption{
		option.WithAPIKey(apiKey),
	}, opts...)
	sdkClient := openai.NewClient(allOpts...)
	return &OpenAIClient{
		sdk:   &sdkClient,
		model: model,
	}
}

// Send translates our Message types to OpenAI SDK types, calls the API,
// and translates the response back.
func (c *OpenAIClient) Send(
	requestCtx context.Context,
	system string,
	messages []Message,
	tools []ToolDef,
) (*Response, error) {
	params := c.buildParams(system, messages, tools)

	completion, sendErr := c.sdk.Chat.Completions.New(requestCtx, params)
	if sendErr != nil {
		return nil, c.translateError(sendErr)
	}

	return c.translateResponse(completion), nil
}

func (c *OpenAIClient) buildParams(system string, messages []Message, tools []ToolDef) openai.ChatCompletionNewParams {
	var oaiMessages []openai.ChatCompletionMessageParamUnion

	// System prompt as first message.
	if system != "" {
		oaiMessages = append(oaiMessages, openai.SystemMessage(system))
	}

	for _, msg := range messages {
		switch msg.Role {
		case RoleUser:
			oaiMessages = append(oaiMessages, openai.UserMessage(msg.Content))

		case RoleAssistant:
			if len(msg.ToolCalls) == 0 {
				oaiMessages = append(oaiMessages, openai.AssistantMessage(msg.Content))
			} else {
				assistantMsg := openai.ChatCompletionAssistantMessageParam{}
				if msg.Content != "" {
					assistantMsg.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(msg.Content),
					}
				}
				toolCalls := make([]openai.ChatCompletionMessageToolCallParam, len(msg.ToolCalls))
				for i, tc := range msg.ToolCalls {
					argsJSON, _ := json.Marshal(tc.Arguments)
					toolCalls[i] = openai.ChatCompletionMessageToolCallParam{
						ID: tc.ID,
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      tc.Name,
							Arguments: string(argsJSON),
						},
					}
				}
				assistantMsg.ToolCalls = toolCalls
				oaiMessages = append(oaiMessages, openai.ChatCompletionMessageParamUnion{
					OfAssistant: &assistantMsg,
				})
			}

		case RoleTool:
			oaiMessages = append(oaiMessages, openai.ToolMessage(msg.Content, msg.ToolID))
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    c.model,
		Messages: oaiMessages,
	}

	if len(tools) > 0 {
		oaiTools := make([]openai.ChatCompletionToolParam, len(tools))
		for i, t := range tools {
			oaiTools[i] = openai.ChatCompletionToolParam{
				Function: shared.FunctionDefinitionParam{
					Name:        t.Name,
					Description: openai.String(t.Description),
					Parameters:  shared.FunctionParameters(t.Parameters),
				},
			}
		}
		params.Tools = oaiTools
	}

	return params
}

func (c *OpenAIClient) translateResponse(completion *openai.ChatCompletion) *Response {
	if len(completion.Choices) == 0 {
		return &Response{}
	}

	choice := completion.Choices[0]
	resp := &Response{
		Content: choice.Message.Content,
		Usage: TokenUsage{
			InputTokens:  int(completion.Usage.PromptTokens),
			OutputTokens: int(completion.Usage.CompletionTokens),
		},
	}

	// Map finish_reason.
	switch choice.FinishReason {
	case "stop":
		resp.StopReason = "end_turn"
	case "tool_calls":
		resp.StopReason = "tool_use"
	default:
		resp.StopReason = choice.FinishReason
	}

	// Parse tool calls.
	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		if tc.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		}
		resp.ToolCalls = append(resp.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}

	return resp
}

func (c *OpenAIClient) translateError(err error) error {
	if err == nil {
		return nil
	}

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
			return motifErrors.New(motifErrors.CodeLLMCallFailed,
				fmt.Sprintf("OpenAI API error (HTTP 400): %s", apiErr.Message))
		case http.StatusUnauthorized:
			return motifErrors.New(motifErrors.CodeLLMAuthFailed,
				"OpenAI API authentication failed — check OPENAI_API_KEY")
		case http.StatusTooManyRequests:
			return motifErrors.New(motifErrors.CodeLLMRateLimited,
				"OpenAI API rate limited after SDK retries exhausted")
		default:
			if apiErr.StatusCode >= 500 {
				return motifErrors.New(motifErrors.CodeLLMServerError,
					fmt.Sprintf("OpenAI API server error (HTTP %d)", apiErr.StatusCode))
			}
			return motifErrors.New(motifErrors.CodeLLMCallFailed,
				fmt.Sprintf("OpenAI API error (HTTP %d): %s", apiErr.StatusCode, apiErr.Message))
		}
	}

	return motifErrors.New(motifErrors.CodeLLMCallFailed,
		fmt.Sprintf("OpenAI call failed: %s", err.Error()))
}
