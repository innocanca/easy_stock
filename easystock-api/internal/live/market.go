package live

import (
	"fmt"
	"math"
	"sort"

	"easystock/api/internal/tushare"
)

// MarketSnapshot holds a full daily market data snapshot.
type MarketSnapshot struct {
	Date          string         `json:"date"`
	Indices       []IndexPoint   `json:"indices"`
	Breadth       MarketBreadth  `json:"breadth"`
	NorthFlow     NorthFlowData  `json:"northFlow"`
	TopSectors    []SectorChange `json:"topSectors"`
	BottomSectors []SectorChange `json:"bottomSectors"`
	TotalAmount   float64        `json:"totalAmount"`
	AmountChgPct  float64        `json:"amountChgPct"`
}

type IndexPoint struct {
	Code    string  `json:"code"`
	Name    string  `json:"name"`
	Close   float64 `json:"close"`
	ChgPct  float64 `json:"chgPct"`
	Amount  float64 `json:"amount"`
	Vol     float64 `json:"vol"`
}

type MarketBreadth struct {
	UpCount       int `json:"upCount"`
	DownCount     int `json:"downCount"`
	FlatCount     int `json:"flatCount"`
	LimitUpCount  int `json:"limitUpCount"`
	LimitDownCount int `json:"limitDownCount"`
	Total         int `json:"total"`
}

type NorthFlowData struct {
	HgtNetBuy float64 `json:"hgtNetBuy"`
	SgtNetBuy float64 `json:"sgtNetBuy"`
	TotalNet  float64 `json:"totalNet"`
}

type SectorChange struct {
	Name   string  `json:"name"`
	AvgPe  float64 `json:"avgPe"`
	ChgPct float64 `json:"chgPct"`
}

var majorIndices = []struct {
	Code string
	Name string
}{
	{"000001.SH", "上证指数"},
	{"399001.SZ", "深证成指"},
	{"399006.SZ", "创业板指"},
}

// CollectMarketSnapshot gathers all market data for a given trade date.
func CollectMarketSnapshot(c *tushare.Client, date string) (*MarketSnapshot, error) {
	snap := &MarketSnapshot{Date: date}

	// 1. Index data
	indices, err := fetchIndices(c, date)
	if err != nil {
		return nil, fmt.Errorf("indices: %w", err)
	}
	snap.Indices = indices

	// 2. Market breadth from daily_basic
	breadth, totalAmt, err := fetchBreadth(c, date)
	if err != nil {
		return nil, fmt.Errorf("breadth: %w", err)
	}
	snap.Breadth = breadth
	snap.TotalAmount = math.Round(totalAmt*100) / 100

	// 3. Limit up/down counts
	luCount, ldCount := fetchLimitCounts(c, date)
	snap.Breadth.LimitUpCount = luCount
	snap.Breadth.LimitDownCount = ldCount

	// 4. North flow
	nf, _ := fetchNorthFlow(c, date)
	snap.NorthFlow = nf

	// 5. Sector performance
	top, bottom := fetchSectorChanges(c, date)
	snap.TopSectors = top
	snap.BottomSectors = bottom

	// 6. Amount change vs previous day
	prevAmt := fetchPrevDayAmount(c, date)
	if prevAmt > 0 {
		snap.AmountChgPct = math.Round((totalAmt/prevAmt-1)*10000) / 100
	}

	return snap, nil
}

func fetchIndices(c *tushare.Client, date string) ([]IndexPoint, error) {
	var result []IndexPoint
	for _, idx := range majorIndices {
		resp, err := c.Call("index_daily", map[string]any{
			"ts_code":    idx.Code,
			"start_date": date,
			"end_date":   date,
		}, "ts_code,close,pct_chg,amount,vol")
		if err != nil {
			continue
		}
		rows, err := tushare.RowsToMaps(resp)
		if err != nil || len(rows) == 0 {
			continue
		}
		r := rows[0]
		result = append(result, IndexPoint{
			Code:   idx.Code,
			Name:   idx.Name,
			Close:  math.Round(tushare.GetFloat(r, "close")*100) / 100,
			ChgPct: math.Round(tushare.GetFloat(r, "pct_chg")*100) / 100,
			Amount: math.Round(tushare.GetFloat(r, "amount")/100000) / 10, // 千元->亿
			Vol:    math.Round(tushare.GetFloat(r, "vol") / 10000),        // 手->万手
		})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no index data for %s", date)
	}
	return result, nil
}

func fetchBreadth(c *tushare.Client, date string) (MarketBreadth, float64, error) {
	rows, err := DailyBasicByTradeDate(c, date, "ts_code,pct_chg,total_mv,amount")
	if err != nil {
		return MarketBreadth{}, 0, err
	}
	var b MarketBreadth
	var totalAmt float64
	b.Total = len(rows)
	for _, r := range rows {
		pct := tushare.GetFloat(r, "pct_chg")
		amt := tushare.GetFloat(r, "amount")
		totalAmt += amt
		if pct > 0.01 {
			b.UpCount++
		} else if pct < -0.01 {
			b.DownCount++
		} else {
			b.FlatCount++
		}
	}
	totalAmt = totalAmt / 100000 // 千元->亿元
	return b, totalAmt, nil
}

