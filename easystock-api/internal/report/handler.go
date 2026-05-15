package report

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	Store *Store
	AI    *AIClient
}

func NewHandler() *Handler {
	return &Handler{
		Store: NewStore(),
		AI:    NewAIClient(),
	}
}

// ---- SSE helpers ----

func sseEvent(w http.ResponseWriter, event, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// sseQuoted sends a JSON-encoded string as one SSE data line (safe for newlines).
func sseQuoted(w http.ResponseWriter, event, s string) {
	b, _ := json.Marshal(s)
	sseEvent(w, event, string(b))
}

// sseKeepalive sends an SSE comment line so proxies do not treat the stream as idle.
func sseKeepalive(w http.ResponseWriter) {
	fmt.Fprintf(w, ": ping\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// ---- Upload (original synchronous) ----

func (h *Handler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if !h.AI.Ready() {
		writeErr(w, http.StatusServiceUnavailable, "AI_API_KEY is not configured")
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "parse form: "+err.Error())
		return
	}

	stockCode := strings.TrimSpace(r.FormValue("stock_code"))
	stockName := strings.TrimSpace(r.FormValue("stock_name"))
	yearStr := strings.TrimSpace(r.FormValue("year"))
	if stockCode == "" || stockName == "" || yearStr == "" {
		writeErr(w, http.StatusBadRequest, "stock_code, stock_name, year are required")
		return
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2000 || year > 2030 {
		writeErr(w, http.StatusBadRequest, "invalid year")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "file is required: "+err.Error())
		return
	}
	defer file.Close()

	if err := h.Store.EnsureDirs(stockCode); err != nil {
		writeErr(w, http.StatusInternalServerError, "create dirs: "+err.Error())
		return
	}

	pdfPath := h.Store.PDFPath(stockCode, year)
	out, err := os.Create(pdfPath)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "save file: "+err.Error())
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		writeErr(w, http.StatusInternalServerError, "write file: "+err.Error())
		return
	}
	out.Close()

	log.Printf("report: extracting text from %s", pdfPath)
	pdfText, err := ExtractPDFText(pdfPath)
	if err != nil {
		log.Printf("report: PDF text extraction failed: %v, will send minimal context to AI", err)
		pdfText = fmt.Sprintf("PDF文本提取失败。请基于你的知识，提供 %s（%s）%d年年度报告的关键财务数据。如果不确定具体数字请设为0。", stockName, stockCode, year)
	} else if len(strings.TrimSpace(pdfText)) < 200 {
		log.Printf("report: extracted text too short (%d chars), supplementing with context", len(pdfText))
		pdfText = fmt.Sprintf("PDF提取的文本较少，可能是扫描版PDF。以下是提取到的内容：\n%s\n\n请基于你的知识补充 %s（%s）%d年的关键财务数据。如果不确定具体数字请设为0。",
			pdfText, stockName, stockCode, year)
	} else {
		log.Printf("report: extracted %d chars from PDF", len(pdfText))
	}

	log.Printf("report: sending to AI for analysis (%s %d)", stockCode, year)
	extractBody := ClampReportText(pdfText, MaxRunesForJSONExtract)
	data, err := h.AI.ExtractFinancials(extractBody, stockCode, stockName, year)
	if err != nil {
		log.Printf("report: AI extraction failed: %v", err)
		writeErr(w, http.StatusBadGateway, "AI analysis failed: "+err.Error())
		return
	}

	if err := h.Store.SaveReport(data); err != nil {
		writeErr(w, http.StatusInternalServerError, "save report: "+err.Error())
		return
	}

	log.Printf("report: successfully processed %s %d", stockCode, year)
	writeJSON(w, UploadResponse{Success: true, Message: "ok", Data: data})
}

// ---- Upload Stream (SSE) ----

