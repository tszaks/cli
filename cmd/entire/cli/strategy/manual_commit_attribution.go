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

// CalculateAttributionWithAccumulated computes final attribution using accumulated prompt data.
// This provides more accurate attribution than tree-only comparison because it captures
// user edits that happened between checkpoints (which would otherwise be mixed into the
// checkpoint snapshots).
//
// The calculation:
// 1. Sum user edits from PromptAttributions (captured at each prompt start)
// 2. Add user edits after the final checkpoint (shadow → head diff)
// 3. Calculate agent lines from base → shadow
// 4. Compute percentages
func CalculateAttributionWithAccumulated(
	baseTree *object.Tree,
	shadowTree *object.Tree,
	headTree *object.Tree,
	filesTouched []string,
	promptAttributions []PromptAttribution,
) *checkpoint.InitialAttribution {
	if len(filesTouched) == 0 {
		return nil
	}

	// Sum accumulated user lines from prompt attributions
	var accumulatedUserAdded, accumulatedUserRemoved int
	for _, pa := range promptAttributions {
		accumulatedUserAdded += pa.UserLinesAdded
		accumulatedUserRemoved += pa.UserLinesRemoved
	}

	// Calculate user edits AFTER the final checkpoint (shadow → head)
	// These are edits the user made after the last agent checkpoint
	var postCheckpointUserAdded, postCheckpointUserRemoved int
	var totalAgentAdded, totalAgentRemoved int

	for _, filePath := range filesTouched {
		baseContent := getFileContent(baseTree, filePath)
		shadowContent := getFileContent(shadowTree, filePath)
		headContent := getFileContent(headTree, filePath)

		// Agent contribution: base → shadow
		_, agentAdded, agentRemoved := diffLines(baseContent, shadowContent)
		totalAgentAdded += agentAdded
		totalAgentRemoved += agentRemoved

		// Post-checkpoint user edits: shadow → head
		_, postUserAdded, postUserRemoved := diffLines(shadowContent, headContent)
		postCheckpointUserAdded += postUserAdded
		postCheckpointUserRemoved += postUserRemoved
	}

	// Total user contribution = accumulated (between checkpoints) + post-checkpoint
	totalUserAdded := accumulatedUserAdded + postCheckpointUserAdded
	totalUserRemoved := accumulatedUserRemoved + postCheckpointUserRemoved

	// Estimate modified lines (user changed existing agent lines)
	humanModified := min(totalUserAdded, totalUserRemoved)
	pureUserAdded := totalUserAdded - humanModified
	pureUserRemoved := totalUserRemoved - humanModified

	// Total lines in commit = agent added + user added (net new lines)
	totalCommitted := totalAgentAdded + pureUserAdded
	if totalCommitted == 0 {
		totalCommitted = totalAgentAdded // Fallback for delete-only
	}

	// Calculate percentage
	var agentPercentage float64
	if totalCommitted > 0 {
		agentPercentage = float64(totalAgentAdded) / float64(totalCommitted) * 100
	}

	return &checkpoint.InitialAttribution{
		CalculatedAt:    time.Now(),
		AgentLines:      totalAgentAdded,
		HumanAdded:      pureUserAdded,
		HumanModified:   humanModified,
		HumanRemoved:    pureUserRemoved,
		TotalCommitted:  totalCommitted,
		AgentPercentage: agentPercentage,
	}
}

// CalculatePromptAttribution computes line-level attribution at the start of a prompt.
// This captures user edits since the last checkpoint BEFORE the agent makes changes.
//
// Parameters:
//   - baseTree: the tree at session start (the base commit)
//   - lastCheckpointTree: the tree from the previous checkpoint (nil if first checkpoint)
//   - worktreeFiles: map of file path → current worktree content for files that changed
//   - checkpointNumber: which checkpoint we're about to create (1-indexed)
//
// Returns the attribution data to store in session state.
func CalculatePromptAttribution(
	baseTree *object.Tree,
	lastCheckpointTree *object.Tree,
	worktreeFiles map[string]string,
	checkpointNumber int,
) PromptAttribution {
	result := PromptAttribution{
		CheckpointNumber: checkpointNumber,
	}

	if len(worktreeFiles) == 0 {
		return result
	}

	// Determine reference tree for user changes (last checkpoint or base)
	referenceTree := lastCheckpointTree
	if referenceTree == nil {
		referenceTree = baseTree
	}

	for filePath, worktreeContent := range worktreeFiles {
		referenceContent := getFileContent(referenceTree, filePath)
		baseContent := getFileContent(baseTree, filePath)

		// User changes: diff(reference, worktree)
		// These are changes since the last checkpoint that the agent didn't make
		_, userAdded, userRemoved := diffLines(referenceContent, worktreeContent)
		result.UserLinesAdded += userAdded
		result.UserLinesRemoved += userRemoved

		// Agent lines so far: diff(base, lastCheckpoint)
		// Only calculate if we have a previous checkpoint
		if lastCheckpointTree != nil {
			checkpointContent := getFileContent(lastCheckpointTree, filePath)
			_, agentAdded, agentRemoved := diffLines(baseContent, checkpointContent)
			result.AgentLinesAdded += agentAdded
			result.AgentLinesRemoved += agentRemoved
		}
	}

	return result
}