func fetchLimitCounts(c *tushare.Client, date string) (int, int) {
	resp, err := c.Call("limit_list_d", map[string]any{
		"trade_date": date,
	}, "ts_code,limit")
	if err != nil {
		// Fallback: try limit_list
		resp, err = c.Call("limit_list", map[string]any{
			"trade_date": date,
		}, "ts_code,limit")
		if err != nil {
			return 0, 0
		}
	}
	rows, err := tushare.RowsToMaps(resp)
	if err != nil {
		return 0, 0
	}
	lu, ld := 0, 0
	for _, r := range rows {
		l := tushare.GetString(r, "limit")
		if l == "U" {
			lu++
		} else if l == "D" {
			ld++
		}
	}
	return lu, ld
}

func fetchNorthFlow(c *tushare.Client, date string) (NorthFlowData, error) {
	resp, err := c.Call("moneyflow_hsgt", map[string]any{
		"trade_date": date,
	}, "trade_date,hgt,sgt,north_money")
	if err != nil {
		return NorthFlowData{}, err
	}
	rows, err := tushare.RowsToMaps(resp)
	if err != nil || len(rows) == 0 {
		return NorthFlowData{}, fmt.Errorf("no north flow data")
	}
	r := rows[0]
	hgt := tushare.GetFloat(r, "hgt") / 10000   // 万元->亿
	sgt := tushare.GetFloat(r, "sgt") / 10000
	total := tushare.GetFloat(r, "north_money") / 10000
	if total == 0 {
		total = hgt + sgt
	}
	return NorthFlowData{
		HgtNetBuy: math.Round(hgt*100) / 100,
		SgtNetBuy: math.Round(sgt*100) / 100,
		TotalNet:  math.Round(total*100) / 100,
	}, nil
}

func fetchSectorChanges(c *tushare.Client, date string) (top []SectorChange, bottom []SectorChange) {
	names, err := StockBasicMap(c)
	if err != nil {
		return nil, nil
	}
	rows, err := DailyBasicByTradeDate(c, date, "ts_code,pe_ttm,pct_chg")
	if err != nil {
		return nil, nil
	}

	type accum struct {
		sumPct float64
		sumPe  float64
		count  int
		peCnt  int
	}
	byIndustry := make(map[string]*accum)
	for _, r := range rows {
		code := tushare.GetString(r, "ts_code")
		sb, ok := names[code]
		if !ok || sb.Industry == "" {
			continue
		}
		a, ok := byIndustry[sb.Industry]
		if !ok {
			a = &accum{}
			byIndustry[sb.Industry] = a
		}
		pct := tushare.GetFloat(r, "pct_chg")
		pe := tushare.GetFloat(r, "pe_ttm")
		a.sumPct += pct
		a.count++
		if pe > 0 && pe < 1000 {
			a.sumPe += pe
			a.peCnt++
		}
	}

	var all []SectorChange
	for name, a := range byIndustry {
		if a.count < 3 {
			continue
		}
		avgPe := 0.0
		if a.peCnt > 0 {
			avgPe = math.Round(a.sumPe/float64(a.peCnt)*10) / 10
		}
		all = append(all, SectorChange{
			Name:   name,
			AvgPe:  avgPe,
			ChgPct: math.Round(a.sumPct/float64(a.count)*100) / 100,
		})
	}

	sort.Slice(all, func(i, j int) bool { return all[i].ChgPct > all[j].ChgPct })

	topN := 5
	if topN > len(all) {
		topN = len(all)
	}
	top = all[:topN]

	bottomN := 5
	if bottomN > len(all) {
		bottomN = len(all)
	}
	bottom = all[len(all)-bottomN:]
	// Reverse bottom so worst is first
	for i, j := 0, len(bottom)-1; i < j; i, j = i+1, j-1 {
		bottom[i], bottom[j] = bottom[j], bottom[i]
	}

	return top, bottom
}

func fetchPrevDayAmount(c *tushare.Client, date string) float64 {
	// Get the trade date before `date`
	resp, err := c.Call("trade_cal", map[string]any{
		"exchange":   "SSE",
		"end_date":   date,
		"is_open":    "1",
	}, "cal_date")
	if err != nil {
		return 0
	}
	rows, err := tushare.RowsToMaps(resp)
	if err != nil {
		return 0
	}
	var dates []string
	for _, r := range rows {
		d := tushare.GetString(r, "cal_date")
		if len(d) == 8 && d < date {
			dates = append(dates, d)
		}
	}
	if len(dates) == 0 {
		return 0
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	prevDate := dates[0]

	prevRows, err := DailyBasicByTradeDate(c, prevDate, "ts_code,amount")
	if err != nil {
		return 0
	}
	var total float64
	for _, r := range prevRows {
		total += tushare.GetFloat(r, "amount")
	}
	return total / 100000 // 千元->亿
}
