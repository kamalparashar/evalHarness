package audio

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// generateTestTone creates a short sine wave WAV for testing.
func generateTestTone(t *testing.T, dir string) string {
	t.Helper()
	outPath := filepath.Join(dir, "test_tone.wav")
	// Generate 1 second of 440Hz sine wave at 16kHz
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1:sample_rate=16000",
		"-ac", "1", "-acodec", "pcm_s16le", "-f", "wav", outPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg not available or failed: %v\n%s", err, string(out))
	}
	return outPath
}

// generateNoise creates a short white noise WAV for testing.
func generateNoise(t *testing.T, dir string) string {
	t.Helper()
	outPath := filepath.Join(dir, "noise.wav")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "anoisesrc=d=2:c=white:r=16000:a=0.5",
		"-ac", "1", "-acodec", "pcm_s16le", "-f", "wav", outPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg not available or failed: %v\n%s", err, string(out))
	}
	return outPath
}

func TestDegrade8kHz(t *testing.T) {
	dir := t.TempDir()
	inputPath := generateTestTone(t, dir)
	outputDir := filepath.Join(dir, "degraded")

	outPath, err := DegradeSingle(inputPath, Cond8kHz, DegradeConfig{
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("DegradeSingle 8khz: %v", err)
	}

	// Verify output exists and is non-empty
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}
	t.Logf("8kHz output: %s (%d bytes)", outPath, info.Size())
}

func TestDegrade8kHzNoise(t *testing.T) {
	dir := t.TempDir()
	inputPath := generateTestTone(t, dir)
	noisePath := generateNoise(t, dir)
	outputDir := filepath.Join(dir, "degraded")

	outPath, err := DegradeSingle(inputPath, Cond8kHzNoise, DegradeConfig{
		OutputDir: outputDir,
		NoisePath: noisePath,
		SNR:       10,
	})
	if err != nil {
		t.Fatalf("DegradeSingle 8khz_noise: %v", err)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}
	t.Logf("8kHz+noise output: %s (%d bytes)", outPath, info.Size())
}

func TestDegradeCleanPassthrough(t *testing.T) {
	dir := t.TempDir()
	inputPath := generateTestTone(t, dir)

	outPath, err := DegradeSingle(inputPath, CondClean, DegradeConfig{})
	if err != nil {
		t.Fatalf("DegradeSingle clean: %v", err)
	}
	if outPath != inputPath {
		t.Errorf("clean condition should return original path, got %s", outPath)
	}
}

func TestDegradeAll(t *testing.T) {
	dir := t.TempDir()
	inputPath := generateTestTone(t, dir)
	noisePath := generateNoise(t, dir)
	outputDir := filepath.Join(dir, "degraded")

	result, err := Degrade(inputPath, DegradeConfig{
		OutputDir: outputDir,
		NoisePath: noisePath,
		SNR:       10,
	})
	if err != nil {
		t.Fatalf("Degrade: %v", err)
	}

	if result.Clean != inputPath {
		t.Errorf("Clean path mismatch")
	}
	if _, err := os.Stat(result.EightKHz); err != nil {
		t.Errorf("8kHz output missing: %v", err)
	}
	if _, err := os.Stat(result.EightKHzNoise); err != nil {
		t.Errorf("8kHz+noise output missing: %v", err)
	}
}

func TestDegradeCaching(t *testing.T) {
	dir := t.TempDir()
	inputPath := generateTestTone(t, dir)
	outputDir := filepath.Join(dir, "degraded")

	config := DegradeConfig{OutputDir: outputDir}

	// First call creates the file
	out1, err := DegradeSingle(inputPath, Cond8kHz, config)
	if err != nil {
		t.Fatal(err)
	}
	info1, _ := os.Stat(out1)

	// Second call should reuse the existing file (no-op)
	out2, err := DegradeSingle(inputPath, Cond8kHz, config)
	if err != nil {
		t.Fatal(err)
	}
	info2, _ := os.Stat(out2)

	if out1 != out2 {
		t.Errorf("expected same path, got %s and %s", out1, out2)
	}
	if info1.ModTime() != info2.ModTime() {
		t.Error("file was regenerated instead of being cached")
	}
}