func (h *Handler) HandleUploadStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if !h.AI.Ready() {
		sseQuoted(w, "error", "AI provider is not configured")
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil {
		sseQuoted(w, "error", "parse form: "+err.Error())
		return
	}

	stockCode := strings.TrimSpace(r.FormValue("stock_code"))
	stockName := strings.TrimSpace(r.FormValue("stock_name"))
	yearStr := strings.TrimSpace(r.FormValue("year"))
	if stockCode == "" || stockName == "" || yearStr == "" {
		sseQuoted(w, "error", "stock_code, stock_name, year are required")
		return
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2000 || year > 2030 {
		sseQuoted(w, "error", "invalid year")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		sseQuoted(w, "error", "file is required: "+err.Error())
		return
	}
	defer file.Close()

	if err := h.Store.EnsureDirs(stockCode); err != nil {
		sseQuoted(w, "error", "create dirs: "+err.Error())
		return
	}

	// Save PDF
	sseQuoted(w, "status", "正在保存文件…")
	pdfPath := h.Store.PDFPath(stockCode, year)
	out, err := os.Create(pdfPath)
	if err != nil {
		sseQuoted(w, "error", "save file: "+err.Error())
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		sseQuoted(w, "error", "write file: "+err.Error())
		return
	}
	out.Close()

	// Extract PDF text
	sseQuoted(w, "status", "正在提取PDF文本…")
	pdfText, err := ExtractPDFText(pdfPath)
	if err != nil {
		log.Printf("report: PDF text extraction failed: %v", err)
		pdfText = fmt.Sprintf("PDF文本提取失败。请基于你的知识，提供 %s（%s）%d年年度报告的关键财务数据。如果不确定具体数字请设为0。", stockName, stockCode, year)
	} else if len(strings.TrimSpace(pdfText)) < 200 {
		pdfText = fmt.Sprintf("PDF提取的文本较少，可能是扫描版PDF。以下是提取到的内容：\n%s\n\n请基于你的知识补充 %s（%s）%d年的关键财务数据。如果不确定具体数字请设为0。",
			pdfText, stockName, stockCode, year)
	} else {
		log.Printf("report: extracted %d chars from PDF", len(pdfText))
	}

	existingWiki, _ := h.Store.MergeWikisExceptYear(stockCode, year)
	hasContext := existingWiki != ""

	chunks := SplitIntoChunks(pdfText, MaxRunesPerChunk, ChunkOverlapRunes)
	totalRunes := RuneCount(pdfText)

	if len(chunks) == 1 {
		sseQuoted(w, "status", fmt.Sprintf("已提取约 %d 字，正在流式生成完整分析…", totalRunes))
		wikiContent := h.streamSinglePass(w, chunks[0], stockName, stockCode, year, existingWiki, hasContext)
		if wikiContent == "" {
			return
		}
		h.saveCninfoWiki(w, stockCode, year, wikiContent)
		return
	}

	sseQuoted(w, "status", fmt.Sprintf("年报共约 %d 字，将分 %d 段逐段深度分析…", totalRunes, len(chunks)))
	chunkResults := make([]string, 0, len(chunks))
	for i, chunk := range chunks {
		sseQuoted(w, "status", fmt.Sprintf("正在分析第 %d/%d 段（约 %d 字）…", i+1, len(chunks), RuneCount(chunk)))
		sseQuoted(w, "wiki_chunk", fmt.Sprintf("\n\n---\n\n## 📖 第 %d/%d 段分析\n\n", i+1, len(chunks)))

		sp := fmt.Sprintf(chunkAnalysisPrompt, i+1, len(chunks))
		um := fmt.Sprintf("## %s（%s）%d年年度报告 — 第%d段\n\n%s", stockName, stockCode, year, i+1, chunk)

		ch := make(chan string, 64)
		var streamErr error
		go func() { streamErr = h.AI.CallStream(sp, um, ch) }()
		var buf strings.Builder
		for token := range ch {
			buf.WriteString(token)
			sseQuoted(w, "wiki_chunk", token)
		}
		if streamErr != nil {
			sseQuoted(w, "status", fmt.Sprintf("第 %d 段分析出错，继续…", i+1))
			continue
		}
		chunkResults = append(chunkResults, buf.String())
	}

	if len(chunkResults) == 0 {
		sseQuoted(w, "error", "所有分段分析均失败")
		return
	}

	sseQuoted(w, "status", "分段分析完成，正在综合生成最终报告…")
	sseQuoted(w, "wiki_chunk", "\n\n---\n\n# 📊 综合分析报告\n\n")

	var synthUser strings.Builder
	if hasContext {
		synthUser.WriteString("## 已有知识库\n\n")
		synthUser.WriteString(existingWiki)
		synthUser.WriteString("\n\n---\n\n")
	}
	synthUser.WriteString(fmt.Sprintf("## %s（%s）%d年年度报告 — 各段分析汇总\n\n", stockName, stockCode, year))
	for i, cr := range chunkResults {
		synthUser.WriteString(fmt.Sprintf("### 第 %d 段分析\n\n%s\n\n", i+1, cr))
	}

	synthPrompt := chunkSynthesisPrompt
	if hasContext {
		synthPrompt = chunkSynthesisWithContextPrompt
	}

	synthCh := make(chan string, 64)
	var synthErr error
	go func() { synthErr = h.AI.CallStream(synthPrompt, synthUser.String(), synthCh) }()
	var finalBuf strings.Builder
	for token := range synthCh {
		finalBuf.WriteString(token)
		sseQuoted(w, "wiki_chunk", token)
	}
	if synthErr != nil {
		sseQuoted(w, "error", "综合报告生成失败: "+synthErr.Error())
		return
	}
	h.saveCninfoWiki(w, stockCode, year, finalBuf.String())
}

// ---- List / Analysis / Delete ----

func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	stockCode := r.PathValue("stock_code")
	if stockCode == "" {
		writeErr(w, http.StatusBadRequest, "stock_code is required")
		return
	}
	reports, err := h.Store.ListReports(stockCode)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if reports == nil {
		reports = []ReportData{}
	}
	writeJSON(w, ListResponse{StockCode: stockCode, Reports: reports})
}

