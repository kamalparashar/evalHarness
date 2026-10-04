// Package scoring provides WER, CER, and text normalization for Indic languages.
package scoring

// WER computes Word Error Rate between reference and hypothesis.
// Returns a value in [0, ∞) — 0 is perfect, >1 is possible if hypothesis is much longer.
func WER(reference, hypothesis string) float64 {
	refWords := tokenizeWords(reference)
	hypWords := tokenizeWords(hypothesis)
	if len(refWords) == 0 {
		if len(hypWords) == 0 {
			return 0
		}
		return float64(len(hypWords)) // all insertions
	}
	dist := levenshteinWords(refWords, hypWords)
	return float64(dist) / float64(len(refWords))
}

// CER computes Character Error Rate between reference and hypothesis.
// Operates on Unicode runes (not bytes).
func CER(reference, hypothesis string) float64 {
	refRunes := []rune(reference)
	hypRunes := []rune(hypothesis)
	if len(refRunes) == 0 {
		if len(hypRunes) == 0 {
			return 0
		}
		return float64(len(hypRunes))
	}
	dist := levenshteinRunes(refRunes, hypRunes)
	return float64(dist) / float64(len(refRunes))
}

// WERDetail returns the raw edit distance and reference length for aggregation.
type WERDetail struct {
	EditDistance int
	RefLength   int
	WER         float64
}

// ComputeWERDetail returns detailed WER info.
func ComputeWERDetail(reference, hypothesis string) WERDetail {
	refWords := tokenizeWords(reference)
	hypWords := tokenizeWords(hypothesis)
	dist := 0
	if len(refWords) > 0 || len(hypWords) > 0 {
		dist = levenshteinWords(refWords, hypWords)
	}
	wer := 0.0
	if len(refWords) > 0 {
		wer = float64(dist) / float64(len(refWords))
	}
	return WERDetail{
		EditDistance: dist,
		RefLength:   len(refWords),
		WER:         wer,
	}
}

// CERDetail returns the raw edit distance and reference length for aggregation.
type CERDetail struct {
	EditDistance int
	RefLength   int
	CER         float64
}

// ComputeCERDetail returns detailed CER info.
func ComputeCERDetail(reference, hypothesis string) CERDetail {
	refRunes := []rune(reference)
	hypRunes := []rune(hypothesis)
	dist := 0
	if len(refRunes) > 0 || len(hypRunes) > 0 {
		dist = levenshteinRunes(refRunes, hypRunes)
	}
	cer := 0.0
	if len(refRunes) > 0 {
		cer = float64(dist) / float64(len(refRunes))
	}
	return CERDetail{
		EditDistance: dist,
		RefLength:   len(refRunes),
		CER:         cer,
	}
}

// --- Levenshtein for word sequences ---

func levenshteinWords(a, b []string) int {
	m, n := len(a), len(b)
	// Use two rows to save memory.
	prev := make([]int, n+1)
	curr := make([]int, n+1)
	for j := 0; j <= n; j++ {
		prev[j] = j
	}
	for i := 1; i <= m; i++ {
		curr[0] = i
		for j := 1; j <= n; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(
				prev[j]+1,      // deletion
				curr[j-1]+1,    // insertion
				prev[j-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}
	return prev[n]
}

// --- Levenshtein for rune sequences ---

func levenshteinRunes(a, b []rune) int {
	m, n := len(a), len(b)
	prev := make([]int, n+1)
	curr := make([]int, n+1)
	for j := 0; j <= n; j++ {
		prev[j] = j
	}
	for i := 1; i <= m; i++ {
		curr[0] = i
		for j := 1; j <= n; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(
				prev[j]+1,
				curr[j-1]+1,
				prev[j-1]+cost,
			)
		}
		prev, curr = curr, prev
	}
	return prev[n]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
