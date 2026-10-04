// Package azure implements the STT provider interface for Azure Speech Services.
package azure

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

// Provider implements stt.Provider for Azure Speech Services.
type Provider struct {
	subscriptionKey string
	region          string
	client          *http.Client
}

// New creates an Azure Speech Services STT provider.
// subscriptionKey from env AZURE_SPEECH_KEY, region from AZURE_SPEECH_REGION.
func New(subscriptionKey, region string) (*Provider, error) {
	if subscriptionKey == "" {
		subscriptionKey = os.Getenv("AZURE_SPEECH_KEY")
	}
	if region == "" {
		region = os.Getenv("AZURE_SPEECH_REGION")
	}
	if subscriptionKey == "" {
		return nil, fmt.Errorf("azure-stt: AZURE_SPEECH_KEY is required")
	}
	if region == "" {
		return nil, fmt.Errorf("azure-stt: AZURE_SPEECH_REGION is required")
	}
	return &Provider{
		subscriptionKey: subscriptionKey,
		region:          region,
		client:          &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (p *Provider) Name() string    { return "azure-speech" }
func (p *Provider) Version() string { return "v1" }

// langMap maps BCP-47 tags to Azure's locale codes.
var langMap = map[string]string{
	"hi-IN": "hi-IN",
	"kn-IN": "kn-IN",
	"hi":    "hi-IN",
	"kn":    "kn-IN",
}

func (p *Provider) Transcribe(ctx context.Context, audioPath string, language string) (*stt.Result, error) {
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("azure-stt: read audio: %w", err)
	}

	azLang := langMap[language]
	if azLang == "" {
		azLang = "hi-IN"
	}

	url := fmt.Sprintf(
		"https://%s.stt.speech.microsoft.com/speech/recognition/conversation/cognitiveservices/v1?language=%s&format=detailed",
		p.region, azLang,
	)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(audioData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", p.subscriptionKey)
	req.Header.Set("Content-Type", "audio/wav")
	req.Header.Set("Accept", "application/json")

	start := time.Now()
	resp, err := p.client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("azure-stt: request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("azure-stt: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("azure-stt: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var azResp azureResponse
	if err := json.Unmarshal(body, &azResp); err != nil {
		return nil, fmt.Errorf("azure-stt: decode: %w", err)
	}

	hypothesis := azResp.DisplayText
	if hypothesis == "" && len(azResp.NBest) > 0 {
		hypothesis = azResp.NBest[0].Display
	}

	// Azure pricing: ~$1/hr = $0.0167/min for standard tier
	durationMin := float64(azResp.Duration) / (10_000_000.0 * 60.0) // Duration is in 100ns ticks
	if durationMin <= 0 {
		durationMin = float64(len(audioData)) / (16000.0 * 2.0 * 60.0)
	}
	costUSD := durationMin * 0.0167

	return &stt.Result{
		Hypothesis: hypothesis,
		LatencyMs:  latency,
		CostUSD:    costUSD,
	}, nil
}

// --- Azure response types ---

type azureResponse struct {
	RecognitionStatus string `json:"RecognitionStatus"`
	DisplayText       string `json:"DisplayText"`
	Duration          int64  `json:"Duration"`
	Offset           int64  `json:"Offset"`
	NBest            []struct {
		Confidence float64 `json:"Confidence"`
		Lexical    string  `json:"Lexical"`
		ITN        string  `json:"ITN"`
		MaskedITN  string  `json:"MaskedITN"`
		Display    string  `json:"Display"`
	} `json:"NBest"`
}
