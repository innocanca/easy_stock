package report

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"easystock/api/internal/live"
	"easystock/api/internal/tushare"
)

const aiRecommendSystemPrompt = `你是一位资深的 A 股投资分析师，擅长根据量化指标和基本面数据进行选股。

用户会给你一组经过初筛的候选股票（已按某种投资风格的综合评分排序），以及当前的投资风格偏好。

你的任务：
1. 从候选池中精选 3~5 只最值得关注的股票
2. 必须来自不同行业/板块，不能出现同行业的两只股票
3. 综合考虑以下因素：
   - 投资风格匹配度（如价值投资偏好高 ROE、低负债；高成长偏好利润增速等）
   - PE 估值水平（是否合理或低估）
   - ROE 水平（盈利能力）
   - 净利润同比增速（成长性）
   - 换手率（流动性和市场关注度，过高可能说明投机过热）
   - 四维评分的均衡性
4. 给出每只股票的推荐理由（2~3句话，言简意赅，突出核心亮点和风险点）
5. 最后给出一段整体组合点评（为什么这几只股票搭配在一起是合理的）

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
	sb.WriteString("| 排名 | 代码 | 名称 | 行业 | 综合分 | PE | ROE | 净利同比 | 换手率 | 价值 | 估值 | 确定性 | 成长 |\n")
	sb.WriteString("|------|------|------|------|--------|-----|------|----------|--------|------|------|--------|------|\n")

	for i, it := range items {
		sector := it.Sector
		if sector == "" {
			sector = "—"
		}
		dims := it.Dimensions
		sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %d | %.1f | %.1f%% | %.1f%% | %.2f%% | %d | %d | %d | %d |\n",
			i+1, it.Code, it.Name, sector, it.Score,
			it.Pe, it.Roe, it.ProfitGrowthYoy, it.TurnoverRate,
			dims["value"], dims["valuation"], dims["certainty"], dims["growth"],
		))
	}

	sb.WriteString("\n请从以上候选中精选 3~5 只来自不同行业的股票，给出推荐理由和组合点评。\n")

	return sb.String()
}