func (h *Handler) HandleAnalysis(w http.ResponseWriter, r *http.Request) {
	stockCode := r.PathValue("stock_code")
	if stockCode == "" {
		writeErr(w, http.StatusBadRequest, "stock_code is required")
		return
	}

	reports, err := h.Store.ListReports(stockCode)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(reports) == 0 {
		writeErr(w, http.StatusNotFound, "no reports found for "+stockCode)
		return
	}

	result := &AnalysisResult{
		StockCode: stockCode,
		StockName: reports[0].StockName,
		Years:     reports,
	}

	if len(reports) >= 2 && h.AI.Ready() {
		forceRefresh := r.URL.Query().Get("refresh") == "1"
		existing, loadErr := h.Store.LoadAnalysis(stockCode)

		if !forceRefresh && loadErr == nil && existing != nil && len(existing.Years) == len(reports) {
			result.Summary = existing.Summary
		} else {
			log.Printf("report: generating multi-year analysis for %s (%d years)", stockCode, len(reports))
			summary, err := h.AI.MultiYearAnalysis(reports)
			if err != nil {
				log.Printf("report: multi-year analysis failed: %v", err)
				result.Summary = "多年度综合分析生成失败: " + err.Error()
			} else {
				result.Summary = summary
				_ = h.Store.SaveAnalysis(result)
			}
		}
	}

	writeJSON(w, result)
}

func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	stockCode := r.PathValue("stock_code")
	yearStr := r.PathValue("year")
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid year")
		return
	}
	_ = h.Store.DeleteReport(stockCode, year)
	writeJSON(w, map[string]bool{"ok": true})
}

// ---- Wiki endpoints ----

