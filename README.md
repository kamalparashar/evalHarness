# Indic Voice Benchmark + Open Eval Harness

An open-source Go harness that benchmarks speech-to-text providers on Indic audio under telephony conditions and evaluates conversational voice agents using simulated callers and a calibrated LLM judge.

## Quick Start

```bash
# Prerequisites: Go 1.21+, ffmpeg, SQLite
# Set your API key
export DEEPGRAM_API_KEY=your_key_here

# Load a manifest of utterances
./evalctl load data/hindi_vistaar.jsonl

# Run STT on all utterances
./evalctl run --provider deepgram --lang hi-IN

# Compute WER/CER scores
./evalctl score
```

## Architecture

```
cmd/evalctl/              CLI: load, run, score, report
internal/audio/           ffmpeg degrade pipeline (8 kHz, noise)
internal/manifest/        JSONL manifest loader
internal/providers/stt/   STT provider adapters (Deepgram, Google, Azure, Sarvam)
internal/scoring/         WER, CER, normalization (Indic-aware)
internal/store/           SQLite schema + queries
scenarios/                YAML: conversation scenarios
data/                     Manifests only (audio is .gitignored)
web/                      Report generator
```

## Supported Languages

| Language | Code | Status |
|---|---|---|
| Hindi | `hi-IN` | ✅ v1 |
| Kannada | `kn-IN` | ✅ v1 |
| Hinglish | `hi-IN` | ✅ v1 |

## STT Providers

| Provider | Status |
|---|---|
| Deepgram (Nova-2) | ✅ Implemented |
| Google Cloud STT | 🔜 M2 |
| Azure Speech Services | 🔜 M2 |
| Sarvam AI | 🔜 M2 |

## Audio Conditions

| Condition | Description |
|---|---|
| `clean` | Original audio, no processing |
| `8khz` | Resampled to 8 kHz with G.711 codec (telephony) |
| `8khz_noise` | 8 kHz + ambient noise at fixed SNR |

## Metrics

- **WER** (Word Error Rate): Primary metric. Computed after Indic-aware normalization.
- **CER** (Character Error Rate): Fairer for agglutinative Indic scripts.
- **Latency**: p50 and p95 round-trip time.
- **Cost**: Per audio minute, per provider.

### Normalization

Text normalization is critical for fair comparison:
1. Unicode NFC normalization
2. Lowercasing
3. Punctuation removal
4. Devanagari/Kannada numeral → ASCII standardization
5. Whitespace collapse

For Hinglish: both Devanagari and Latin scripts are kept; cross-script fuzzy matching is applied at the word level.

## Data Sources

This project uses publicly available, properly licensed datasets:

- **[Vistaar](https://github.com/AI4Bharat/vistaar)** (AI4Bharat, MIT) — Primary benchmark bundle across 12 Indian languages
- **[Kathbath](https://github.com/AI4Bharat/kathbath)** — ~1,684 hours of labelled speech in 12 languages

Audio files are **never committed** to the repository. Only manifests (JSONL pointers) are tracked.

**Known limitation:** These datasets are mostly read speech, while real telephony calls are spontaneous. We mitigate this with the degrade pipeline and report conditions separately.

## Building

```bash
go build -o evalctl ./cmd/evalctl/
go test ./...
```

## License

MIT — see [LICENSE](LICENSE).

## Citation

If you use this benchmark, please cite the upstream datasets:

```bibtex
@inproceedings{vistaar2023,
  title={Vistaar: Diverse Benchmarks and Training Sets for Indian Language ASR},
  author={Jain, Kaushal Kumar and others},
  year={2023}
}
```
