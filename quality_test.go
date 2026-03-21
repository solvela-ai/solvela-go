package rustyclaw

import (
	"strings"
	"testing"
)

func TestCheckDegradedEmptyContent(t *testing.T) {
	reason := CheckDegraded("")
	if reason != DegradedEmptyContent {
		t.Errorf("got %q, want %q", reason, DegradedEmptyContent)
	}
}

func TestCheckDegradedWhitespaceOnly(t *testing.T) {
	reason := CheckDegraded("   \t\n  ")
	if reason != DegradedEmptyContent {
		t.Errorf("got %q, want %q", reason, DegradedEmptyContent)
	}
}

func TestCheckDegradedKnownErrorPhrase(t *testing.T) {
	reason := CheckDegraded("I cannot help with that request.")
	if reason != DegradedKnownErrorPhrase {
		t.Errorf("got %q, want %q", reason, DegradedKnownErrorPhrase)
	}
}

func TestCheckDegradedKnownErrorPhraseCaseInsensitive(t *testing.T) {
	reason := CheckDegraded("AS AN AI language model, I can help.")
	if reason != DegradedKnownErrorPhrase {
		t.Errorf("got %q, want %q", reason, DegradedKnownErrorPhrase)
	}
}

func TestCheckDegradedRepetitiveLoop(t *testing.T) {
	// Build content with a trigram repeated 5+ times
	repeated := strings.Repeat("the quick brown fox jumps ", 6)
	reason := CheckDegraded(repeated)
	if reason != DegradedRepetitiveLoop {
		t.Errorf("got %q, want %q", reason, DegradedRepetitiveLoop)
	}
}

func TestCheckDegradedTruncatedMidWord(t *testing.T) {
	// >100 chars ending with alphanumeric
	content := strings.Repeat("a", 101) + "b"
	reason := CheckDegraded(content)
	if reason != DegradedTruncatedMidWord {
		t.Errorf("got %q, want %q", reason, DegradedTruncatedMidWord)
	}
}

func TestCheckDegradedNormalContent(t *testing.T) {
	reason := CheckDegraded("This is a perfectly normal response.")
	if reason != "" {
		t.Errorf("got %q, want empty (not degraded)", reason)
	}
}

func TestCheckDegradedContentEndingWithPeriod(t *testing.T) {
	// >100 chars ending with period should NOT be truncated
	// Use diverse words to avoid triggering repetitive loop detection
	content := "The quick brown fox jumps over the lazy dog and then runs through the forest while the birds sing their beautiful songs in the morning light."
	if len(content) <= 100 {
		t.Fatalf("test content too short: %d chars", len(content))
	}
	reason := CheckDegraded(content)
	if reason != "" {
		t.Errorf("got %q, want empty (period ending is not truncated)", reason)
	}
}

func TestCheckDegradedShortContentEndingAlphanumeric(t *testing.T) {
	// <=100 chars ending with alphanumeric should NOT be truncated
	reason := CheckDegraded("Short text ending with word")
	if reason != "" {
		t.Errorf("got %q, want empty (short content not checked for truncation)", reason)
	}
}
