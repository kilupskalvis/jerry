package runtime

import (
	"encoding/json"
	"fmt"
)

type claudeCodeResult struct {
	Type      string           `json:"type"`
	Subtype   string           `json:"subtype"`
	ResultStr string           `json:"result"`
	CostUSD   float64          `json:"cost_usd"`
	IsError   bool             `json:"is_error"`
	Usage     *claudeCodeUsage `json:"usage"`
}

type claudeCodeUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

func parseClaudeCodeOutput(data []byte) (Result, error) {
	var cr claudeCodeResult
	if err := json.Unmarshal(data, &cr); err != nil {
		return Result{}, fmt.Errorf("claude-code output is not valid JSON: %w", err)
	}

	res := Result{Text: cr.ResultStr}
	if cr.Usage != nil {
		res.Usage = &Usage{
			InputTokens:  cr.Usage.InputTokens,
			OutputTokens: cr.Usage.OutputTokens,
			CostUSD:      cr.CostUSD,
		}
	}

	if cr.IsError {
		return res, fmt.Errorf("claude-code run failed (%s): %s", cr.Subtype, cr.ResultStr)
	}

	return res, nil
}
