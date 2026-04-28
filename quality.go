package solvela

import "strings"

// DegradedReason describes why a response was considered degraded.
type DegradedReason string

const (
	DegradedEmptyContent     DegradedReason = "empty_content"
	DegradedKnownErrorPhrase DegradedReason = "known_error_phrase"
	DegradedRepetitiveLoop   DegradedReason = "repetitive_loop"
	DegradedTruncatedMidWord DegradedReason = "truncated_mid_word"
)

var knownErrorPhrases = []string{
	"i cannot",
	"as an ai",
	"i'm sorry, but i",
}

// CheckDegraded checks if response content is degraded.
// Returns empty string if OK, or a DegradedReason.
func CheckDegraded(content string) DegradedReason {
	// 1. Empty/whitespace
	if strings.TrimSpace(content) == "" {
		return DegradedEmptyContent
	}

	// 2. Known error phrases (case-insensitive)
	lower := strings.ToLower(content)
	for _, phrase := range knownErrorPhrases {
		if strings.Contains(lower, phrase) {
			return DegradedKnownErrorPhrase
		}
	}

	// 3. Repetitive 3-word phrases (any trigram appears 5+ times)
	words := strings.Fields(content)
	if len(words) >= 15 {
		counts := make(map[string]int)
		for i := 0; i <= len(words)-3; i++ {
			trigram := words[i] + " " + words[i+1] + " " + words[i+2]
			counts[trigram]++
			if counts[trigram] >= 5 {
				return DegradedRepetitiveLoop
			}
		}
	}

	// 4. Truncated mid-word (>100 chars, ends with alphanumeric)
	if len(content) > 100 {
		last := content[len(content)-1]
		if (last >= 'a' && last <= 'z') || (last >= 'A' && last <= 'Z') || (last >= '0' && last <= '9') {
			return DegradedTruncatedMidWord
		}
	}

	return ""
}
