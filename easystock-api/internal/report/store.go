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
	return nil
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "..", "_")
	return s
}
