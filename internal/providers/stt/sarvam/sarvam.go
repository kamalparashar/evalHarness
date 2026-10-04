// Package sarvam implements the STT provider interface for Sarvam AI.
package sarvam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kamalparashar/evalharness/internal/providers/stt"
)

const apiURL = "https://api.sarvam.ai/speech-to-text-translate"

// Provider implements stt.Provider for Sarvam AI.
type Provider struct {
	apiKey string
	model  string
	client *http.Client
}

// New creates a Sarvam AI STT provider.
// apiKey from env SARVAM_API_KEY.
// model defaults to "saarika:v2" if empty.
func New(apiKey, model string) (*Provider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("SARVAM_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("sarvam: SARVAM_API_KEY is required")
	}
	if model == "" {
		model = "saarika:v2"
	}
	return &Provider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (p *Provider) Name() string    { return "sarvam" }
func (p *Provider) Version() string { return p.model }

// langMap maps BCP-47 tags to Sarvam's language codes.
var langMap = map[string]string{
	"hi-IN": "hi-IN",
	"kn-IN": "kn-IN",
	"hi":    "hi-IN",
	"kn":    "kn-IN",
}

func (p *Provider) Transcribe(ctx context.Context, audioPath string, language string) (*stt.Result, error) {
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("sarvam: read audio: %w", err)
	}

	svLang := langMap[language]
	if svLang == "" {
		svLang = "hi-IN"
	}

	// Sarvam uses multipart form upload
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add the audio file
	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, fmt.Errorf("sarvam: create form file: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return nil, fmt.Errorf("sarvam: write audio: %w", err)
	}

	// Add language code
	if err := writer.WriteField("language_code", svLang); err != nil {
		return nil, fmt.Errorf("sarvam: write field: %w", err)
	}

	// Add model
	if err := writer.WriteField("model", p.model); err != nil {
		return nil, fmt.Errorf("sarvam: write field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("sarvam: close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("API-Subscription-Key", p.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	start := time.Now()
	resp, err := p.client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("sarvam: request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("sarvam: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sarvam: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var svResp sarvamResponse
	if err := json.Unmarshal(body, &svResp); err != nil {
		return nil, fmt.Errorf("sarvam: decode: %w", err)
	}

	// Sarvam pricing varies; estimate ~$0.01/min
	durationMin := float64(len(audioData)) / (16000.0 * 2.0 * 60.0)
	costUSD := durationMin * 0.01

	return &stt.Result{
		Hypothesis: svResp.Transcript,
		LatencyMs:  latency,
		CostUSD:    costUSD,
	}, nil
}

// --- Sarvam response types ---

type sarvamResponse struct {
	Transcript string `json:"transcript"`
	LanguageCode string `json:"language_code"`
}
