// Structured execution logging: writes typed events to log.jsonl (NDJSON).

package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogType identifies the kind of log entry.
type LogType string

const (
	LogPipelineStart LogType = "pipeline_start"
	LogPipelineEnd   LogType = "pipeline_end"
	LogStepStart     LogType = "step_start"
	LogStepEnd       LogType = "step_end"
	LogLLMCall       LogType = "llm_call"
	LogToolCall      LogType = "tool_call"
	LogCompaction    LogType = "compaction"
	LogRetry         LogType = "retry"
	LogWarning       LogType = "warning"
	LogError         LogType = "error"
)

// LogEntry represents a single event in the execution log.
type LogEntry struct {
	Timestamp time.Time       `json:"timestamp"`
	Type      LogType         `json:"type"`
	Step      string          `json:"step,omitempty"`
	Data      json.RawMessage `json:"data"`
}

// Typed payloads for each LogType.

type PipelineStartData struct {
	RunID         string   `json:"run_id"`
	Pipeline      string   `json:"pipeline"`
	TriggerIntent string   `json:"trigger_intent,omitempty"`
	Steps         []string `json:"steps"`
}

type PipelineEndData struct {
	Status         string `json:"status"`
	DurationMs     int64  `json:"duration_ms"`
	TotalTokens    int    `json:"total_tokens"`
	StepsCompleted int    `json:"steps_completed"`
	StepsFailed    int    `json:"steps_failed"`
	StepsSkipped   int    `json:"steps_skipped"`
}

type StepStartData struct {
	Type          string `json:"type"`
	AgentFile     string `json:"agent_file,omitempty"`
	Model         string `json:"model,omitempty"`
	MaxIterations int    `json:"max_iterations,omitempty"`
}

type StepEndData struct {
	Status       string `json:"status"`
	DurationMs   int64  `json:"duration_ms"`
	Iterations   int    `json:"iterations,omitempty"`
	LLMCalls     int    `json:"llm_calls,omitempty"`
	ToolCalls    int    `json:"tool_calls,omitempty"`
	TokensInput  int    `json:"tokens_input,omitempty"`
	TokensOutput int    `json:"tokens_output,omitempty"`
	OutputKey    string `json:"output_key,omitempty"`
}

type LLMCallData struct {
	Iteration          int      `json:"iteration"`
	Model              string   `json:"model"`
	TokensInput        int      `json:"tokens_input"`
	TokensOutput       int      `json:"tokens_output"`
	DurationMs         int64    `json:"duration_ms"`
	StopReason         string   `json:"stop_reason"`
	ToolCallsRequested []string `json:"tool_calls_requested,omitempty"`
}

type ToolCallData struct {
	Iteration       int            `json:"iteration"`
	Tool            string         `json:"tool"`
	Arguments       map[string]any `json:"arguments,omitempty"`
	DurationMs      int64          `json:"duration_ms"`
	ResultSizeBytes int            `json:"result_size_bytes"`
	Success         bool           `json:"success"`
}

type CompactionData struct {
	Trigger             string `json:"trigger"`
	MessagesBefore      int    `json:"messages_before"`
	MessagesAfter       int    `json:"messages_after"`
	SummarizationTokens int    `json:"summarization_tokens"`
	Attempt             int    `json:"attempt"`
}

type RetryData struct {
	Attempt     int    `json:"attempt"`
	MaxAttempts int    `json:"max_attempts"`
	Backoff     string `json:"backoff"`
	WaitMs      int64  `json:"wait_ms"`
	Error       string `json:"error"`
}

// LogWriter appends log entries to a run's log.jsonl file. Thread-safe.
type LogWriter struct {
	file *os.File
	mu   sync.Mutex
}

// NewLogWriter creates a log writer for the given run directory.
// Creates log.jsonl if it doesn't exist.
func NewLogWriter(runDir string) (*LogWriter, error) {
	logPath := filepath.Join(runDir, "log.jsonl")
	file, openErr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if openErr != nil {
		return nil, fmt.Errorf("failed to open log file: %w", openErr)
	}
	return &LogWriter{file: file}, nil
}

// Log writes a typed event to the log file.
func (w *LogWriter) Log(logType LogType, step string, data any) {
	if w == nil {
		return
	}

	dataJSON, marshalErr := json.Marshal(data)
	if marshalErr != nil {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Type:      logType,
		Step:      step,
		Data:      dataJSON,
	}

	line, lineErr := json.Marshal(entry)
	if lineErr != nil {
		return
	}
	line = append(line, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()
	_, _ = w.file.Write(line)
}

// Close flushes and closes the log file.
func (w *LogWriter) Close() error {
	if w == nil {
		return nil
	}
	return w.file.Close()
}

// ReadLogEntries reads all entries from a log.jsonl file.
// Also checks for the legacy log.json filename.
func ReadLogEntries(runDir string) ([]LogEntry, error) {
	logPath := filepath.Join(runDir, "log.jsonl")
	data, readErr := os.ReadFile(logPath)
	if readErr != nil {
		// Try legacy filename.
		logPath = filepath.Join(runDir, "log.json")
		data, readErr = os.ReadFile(logPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				return nil, nil
			}
			return nil, readErr
		}
	}

	var entries []LogEntry
	decoder := json.NewDecoder(bytes.NewReader(data))
	for decoder.More() {
		var entry LogEntry
		if decErr := decoder.Decode(&entry); decErr != nil {
			continue // skip malformed lines
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
