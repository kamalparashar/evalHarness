package scoring

import (
	"math"
	"testing"
)

func almostEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestWER_Identical(t *testing.T) {
	w := WER("hello world", "hello world")
	if w != 0 {
		t.Errorf("expected WER=0 for identical strings, got %f", w)
	}
}

func TestWER_CompletelyWrong(t *testing.T) {
	w := WER("hello world", "foo bar baz")
	// 2 substitutions + 1 insertion = 3 edits / 2 ref words = 1.5
	if !almostEqual(w, 1.5, 0.01) {
		t.Errorf("expected WER≈1.5, got %f", w)
	}
}

func TestWER_Empty(t *testing.T) {
	if WER("", "") != 0 {
		t.Error("WER of two empty strings should be 0")
	}
	if WER("", "hello") != 1 {
		t.Error("WER with empty ref and one hyp word should be 1")
	}
}

func TestWER_Hindi(t *testing.T) {
	ref := "मैं घर जा रहा हूँ"
	hyp := "मैं घर जा रहा हूं"
	// One substitution (हूँ → हूं) out of 5 words = 0.2
	w := WER(ref, hyp)
	if !almostEqual(w, 0.2, 0.01) {
		t.Errorf("expected WER≈0.2, got %f", w)
	}
}

func TestCER_Identical(t *testing.T) {
	c := CER("hello", "hello")
	if c != 0 {
		t.Errorf("expected CER=0, got %f", c)
	}
}

func TestCER_Hindi(t *testing.T) {
	ref := "नमस्ते"
	hyp := "नमस्त" // missing final 'े' (one rune)
	c := CER(ref, hyp)
	// ref has 6 runes: न म स ् त े → hyp has 5: न म स ् त
	// edit distance = 1, CER = 1/6 ≈ 0.167
	if !almostEqual(c, 1.0/6.0, 0.02) {
		t.Errorf("expected CER≈0.167, got %f", c)
	}
}

func TestCER_Kannada(t *testing.T) {
	ref := "ನಮಸ್ಕಾರ"
	hyp := "ನಮಸ್ಕಾರ"
	c := CER(ref, hyp)
	if c != 0 {
		t.Errorf("expected CER=0 for identical Kannada strings, got %f", c)
	}
}

func TestWERDetail(t *testing.T) {
	d := ComputeWERDetail("one two three", "one four three")
	if d.EditDistance != 1 {
		t.Errorf("expected edit distance 1, got %d", d.EditDistance)
	}
	if d.RefLength != 3 {
		t.Errorf("expected ref length 3, got %d", d.RefLength)
	}
}
