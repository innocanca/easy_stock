package report

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	data, err := h.AI.ExtractFinancials(pdfText, stockCode, stockName, year)
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

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
