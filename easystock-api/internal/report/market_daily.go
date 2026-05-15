package report

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"easystock/api/internal/live"
	"easystock/api/internal/tushare"
)

const MarketDailySystemPrompt = `你是一位资深的 A 股市场分析师。用户会给你当天的市场数据快照，请你撰写一份专业、简明的每日市场日报。

要求：
1. 使用 Markdown 格式
2. 必须包含以下章节：
   ## 今日概述
   一句话总结今日市场特征（如"放量上攻"、"缩量震荡"、"普跌格局"等）
   
   ## 指数表现
   分析三大指数的涨跌情况及市场情绪
   
   ## 资金面
   - 北向资金流入/流出情况及其含义
   - 全市场成交额及与前日对比，分析量能意义
   
   ## 涨跌统计
   分析涨跌家数比例，是否存在结构性行情
   
   ## 板块轮动
   分析今日领涨和领跌板块的逻辑
   
   ## 明日关注
   基于今日走势给出明日需关注的要点（1-3个）

3. 语言简练，数据引用精确
4. 客观分析，不做具体买卖建议
5. 在结尾注明：以上为 AI 生成的市场日报，仅供参考`

// HandleMarketCollect triggers data collection + AI summary for a trade date.
// SSE stream: status -> snapshot -> chunk (AI) -> done
func (h *Handler) HandleMarketCollect(w http.ResponseWriter, r *http.Request, tc *tushare.Client, ms *live.MarketStore) {
	if tc == nil {
		writeErr(w, http.StatusServiceUnavailable, "TUSHARE_TOKEN required")
		return
	}

	date := r.URL.Query().Get("date")
	if date == "" {
		d, err := live.LatestTradeDate(tc)
		if err != nil {
			writeErr(w, http.StatusBadGateway, fmt.Sprintf("获取交易日失败: %v", err))
			return
		}
		date = d
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Check if already collected
	if ms.HasSnapshot(date) && ms.HasSummary(date) {
		snap, _ := ms.LoadSnapshot(date)
		summary, _ := ms.LoadSummary(date)
		if snap != nil && summary != "" {
			b, _ := json.Marshal(snap)
			sseEvent(w, "snapshot", string(b))
			sseQuoted(w, "chunk", summary)
			sseQuoted(w, "done", "已有缓存")
			return
		}
	}

	sseQuoted(w, "status", fmt.Sprintf("正在采集 %s 市场数据…", date))

	snap, err := live.CollectMarketSnapshot(tc, date)
	if err != nil {
		sseQuoted(w, "error", fmt.Sprintf("数据采集失败: %v", err))
		return
	}

	if err := ms.SaveSnapshot(snap); err != nil {
		log.Printf("market: save snapshot failed: %v", err)
	}

	b, _ := json.Marshal(snap)
	sseEvent(w, "snapshot", string(b))

	if !h.AI.Ready() {
		sseQuoted(w, "status", "AI 未配置，跳过日报生成")
		sseQuoted(w, "done", "仅数据")
		return
	}

	sseQuoted(w, "status", "数据采集完成，AI 正在生成市场日报…")

	userMsg := formatSnapshotForAI(snap)

	ch := make(chan string, 64)
	var streamErr error
	go func() { streamErr = h.AI.CallStream(MarketDailySystemPrompt, userMsg, ch) }()

	var summaryBuf strings.Builder
	for token := range ch {
		summaryBuf.WriteString(token)
		sseQuoted(w, "chunk", token)
	}

	if streamErr != nil {
		log.Printf("market daily AI error: %v", streamErr)
		sseQuoted(w, "error", fmt.Sprintf("AI 生成失败: %v", streamErr))
		return
	}

	summary := summaryBuf.String()
	if err := ms.SaveSummary(date, summary); err != nil {
		log.Printf("market: save summary failed: %v", err)
	}

	sseQuoted(w, "done", "完成")
}

// HandleMarketDaily returns snapshot + summary JSON for a specific date.
func (h *Handler) HandleMarketDaily(w http.ResponseWriter, r *http.Request, ms *live.MarketStore) {
	date := r.URL.Query().Get("date")
	if date == "" {
		writeErr(w, http.StatusBadRequest, "date parameter required")
		return
	}

	snap, err := ms.LoadSnapshot(date)
	if err != nil {
		writeErr(w, http.StatusNotFound, fmt.Sprintf("no data for %s", date))
		return
	}

	summary, _ := ms.LoadSummary(date)

	result := map[string]any{
		"snapshot": snap,
		"summary":  summary,
	}
	b, _ := json.Marshal(result)
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

// HandleMarketHistory returns recent N days of market snapshots.
func (h *Handler) HandleMarketHistory(w http.ResponseWriter, r *http.Request, ms *live.MarketStore) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}
	if days < 1 {
		days = 1
	}
	if days > 365 {
		days = 365
	}

	items, err := ms.ListSnapshots(days)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []live.MarketDailySummaryItem{}
	}

	b, _ := json.Marshal(items)
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func formatSnapshotForAI(snap *live.MarketSnapshot) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# A股市场日报数据 — %s\n\n", snap.Date))

	sb.WriteString("## 三大指数\n\n")
	sb.WriteString("| 指数 | 收盘 | 涨跌幅 | 成交额(亿) |\n")
	sb.WriteString("|------|------|--------|------------|\n")
	for _, idx := range snap.Indices {
		sb.WriteString(fmt.Sprintf("| %s | %.2f | %.2f%% | %.1f |\n",
			idx.Name, idx.Close, idx.ChgPct, idx.Amount))
	}

	sb.WriteString(fmt.Sprintf("\n## 市场宽度\n\n- 上涨: %d 家\n- 下跌: %d 家\n- 平盘: %d 家\n- 涨停: %d 家\n- 跌停: %d 家\n- 总计: %d 家\n",
		snap.Breadth.UpCount, snap.Breadth.DownCount, snap.Breadth.FlatCount,
		snap.Breadth.LimitUpCount, snap.Breadth.LimitDownCount, snap.Breadth.Total))

	sb.WriteString(fmt.Sprintf("\n## 成交量\n\n- 全市场成交额: %.1f 亿\n- 较前日: %.2f%%\n",
		snap.TotalAmount, snap.AmountChgPct))

	sb.WriteString(fmt.Sprintf("\n## 北向资金\n\n- 沪股通净买入: %.2f 亿\n- 深股通净买入: %.2f 亿\n- 合计: %.2f 亿\n",
		snap.NorthFlow.HgtNetBuy, snap.NorthFlow.SgtNetBuy, snap.NorthFlow.TotalNet))

	if len(snap.TopSectors) > 0 {
		sb.WriteString("\n## 领涨板块\n\n| 行业 | 平均涨幅 | 平均PE |\n|------|----------|--------|\n")
		for _, s := range snap.TopSectors {
			sb.WriteString(fmt.Sprintf("| %s | %.2f%% | %.1f |\n", s.Name, s.ChgPct, s.AvgPe))
		}
	}

	if len(snap.BottomSectors) > 0 {
		sb.WriteString("\n## 领跌板块\n\n| 行业 | 平均跌幅 | 平均PE |\n|------|----------|--------|\n")
		for _, s := range snap.BottomSectors {
			sb.WriteString(fmt.Sprintf("| %s | %.2f%% | %.1f |\n", s.Name, s.ChgPct, s.AvgPe))
		}
	}

	return sb.String()
}
