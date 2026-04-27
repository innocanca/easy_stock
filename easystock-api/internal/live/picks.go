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

// Sector list/detail: daily_basic grouped by stock_basic.industry (same cache TTL as SectorCacheTTL).

type pickItem struct {
	Code            string         `json:"code"`
	Name            string         `json:"name"`
	Score           int            `json:"score"`
	Pe              float64        `json:"pe"`
	Roe             float64        `json:"roe"`
	ProfitGrowthYoy float64        `json:"profitGrowthYoy"`
	Dimensions      map[string]int `json:"dimensions"`
	ScoreNote       string         `json:"scoreNote"`
	ProfitTrend     []profitTrend  `json:"profitTrend"`
	PeTrend         []peTrend      `json:"peTrend"`
}

type profitTrend struct {
	Q      string  `json:"q"`
	Profit float64 `json:"profit"`
}

type peTrend struct {
	Q  string  `json:"q"`
	Pe float64 `json:"pe"`
}

var (
	picksMu   sync.Mutex
	picksBody []byte
	picksAt   time.Time
)

// PicksJSON returns /api/picks payload from Tushare (cached).
func PicksJSON(c *tushare.Client) ([]byte, error) {
	picksMu.Lock()
	defer picksMu.Unlock()
	ttl := PicksCacheTTL()
	if picksBody != nil && time.Since(picksAt) < ttl {
		return picksBody, nil
	}
	body, err := buildPicks(c)
	if err != nil {
		return nil, err
	}
	picksBody = body
	picksAt = time.Now()
	return picksBody, nil
}

func buildPicks(c *tushare.Client) ([]byte, error) {
	date, err := LatestTradeDate(c)
	if err != nil {
		return nil, err
	}
	names, err := StockBasicMap(c)
	if err != nil {
		return nil, fmt.Errorf("stock_basic: %w", err)
	}
	resp, err := c.Call("daily_basic", map[string]any{
		"trade_date": date,
	}, "ts_code,pe_ttm,pb,total_mv,circ_mv,turnover_rate")
	if err != nil {
		return nil, fmt.Errorf("daily_basic: %w", err)
	}
	rows, err := tushare.RowsToMaps(resp)
	if err != nil {
		return nil, err
	}
	type rowSort struct {
		m  map[string]interface{}
		mv float64
		pe float64
	}
	var list []rowSort
	for _, m := range rows {
		pe := tushare.GetFloat(m, "pe_ttm")
		if pe <= 0 || math.IsNaN(pe) {
			continue
		}
		tmv := tushare.GetFloat(m, "total_mv")
		if tmv <= 0 {
			continue
		}
		list = append(list, rowSort{m: m, mv: tmv, pe: pe})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].mv > list[j].mv
	})
	limit := PickLimit()
	if len(list) > limit {
		list = list[:limit]
	}
	out := make([]pickItem, 0, len(list))
	for _, r := range list {
		code := tushare.GetString(r.m, "ts_code")
		name := stockName(names, code)
		pe := r.pe
		mv := r.mv
		score := 55 + int(math.Min(39, mv/1500000))
		if score > 99 {
			score = 99
		}
		valDim := int(math.Max(15, math.Min(95, 120-pe*1.8)))
		dims := map[string]int{
			"value":       72,
			"valuation":   valDim,
			"certainty":   68,
			"growth":      62,
		}
		qLabel := []string{"T-3", "T-2", "T-1", "T"}
		pt := make([]profitTrend, 4)
		ptPe := make([]peTrend, 4)
		for i := 0; i < 4; i++ {
			scale := 1 + float64(i)*0.02
			pt[i] = profitTrend{Q: qLabel[i], Profit: mv / (10000 * scale)}
			ptPe[i] = peTrend{Q: qLabel[i], Pe: pe * (1 + float64(i-2)*0.02)}
		}
		out = append(out, pickItem{
			Code:            code,
			Name:            name,
			Score:           score,
			Pe:              math.Round(pe*10) / 10,
			Roe:             0,
			ProfitGrowthYoy: 0,
			Dimensions:      dims,
			ScoreNote:       fmt.Sprintf("市值排序示例榜（trade_date=%s）。PE(ttm)=%.1f，总市值(万元)=%.0f。接入更多接口后可替换为自定义打分。", date, pe, mv),
			ProfitTrend:     pt,
			PeTrend:         ptPe,
		})
	}
	return json.Marshal(out)
}
