package strategy

import (
	"strings"
	"time"

	"entire.io/cli/cmd/entire/cli/checkpoint"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// CalculateAttribution computes line-level attribution for the commit by comparing:
// - baseTree: state before the session (parent commit)
// - checkpointTree: what the agent wrote (shadow branch)
// - committedTree: what was actually committed (HEAD)
//
// This measures how much of the commit's diff came from the agent vs human edits.
// Only counts lines that actually changed in the commit, not total file sizes.
//
// Returns nil if filesTouched is empty.
func CalculateAttribution(
	baseTree *object.Tree,
	checkpointTree *object.Tree,
	committedTree *object.Tree,
	filesTouched []string,
) *checkpoint.InitialAttribution {
	if len(filesTouched) == 0 {
		return nil
	}

	var totalAgentAdded, totalHumanAdded, totalHumanModified, totalHumanRemoved, totalCommitAdded int

	for _, filePath := range filesTouched {
		baseContent := getFileContent(baseTree, filePath)
		checkpointContent := getFileContent(checkpointTree, filePath)
		committedContent := getFileContent(committedTree, filePath)

		// Skip if nothing changed in the commit for this file
		if baseContent == committedContent {
			continue
		}

		// Lines added in this commit (base → committed)
		_, commitAdded, commitRemoved := diffLines(baseContent, committedContent)

		// Lines human changed from agent's work (checkpoint → committed)
		_, humanAdded, humanRemoved := diffLines(checkpointContent, committedContent)

		// Agent's contribution = lines added in commit that came from checkpoint (not human)
		// If checkpoint == committed, all commit additions came from agent
		// If human added lines, subtract those from the total
		agentAdded := commitAdded - humanAdded
		if agentAdded < 0 {
			agentAdded = 0
		}

		// Estimate modified lines (human changed existing agent lines)
		humanModified := min(humanAdded, humanRemoved)
		pureHumanAdded := humanAdded - humanModified
		pureHumanRemoved := humanRemoved - humanModified

		// For removed lines in commit: if agent removed them (not in checkpoint), don't count as human
		// Only count as human removed if agent kept them but human removed
		agentRemovedFromBase := countLinesStr(baseContent) - countLinesStr(checkpointContent)
		if agentRemovedFromBase < 0 {
			agentRemovedFromBase = 0
		}
		actualHumanRemoved := commitRemoved - agentRemovedFromBase
		if actualHumanRemoved < 0 {
			actualHumanRemoved = 0
		}
		// But cap it at what we detected from checkpoint→committed diff
		if actualHumanRemoved > pureHumanRemoved {
			actualHumanRemoved = pureHumanRemoved
		}

		totalAgentAdded += agentAdded
		totalHumanAdded += pureHumanAdded
		totalHumanModified += humanModified
		totalHumanRemoved += actualHumanRemoved
		totalCommitAdded += commitAdded
	}

	// Total lines in commit = lines added (what we're attributing)
	totalInCommit := totalCommitAdded
	if totalInCommit == 0 {
		// If only deletions, use agent lines as the metric
		totalInCommit = totalAgentAdded
	}

	// Calculate percentage (avoid division by zero)
	var agentPercentage float64
	if totalInCommit > 0 {
		agentPercentage = float64(totalAgentAdded) / float64(totalInCommit) * 100
	}

	return &checkpoint.InitialAttribution{
		CalculatedAt:    time.Now(),
		AgentLines:      totalAgentAdded,
		HumanAdded:      totalHumanAdded,
		HumanModified:   totalHumanModified,
		HumanRemoved:    totalHumanRemoved,
		TotalCommitted:  totalInCommit,
		AgentPercentage: agentPercentage,
	}
}

// getFileContent retrieves the content of a file from a tree.
// Returns empty string if the file doesn't exist or can't be read.
func getFileContent(tree *object.Tree, path string) string {
	if tree == nil {
		return ""
	}

	file, err := tree.File(path)
	if err != nil {
		return ""
	}

	content, err := file.Contents()
	if err != nil {
		return ""
	}

	// Skip binary files (contain null bytes)
	if strings.Contains(content, "\x00") {
		return ""
	}

	return content
}

// diffLines compares two strings and returns line-level diff stats.
// Returns (unchanged, added, removed) line counts.
func diffLines(checkpointContent, committedContent string) (unchanged, added, removed int) {
	// Handle edge cases
	if checkpointContent == committedContent {
		return countLinesStr(committedContent), 0, 0
	}
	if checkpointContent == "" {
		return 0, countLinesStr(committedContent), 0
	}
	if committedContent == "" {
		return 0, 0, countLinesStr(checkpointContent)
	}

	dmp := diffmatchpatch.New()

	// Convert to line-based diff using DiffLinesToChars/DiffCharsToLines pattern
	text1, text2, lineArray := dmp.DiffLinesToChars(checkpointContent, committedContent)
	diffs := dmp.DiffMain(text1, text2, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	for _, d := range diffs {
		lines := countLinesInText(d.Text)
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			unchanged += lines
		case diffmatchpatch.DiffInsert:
			added += lines
		case diffmatchpatch.DiffDelete:
			removed += lines
		}
	}

	return unchanged, added, removed
}

// countLinesStr returns the number of lines in content string.
// An empty string has 0 lines. A string without newlines has 1 line.
func countLinesStr(content string) int {
	if content == "" {
		return 0
	}
	lines := strings.Count(content, "\n")
	// If content doesn't end with newline, add 1 for the last line
	if !strings.HasSuffix(content, "\n") {
		lines++
	}
	return lines
}

// countLinesInText counts lines in a diff text segment.
// Similar to countLines but handles the diff output format.
func countLinesInText(text string) int {
	if text == "" {
		return 0
	}
	// Count newlines as line separators
	lines := strings.Count(text, "\n")
	// If text doesn't end with newline and is not empty, count the last line
	if !strings.HasSuffix(text, "\n") && len(text) > 0 {
		lines++
	}
	return lines
}
