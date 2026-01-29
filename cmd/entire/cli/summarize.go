package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"entire.io/cli/cmd/entire/cli/checkpoint"
)

// SummaryGenerator generates checkpoint summaries using an LLM.
type SummaryGenerator interface {
	// Generate creates a summary from checkpoint data.
	// Returns the generated summary or an error if generation fails.
	Generate(ctx context.Context, input SummaryInput) (*checkpoint.Summary, error)
}

// SummaryInput contains condensed checkpoint data for summarization.
type SummaryInput struct {
	// Transcript is the condensed transcript entries
	Transcript []TranscriptEntry

	// FilesTouched are the files modified during the session
	FilesTouched []string
}

// TranscriptEntryType represents the type of a transcript entry.
type TranscriptEntryType string

const (
	// EntryTypeUser indicates a user prompt entry.
	EntryTypeUser TranscriptEntryType = "user"
	// EntryTypeAssistant indicates an assistant response entry.
	EntryTypeAssistant TranscriptEntryType = "assistant"
	// EntryTypeTool indicates a tool call entry.
	EntryTypeTool TranscriptEntryType = "tool"
)

// TranscriptEntry represents one item in the condensed transcript.
type TranscriptEntry struct {
	// Type is the entry type (user, assistant, tool)
	Type TranscriptEntryType

	// Content is the text content for user/assistant entries
	Content string

	// ToolName is the name of the tool (for tool entries)
	ToolName string

	// ToolDetail is a description or file path (for tool entries)
	ToolDetail string
}

// BuildCondensedTranscript extracts a condensed view of the transcript.
// It processes user prompts, assistant responses, and tool calls into
// a simplified format suitable for LLM summarization.
func BuildCondensedTranscript(transcript []transcriptLine) []TranscriptEntry {
	var entries []TranscriptEntry

	for _, line := range transcript {
		switch line.Type {
		case transcriptTypeUser:
			if entry := extractUserEntry(line); entry != nil {
				entries = append(entries, *entry)
			}
		case transcriptTypeAssistant:
			assistantEntries := extractAssistantEntries(line)
			entries = append(entries, assistantEntries...)
		}
	}

	return entries
}

// extractUserEntry extracts a user entry from a transcript line.
// Returns nil if the line doesn't contain a valid user prompt.
func extractUserEntry(line transcriptLine) *TranscriptEntry {
	// Use shared helper for user content extraction
	content := extractUserContentFromMessage(line.Message)
	if content == "" {
		return nil
	}
	return &TranscriptEntry{
		Type:    EntryTypeUser,
		Content: content,
	}
}

// extractAssistantEntries extracts assistant and tool entries from a transcript line.
func extractAssistantEntries(line transcriptLine) []TranscriptEntry {
	var msg assistantMessage
	if err := json.Unmarshal(line.Message, &msg); err != nil {
		return nil
	}

	var entries []TranscriptEntry

	for _, block := range msg.Content {
		switch block.Type {
		case contentTypeText:
			if block.Text != "" {
				entries = append(entries, TranscriptEntry{
					Type:    EntryTypeAssistant,
					Content: block.Text,
				})
			}
		case contentTypeToolUse:
			var input toolInput
			_ = json.Unmarshal(block.Input, &input) //nolint:errcheck // Best-effort parsing

			detail := input.Description
			if detail == "" {
				detail = input.Command
			}
			if detail == "" {
				detail = input.FilePath
			}
			if detail == "" {
				detail = input.NotebookPath
			}
			if detail == "" {
				detail = input.Pattern
			}

			entries = append(entries, TranscriptEntry{
				Type:       EntryTypeTool,
				ToolName:   block.Name,
				ToolDetail: detail,
			})
		}
	}

	return entries
}

// FormatCondensedTranscript formats a SummaryInput into a human-readable string for LLM.
// The format is:
//
//	[User] user prompt here
//
//	[Assistant] assistant response here
//
//	[Tool] ToolName: description or file path
func FormatCondensedTranscript(input SummaryInput) string {
	var sb strings.Builder

	for i, entry := range input.Transcript {
		if i > 0 {
			sb.WriteString("\n")
		}

		switch entry.Type {
		case EntryTypeUser:
			sb.WriteString("[User] ")
			sb.WriteString(entry.Content)
			sb.WriteString("\n")
		case EntryTypeAssistant:
			sb.WriteString("[Assistant] ")
			sb.WriteString(entry.Content)
			sb.WriteString("\n")
		case EntryTypeTool:
			sb.WriteString("[Tool] ")
			sb.WriteString(entry.ToolName)
			if entry.ToolDetail != "" {
				sb.WriteString(": ")
				sb.WriteString(entry.ToolDetail)
			}
			sb.WriteString("\n")
		}
	}

	if len(input.FilesTouched) > 0 {
		sb.WriteString("\n[Files Modified]\n")
		for _, file := range input.FilesTouched {
			fmt.Fprintf(&sb, "- %s\n", file)
		}
	}

	return sb.String()
}
