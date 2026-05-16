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
	Mv         float64 `json:"mv"`
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

const (
	// We compute sector-level averages from the top-N by market cap to keep the
	// number of per-stock financial calls bounded (and avoid Tushare rate limits).
	sectorMetricSampleTopN = 20
	// For sector detail table, we enrich ROE for the top-N by market cap only.
	sectorRowRoeTopN = 50
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

	// Enrich sector detail with financial metrics on-demand.
	// Prebuilding ROE/revenue growth for all sectors would require too many
	// per-stock calls and often hits Tushare rate limits.
	var d sectorDetailOut
	if err := json.Unmarshal(raw, &d); err != nil {
		return raw, nil // best-effort: still serve base payload
	}

	// Select top by market cap for metric computation.
	sample := make([]sectorRowOut, len(d.Stocks))
	copy(sample, d.Stocks)
	sort.Slice(sample, func(i, j int) bool { return sample[i].Mv > sample[j].Mv })
	if len(sample) > sectorMetricSampleTopN {
		sample = sample[:sectorMetricSampleTopN]
	}
	topRows := make([]sectorRowOut, len(d.Stocks))
	copy(topRows, d.Stocks)
	sort.Slice(topRows, func(i, j int) bool { return topRows[i].Mv > topRows[j].Mv })
	if len(topRows) > sectorRowRoeTopN {
		topRows = topRows[:sectorRowRoeTopN]
	}

	need := make(map[string]struct{}, len(sample)+len(topRows))
	for _, r := range sample {
		need[r.Code] = struct{}{}
	}
	for _, r := range topRows {
		need[r.Code] = struct{}{}
	}

	type fiOut struct {
		ok    bool
		roe   float64
		trYoy float64
	}
	fiByCode := make(map[string]fiOut, len(need))
	var mu sync.Mutex

	// Best-effort parallelism with bounded concurrency.
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for code := range need {
		wg.Add(1)
		go func(tsCode string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			fi := fetchFinaSnap(c, tsCode)
			mu.Lock()
			fiByCode[tsCode] = fiOut{ok: fi.ok, roe: fi.roe, trYoy: fi.trYoy}
			mu.Unlock()
		}(code)
	}
	wg.Wait()

	// Compute averages from sample only.
	var roeSum, trSum float64
	var nFi int
	for _, r := range sample {
		if v, ok := fiByCode[r.Code]; ok && v.ok {
			roeSum += v.roe
			trSum += v.trYoy
			nFi++
		}
	}
	d.Sector.AvgRoe = -1
	d.Sector.RevGrowth = -1
	if nFi > 0 {
		d.Sector.AvgRoe = math.Round((roeSum/float64(nFi))*10) / 10
		d.Sector.RevGrowth = math.Round((trSum/float64(nFi))*10) / 10
	}

	// Enrich row ROE for fetched codes only; others remain -1.
	for i := range d.Stocks {
		d.Stocks[i].Roe = -1
		if v, ok := fiByCode[d.Stocks[i].Code]; ok && v.ok {
			d.Stocks[i].Roe = math.Round(v.roe*10) / 10
		}
	}

	return json.Marshal(d)
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
	rows, err := DailyBasicByTradeDate(c, date, "ts_code,pe_ttm,pb,total_mv")
	if err != nil {
		return nil, err
	}

	type stockRec struct {
		code string
		name string
		pe   float64
		mv   float64 // total_mv (万元)
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
		mv := tushare.GetFloat(row, "total_mv")
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
		groups[ind] = append(groups[ind], stockRec{code: code, name: name, pe: pe, mv: mv})
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
			AvgRoe:     -1,
			RevGrowth:  -1,
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
				Roe:        -1,
				VsSectorPe: math.Round(vs*10000) / 10000,
				Mv:         math.Round(r.mv*100) / 100,
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
				AvgRoe:     -1,
				RevGrowth:  -1,
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
