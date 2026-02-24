package strategy

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/entireio/cli/cmd/entire/cli/agent"
)

func TestCalculateTokenUsage_CursorReturnsNil(t *testing.T) {
	t.Parallel()

	// Cursor transcripts don't contain token usage data, so calculateTokenUsage
	// should return nil (not an empty struct) to signal "no data available".
	transcript := []byte(`{"role":"user","message":{"content":[{"type":"text","text":"hello"}]}}`)

	result := calculateTokenUsage(agent.AgentTypeCursor, transcript, 0)
	if result != nil {
		t.Errorf("calculateTokenUsage(Cursor) = %+v, want nil", result)
	}
}

func TestCalculateTokenUsage_EmptyData(t *testing.T) {
	t.Parallel()

	result := calculateTokenUsage(agent.AgentTypeClaudeCode, nil, 0)
	if result == nil {
		t.Fatal("calculateTokenUsage(empty) = nil, want non-nil empty struct")
	}
	if result.InputTokens != 0 || result.OutputTokens != 0 {
		t.Errorf("expected zero tokens for empty data, got %+v", result)
	}
}

func TestCalculateTokenUsage_ClaudeCodeBasic(t *testing.T) {
	t.Parallel()

	// Claude Code JSONL: "usage" with "id" lives inside the "message" JSON object
	lines := []string{
		`{"type":"human","uuid":"u1","message":{"content":"hello"}}`,
		`{"type":"assistant","uuid":"u2","message":{"id":"msg_001","usage":{"input_tokens":10,"output_tokens":5}}}`,
	}
	data := []byte(strings.Join(lines, "\n") + "\n")

	result := calculateTokenUsage(agent.AgentTypeClaudeCode, data, 0)
	if result == nil {
		t.Fatal("calculateTokenUsage(ClaudeCode) = nil, want non-nil")
	}
	if result.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", result.OutputTokens)
	}
	if result.APICallCount != 1 {
		t.Errorf("APICallCount = %d, want 1", result.APICallCount)
	}
}

func TestCalculateTokenUsage_ClaudeCodeWithOffset(t *testing.T) {
	t.Parallel()

	// 4-line transcript; start at offset 2 to only count the second pair
	lines := []string{
		`{"type":"human","uuid":"u1","message":{"content":"first"}}`,
		`{"type":"assistant","uuid":"u2","message":{"id":"msg_001","usage":{"input_tokens":10,"output_tokens":5}}}`,
		`{"type":"human","uuid":"u3","message":{"content":"second"}}`,
		`{"type":"assistant","uuid":"u4","message":{"id":"msg_002","usage":{"input_tokens":20,"output_tokens":15}}}`,
	}
	data := []byte(strings.Join(lines, "\n") + "\n")

	full := calculateTokenUsage(agent.AgentTypeClaudeCode, data, 0)
	sliced := calculateTokenUsage(agent.AgentTypeClaudeCode, data, 2)

	if full == nil || sliced == nil {
		t.Fatal("expected non-nil results")
	}
	if full.OutputTokens != 20 {
		t.Errorf("full OutputTokens = %d, want 20", full.OutputTokens)
	}
	if sliced.OutputTokens != 15 {
		t.Errorf("sliced OutputTokens = %d, want 15", sliced.OutputTokens)
	}
}

func TestGenerateContextFromPrompts_CJKTruncation(t *testing.T) {
	t.Parallel()

	// 600 CJK characters exceeds the 500-rune truncation limit.
	prompt := strings.Repeat("あ", 600)

	result := generateContextFromPrompts([]string{prompt})

	if !utf8.Valid(result) {
		t.Error("generateContextFromPrompts produced invalid UTF-8 when truncating a CJK prompt")
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "...") {
		t.Error("expected truncated CJK prompt to contain '...' suffix")
	}
	// Should not contain more than 500 CJK characters
	if strings.Contains(resultStr, strings.Repeat("あ", 501)) {
		t.Error("CJK prompt was not truncated")
	}
}

func TestGenerateContextFromPrompts_EmojiTruncation(t *testing.T) {
	t.Parallel()

	// 600 emoji exceeds the 500-rune truncation limit.
	prompt := strings.Repeat("🎉", 600)

	result := generateContextFromPrompts([]string{prompt})

	if !utf8.Valid(result) {
		t.Error("generateContextFromPrompts produced invalid UTF-8 when truncating an emoji prompt")
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "...") {
		t.Error("expected truncated emoji prompt to contain '...' suffix")
	}
}

func TestGenerateContextFromPrompts_ASCIITruncation(t *testing.T) {
	t.Parallel()

	// Pure ASCII: should truncate at 500 runes with "..." suffix.
	prompt := strings.Repeat("a", 600)

	result := generateContextFromPrompts([]string{prompt})

	if !utf8.Valid(result) {
		t.Error("generateContextFromPrompts produced invalid UTF-8 when truncating an ASCII prompt")
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "...") {
		t.Error("expected truncated prompt to contain '...' suffix")
	}

	if strings.Contains(resultStr, strings.Repeat("a", 501)) {
		t.Error("prompt was not truncated")
	}
}

func TestGenerateContextFromPrompts_ShortCJKNotTruncated(t *testing.T) {
	t.Parallel()

	// 200 CJK characters is under the 500-rune limit, should not be truncated.
	prompt := strings.Repeat("あ", 200)

	result := generateContextFromPrompts([]string{prompt})

	if !utf8.Valid(result) {
		t.Error("generateContextFromPrompts produced invalid UTF-8")
	}

	resultStr := string(result)
	if strings.Contains(resultStr, "...") {
		t.Error("short CJK prompt should not be truncated")
	}
}
