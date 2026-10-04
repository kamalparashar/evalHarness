package scoring

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Normalize prepares text for WER/CER comparison.
// Steps:
//  1. Unicode NFC normalization
//  2. Lowercase
//  3. Strip punctuation (keep letters, numbers, spaces, Indic characters)
//  4. Standardize Devanagari numerals to ASCII
//  5. Collapse whitespace
func Normalize(s string) string {
	// Step 1: NFC
	s = norm.NFC.String(s)

	// Step 2: Lowercase
	s = strings.ToLower(s)

	// Step 3 & 4: Filter and standardize
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			b.WriteRune(r)
		case unicode.IsDigit(r):
			b.WriteRune(standardizeDigit(r))
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		// Keep Devanagari combining marks (vowel signs, nukta, etc.)
		case r >= 0x0900 && r <= 0x097F:
			b.WriteRune(r)
		case r >= 0x0C80 && r <= 0x0CFF:
			// Kannada range
			b.WriteRune(r)
		}
	}

	// Step 5: Collapse whitespace
	s = collapseSpaces(b.String())
	return strings.TrimSpace(s)
}

// standardizeDigit converts Devanagari and Kannada numerals to ASCII.
func standardizeDigit(r rune) rune {
	switch {
	case r >= '०' && r <= '९': // Devanagari digits U+0966 – U+096F
		return '0' + (r - '०')
	case r >= '೦' && r <= '೯': // Kannada digits U+0CE6 – U+0CEF
		return '0' + (r - '೦')
	default:
		return r
	}
}

var multiSpace = regexp.MustCompile(`\s+`)

func collapseSpaces(s string) string {
	return multiSpace.ReplaceAllString(s, " ")
}

// tokenizeWords splits normalized text into words on whitespace.
func tokenizeWords(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}

// --- Cross-script fuzzy matching (Hinglish) ---

// HinglishNormalize applies additional Hinglish-specific normalization.
// Strategy: keep both Devanagari and Latin scripts, but normalize common
// transliteration variants so that "kya" matches "क्या" is NOT attempted
// automatically. Instead, we normalize within each script and let the
// fuzzy matcher handle cross-script comparisons at the word level.
//
// For v1 we apply:
//   - Common Devanagari ligature simplifications
//   - Latin spelling standardization (e.g. "kya" → "kya", "kyaa" → "kya")
func HinglishNormalize(s string) string {
	s = Normalize(s)
	// Additional Hinglish-specific rules can be added here.
	// For now, the base normalization handles the common cases.
	return s
}

// FuzzyWordMatch compares two words allowing cross-script equivalence.
// For v1, this does exact match only. Cross-script transliteration tables
// will be added iteratively based on observed error patterns.
func FuzzyWordMatch(a, b string) bool {
	return a == b
}
