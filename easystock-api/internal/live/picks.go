package live

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"easystock/api/internal/tushare"
)

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

type rowSort struct {
	m  map[string]interface{}
	mv float64
	pe float64
}

type finaSnap struct {
	endDate      string
	roe          float64
	grossMargin  float64
	debtToAssets float64
	netMargin    float64
	netprofitYoy float64
	trYoy        float64
	ok           bool
}

type picksResponse struct {
	Items    []pickItem `json:"items"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}

type picksFullCache struct {
	at    time.Time
	date  string
	minMv float64
	items []pickItem
}

var (
	picksMu         sync.Mutex
	picksFullCached *picksFullCache
)

// PicksJSON returns /api/picks JSON: 市值≥min_mv_wan（默认500亿）的全部标的，按综合分排序；支持 score 区间与分页。
// 查询参数：page, page_size, min_mv_wan, score_min, score_max（均为可选）。
func PicksJSON(c *tushare.Client, q url.Values) ([]byte, error) {
	page := parseIntDefault(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	pageSize := parseIntDefault(q.Get("page_size"), 12)
	if pageSize < 1 {
		pageSize = 12
	}
	if pageSize > 50 {
		pageSize = 50
	}
	minMv := parseFloatDefault(q.Get("min_mv_wan"), DefaultMinMvWan())
	if minMv < 0 {
		minMv = DefaultMinMvWan()
	}
	scoreMin := parseIntDefault(q.Get("score_min"), 1)
	scoreMax := parseIntDefault(q.Get("score_max"), 99)
	if scoreMin < 1 {
		scoreMin = 1
	}
	if scoreMax > 99 {
		scoreMax = 99
	}
	if scoreMin > scoreMax {
		scoreMin, scoreMax = scoreMax, scoreMin
	}

	full, err := getCachedFullPicks(c, minMv)
	if err != nil {
		return nil, err
	}

	filtered := full[:0]
	for _, it := range full {
		if it.Score >= scoreMin && it.Score <= scoreMax {
			filtered = append(filtered, it)
		}
	}

	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pageItems := filtered[start:end]

	out := picksResponse{
		Items:    pageItems,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	return json.Marshal(out)
}

func getCachedFullPicks(c *tushare.Client, minMv float64) ([]pickItem, error) {
	picksMu.Lock()
	defer picksMu.Unlock()
	date, err := LatestTradeDate(c)
	if err != nil {
		return nil, err
	}
	ttl := PicksCacheTTL()
	if picksFullCached != nil && time.Since(picksFullCached.at) < ttl &&
		picksFullCached.date == date && picksFullCached.minMv == minMv {
		return picksFullCached.items, nil
	}
	items, err := buildPicksFull(c, minMv)
	if err != nil {
		return nil, err
	}
	picksFullCached = &picksFullCache{at: time.Now(), date: date, minMv: minMv, items: items}
	return items, nil
}

func buildPicksFull(c *tushare.Client, minMvWan float64) ([]pickItem, error) {
	date, err := LatestTradeDate(c)
	if err != nil {
		return nil, err
	}
	names, err := StockBasicMap(c)
	if err != nil {
		return nil, fmt.Errorf("stock_basic: %w", err)
	}
	rows, err := DailyBasicByTradeDate(c, date, "ts_code,pe_ttm,pb,total_mv,circ_mv,turnover_rate")
	if err != nil {
		return nil, err
	}
	var list []rowSort
	var pesAll []float64
	for _, m := range rows {
		pe := tushare.GetFloat(m, "pe_ttm")
		if pe <= 0 || math.IsNaN(pe) {
			continue
		}
		tmv := tushare.GetFloat(m, "total_mv")
		if tmv <= 0 {
			continue
		}
		pesAll = append(pesAll, pe)
		list = append(list, rowSort{m: m, mv: tmv, pe: pe})
	}
	sort.Float64s(pesAll)
	medianPe := medianSorted(pesAll)

	var filtered []rowSort
	for _, r := range list {
		if r.mv >= minMvWan {
			filtered = append(filtered, r)
		}
	}
	list = filtered

	out := make([]pickItem, len(list))
	var wg sync.WaitGroup
	sem := make(chan struct{}, PicksConcurrency())
	for i := range list {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[idx] = buildOnePick(c, names, date, medianPe, list[idx])
		}(i)
	}
	wg.Wait()

	type scored struct {
		item pickItem
		mv   float64
	}
	paired := make([]scored, len(list))
	for i := range list {
		paired[i] = scored{item: out[i], mv: list[i].mv}
	}
	sort.Slice(paired, func(i, j int) bool {
		a, b := paired[i].item, paired[j].item
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if paired[i].mv != paired[j].mv {
			return paired[i].mv > paired[j].mv
		}
		return paired[i].item.Code < paired[j].item.Code
	})
	result := make([]pickItem, len(paired))
	for i := range paired {
		result[i] = paired[i].item
	}
	return result, nil
}

func parseIntDefault(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func parseFloatDefault(s string, def float64) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return n
}

func buildOnePick(c *tushare.Client, names map[string]StockBasicRow, tradeDate string, medianPe float64, r rowSort) pickItem {
	code := tushare.GetString(r.m, "ts_code")
	name := stockName(names, code)
	pe := r.pe
	mv := tushare.GetFloat(r.m, "total_mv")

	fi := fetchFinaSnap(c, code)
	profitPts, incErr := fetchQuarterlyProfitsYi(c, code)

	val := dimValue(fi)
	valu := dimValuation(pe, medianPe)
	cert := dimCertainty(fi)
	grow := dimGrowth(fi)

	dims := map[string]int{
		"value":       val,
		"valuation":   valu,
		"certainty":   cert,
		"growth":      grow,
	}

	raw := 0.28*float64(val) + 0.28*float64(valu) + 0.22*float64(cert) + 0.22*float64(grow)
	score := int(math.Round(raw))
	if score < 1 {
		score = 1
	}
	if score > 99 {
		score = 99
	}

	roe := 0.0
	pgy := 0.0
	fiEnd := "—"
	if fi.ok {
		roe = math.Round(fi.roe*10) / 10
		pgy = math.Round(fi.netprofitYoy*10) / 10
		fiEnd = fi.endDate
	}

	pt, ptPe := buildTrends(pe, r.mv, profitPts, incErr)

	note := fmt.Sprintf(
		"行情日 %s，总市值 %.0f 万元，PE(ttm) %.1f，当日样本 PE 中位数 %.1f。",
		tradeDate, mv, pe, medianPe,
	)
	if fi.ok {
		note += fmt.Sprintf(
			" 财报期末 %s：ROE %.1f%%，毛利率 %.1f%%，资产负债率 %.1f%%，净利同比 %.1f%%，营收同比 %.1f%%。",
			fiEnd, fi.roe, fi.grossMargin, fi.debtToAssets, fi.netprofitYoy, fi.trYoy,
		)
	} else {
		note += " 财务指标接口暂无数据，价值/确定性/成长按中性处理；估值仍可比 PE。"
	}
	if incErr != nil {
		note += fmt.Sprintf(" 季度利润表不可用（%v），走势图用市值缩放占位。", incErr)
	} else {
		note += " 利润为合并报表归属净利（亿元，最近至多四季）；PE 走势按当期 PE 与历史季度利润比例回溯近似。"
	}

	return pickItem{
		Code:            code,
		Name:            name,
		Score:           score,
		Pe:              math.Round(pe*10) / 10,
		Roe:             roe,
		ProfitGrowthYoy: pgy,
		Dimensions:      dims,
		ScoreNote:       note,
		ProfitTrend:     pt,
		PeTrend:         ptPe,
	}
}

func fetchFinaSnap(c *tushare.Client, tsCode string) finaSnap {
	resp, err := c.Call("fina_indicator", map[string]any{
		"ts_code": tsCode,
	}, "ts_code,end_date,roe,grossprofit_margin,debt_to_assets,netprofit_margin,netprofit_yoy,tr_yoy")
	if err != nil {
		return finaSnap{}
	}
	fiRows, err := tushare.RowsToMaps(resp)
	if err != nil || len(fiRows) == 0 {
		return finaSnap{}
	}
	sort.Slice(fiRows, func(i, j int) bool {
		return tushare.GetString(fiRows[i], "end_date") > tushare.GetString(fiRows[j], "end_date")
	})
	row := fiRows[0]
	return finaSnap{
		endDate:      tushare.GetString(row, "end_date"),
		roe:          tushare.GetFloat(row, "roe"),
		grossMargin:  tushare.GetFloat(row, "grossprofit_margin"),
		debtToAssets: tushare.GetFloat(row, "debt_to_assets"),
		netMargin:    tushare.GetFloat(row, "netprofit_margin"),
		netprofitYoy: tushare.GetFloat(row, "netprofit_yoy"),
		trYoy:        tushare.GetFloat(row, "tr_yoy"),
		ok:           true,
	}
}

type qProfit struct {
	endDate string
	yi      float64 // 亿元
}

func fetchQuarterlyProfitsYi(c *tushare.Client, tsCode string) ([]qProfit, error) {
	resp, err := c.Call("income", map[string]any{
		"ts_code": tsCode,
	}, "end_date,n_income_attr_p")
	if err != nil {
		return nil, err
	}
	incRows, err := tushare.RowsToMaps(resp)
	if err != nil || len(incRows) == 0 {
		return nil, fmt.Errorf("无利润表数据")
	}
	sort.Slice(incRows, func(i, j int) bool {
		return tushare.GetString(incRows[i], "end_date") > tushare.GetString(incRows[j], "end_date")
	})
	var out []qProfit
	seen := make(map[string]struct{})
	for _, row := range incRows {
		ed := tushare.GetString(row, "end_date")
		if ed == "" {
			continue
		}
		if _, ok := seen[ed]; ok {
			continue
		}
		seen[ed] = struct{}{}
		// n_income_attr_p：万元 → 亿元
		wan := tushare.GetFloat(row, "n_income_attr_p")
		out = append(out, qProfit{endDate: ed, yi: wan / 10000.0})
		if len(out) >= 4 {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("无有效季度净利")
	}
	// 转为时间正序（旧 → 新），便于展示走势
	sort.Slice(out, func(i, j int) bool {
		return out[i].endDate < out[j].endDate
	})
	return out, nil
}

func buildTrends(pe float64, totalMvWan float64, profits []qProfit, incErr error) ([]profitTrend, []peTrend) {
	if incErr != nil || len(profits) == 0 {
		return syntheticTrends(pe, totalMvWan)
	}
	pt := make([]profitTrend, len(profits))
	ptPe := make([]peTrend, len(profits))
	last := profits[len(profits)-1].yi
	for i := range profits {
		q := quarterLabel(profits[i].endDate)
		pt[i] = profitTrend{Q: q, Profit: math.Round(profits[i].yi*10) / 10}
		pAdj := pe
		if profits[i].yi > 0 && last > 0 {
			pAdj = pe * (last / profits[i].yi)
		}
		if pAdj < 1 {
			pAdj = 1
		}
		if pAdj > 500 {
			pAdj = 500
		}
		ptPe[i] = peTrend{Q: q, Pe: math.Round(pAdj*10) / 10}
	}
	return pt, ptPe
}

func syntheticTrends(pe float64, totalMvWan float64) ([]profitTrend, []peTrend) {
	qLabel := []string{"T-3", "T-2", "T-1", "T"}
	pt := make([]profitTrend, 4)
	ptPe := make([]peTrend, 4)
	for i := 0; i < 4; i++ {
		scale := 1 + float64(i)*0.02
		pv := totalMvWan / (10000 * scale)
		if totalMvWan <= 0 {
			pv = 0
		}
		pt[i] = profitTrend{Q: qLabel[i], Profit: math.Round(pv*10) / 10}
		ptPe[i] = peTrend{Q: qLabel[i], Pe: math.Round(pe*10) / 10}
	}
	return pt, ptPe
}

func quarterLabel(endDate string) string {
	if len(endDate) < 8 {
		return endDate
	}
	yy := endDate[2:4]
	mmdd := endDate[4:8]
	q := 1
	switch mmdd {
	case "0331":
		q = 1
	case "0630":
		q = 2
	case "0930":
		q = 3
	case "1231":
		q = 4
	default:
		q = 1
	}
	return fmt.Sprintf("%sQ%d", yy, q)
}

func dimValue(f finaSnap) int {
	if !f.ok {
		return clampDim(52)
	}
	d := math.Min(100, f.debtToAssets)
	v := 18 + f.roe*1.55 + f.grossMargin*0.11 + (100-d)*0.14
	return clampDim(v)
}

func dimValuation(pe, medianPe float64) int {
	if medianPe <= 0 || math.IsNaN(medianPe) {
		return clampDim(120 - pe*1.8)
	}
	// PE 低于全市场（样本）中位数 → 估值维度分数更高
	ratio := (medianPe - pe) / medianPe
	v := 55 + 38*ratio
	return clampDim(v)
}

func dimCertainty(f finaSnap) int {
	if !f.ok {
		return clampDim(50)
	}
	d := math.Min(100, f.debtToAssets)
	vol := math.Abs(f.netprofitYoy)
	v := 38 + f.roe*1.35 + (100-d)*0.18 - math.Min(22, vol*0.14)
	return clampDim(v)
}

func dimGrowth(f finaSnap) int {
	if !f.ok {
		return clampDim(50)
	}
	v := 52 + 0.38*f.netprofitYoy + 0.22*f.trYoy
	return clampDim(v)
}

func clampDim(x float64) int {
	v := int(math.Round(x))
	if v < 15 {
		return 15
	}
	if v > 95 {
		return 95
	}
	return v
}

func medianSorted(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}
