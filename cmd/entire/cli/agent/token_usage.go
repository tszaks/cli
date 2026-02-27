package agent

import (
	"context"

	"github.com/entireio/cli/cmd/entire/cli/logging"
)

// CalculateTokenUsage calculates token usage from transcript data.
// Returns nil if the agent doesn't support token calculation or on error.
// Errors are silently ignored because this runs in agent hook paths where
// stderr is unreliable, and callers treat nil token usage as "no data available".
func CalculateTokenUsage(ctx context.Context, ag Agent, transcriptData []byte, transcriptLinesAtStart int, subagentsDir string) *TokenUsage {
	if ag == nil {
		return nil
	}
	// Calculate token usage - prefer SubagentAwareExtractor to include subagent tokens
	if subagentExtractor, ok := ag.(SubagentAwareExtractor); ok {
		usage, err := subagentExtractor.CalculateTotalTokenUsage(transcriptData, transcriptLinesAtStart, subagentsDir)
		if err != nil {
			logging.Debug(ctx, "failed subagent aware token extraction", "err", err)
			return nil
		}
		return usage
	} else if calculator, ok := ag.(TokenCalculator); ok {
		// Fall back to basic token calculation (main transcript only)
		usage, err := calculator.CalculateTokenUsage(transcriptData, transcriptLinesAtStart)
		if err != nil {
			logging.Debug(ctx, "failed token extraction", "err", err)
			return nil
		}
		return usage
	}
	return nil
}
