package report

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"easystock/api/internal/live"
	"easystock/api/internal/tushare"
)

const aiRecommendSystemPrompt = `你是一位资深的 A 股投资分析师，擅长根据量化指标、技术面和基本面数据进行综合选股。

用户会给你一组经过初筛的候选股票（已按某种投资风格的综合评分排序），以及当前的投资风格偏好。

数据列说明：
- 综合分：按当前风格权重计算的池内百分位排名得分（0~100）
- PE：市盈率 TTM，越低越便宜（但要结合行业看）
- ROE：年度净资产收益率，衡量盈利能力
- 净利同比：最近年报净利润同比增速，衡量成长性
- 换手率：当日自由流通换手率，反映市场交易活跃度
- 量比：当日成交量与过去5日均量之比，>1 放量、<1 缩量
- 收盘价：最新交易日收盘价（元）
- 价格分位：当前价格在近1年日线收盘价中的百分位（0=近1年最低，100=近1年最高）
- 四维评分（价值/估值/确定性/成长）：各维度原始评分（15~95）

你的任务：
1. 从候选池中精选 3~5 只最值得关注的股票
2. 必须来自不同行业/板块，不能出现同行业的两只股票
3. 综合考虑以下因素（按重要性）：
   a. 投资风格匹配度
   b. 价格历史分位（偏好处于中低位的标的，分位过高意味着追高风险）
   c. 量能特征（量比配合价格趋势判断，放量上涨或缩量回调为佳）
   d. PE 估值水平（是否合理或低估）
   e. ROE 水平（盈利能力是否优秀）
   f. 净利润同比增速（成长性是否突出）
   g. 换手率（流动性充足但不宜过高，>8% 可能过度投机）
   h. 四维评分的均衡性
4. 给出每只股票的推荐理由（3~4句话，必须涉及价格分位、量能和基本面）
5. 最后给出一段整体组合点评（行业分散度、风险收益特征）

输出格式要求：
- 使用 Markdown
- 每只推荐股票用 ## 标题（包含股票名称和代码）
- 推荐理由用要点列表
- 最后用 ## 组合点评 总结

注意：你是辅助参考，不构成投资建议。请在结尾加上风险提示。`

// HandleAiRecommend streams AI stock recommendations via SSE.
func (h *Handler) HandleAiRecommend(w http.ResponseWriter, r *http.Request, tc *tushare.Client) {
	if !h.AI.Ready() {
		writeErr(w, http.StatusServiceUnavailable, "AI not configured")
		return
	}
	if tc == nil {
		writeErr(w, http.StatusServiceUnavailable, "TUSHARE_TOKEN required")
		return
	}

	styleID := r.URL.Query().Get("style")
	if styleID == "" {
		styleID = "balanced"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sseQuoted(w, "status", "正在获取候选股票池…")

	topN := 30
	items, style, err := live.TopPicksForAI(tc, styleID, topN)
	if err != nil {
		sseQuoted(w, "error", fmt.Sprintf("获取候选池失败: %v", err))
		return
	}
	if len(items) == 0 {
		sseQuoted(w, "error", "候选池为空")
		return
	}

	sseQuoted(w, "status", fmt.Sprintf("已获取 %d 只候选股票，AI 正在分析选股…", len(items)))

	userMsg := buildAiRecommendUserMsg(items, style)

	ch := make(chan string, 64)
	var streamErr error
	go func() { streamErr = h.AI.CallStream(aiRecommendSystemPrompt, userMsg, ch) }()

	for token := range ch {
		sseQuoted(w, "chunk", token)
	}

	if streamErr != nil {
		log.Printf("ai-recommend stream error: %v", streamErr)
		sseQuoted(w, "error", fmt.Sprintf("AI 分析出错: %v", streamErr))
		return
	}

	sseQuoted(w, "done", "完成")
}

func buildAiRecommendUserMsg(items []live.PickItem, style live.PickStyle) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 当前投资风格：%s\n", style.Label))
	sb.WriteString(fmt.Sprintf("风格说明：%s\n\n", style.Desc))
	sb.WriteString("## 候选股票池（已按该风格综合分从高到低排序）\n\n")
	sb.WriteString("| # | 代码 | 名称 | 行业 | 分 | PE | ROE% | 净利同比% | 换手率% | 量比 | 收盘价 | 价格分位% | 价值 | 估值 | 确定性 | 成长 |\n")
	sb.WriteString("|---|------|------|------|-----|-----|------|-----------|---------|------|--------|-----------|------|------|--------|------|\n")

	for i, it := range items {
		sector := it.Sector
		if sector == "" {
			sector = "—"
		}
		dims := it.Dimensions
		pricePct := "N/A"
		if it.PricePct >= 0 {
			pricePct = fmt.Sprintf("%.1f", it.PricePct)
		}
		sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %d | %.1f | %.1f | %.1f | %.2f | %.2f | %.2f | %s | %d | %d | %d | %d |\n",
			i+1, it.Code, it.Name, sector, it.Score,
			it.Pe, it.Roe, it.ProfitGrowthYoy, it.TurnoverRate,
			it.VolumeRatio, it.Close, pricePct,
			dims["value"], dims["valuation"], dims["certainty"], dims["growth"],
		))
	}

	sb.WriteString("\n请从以上候选中精选 3~5 只来自不同行业的股票。重点关注价格分位处于中低位（<60%）、量能健康的标的。给出推荐理由和组合点评。\n")

	return sb.String()
}
