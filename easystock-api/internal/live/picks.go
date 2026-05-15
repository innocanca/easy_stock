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

type PickItem struct {
	Code            string         `json:"code"`
	Name            string         `json:"name"`
	Sector          string         `json:"sector"`
	Score           int            `json:"score"`
	Pe              float64        `json:"pe"`
	Roe             float64        `json:"roe"`
	ProfitGrowthYoy float64        `json:"profitGrowthYoy"`
	TurnoverRate    float64        `json:"turnoverRate"`
	VolumeRatio     float64        `json:"volumeRatio"`
	Close           float64        `json:"close"`
	PricePct        float64        `json:"pricePct"`
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
	Items    []PickItem `json:"items"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}

// PickStyle defines scoring weights for a specific investment style.
type PickStyle struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Desc       string  `json:"desc"`
	WeightVal  float64 `json:"-"`
	WeightValu float64 `json:"-"`
	WeightCert float64 `json:"-"`
	WeightGrow float64 `json:"-"`
}

var pickStyleOrder = []string{"balanced", "value", "bluechip", "growth", "dividend"}

var pickStyles = map[string]PickStyle{
	"balanced": {ID: "balanced", Label: "综合均衡", Desc: "价值、估值、确定性、成长四维均衡", WeightVal: 0.28, WeightValu: 0.28, WeightCert: 0.22, WeightGrow: 0.22},
	"value":    {ID: "value", Label: "价值投资", Desc: "偏爱高 ROE、高毛利、低负债的优质企业", WeightVal: 0.40, WeightValu: 0.30, WeightCert: 0.20, WeightGrow: 0.10},
	"bluechip": {ID: "bluechip", Label: "低估蓝筹", Desc: "偏爱低 PE、大市值的低估值蓝筹股", WeightVal: 0.15, WeightValu: 0.50, WeightCert: 0.25, WeightGrow: 0.10},
	"growth":   {ID: "growth", Label: "高成长", Desc: "偏爱净利润和营收高速增长的成长股", WeightVal: 0.10, WeightValu: 0.15, WeightCert: 0.15, WeightGrow: 0.60},
	"dividend": {ID: "dividend", Label: "稳健红利", Desc: "偏爱低波动、高 ROE 的稳健分红股", WeightVal: 0.20, WeightValu: 0.20, WeightCert: 0.50, WeightGrow: 0.10},
}

// ListPickStyles returns available styles in display order.
func ListPickStyles() []PickStyle {
	out := make([]PickStyle, 0, len(pickStyleOrder))
	for _, id := range pickStyleOrder {
		out = append(out, pickStyles[id])
	}
	return out
}

func getStyle(id string) PickStyle {
	if s, ok := pickStyles[id]; ok {
		return s
	}
	return pickStyles["balanced"]
}

type picksFullCache struct {
	at    time.Time
	date  string
	minMv float64
	items []PickItem
}

var (
	picksMu        sync.Mutex
	picksCacheMap  = make(map[string]*picksFullCache) // key: styleID
)

// Per-stock financial data caches (24h TTL)
type finaSnapCacheEntry struct {
	data finaSnap
	at   time.Time
}
type profitsCacheEntry struct {
	data []qProfit
	err  error
	at   time.Time
}

var (
	finaSnapCache   = make(map[string]finaSnapCacheEntry)
	finaSnapMu      sync.RWMutex
	profitsCache    = make(map[string]profitsCacheEntry)
	profitsMu       sync.RWMutex
	finaCacheTTL    = 24 * time.Hour
)

// WarmUpPicks pre-builds the picks cache in a background goroutine so the
// first user request doesn't have to wait.
func WarmUpPicks(c *tushare.Client) {
	go func() {
		minMv := DefaultMinMvWan()
		_, err := getCachedFullPicks(c, minMv, "balanced")
		if err != nil {
			fmt.Printf("picks warm-up failed: %v\n", err)
		} else {
			fmt.Println("picks warm-up: cache ready (balanced)")
		}
	}()
}

// TopPicksForAI returns top N picks for AI recommendation (used by the AI stock picker).
func TopPicksForAI(c *tushare.Client, styleID string, topN int) ([]PickItem, PickStyle, error) {
	style := getStyle(styleID)
	minMv := DefaultMinMvWan()
	items, err := getCachedFullPicks(c, minMv, styleID)
	if err != nil {
		return nil, style, err
	}
	if topN > len(items) {
		topN = len(items)
	}
	top := make([]PickItem, topN)
	copy(top, items[:topN])

	// Enrich with price percentile (concurrently, best-effort)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i := range top {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			top[idx].PricePct = pricePctile(c, top[idx].Code, top[idx].Close)
		}(i)
	}
	wg.Wait()

	return top, style, nil
}

// pricePctile computes where the current close price sits within the past ~1 year
// of daily closes (0 = lowest, 100 = highest). Returns -1 on failure.
func pricePctile(c *tushare.Client, tsCode string, curClose float64) float64 {
	if curClose <= 0 {
		return -1
	}
	resp, err := c.Call("daily", map[string]any{
		"ts_code": tsCode,
		"fields":  "close",
	}, "close")
	if err != nil {
		return -1
	}
	rows, err := tushare.RowsToMaps(resp)
	if err != nil || len(rows) < 20 {
		return -1
	}
	// daily returns most recent first; take up to 250 trading days (~1 year)
	limit := 250
	if len(rows) > limit {
		rows = rows[:limit]
	}
	below := 0
	for _, r := range rows {
		c := tushare.GetFloat(r, "close")
		if c > 0 && curClose >= c {
			below++
		}
	}
	pct := float64(below) / float64(len(rows)) * 100
	return math.Round(pct*10) / 10
}

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

	styleID := q.Get("style")
	if styleID == "" {
		styleID = "balanced"
	}

	full, err := getCachedFullPicks(c, minMv, styleID)
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

func getCachedFullPicks(c *tushare.Client, minMv float64, styleID string) ([]PickItem, error) {
	style := getStyle(styleID)
	picksMu.Lock()
	defer picksMu.Unlock()
	date, err := LatestTradeDate(c)
	if err != nil {
		return nil, err
	}
	ttl := PicksCacheTTL()
	if cached := picksCacheMap[style.ID]; cached != nil &&
		time.Since(cached.at) < ttl && cached.date == date && cached.minMv == minMv {
		return cached.items, nil
	}
	items, err := buildPicksFull(c, minMv, style)
	if err != nil {
		return nil, err
	}
	picksCacheMap[style.ID] = &picksFullCache{at: time.Now(), date: date, minMv: minMv, items: items}
	return items, nil
}

func buildPicksFull(c *tushare.Client, minMvWan float64, style PickStyle) ([]PickItem, error) {
	date, err := LatestTradeDate(c)
	if err != nil {
		return nil, err
	}
	names, err := StockBasicMap(c)
	if err != nil {
		return nil, fmt.Errorf("stock_basic: %w", err)
	}
	rows, err := DailyBasicByTradeDate(c, date, "ts_code,pe_ttm,pb,total_mv,circ_mv,turnover_rate,turnover_rate_f,volume_ratio,close")
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

	out := make([]PickItem, len(list))
	var wg sync.WaitGroup
	sem := make(chan struct{}, PicksConcurrency())
	for i := range list {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[idx] = buildOnePick(c, names, date, medianPe, list[idx], style)
		}(i)
	}
	wg.Wait()

	// Rank-normalize: convert raw dimension scores to within-pool percentile
	// ranks so that weight changes across styles produce meaningful re-ordering.
	rankComposite(out, style)

	type scored struct {
		item PickItem
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
	result := make([]PickItem, len(paired))
	for i := range paired {
		result[i] = paired[i].item
	}
	return result, nil
}

// rankComposite re-computes Score using within-pool percentile ranks for each
// dimension, so different style weights create meaningful ordering differences.
// Raw dimension scores in Dimensions map are preserved for UI display.
func rankComposite(items []PickItem, style PickStyle) {
	n := len(items)
	if n < 2 {
		return
	}

	dims := []string{"value", "valuation", "certainty", "growth"}
	weights := []float64{style.WeightVal, style.WeightValu, style.WeightCert, style.WeightGrow}

	// For each dimension, compute percentile rank (0–100) within the pool.
	pctRanks := make([][]float64, 4)
	for di, dim := range dims {
		indices := make([]int, n)
		for i := range indices {
			indices[i] = i
		}
		sort.Slice(indices, func(a, b int) bool {
			return items[indices[a]].Dimensions[dim] < items[indices[b]].Dimensions[dim]
		})
		pctRanks[di] = make([]float64, n)
		for rank, idx := range indices {
			pctRanks[di][idx] = float64(rank) / float64(n-1) * 100
		}
	}

	// Composite score = weighted sum of percentile ranks
	for i := range items {
		raw := 0.0
		for di := range dims {
			raw += weights[di] * pctRanks[di][i]
		}
		items[i].Score = int(math.Round(raw))
	}
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

func buildOnePick(c *tushare.Client, names map[string]StockBasicRow, tradeDate string, medianPe float64, r rowSort, style PickStyle) PickItem {
	code := tushare.GetString(r.m, "ts_code")
	name := stockName(names, code)
	sector := ""
	if sb, ok := names[code]; ok {
		sector = sb.Industry
	}
	pe := r.pe
	mv := tushare.GetFloat(r.m, "total_mv")
	turnoverRate := tushare.GetFloat(r.m, "turnover_rate_f")
	if turnoverRate == 0 {
		turnoverRate = tushare.GetFloat(r.m, "turnover_rate")
	}
	volumeRatio := tushare.GetFloat(r.m, "volume_ratio")
	closePrice := tushare.GetFloat(r.m, "close")

	fi := fetchFinaSnap(c, code)
	profitPts, incErr := fetchQuarterlyProfitsYi(c, code)

	val := dimValue(fi)
	valu := dimValuation(pe, medianPe)
	cert := dimCertainty(fi)
	grow := dimGrowth(fi)

	dims := map[string]int{
		"value":     val,
		"valuation": valu,
		"certainty": cert,
		"growth":    grow,
	}

	raw := style.WeightVal*float64(val) + style.WeightValu*float64(valu) + style.WeightCert*float64(cert) + style.WeightGrow*float64(grow)
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

	return PickItem{
		Code:            code,
		Name:            name,
		Sector:          sector,
		Score:           score,
		Pe:              math.Round(pe*10) / 10,
		Roe:             roe,
		ProfitGrowthYoy: pgy,
		TurnoverRate:    math.Round(turnoverRate*100) / 100,
		VolumeRatio:     math.Round(volumeRatio*100) / 100,
		Close:           math.Round(closePrice*100) / 100,
		Dimensions:      dims,
		ScoreNote:       note,
		ProfitTrend:     pt,
		PeTrend:         ptPe,
	}
}

func fetchFinaSnap(c *tushare.Client, tsCode string) finaSnap {
	finaSnapMu.RLock()
	if e, ok := finaSnapCache[tsCode]; ok && time.Since(e.at) < finaCacheTTL {
		finaSnapMu.RUnlock()
		return e.data
	}
	finaSnapMu.RUnlock()

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
	snap := finaSnap{
		endDate:      tushare.GetString(row, "end_date"),
		roe:          tushare.GetFloat(row, "roe"),
		grossMargin:  tushare.GetFloat(row, "grossprofit_margin"),
		debtToAssets: tushare.GetFloat(row, "debt_to_assets"),
		netMargin:    tushare.GetFloat(row, "netprofit_margin"),
		netprofitYoy: tushare.GetFloat(row, "netprofit_yoy"),
		trYoy:        tushare.GetFloat(row, "tr_yoy"),
		ok:           true,
	}

	finaSnapMu.Lock()
	finaSnapCache[tsCode] = finaSnapCacheEntry{data: snap, at: time.Now()}
	finaSnapMu.Unlock()
	return snap
}

type qProfit struct {
	endDate string
	yi      float64 // 亿元
}

func fetchQuarterlyProfitsYi(c *tushare.Client, tsCode string) ([]qProfit, error) {
	profitsMu.RLock()
	if e, ok := profitsCache[tsCode]; ok && time.Since(e.at) < finaCacheTTL {
		profitsMu.RUnlock()
		return e.data, e.err
	}
	profitsMu.RUnlock()

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
		wan := tushare.GetFloat(row, "n_income_attr_p")
		out = append(out, qProfit{endDate: ed, yi: wan / 10000.0})
		if len(out) >= 4 {
			break
		}
	}
	if len(out) == 0 {
		retErr := fmt.Errorf("无有效季度净利")
		profitsMu.Lock()
		profitsCache[tsCode] = profitsCacheEntry{err: retErr, at: time.Now()}
		profitsMu.Unlock()
		return nil, retErr
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].endDate < out[j].endDate
	})

	profitsMu.Lock()
	profitsCache[tsCode] = profitsCacheEntry{data: out, at: time.Now()}
	profitsMu.Unlock()
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
