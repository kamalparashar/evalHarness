// Package google implements the STT provider interface for Google Cloud Speech-to-Text.
package google

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/kamalparashar/evalharness/internal/providers/stt"
)

const apiURL = "https://speech.googleapis.com/v1/speech:recognize"

// Provider implements stt.Provider for Google Cloud STT.
type Provider struct {
	apiKey string
	model  string
	client *http.Client
}

// New creates a Google Cloud STT provider.
// apiKey is read from env GOOGLE_STT_API_KEY if empty.
// model defaults to "latest_long" if empty.
func New(apiKey, model string) (*Provider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_STT_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("google-stt: GOOGLE_STT_API_KEY is required")
	}
	if model == "" {
		model = "latest_long"
	}
	return &Provider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (p *Provider) Name() string    { return "google-cloud-stt" }
func (p *Provider) Version() string { return p.model }

// langMap maps BCP-47 tags to Google's language codes.
var langMap = map[string]string{
	"hi-IN": "hi-IN",
	"kn-IN": "kn-IN",
	"hi":    "hi-IN",
	"kn":    "kn-IN",
}

func (p *Provider) Transcribe(ctx context.Context, audioPath string, language string) (*stt.Result, error) {
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("google-stt: read audio: %w", err)
	}

	gcLang := langMap[language]
	if gcLang == "" {
		gcLang = "hi-IN"
	}

	// Build the synchronous recognition request
	reqBody := recognizeRequest{
		Config: recognitionConfig{
			Encoding:        "LINEAR16",
			LanguageCode:    gcLang,
			Model:           p.model,
			EnableAutoPunct: false,
		},
		Audio: recognitionAudio{
			Content: base64.StdEncoding.EncodeToString(audioData),
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("google-stt: marshal: %w", err)
	}

	url := fmt.Sprintf("%s?key=%s", apiURL, p.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := p.client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("google-stt: request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("google-stt: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google-stt: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var gcResp recognizeResponse
	if err := json.Unmarshal(body, &gcResp); err != nil {
		return nil, fmt.Errorf("google-stt: decode: %w", err)
	}

	hypothesis := ""
	if len(gcResp.Results) > 0 && len(gcResp.Results[0].Alternatives) > 0 {
		hypothesis = gcResp.Results[0].Alternatives[0].Transcript
	}

	// Google Cloud STT pricing: ~$0.006/15s = $0.024/min for standard model
	durationMin := float64(len(audioData)) / (16000.0 * 2.0 * 60.0) // rough estimate from LINEAR16 at 16kHz
	costUSD := durationMin * 0.024

	return &stt.Result{
		Hypothesis: hypothesis,
		LatencyMs:  latency,
		CostUSD:    costUSD,
	}, nil
}

// --- Google Cloud STT request/response types ---

type recognizeRequest struct {
	Config recognitionConfig `json:"config"`
	Audio  recognitionAudio  `json:"audio"`
}

type recognitionConfig struct {
	Encoding        string `json:"encoding"`
	LanguageCode    string `json:"languageCode"`
	Model           string `json:"model"`
	EnableAutoPunct bool   `json:"enableAutomaticPunctuation"`
}

type recognitionAudio struct {
	Content string `json:"content"`
}

type recognizeResponse struct {
	Results []struct {
		Alternatives []struct {
			Transcript string  `json:"transcript"`
			Confidence float64 `json:"confidence"`
		} `json:"alternatives"`
	} `json:"results"`
}
