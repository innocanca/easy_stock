package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Store struct {
	BaseDir string
}

func NewStore() *Store {
	dir := strings.TrimSpace(os.Getenv("REPORT_DATA_DIR"))
	if dir == "" {
		dir = "data/reports"
	}
	return &Store{BaseDir: dir}
}

func (s *Store) stockDir(stockCode string) string {
	return filepath.Join(s.BaseDir, sanitize(stockCode))
}

func (s *Store) uploadsDir(stockCode string) string {
	return filepath.Join(s.BaseDir, sanitize(stockCode), "uploads")
}

func (s *Store) EnsureDirs(stockCode string) error {
	if err := os.MkdirAll(s.stockDir(stockCode), 0o755); err != nil {
		return err
	}
	return os.MkdirAll(s.uploadsDir(stockCode), 0o755)
}

func (s *Store) PDFPath(stockCode string, year int) string {
	return filepath.Join(s.uploadsDir(stockCode), fmt.Sprintf("%d.pdf", year))
}

func (s *Store) reportPath(stockCode string, year int) string {
	return filepath.Join(s.stockDir(stockCode), fmt.Sprintf("%d.json", year))
}

func (s *Store) analysisPath(stockCode string) string {
	return filepath.Join(s.stockDir(stockCode), "analysis.json")
}

func (s *Store) SaveReport(data *ReportData) error {
	if err := s.EnsureDirs(data.StockCode); err != nil {
		return err
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.reportPath(data.StockCode, data.Year), b, 0o644)
}

func (s *Store) LoadReport(stockCode string, year int) (*ReportData, error) {
	b, err := os.ReadFile(s.reportPath(stockCode, year))
	if err != nil {
		return nil, err
	}
	var data ReportData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (s *Store) ListReports(stockCode string) ([]ReportData, error) {
	dir := s.stockDir(stockCode)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var reports []ReportData
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || e.Name() == "analysis.json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var r ReportData
		if json.Unmarshal(b, &r) == nil {
			reports = append(reports, r)
		}
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].Year < reports[j].Year })
	return reports, nil
}

func (s *Store) SaveAnalysis(result *AnalysisResult) error {
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.analysisPath(result.StockCode), b, 0o644)
}

func (s *Store) LoadAnalysis(stockCode string) (*AnalysisResult, error) {
	b, err := os.ReadFile(s.analysisPath(stockCode))
	if err != nil {
		return nil, err
	}
	var result AnalysisResult
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Store) DeleteReport(stockCode string, year int) error {
	os.Remove(s.reportPath(stockCode, year))
	os.Remove(s.PDFPath(stockCode, year))
	os.Remove(s.wikiPath(stockCode, year))
	return nil
}

func (s *Store) wikiPath(stockCode string, year int) string {
	return filepath.Join(s.stockDir(stockCode), fmt.Sprintf("%d_wiki.md", year))
}

func (s *Store) SaveWiki(stockCode string, year int, markdown string) error {
	if err := s.EnsureDirs(stockCode); err != nil {
		return err
	}
	return os.WriteFile(s.wikiPath(stockCode, year), []byte(markdown), 0o644)
}

func (s *Store) LoadWiki(stockCode string, year int) (string, error) {
	b, err := os.ReadFile(s.wikiPath(stockCode, year))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Store) ListWikis(stockCode string) (map[int]string, error) {
	dir := s.stockDir(stockCode)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	wikis := make(map[int]string)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, "_wiki.md") {
			continue
		}
		yearStr := strings.TrimSuffix(name, "_wiki.md")
		var year int
		if _, err := fmt.Sscanf(yearStr, "%d", &year); err != nil {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		wikis[year] = string(b)
	}
	return wikis, nil
}

func (s *Store) MergeWikis(stockCode string) (string, error) {
	return s.mergeWikis(stockCode, 0)
}

// MergeWikisExceptYear merges all year wikis except excludeYear (0 = include all).
// Used when re-uploading a year so the new wiki is not compared to its own previous file.
func (s *Store) MergeWikisExceptYear(stockCode string, excludeYear int) (string, error) {
	return s.mergeWikis(stockCode, excludeYear)
}

func (s *Store) mergeWikis(stockCode string, excludeYear int) (string, error) {
	wikis, err := s.ListWikis(stockCode)
	if err != nil {
		return "", err
	}
	if len(wikis) == 0 {
		return "", nil
	}
	years := make([]int, 0, len(wikis))
	for y := range wikis {
		if excludeYear != 0 && y == excludeYear {
			continue
		}
		years = append(years, y)
	}
	if len(years) == 0 {
		return "", nil
	}
	sort.Ints(years)
	var buf strings.Builder
	for i, y := range years {
		if i > 0 {
			buf.WriteString("\n\n---\n\n")
		}
		buf.WriteString(wikis[y])
	}
	return buf.String(), nil
}

func (s *Store) ListStocksWithWiki() ([]string, error) {
	entries, err := os.ReadDir(s.BaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var codes []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub, err := os.ReadDir(filepath.Join(s.BaseDir, e.Name()))
		if err != nil {
			continue
		}
		for _, f := range sub {
			if strings.HasSuffix(f.Name(), "_wiki.md") {
				codes = append(codes, e.Name())
				break
			}
		}
	}
	return codes, nil
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "..", "_")
	return s
}
