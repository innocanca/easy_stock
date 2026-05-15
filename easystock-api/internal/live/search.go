package live

import (
	"encoding/json"
	"strings"

	"easystock/api/internal/tushare"
)

type SearchResult struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// SearchStocks returns stocks whose ts_code or name contains the query (case-insensitive, max 20).
func SearchStocks(c *tushare.Client, query string) ([]byte, error) {
	m, err := StockBasicMap(c)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return json.Marshal([]SearchResult{})
	}

	var results []SearchResult
	for code, row := range m {
		if len(results) >= 20 {
			break
		}
		codeLower := strings.ToLower(code)
		nameLower := strings.ToLower(row.Name)
		codeNoDot := strings.Replace(codeLower, ".", "", 1)
		if strings.Contains(codeLower, q) || strings.Contains(nameLower, q) || strings.Contains(codeNoDot, q) {
			results = append(results, SearchResult{Code: code, Name: row.Name})
		}
	}

	return json.Marshal(results)
}
