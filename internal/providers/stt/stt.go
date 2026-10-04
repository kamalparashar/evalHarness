// Package stt defines the speech-to-text provider interface and common types.
package stt

import "context"

// Result holds the output of a single transcription call.
type Result struct {
	Hypothesis string  // The transcribed text
	LatencyMs  int64   // Round-trip latency in milliseconds
	CostUSD    float64 // Estimated cost in USD
}

// Provider is the interface every STT adapter must implement.
type Provider interface {
	// Name returns the provider's display name (e.g. "google-cloud-stt").
	Name() string

	// Version returns the provider/model version string.
	Version() string

	// Transcribe sends an audio file to the provider and returns the transcript.
	// audioPath is the path to a WAV/FLAC file on disk.
	// language is a BCP-47 tag (e.g. "hi-IN", "kn-IN").
	Transcribe(ctx context.Context, audioPath string, language string) (*Result, error)
}
