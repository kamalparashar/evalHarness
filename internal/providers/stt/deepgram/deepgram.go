// Package deepgram implements the STT provider interface for Deepgram.
package deepgram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/kamalparashar/evalharness/internal/providers/stt"
)

const apiBase = "https://api.deepgram.com/v1/listen"

// Provider implements stt.Provider for Deepgram.
type Provider struct {
	apiKey  string
	model   string
	client  *http.Client
}

// New creates a Deepgram STT provider.
// apiKey is read from env DEEPGRAM_API_KEY if empty.
// model defaults to "nova-2" if empty.
func New(apiKey, model string) (*Provider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("DEEPGRAM_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("deepgram: DEEPGRAM_API_KEY is required")
	}
	if model == "" {
		model = "nova-2"
	}
	return &Provider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (p *Provider) Name() string    { return "deepgram" }
func (p *Provider) Version() string { return p.model }

// langMap maps our BCP-47 tags to Deepgram's language codes.
var langMap = map[string]string{
	"hi-IN": "hi",
	"kn-IN": "kn",
	"hi":    "hi",
	"kn":    "kn",
	// Hinglish: Deepgram doesn't have a dedicated code; use "hi" with detect_language
}

func (p *Provider) Transcribe(ctx context.Context, audioPath string, language string) (*stt.Result, error) {
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("deepgram: read audio: %w", err)
	}

	dgLang := langMap[language]
	if dgLang == "" {
		dgLang = "hi" // fallback
	}

	url := fmt.Sprintf("%s?model=%s&language=%s&punctuate=false&smart_format=false",
		apiBase, p.model, dgLang)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(audioData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+p.apiKey)
	req.Header.Set("Content-Type", "audio/wav")

	start := time.Now()
	resp, err := p.client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("deepgram: request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("deepgram: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deepgram: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var dgResp deepgramResponse
	if err := json.Unmarshal(body, &dgResp); err != nil {
		return nil, fmt.Errorf("deepgram: decode: %w", err)
	}

	hypothesis := ""
	if len(dgResp.Results.Channels) > 0 && len(dgResp.Results.Channels[0].Alternatives) > 0 {
		hypothesis = dgResp.Results.Channels[0].Alternatives[0].Transcript
	}

	// Estimate cost: Deepgram Nova-2 is roughly $0.0043/min
	durationSec := dgResp.Metadata.Duration
	costUSD := (durationSec / 60.0) * 0.0043

	return &stt.Result{
		Hypothesis: hypothesis,
		LatencyMs:  latency,
		CostUSD:    costUSD,
	}, nil
}

// --- Deepgram response types ---

type deepgramResponse struct {
	Metadata struct {
		Duration float64 `json:"duration"`
	} `json:"metadata"`
	Results struct {
		Channels []struct {
			Alternatives []struct {
				Transcript string  `json:"transcript"`
				Confidence float64 `json:"confidence"`
			} `json:"alternatives"`
		} `json:"channels"`
	} `json:"results"`
}
