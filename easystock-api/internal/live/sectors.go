package live

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"easystock/api/internal/tushare"
)

type sectorBenchOut struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	AvgPe      float64 `json:"avgPe"`
	AvgRoe     float64 `json:"avgRoe"`
	RevGrowth  float64 `json:"revGrowth"`
	VsMarketPe float64 `json:"vsMarketPe"`
}

type sectorRowOut struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Pe         float64 `json:"pe"`
	Roe        float64 `json:"roe"`
	VsSectorPe float64 `json:"vsSectorPe"`
}

type sectorDetailOut struct {
	Sector sectorBenchOut `json:"sector"`
	Stocks []sectorRowOut `json:"stocks"`
	News   []any          `json:"news"`
}

type sectorBundle struct {
	ListJSON []byte
	Detail   map[string][]byte
}

var (
	sectorMu     sync.Mutex
	sectorCached *sectorBundle
	sectorAt     time.Time
)

func sectorBundleCached(c *tushare.Client) (*sectorBundle, error) {
	sectorMu.Lock()
	defer sectorMu.Unlock()
	if sectorCached != nil && time.Since(sectorAt) < SectorCacheTTL() {
		return sectorCached, nil
	}
	b, err := buildSectorBundle(c)
	if err != nil {
		return nil, err
	}
	sectorCached = b
	sectorAt = time.Now()
	return sectorCached, nil
}

func SectorsJSON(c *tushare.Client) ([]byte, error) {
	b, err := sectorBundleCached(c)
	if err != nil {
		return nil, err
	}
	return b.ListJSON, nil
}

func SectorDetailJSON(c *tushare.Client, id string) ([]byte, error) {
	b, err := sectorBundleCached(c)
	if err != nil {
		return nil, err
	}
	raw, ok := b.Detail[id]
	if !ok {
		return nil, ErrSectorNotFound
	}
	return raw, nil
}

func buildSectorBundle(c *tushare.Client) (*sectorBundle, error) {
	date, err := LatestTradeDate(c)
	if err != nil {
		return nil, err
	}
	names, err := StockBasicMap(c)
	if err != nil {
		return nil, err
	}
	resp, err := c.Call("daily_basic", map[string]any{
		"trade_date": date,
	}, "ts_code,pe_ttm,pb,total_mv")
	if err != nil {
		return nil, fmt.Errorf("daily_basic: %w", err)
	}
	rows, err := tushare.RowsToMaps(resp)
	if err != nil {
		return nil, err
	}

	type stockRec struct {
		code string
		name string
		pe   float64
	}
	groups := make(map[string][]stockRec)

	var mPeSum float64
	var mPeN int

	for _, row := range rows {
		code := tushare.GetString(row, "ts_code")
		if code == "" {
			continue
		}
		pe := tushare.GetFloat(row, "pe_ttm")
		if pe <= 0 || math.IsNaN(pe) {
			continue
		}
		mPeSum += pe
		mPeN++

		sb := names[code]
		ind := sb.Industry
		if ind == "" {
			ind = "Other"
		}
		name := sb.Name
		if name == "" {
			name = code
		}
		groups[ind] = append(groups[ind], stockRec{code: code, name: name, pe: pe})
	}

	if mPeN == 0 {
		return nil, fmt.Errorf("daily_basic: no valid pe_ttm rows")
	}
	marketAvgPe := mPeSum / float64(mPeN)

	minN := MinStocksPerIndustry()
	maxSec := MaxSectors()

	var industries []string
	for ind, recs := range groups {
		if len(recs) >= minN {
			industries = append(industries, ind)
		}
	}
	sort.Strings(industries)
	if len(industries) > maxSec {
		industries = industries[:maxSec]
	}

	detail := make(map[string][]byte)
	var benches []sectorBenchOut

	for _, ind := range industries {
		recs := groups[ind]
		var peSum float64
		for _, r := range recs {
			peSum += r.pe
		}
		nf := float64(len(recs))
		avgPe := peSum / nf
		id := SectorIDFromIndustry(ind)
		ratio := 1.0
		if marketAvgPe > 0 {
			ratio = avgPe / marketAvgPe
		}

		benches = append(benches, sectorBenchOut{
			ID:         id,
			Name:       ind,
			AvgPe:      math.Round(avgPe*100) / 100,
			AvgRoe:     0,
			RevGrowth:  0,
			VsMarketPe: math.Round(ratio*100) / 100,
		})

		stocksOut := make([]sectorRowOut, 0, len(recs))
		for _, r := range recs {
			vs := 0.0
			if avgPe > 0 {
				vs = (r.pe - avgPe) / avgPe
			}
			stocksOut = append(stocksOut, sectorRowOut{
				Code:       r.code,
				Name:       r.name,
				Pe:         math.Round(r.pe*100) / 100,
				Roe:        0,
				VsSectorPe: math.Round(vs*10000) / 10000,
			})
		}
		sort.Slice(stocksOut, func(i, j int) bool {
			return stocksOut[i].VsSectorPe > stocksOut[j].VsSectorPe
		})

		payload := sectorDetailOut{
			Sector: sectorBenchOut{
				ID:         id,
				Name:       ind,
				AvgPe:      math.Round(avgPe*100) / 100,
				AvgRoe:     0,
				RevGrowth:  0,
				VsMarketPe: math.Round(ratio*100) / 100,
			},
			Stocks: stocksOut,
			News:   []any{},
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		detail[id] = raw
	}

	listJSON, err := json.Marshal(benches)
	if err != nil {
		return nil, err
	}

	return &sectorBundle{ListJSON: listJSON, Detail: detail}, nil
}
