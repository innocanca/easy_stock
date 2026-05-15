package report

import (
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

const maxExtractBytes = 600000

// MaxRunesPerChunk is the max runes sent per AI call in chunked mode.
const MaxRunesPerChunk = 60000

// ChunkOverlapRunes gives context continuity between chunks.
const ChunkOverlapRunes = 2000

const MaxRunesForJSONExtract = 38000

func ClampReportText(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return strings.TrimSpace(string(r[:maxRunes]) + "\n\n（后文已省略，分析仅依据以上片段。）")
}

// SplitIntoChunks splits long text into overlapping chunks for multi-pass analysis.
// If the text fits in one chunk it returns a single-element slice.
func SplitIntoChunks(text string, chunkRunes, overlapRunes int) []string {
	runes := []rune(strings.TrimSpace(text))
	total := len(runes)
	if total <= chunkRunes {
		return []string{string(runes)}
	}

	var chunks []string
	start := 0
	for start < total {
		end := start + chunkRunes
		if end > total {
			end = total
		}
		chunks = append(chunks, string(runes[start:end]))
		if end >= total {
			break
		}
		start = end - overlapRunes
		if start < 0 {
			start = 0
		}
	}
	return chunks
}

func ExtractPDFText(filepath string) (string, error) {
	f, r, err := pdf.Open(filepath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf strings.Builder
	n := r.NumPage()
	for i := 1; i <= n; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		buf.WriteString(text)
		buf.WriteString("\n")
		if buf.Len() > maxExtractBytes {
			break
		}
	}
	result := buf.String()
	if len(result) > maxExtractBytes {
		result = result[:maxExtractBytes]
	}
	return result, nil
}

func RuneCount(s string) int {
	return utf8.RuneCountInString(s)
}
