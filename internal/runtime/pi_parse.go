package runtime

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
)

// piResult is the outcome of parsing a pi session stream.
type piResult struct {
	Text  string
	Usage *Usage
}

// piEvent is one JSONL line. Only the fields Jerry needs are decoded.
type piEvent struct {
	Type    string     `json:"type"`
	Message *piMessage `json:"message"`
}

type piMessage struct {
	Role         string           `json:"role"`
	Content      []piContentBlock `json:"content"`
	Usage        *piUsage         `json:"usage"`
	StopReason   string           `json:"stopReason"`
	ErrorMessage string           `json:"errorMessage"`
}

type piContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type piUsage struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cacheRead"`
	CacheWrite int64 `json:"cacheWrite"`
	Cost       struct {
		Total float64 `json:"total"`
	} `json:"cost"`
}

// parseSession reads pi's JSONL event stream and returns the final
// assistant message's text and usage. It errors when the stream is
// corrupt, carries no assistant message, or the assistant ended in an
// error/aborted state.
func parseSession(data []byte) (piResult, error) {
	var last *piMessage
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024) // pi lines can be large
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var ev piEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			return piResult{}, fmt.Errorf("corrupt pi output line: %w", err)
		}
		if ev.Type == "message_end" && ev.Message != nil && ev.Message.Role == "assistant" {
			last = ev.Message
		}
	}
	if err := sc.Err(); err != nil {
		return piResult{}, fmt.Errorf("reading pi output: %w", err)
	}
	if last == nil {
		return piResult{}, fmt.Errorf("pi produced no assistant message")
	}
	if last.StopReason == "error" || last.StopReason == "aborted" {
		msg := last.ErrorMessage
		if msg == "" {
			msg = last.StopReason
		}
		return piResult{}, fmt.Errorf("pi run failed (%s): %s", last.StopReason, msg)
	}

	var text string
	for _, b := range last.Content {
		if b.Type != "text" {
			continue
		}
		if text != "" {
			text += "\n"
		}
		text += b.Text
	}

	res := piResult{Text: text}
	if last.Usage != nil {
		res.Usage = &Usage{
			InputTokens:  last.Usage.Input + last.Usage.CacheRead + last.Usage.CacheWrite,
			OutputTokens: last.Usage.Output,
			CostUSD:      last.Usage.Cost.Total,
		}
	}
	return res, nil
}
