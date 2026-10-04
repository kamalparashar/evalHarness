// Package audio provides the ffmpeg-based audio degradation pipeline.
//
// It simulates telephony conditions by:
//  1. Resampling to 8 kHz mono (telephone bandwidth)
//  2. Applying G.711 µ-law codec simulation
//  3. Optionally mixing ambient noise at a specified SNR
//
// All operations use ffmpeg as a subprocess — no CGo bindings needed.
package audio

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Condition represents an audio degradation level.
type Condition string

const (
	CondClean     Condition = "clean"
	Cond8kHz      Condition = "8khz"
	Cond8kHzNoise Condition = "8khz_noise"
)

// DegradeConfig holds the parameters for audio degradation.
type DegradeConfig struct {
	// OutputDir is where degraded files are written.
	OutputDir string

	// NoisePath is the path to a noise audio file (WAV) for SNR mixing.
	// Required only for Cond8kHzNoise.
	NoisePath string

	// SNR is the signal-to-noise ratio in dB for noise mixing. Default: 10.
	SNR int
}

// DegradeResult holds the output paths for each condition.
type DegradeResult struct {
	Clean     string // original (copied or symlinked)
	EightKHz  string // resampled to 8 kHz µ-law
	EightKHzNoise string // 8 kHz + noise at SNR
}

// Degrade takes a clean audio file and produces degraded versions.
// It creates files in config.OutputDir named by a hash of the input path.
func Degrade(inputPath string, config DegradeConfig) (*DegradeResult, error) {
	if err := checkFFmpeg(); err != nil {
		return nil, err
	}

	if config.OutputDir == "" {
		return nil, fmt.Errorf("audio: OutputDir is required")
	}
	if config.SNR <= 0 {
		config.SNR = 10
	}

	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("audio: mkdir %s: %w", config.OutputDir, err)
	}

	// Generate a stable base name from input path
	baseName := hashPath(inputPath)
	ext := ".wav"

	result := &DegradeResult{}

	// Clean: just copy/verify the original exists
	result.Clean = inputPath
	if _, err := os.Stat(inputPath); err != nil {
		return nil, fmt.Errorf("audio: input file not found: %w", err)
	}

	// 8 kHz telephony: resample to 8 kHz mono, simulate G.711 µ-law
	result.EightKHz = filepath.Join(config.OutputDir, baseName+"_8khz"+ext)
	if err := degrade8kHz(inputPath, result.EightKHz); err != nil {
		return nil, fmt.Errorf("audio: 8khz degrade: %w", err)
	}

	// 8 kHz + noise: add noise at specified SNR
	if config.NoisePath != "" {
		result.EightKHzNoise = filepath.Join(config.OutputDir, baseName+"_8khz_noise"+ext)
		if err := degrade8kHzNoise(result.EightKHz, config.NoisePath, result.EightKHzNoise, config.SNR); err != nil {
			return nil, fmt.Errorf("audio: 8khz+noise degrade: %w", err)
		}
	}

	return result, nil
}

// DegradeSingle produces a single degraded file for the given condition.
func DegradeSingle(inputPath string, cond Condition, config DegradeConfig) (string, error) {
	switch cond {
	case CondClean:
		return inputPath, nil
	case Cond8kHz:
		if config.OutputDir == "" {
			return "", fmt.Errorf("audio: OutputDir is required")
		}
		if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
			return "", err
		}
		outPath := filepath.Join(config.OutputDir, hashPath(inputPath)+"_8khz.wav")
		if _, err := os.Stat(outPath); err == nil {
			return outPath, nil // already exists
		}
		return outPath, degrade8kHz(inputPath, outPath)
	case Cond8kHzNoise:
		if config.NoisePath == "" {
			return "", fmt.Errorf("audio: NoisePath is required for 8khz_noise condition")
		}
		// First get the 8kHz version
		eightKHz, err := DegradeSingle(inputPath, Cond8kHz, config)
		if err != nil {
			return "", err
		}
		outPath := filepath.Join(config.OutputDir, hashPath(inputPath)+"_8khz_noise.wav")
		if _, err := os.Stat(outPath); err == nil {
			return outPath, nil // already exists
		}
		return outPath, degrade8kHzNoise(eightKHz, config.NoisePath, outPath, config.SNR)
	default:
		return "", fmt.Errorf("audio: unknown condition %q", cond)
	}
}

