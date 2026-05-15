package report

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

func (h *Handler) HandleChat(w http.ResponseWriter, r *http.Request) {
	if !h.AI.Ready() {
		http.Error(w, `{"error":"AI not configured"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		StockCode string `json:"stock_code"`
		StockName string `json:"stock_name"`
		Question  string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	req.StockCode = strings.TrimSpace(req.StockCode)
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		http.Error(w, `{"error":"question is required"}`, http.StatusBadRequest)
		return
	}

	// Gather context from stored reports
	var contextParts []string
	if req.StockCode != "" {
		reports, _ := h.Store.ListReports(req.StockCode)
		if len(reports) > 0 {
			for _, rpt := range reports {
				part := fmt.Sprintf(
					"%d年: 营收%.2f亿, 净利润%.2f亿, 毛利率%.1f%%, 净利率%.1f%%, ROE%.1f%%, 负债率%.1f%%, EPS%.2f, 每股分红%.2f",
					rpt.Year, rpt.Revenue, rpt.NetProfit,
					rpt.GrossMargin*100, rpt.NetMargin*100, rpt.ROE*100,
					rpt.DebtRatio*100, rpt.EPS, rpt.DividendPerShare,
				)
				if rpt.Highlights != "" {
					part += " 亮点:" + rpt.Highlights
				}
				contextParts = append(contextParts, part)
			}
		}
	}

	stockLabel := req.StockName
	if stockLabel == "" {
		stockLabel = req.StockCode
	}
	if stockLabel == "" {
		stockLabel = "用户询问的股票"
	}

	system := fmt.Sprintf(`你是一位专业的价值投资分析师助手。用户正在研究 %s（%s）。
请基于你的专业知识和以下已有财务数据，回答用户的问题。
回答要专业、简洁、有数据支撑。如果数据不足以回答，请说明并给出基于常识的判断。
使用中文回答。`, stockLabel, req.StockCode)

	if len(contextParts) > 0 {
		system += "\n\n已有年报数据:\n" + strings.Join(contextParts, "\n")
	}

	// SSE streaming
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := make(chan string, 64)
	errCh := make(chan error, 1)
	go func() {
		if h.AI.Mode == "cursor" {
			errCh <- h.AI.callCursorStream(system, req.Question, ch)
		} else {
			errCh <- h.AI.callHTTPStream(system, req.Question, ch)
		}
	}()

	for chunk := range ch {
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "event: chunk\ndata: %s\n\n", data)
		flusher.Flush()
	}

	if err := <-errCh; err != nil {
		log.Printf("chat AI error: %v", err)
		errMsg, _ := json.Marshal(err.Error())
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", errMsg)
		flusher.Flush()
		return
	}

	fmt.Fprintf(w, "event: done\ndata: \"ok\"\n\n")
	flusher.Flush()
}
