package live

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MarketStore manages file-based storage for daily market snapshots and AI summaries.
type MarketStore struct {
	BaseDir string
}

func NewMarketStore() *MarketStore {
	dir := strings.TrimSpace(os.Getenv("MARKET_DATA_DIR"))
	if dir == "" {
		dir = "data/market"
	}
	return &MarketStore{BaseDir: dir}
}

func (s *MarketStore) ensureDir() error {
	return os.MkdirAll(s.BaseDir, 0o755)
}

func (s *MarketStore) snapshotPath(date string) string {
	return filepath.Join(s.BaseDir, date+".json")
}

func (s *MarketStore) summaryPath(date string) string {
	return filepath.Join(s.BaseDir, date+"_summary.md")
}

func (s *MarketStore) SaveSnapshot(snap *MarketSnapshot) error {
	if err := s.ensureDir(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.snapshotPath(snap.Date), b, 0o644)
}

func (s *MarketStore) LoadSnapshot(date string) (*MarketSnapshot, error) {
	b, err := os.ReadFile(s.snapshotPath(date))
	if err != nil {
		return nil, err
	}
	var snap MarketSnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func (s *MarketStore) SaveSummary(date, markdown string) error {
	if err := s.ensureDir(); err != nil {
		return err
	}
	return os.WriteFile(s.summaryPath(date), []byte(markdown), 0o644)
}

func (s *MarketStore) LoadSummary(date string) (string, error) {
	b, err := os.ReadFile(s.summaryPath(date))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *MarketStore) HasSnapshot(date string) bool {
	_, err := os.Stat(s.snapshotPath(date))
	return err == nil
}

func (s *MarketStore) HasSummary(date string) bool {
	_, err := os.Stat(s.summaryPath(date))
	return err == nil
}

// MarketDailySummaryItem is returned by ListSnapshots for the history API.
type MarketDailySummaryItem struct {
	Date         string       `json:"date"`
	Indices      []IndexPoint `json:"indices"`
	TotalAmount  float64      `json:"totalAmount"`
	AmountChgPct float64      `json:"amountChgPct"`
	HasSummary   bool         `json:"hasSummary"`
}

// ListSnapshots returns the most recent N snapshots in reverse chronological order.
func (s *MarketStore) ListSnapshots(limit int) ([]MarketDailySummaryItem, error) {
	entries, err := os.ReadDir(s.BaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var dates []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".json") && len(name) == 13 { // YYYYMMDD.json
			dates = append(dates, strings.TrimSuffix(name, ".json"))
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	if limit > 0 && len(dates) > limit {
		dates = dates[:limit]
	}

	var result []MarketDailySummaryItem
	for _, d := range dates {
		snap, err := s.LoadSnapshot(d)
		if err != nil {
			continue
		}
		result = append(result, MarketDailySummaryItem{
			Date:         snap.Date,
			Indices:      snap.Indices,
			TotalAmount:  snap.TotalAmount,
			AmountChgPct: snap.AmountChgPct,
			HasSummary:   s.HasSummary(d),
		})
	}
	return result, nil
}