// --- ffmpeg operations ---

// degrade8kHz resamples to 8 kHz mono and simulates G.711 µ-law codec.
// Pipeline: input → resample 8kHz mono → µ-law encode → decode back to PCM
// This round-trip through µ-law introduces the characteristic telephony distortion.
func degrade8kHz(inputPath, outputPath string) error {
	// Step 1: Convert to 8 kHz mono µ-law
	mulawPath := outputPath + ".mulaw.wav"
	args := []string{
		"-y", "-i", inputPath,
		"-ar", "8000",       // 8 kHz sample rate
		"-ac", "1",          // mono
		"-acodec", "pcm_mulaw", // G.711 µ-law
		"-f", "wav",
		mulawPath,
	}
	if err := runFFmpeg(args...); err != nil {
		return fmt.Errorf("µ-law encode: %w", err)
	}
	defer os.Remove(mulawPath)

	// Step 2: Decode back to 16-bit PCM (so STT providers can read it)
	args = []string{
		"-y", "-i", mulawPath,
		"-ar", "8000",
		"-ac", "1",
		"-acodec", "pcm_s16le",
		"-f", "wav",
		outputPath,
	}
	if err := runFFmpeg(args...); err != nil {
		return fmt.Errorf("PCM decode: %w", err)
	}
	return nil
}

// degrade8kHzNoise mixes 8 kHz audio with noise at the specified SNR.
// Uses ffmpeg's amix filter with volume adjustment for target SNR.
func degrade8kHzNoise(inputPath, noisePath, outputPath string, snrDB int) error {
	// SNR = 20 * log10(signal_amplitude / noise_amplitude)
	// noise_volume = 10^(-snrDB/20)
	// For SNR=10dB, noise_volume ≈ 0.316
	// For SNR=5dB,  noise_volume ≈ 0.562
	// For SNR=20dB, noise_volume ≈ 0.1
	noiseVol := 1.0
	for i := 0; i < snrDB; i++ {
		noiseVol *= 0.891 // ≈ 10^(-1/20)
	}
	noiseVolStr := fmt.Sprintf("%.4f", noiseVol)

	// Use ffmpeg complex filter:
	// 1. Take input audio as-is
	// 2. Take noise, resample to match, loop if shorter, adjust volume
	// 3. Mix them together
	filterComplex := fmt.Sprintf(
		"[1:a]aresample=8000,aformat=sample_fmts=s16:channel_layouts=mono,aloop=-1:size=8000,volume=%s[noise];"+
			"[0:a][noise]amix=inputs=2:duration=first:dropout_transition=0[out]",
		noiseVolStr,
	)

	args := []string{
		"-y",
		"-i", inputPath,
		"-i", noisePath,
		"-filter_complex", filterComplex,
		"-map", "[out]",
		"-ar", "8000",
		"-ac", "1",
		"-acodec", "pcm_s16le",
		"-f", "wav",
		outputPath,
	}
	return runFFmpeg(args...)
}

// --- helpers ---

func runFFmpeg(args ...string) error {
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = nil // suppress ffmpeg's verbose stderr
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg %s: %w\noutput: %s", strings.Join(args[:3], " "), err, string(output))
	}
	return nil
}

func checkFFmpeg() error {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("audio: ffmpeg not found in PATH — install with: sudo apt install ffmpeg")
	}
	return nil
}

func hashPath(path string) string {
	h := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", h[:8]) // 16-char hex prefix
}
