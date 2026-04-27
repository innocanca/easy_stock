package store

import (
	"encoding/json"
	"fmt"
	"os"

	"easystock/api/staticdata"
)

// Dataset mirrors staticdata/dataset.json (aligned with easyStock/shared/dataset.ts).
type Dataset struct {
	Picks        []json.RawMessage          `json:"picks"`
	Stocks       map[string]json.RawMessage `json:"stocks"`
	Sectors      []json.RawMessage          `json:"sectors"`
	SectorStocks map[string]json.RawMessage `json:"sectorStocks"`
	SectorNews   map[string]json.RawMessage `json:"sectorNews"`
}

type Store struct {
	picksBytes   []byte
	stockByCode  map[string][]byte
	sectorList   []byte
	sectorDetail map[string][]byte
}

func Load() (*Store, error) {
	src := staticdata.DatasetJSON
	if p := os.Getenv("DATASET_PATH"); p != "" {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read DATASET_PATH: %w", err)
		}
		src = b
	}

	var d Dataset
	if err := json.Unmarshal(src, &d); err != nil {
		return nil, fmt.Errorf("parse dataset: %w", err)
	}

	s := &Store{
		picksBytes:   mustMarshal(d.Picks),
		stockByCode:  map[string][]byte{},
		sectorDetail: map[string][]byte{},
	}

	for code, raw := range d.Stocks {
		s.stockByCode[code] = []byte(raw)
	}

	s.sectorList = mustMarshal(d.Sectors)

	type sectorPayload struct {
		Sector json.RawMessage `json:"sector"`
		Stocks json.RawMessage `json:"stocks"`
		News   json.RawMessage `json:"news"`
	}

	for _, secRaw := range d.Sectors {
		var meta struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(secRaw, &meta); err != nil {
			return nil, fmt.Errorf("sector meta: %w", err)
		}
		id := meta.ID
		stocks := d.SectorStocks[id]
		if stocks == nil {
			stocks = []byte("[]")
		}
		news := d.SectorNews[id]
		if news == nil {
			news = []byte("[]")
		}
		out, err := json.Marshal(sectorPayload{
			Sector: secRaw,
			Stocks: stocks,
			News:   news,
		})
		if err != nil {
			return nil, err
		}
		s.sectorDetail[id] = out
	}

	return s, nil
}

func mustMarshal(v any) []byte {
	if v == nil {
		return []byte("null")
	}
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func (s *Store) PicksJSON() []byte {
	return s.picksBytes
}

func (s *Store) StockJSON(code string) ([]byte, bool) {
	b, ok := s.stockByCode[code]
	return b, ok
}

func (s *Store) SectorsJSON() []byte {
	return s.sectorList
}

func (s *Store) SectorDetailJSON(id string) ([]byte, bool) {
	b, ok := s.sectorDetail[id]
	return b, ok
}
