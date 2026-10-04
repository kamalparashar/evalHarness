package scoring

import (
	"testing"
)

func TestNormalize_Basic(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello, World!", "hello world"},
		{"  multiple   spaces  ", "multiple spaces"},
		{"UPPERCASE", "uppercase"},
	}
	for _, tc := range tests {
		got := Normalize(tc.input)
		if got != tc.expected {
			t.Errorf("Normalize(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestNormalize_DevanagariNumerals(t *testing.T) {
	input := "कमरा नंबर १२३"
	got := Normalize(input)
	// Should convert Devanagari numerals to ASCII
	expected := "कमरा नंबर 123"
	if got != expected {
		t.Errorf("Normalize(%q) = %q, want %q", input, got, expected)
	}
}

func TestNormalize_KannadaNumerals(t *testing.T) {
	input := "ಸಂಖ್ಯೆ ೧೨೩"
	got := Normalize(input)
	expected := "ಸಂಖ್ಯೆ 123"
	if got != expected {
		t.Errorf("Normalize(%q) = %q, want %q", input, got, expected)
	}
}

func TestNormalize_StripPunctuation(t *testing.T) {
	input := "नमस्ते! क्या हाल है?"
	got := Normalize(input)
	// Punctuation should be removed
	expected := "नमस्ते क्या हाल है"
	if got != expected {
		t.Errorf("Normalize(%q) = %q, want %q", input, got, expected)
	}
}

func TestNormalize_NFC(t *testing.T) {
	// Precomposed vs decomposed forms should normalize identically
	// Using a basic ASCII example; Indic NFC is mostly already composed
	input1 := "café"  // precomposed é (U+00E9)
	input2 := "café" // decomposed e + combining acute (U+0301)
	n1 := Normalize(input1)
	n2 := Normalize(input2)
	if n1 != n2 {
		t.Errorf("NFC normalization failed: %q != %q", n1, n2)
	}
}

func TestTokenizeWords(t *testing.T) {
	words := tokenizeWords("  hello   world  ")
	if len(words) != 2 || words[0] != "hello" || words[1] != "world" {
		t.Errorf("tokenizeWords unexpected: %v", words)
	}
}

func TestTokenizeWords_Empty(t *testing.T) {
	words := tokenizeWords("")
	if words != nil {
		t.Errorf("expected nil for empty string, got %v", words)
	}
}
