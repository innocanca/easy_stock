package report

import (
	"strings"

	"github.com/ledongthuc/pdf"
)

const maxTextLen = 80000

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
