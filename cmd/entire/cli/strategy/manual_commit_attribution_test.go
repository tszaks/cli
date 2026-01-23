package strategy

import (
	"testing"
)

const testThreeLines = "line1\nline2\nline3\n"

func TestDiffLines_NoChanges(t *testing.T) {
	content := testThreeLines
	unchanged, added, removed := diffLines(content, content)

	if unchanged != 3 {
		t.Errorf("expected 3 unchanged lines, got %d", unchanged)
	}
	if added != 0 {
		t.Errorf("expected 0 added lines, got %d", added)
	}
	if removed != 0 {
		t.Errorf("expected 0 removed lines, got %d", removed)
	}
}

func TestDiffLines_AllAdded(t *testing.T) {
	checkpoint := ""
	committed := testThreeLines
	unchanged, added, removed := diffLines(checkpoint, committed)

	if unchanged != 0 {
		t.Errorf("expected 0 unchanged lines, got %d", unchanged)
	}
	if added != 3 {
		t.Errorf("expected 3 added lines, got %d", added)
	}
	if removed != 0 {
		t.Errorf("expected 0 removed lines, got %d", removed)
	}
}

func TestDiffLines_AllRemoved(t *testing.T) {
	checkpoint := testThreeLines
	committed := ""
	unchanged, added, removed := diffLines(checkpoint, committed)

	if unchanged != 0 {
		t.Errorf("expected 0 unchanged lines, got %d", unchanged)
	}
	if added != 0 {
		t.Errorf("expected 0 added lines, got %d", added)
	}
	if removed != 3 {
		t.Errorf("expected 3 removed lines, got %d", removed)
	}
}

func TestDiffLines_MixedChanges(t *testing.T) {
	checkpoint := testThreeLines
	committed := "line1\nmodified\nline3\nnew line\n"
	unchanged, added, removed := diffLines(checkpoint, committed)

	// line1 and line3 unchanged (2)
	// line2 removed (1)
	// modified and new line added (2)
	if unchanged != 2 {
		t.Errorf("expected 2 unchanged lines, got %d", unchanged)
	}
	if added != 2 {
		t.Errorf("expected 2 added lines, got %d", added)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed line, got %d", removed)
	}
}

func TestDiffLines_WithoutTrailingNewline(t *testing.T) {
	checkpoint := "line1\nline2"
	committed := "line1\nline2"
	unchanged, added, removed := diffLines(checkpoint, committed)

	if unchanged != 2 {
		t.Errorf("expected 2 unchanged lines, got %d", unchanged)
	}
	if added != 0 {
		t.Errorf("expected 0 added lines, got %d", added)
	}
	if removed != 0 {
		t.Errorf("expected 0 removed lines, got %d", removed)
	}
}

func TestCountLinesStr(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{"empty", "", 0},
		{"single line no newline", "hello", 1},
		{"single line with newline", "hello\n", 1},
		{"two lines", "hello\nworld\n", 2},
		{"two lines no trailing newline", "hello\nworld", 2},
		{"three lines", "a\nb\nc\n", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countLinesStr(tt.content)
			if got != tt.expected {
				t.Errorf("countLinesStr(%q) = %d, want %d", tt.content, got, tt.expected)
			}
		})
	}
}

func TestCalculateAttribution_NilTrees(t *testing.T) {
	result := CalculateAttribution(nil, nil, nil, []string{"file.txt"})

	// Should handle nil trees gracefully
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// With nil trees, all files will have empty content, so no lines changed
	if result.TotalCommitted != 0 {
		t.Errorf("expected 0 total committed, got %d", result.TotalCommitted)
	}
}

func TestCalculateAttribution_EmptyFilesTouched(t *testing.T) {
	result := CalculateAttribution(nil, nil, nil, []string{})

	// Should return nil for empty files list
	if result != nil {
		t.Errorf("expected nil result for empty filesTouched, got %+v", result)
	}
}

func TestCalculateAttribution_PercentageCalculation(t *testing.T) {
	// Test the percentage calculation manually
	// If we have 80 agent lines and 100 total lines, percentage should be 80%

	// Since we can't easily mock object.Tree, we test the math via diffLines
	checkpoint := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\n"
	committed := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nnew1\nnew2\n"

	unchanged, added, removed := diffLines(checkpoint, committed)

	if unchanged != 8 {
		t.Errorf("expected 8 unchanged, got %d", unchanged)
	}
	if added != 2 {
		t.Errorf("expected 2 added, got %d", added)
	}
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}

	// Total committed = 10, agent = 8, so percentage = 80%
	totalCommitted := countLinesStr(committed)
	if totalCommitted != 10 {
		t.Errorf("expected 10 total committed, got %d", totalCommitted)
	}

	percentage := float64(unchanged) / float64(totalCommitted) * 100
	if percentage != 80.0 {
		t.Errorf("expected 80%% agent percentage, got %.1f%%", percentage)
	}
}

func TestCalculateAttribution_ModifiedEstimation(t *testing.T) {
	// Test that we estimate modified lines correctly
	// When we have both additions and removals, min(added, removed) is "modified"

	checkpoint := "original1\noriginal2\noriginal3\n"
	committed := "modified1\nmodified2\noriginal3\nnew line\n"

	unchanged, added, removed := diffLines(checkpoint, committed)

	// original3 is unchanged (1)
	// original1, original2 removed (2)
	// modified1, modified2, new line added (3)
	if unchanged != 1 {
		t.Errorf("expected 1 unchanged, got %d", unchanged)
	}

	// Estimate: min(3, 2) = 2 modified, so:
	// humanModified = 2
	// humanAdded = 3 - 2 = 1
	// humanRemoved = 2 - 2 = 0
	humanModified := min(added, removed)
	humanAdded := added - humanModified
	humanRemoved := removed - humanModified

	if humanModified != 2 {
		t.Errorf("expected 2 modified, got %d", humanModified)
	}
	if humanAdded != 1 {
		t.Errorf("expected 1 added (after subtracting modified), got %d", humanAdded)
	}
	if humanRemoved != 0 {
		t.Errorf("expected 0 removed (after subtracting modified), got %d", humanRemoved)
	}
}