func (h *Handler) HandleWiki(w http.ResponseWriter, r *http.Request) {
	stockCode := r.PathValue("stock_code")
	if stockCode == "" {
		writeErr(w, http.StatusBadRequest, "stock_code is required")
		return
	}
	merged, err := h.Store.MergeWikis(stockCode)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if merged == "" {
		writeErr(w, http.StatusNotFound, "no wiki found for "+stockCode)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Write([]byte(merged))
}

func (h *Handler) HandleWikiYear(w http.ResponseWriter, r *http.Request) {
	stockCode := r.PathValue("stock_code")
	yearStr := r.PathValue("year")
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid year")
		return
	}
	content, err := h.Store.LoadWiki(stockCode, year)
	if err != nil {
		writeErr(w, http.StatusNotFound, "wiki not found")
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Write([]byte(content))
}

func (h *Handler) HandleWikiList(w http.ResponseWriter, _ *http.Request) {
	codes, err := h.Store.ListStocksWithWiki()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if codes == nil {
		codes = []string{}
	}
	writeJSON(w, map[string]any{"stocks": codes})
}

func (h *Handler) HandleWikiMeta(w http.ResponseWriter, r *http.Request) {
	stockCode := r.PathValue("stock_code")
	wikis, err := h.Store.ListWikis(stockCode)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	years := make([]int, 0, len(wikis))
	for y := range wikis {
		years = append(years, y)
	}
	sort.Ints(years)
	reports, _ := h.Store.ListReports(stockCode)
	stockName := stockCode
	if len(reports) > 0 {
		stockName = reports[0].StockName
	}
	writeJSON(w, map[string]any{
		"stock_code": stockCode,
		"stock_name": stockName,
		"years":      years,
	})
}

// ---- Cninfo (巨潮资讯网) ----

func (h *Handler) HandleCninfoNews(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if code == "" {
		writeErr(w, http.StatusBadRequest, "code is required")
		return
	}
	items, err := SearchCninfoNews(code, name, 30)
	if err != nil {
		log.Printf("cninfo news %s: %v", code, err)
		writeErr(w, http.StatusBadGateway, "巨潮资讯查询失败: "+err.Error())
		return
	}
	writeJSON(w, items)
}

func (h *Handler) HandleCninfoSearch(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if code == "" {
		writeErr(w, http.StatusBadRequest, "code is required")
		return
	}
	reports, err := SearchCninfoReports(code, name)
	if err != nil {
		log.Printf("cninfo search %s: %v", code, err)
		writeErr(w, http.StatusBadGateway, "巨潮查询失败: "+err.Error())
		return
	}
	writeJSON(w, reports)
}

// HandleCninfoAnalyze downloads a report from cninfo and streams AI analysis via SSE.
// For long reports it splits text into chunks, analyzes each, then synthesizes.
func (h *Handler) HandleCninfoAnalyze(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if !h.AI.Ready() {
		sseQuoted(w, "error", "AI provider is not configured")
		return
	}

	var req struct {
		StockCode   string `json:"stock_code"`
		StockName   string `json:"stock_name"`
		Year        int    `json:"year"`
		DownloadURL string `json:"download_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sseQuoted(w, "error", "invalid JSON: "+err.Error())
		return
	}
	if req.StockCode == "" || req.Year == 0 || req.DownloadURL == "" {
		sseQuoted(w, "error", "stock_code, year, download_url are required")
		return
	}
	if req.StockName == "" {
		req.StockName = req.StockCode
	}

	if err := h.Store.EnsureDirs(req.StockCode); err != nil {
		sseQuoted(w, "error", "create dirs: "+err.Error())
		return
	}

	sseQuoted(w, "status", fmt.Sprintf("正在从巨潮资讯网下载 %d 年度报告…", req.Year))
	pdfPath := h.Store.PDFPath(req.StockCode, req.Year)
	if err := DownloadCninfoPDF(req.DownloadURL, pdfPath); err != nil {
		log.Printf("cninfo download %s %d: %v", req.StockCode, req.Year, err)
		sseQuoted(w, "error", "年报下载失败: "+err.Error())
		return
	}
	sseQuoted(w, "status", "下载完成，正在提取PDF文本…")

	pdfText, err := ExtractPDFText(pdfPath)
	if err != nil {
		log.Printf("cninfo: PDF text extraction failed: %v", err)
		pdfText = fmt.Sprintf("PDF文本提取失败。请基于你的知识分析 %s（%s）%d年年度报告。", req.StockName, req.StockCode, req.Year)
	} else if len(strings.TrimSpace(pdfText)) < 200 {
		pdfText = fmt.Sprintf("PDF提取的文本较少：\n%s\n\n请基于你的知识补充分析 %s（%s）%d年年度报告。",
			pdfText, req.StockName, req.StockCode, req.Year)
	} else {
		log.Printf("cninfo: extracted %d chars (%d runes) from PDF", len(pdfText), RuneCount(pdfText))
	}

	existingWiki, _ := h.Store.MergeWikisExceptYear(req.StockCode, req.Year)
	hasContext := existingWiki != ""

	chunks := SplitIntoChunks(pdfText, MaxRunesPerChunk, ChunkOverlapRunes)
	totalRunes := RuneCount(pdfText)

	if len(chunks) == 1 {
		// Single-pass: text fits in one call
		sseQuoted(w, "status", fmt.Sprintf("已提取约 %d 字，正在流式生成完整分析…", totalRunes))
		wikiContent := h.streamSinglePass(w, chunks[0], req.StockName, req.StockCode, req.Year, existingWiki, hasContext)
		if wikiContent == "" {
			return
		}
		h.saveCninfoWiki(w, req.StockCode, req.Year, wikiContent)
		return
	}

	// Multi-pass: chunked analysis then synthesis
	sseQuoted(w, "status", fmt.Sprintf("年报共约 %d 字，将分 %d 段逐段深度分析…", totalRunes, len(chunks)))

	chunkResults := make([]string, 0, len(chunks))
	for i, chunk := range chunks {
		sseQuoted(w, "status", fmt.Sprintf("正在分析第 %d/%d 段（约 %d 字）…", i+1, len(chunks), RuneCount(chunk)))
		sseQuoted(w, "wiki_chunk", fmt.Sprintf("\n\n---\n\n## 📖 第 %d/%d 段分析\n\n", i+1, len(chunks)))

		systemPrompt := fmt.Sprintf(chunkAnalysisPrompt, i+1, len(chunks))
		userMsg := fmt.Sprintf("## %s（%s）%d年年度报告 — 第%d段\n\n%s", req.StockName, req.StockCode, req.Year, i+1, chunk)

		ch := make(chan string, 64)
		var streamErr error
		go func() { streamErr = h.AI.CallStream(systemPrompt, userMsg, ch) }()

		var buf strings.Builder
		for token := range ch {
			buf.WriteString(token)
			sseQuoted(w, "wiki_chunk", token)
		}
		if streamErr != nil {
			log.Printf("cninfo: chunk %d/%d failed: %v", i+1, len(chunks), streamErr)
			sseQuoted(w, "status", fmt.Sprintf("第 %d 段分析出错，继续下一段…", i+1))
			continue
		}
		chunkResults = append(chunkResults, buf.String())
	}

	if len(chunkResults) == 0 {
		sseQuoted(w, "error", "所有分段分析均失败")
		return
	}

	// Synthesis pass
	sseQuoted(w, "status", "所有分段分析完成，正在综合生成最终报告…")
	sseQuoted(w, "wiki_chunk", "\n\n---\n\n# 📊 综合分析报告\n\n")

	var synthUser strings.Builder
	if hasContext {
		synthUser.WriteString("## 已有知识库\n\n")
		synthUser.WriteString(existingWiki)
		synthUser.WriteString("\n\n---\n\n")
	}
	synthUser.WriteString(fmt.Sprintf("## %s（%s）%d年年度报告 — 各段分析汇总\n\n", req.StockName, req.StockCode, req.Year))
	for i, cr := range chunkResults {
		synthUser.WriteString(fmt.Sprintf("### 第 %d 段分析\n\n%s\n\n", i+1, cr))
	}

	synthPrompt := chunkSynthesisPrompt
	if hasContext {
		synthPrompt = chunkSynthesisWithContextPrompt
	}

	synthCh := make(chan string, 64)
	var synthErr error
	go func() { synthErr = h.AI.CallStream(synthPrompt, synthUser.String(), synthCh) }()

	var finalBuf strings.Builder
	for token := range synthCh {
		finalBuf.WriteString(token)
		sseQuoted(w, "wiki_chunk", token)
	}

	if synthErr != nil {
		log.Printf("cninfo: synthesis failed: %v", synthErr)
		sseQuoted(w, "error", "综合报告生成失败: "+synthErr.Error())
		return
	}

	h.saveCninfoWiki(w, req.StockCode, req.Year, finalBuf.String())
}

func (h *Handler) streamSinglePass(w http.ResponseWriter, text, stockName, stockCode string, year int, existingWiki string, hasContext bool) string {
	systemPrompt := h.AI.SummaryStreamSystemPrompt(hasContext)
	userMsg := h.AI.WikiUserMsg(text, stockName, stockCode, year, existingWiki)

	ch := make(chan string, 64)
	errCh := make(chan error, 1)
	start := time.Now()
	log.Printf("cninfo: single-pass start %s %d (provider=%s text_chars=%d ctx_chars=%d)", stockCode, year, h.AI.ProviderInfo(), len(text), len(existingWiki))
	go func() { errCh <- h.AI.CallStream(systemPrompt, userMsg, ch) }()

	var buf strings.Builder
	for token := range ch {
		buf.WriteString(token)
		sseQuoted(w, "wiki_chunk", token)
	}

	streamErr := <-errCh
	if streamErr != nil {
		log.Printf("cninfo: single-pass failed %s %d after %s: %v", stockCode, year, time.Since(start).Round(time.Millisecond), streamErr)
		sseQuoted(w, "error", "分析生成失败: "+streamErr.Error())
		return ""
	}
	log.Printf("cninfo: single-pass done %s %d dur=%s out_chars=%d", stockCode, year, time.Since(start).Round(time.Millisecond), buf.Len())
	if strings.TrimSpace(buf.String()) == "" {
		log.Printf("cninfo: single-pass warning: empty output %s %d (provider=%s)", stockCode, year, h.AI.ProviderInfo())
	}
	return buf.String()
}

func (h *Handler) saveCninfoWiki(w http.ResponseWriter, stockCode string, year int, raw string) {
	content := strings.TrimPrefix(strings.TrimSpace(raw), "```markdown")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(strings.TrimSpace(content), "```")
	content = strings.TrimSpace(content)

	if err := h.Store.SaveWiki(stockCode, year, content); err != nil {
		log.Printf("cninfo: save wiki failed: %v", err)
	}
	if strings.TrimSpace(content) == "" {
		log.Printf("cninfo: warning: saved empty analysis for %s %d (raw_chars=%d provider=%s raw_preview=%q)",
			stockCode, year, len(raw), h.AI.ProviderInfo(), truncStr(strings.TrimSpace(raw), 400))
	}
	log.Printf("cninfo: analysis saved for %s %d (%d chars)", stockCode, year, len(content))
	sseQuoted(w, "status", "分析已保存至知识库")
	sseQuoted(w, "done", "ok")
}

// ---- Utilities ----

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
