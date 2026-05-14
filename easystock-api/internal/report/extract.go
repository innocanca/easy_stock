package report

import (
	"strings"

	"github.com/ledongthuc/pdf"
)

const maxTextLen = 80000

// MaxRunesForAI caps how much report text is sent to the model to reduce
// context overflows and timeouts while keeping the head of the document
// (where key metrics usually appear).
const MaxRunesForAI = 52000

// MaxRunesForJSONExtract is slightly smaller to leave room for system prompt
// and the model's JSON response.
const MaxRunesForJSONExtract = 38000

func ClampReportText(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return strings.TrimSpace(string(r[:maxRunes]) + "\n\n（后文已省略，分析仅依据以上片段。）")
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
		if buf.Len() > maxTextLen {
			break
		}
	}
	result := buf.String()
	if len(result) > maxTextLen {
		result = result[:maxTextLen]
	}
	return result, nil
}
