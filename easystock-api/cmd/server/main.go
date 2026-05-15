package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"easystock/api/internal/live"
	"easystock/api/internal/report"
	"easystock/api/internal/tushare"

	"github.com/joho/godotenv"
)

func main() {
	loadDotEnv()

	token := strings.TrimSpace(os.Getenv("TUSHARE_TOKEN"))
	var tc *tushare.Client
	if token != "" {
		tc = tushare.NewClient(token)
		log.Printf("Tushare: GET /api/picks, /api/stocks/{code}, /api/sectors* use live data only (no mock fallback when token missing).")
		live.WarmUpPicks(tc)
		startMarketScheduler(tc)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		tok := "false"
		if tc != nil {
			tok = "true"
		}
		_, _ = w.Write([]byte(`{"ok":true,"tushare":` + tok + `}`))
	})

	// GET /api/tushare/ping — 单次最轻 trade_cal，总额 10s，用于区分「到 Tushare 不通」与「某个重接口超时」。
	mux.HandleFunc("GET /api/tushare/ping", func(w http.ResponseWriter, _ *http.Request) {
		if tc == nil {
			writeError(w, http.StatusServiceUnavailable, "TUSHARE_TOKEN is required (mock fallback disabled)")
			return
		}
		took, err := tc.PingTradeCal(10 * time.Second)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		out := map[string]any{
			"ok":       true,
			"took_ms":  took.Milliseconds(),
			"base_url": tc.EffectiveBaseURL(),
			"api":      "trade_cal",
		}
		b, _ := json.Marshal(out)
		writeJSON(w, b)
	})

	mux.HandleFunc("GET /api/picks/styles", func(w http.ResponseWriter, _ *http.Request) {
		b, _ := json.Marshal(live.ListPickStyles())
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})

	mux.HandleFunc("GET /api/picks", func(w http.ResponseWriter, r *http.Request) {
		if tc == nil {
			writeError(w, http.StatusServiceUnavailable, "TUSHARE_TOKEN is required (mock fallback disabled)")
			return
		}
		b, err := live.PicksJSON(tc, r.URL.Query())
		if err != nil {
			log.Printf("tushare picks: %v", err)
			writeError(w, http.StatusBadGateway, tc.FormatErrWithTimeoutProbe(err))
			return
		}
		writeJSON(w, b)
	})

	mux.HandleFunc("GET /api/sectors", func(w http.ResponseWriter, _ *http.Request) {
		if tc == nil {
			writeError(w, http.StatusServiceUnavailable, "TUSHARE_TOKEN is required (mock fallback disabled)")
			return
		}
		b, err := live.SectorsJSON(tc)
		if err != nil {
			log.Printf("tushare sectors: %v", err)
			writeError(w, http.StatusBadGateway, tc.FormatErrWithTimeoutProbe(err))
			return
		}
		writeJSON(w, b)
	})

	mux.HandleFunc("GET /api/sectors/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if tc == nil {
			writeError(w, http.StatusServiceUnavailable, "TUSHARE_TOKEN is required (mock fallback disabled)")
			return
		}
		b, err := live.SectorDetailJSON(tc, id)
		if err != nil {
			if errors.Is(err, live.ErrSectorNotFound) {
				http.NotFound(w, r)
				return
			}
			log.Printf("tushare sector %s: %v", id, err)
			writeError(w, http.StatusBadGateway, tc.FormatErrWithTimeoutProbe(err))
			return
		}
		writeJSON(w, b)
	})

	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		if tc == nil {
			writeError(w, http.StatusServiceUnavailable, "TUSHARE_TOKEN is required")
			return
		}
		q := r.URL.Query().Get("q")
		b, err := live.SearchStocks(tc, q)
		if err != nil {
			log.Printf("search: %v", err)
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, b)
	})

	mux.HandleFunc("GET /api/stocks/{code}", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		if tc == nil {
			writeError(w, http.StatusServiceUnavailable, "TUSHARE_TOKEN is required (mock fallback disabled)")
			return
		}
		b, err := live.StockDetailJSON(tc, code)
		if err != nil {
			if errors.Is(err, live.ErrStockNotFound) {
				http.NotFound(w, r)
				return
			}
			log.Printf("tushare stock %s: %v", code, err)
			writeError(w, http.StatusBadGateway, tc.FormatErrWithTimeoutProbe(err))
			return
		}
		writeJSON(w, b)
	})

	mux.HandleFunc("GET /api/stocks/{code}/pe-history", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		if tc == nil {
			writeError(w, http.StatusServiceUnavailable, "TUSHARE_TOKEN is required")
			return
		}
		b, err := live.PeHistoryJSON(tc, code)
		if err != nil {
			log.Printf("pe-history %s: %v", code, err)
			writeError(w, http.StatusBadGateway, tc.FormatErrWithTimeoutProbe(err))
			return
		}
		writeJSON(w, b)
	})

	rh := report.NewHandler()
	ms := live.NewMarketStore()

	// Market daily APIs
	mux.HandleFunc("GET /api/market/collect", func(w http.ResponseWriter, r *http.Request) {
		rh.HandleMarketCollect(w, r, tc, ms)
	})
	mux.HandleFunc("GET /api/market/daily", func(w http.ResponseWriter, r *http.Request) {
		rh.HandleMarketDaily(w, r, ms)
	})
	mux.HandleFunc("GET /api/market/history", func(w http.ResponseWriter, r *http.Request) {
		rh.HandleMarketHistory(w, r, ms)
	})

	mux.HandleFunc("POST /api/reports/upload", rh.HandleUpload)
	mux.HandleFunc("POST /api/reports/upload/stream", rh.HandleUploadStream)
	mux.HandleFunc("GET /api/reports/{stock_code}", rh.HandleList)
	mux.HandleFunc("GET /api/reports/{stock_code}/analysis", rh.HandleAnalysis)
	mux.HandleFunc("DELETE /api/reports/{stock_code}/{year}", rh.HandleDelete)

	mux.HandleFunc("POST /api/chat", rh.HandleChat)
	mux.HandleFunc("GET /api/cninfo/reports", rh.HandleCninfoSearch)
	mux.HandleFunc("GET /api/cninfo/news", rh.HandleCninfoNews)
	mux.HandleFunc("POST /api/cninfo/analyze", rh.HandleCninfoAnalyze)
	mux.HandleFunc("GET /api/picks/ai-recommend", func(w http.ResponseWriter, r *http.Request) {
		rh.HandleAiRecommend(w, r, tc)
	})

	mux.HandleFunc("GET /api/wiki", rh.HandleWikiList)
	mux.HandleFunc("GET /api/wiki/{stock_code}/meta", rh.HandleWikiMeta)
	mux.HandleFunc("GET /api/wiki/{stock_code}/{year}", rh.HandleWikiYear)
	mux.HandleFunc("GET /api/wiki/{stock_code}", rh.HandleWiki)

	if rh.AI.Ready() {
		log.Printf("Report AI: ready — %s", rh.AI.ProviderInfo())
	} else {
		log.Printf("Report AI: neither CURSOR_API_KEY nor AI_API_KEY is set — upload/analysis endpoints will return 503")
	}

	addr := ":4000"
	if p := os.Getenv("PORT"); p != "" {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, ":") {
			addr = p
		} else {
			addr = ":" + p
		}
	}

	h := corsMiddleware(mux)
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// loadDotEnv loads optional .env files (does not override existing OS env).
// Tries monorepo path first, then cwd — so `go run` works from repo root or easystock-api/.
func loadDotEnv() {
	for _, p := range []string{"easystock-api/.env", ".env"} {
		if err := godotenv.Load(p); err == nil {
			log.Printf("env: loaded %q", p)
		}
	}
}

// startMarketScheduler launches a background goroutine that collects market
// data and generates an AI summary every trading day after 18:30 CST.
func startMarketScheduler(tc *tushare.Client) {
	ms := live.NewMarketStore()
	ai := report.NewAIClient()
	go func() {
		cst := time.FixedZone("CST", 8*3600)
		for {
			now := time.Now().In(cst)
			// Target 18:30 CST today (or tomorrow if already past)
			target := time.Date(now.Year(), now.Month(), now.Day(), 18, 30, 0, 0, cst)
			if now.After(target) {
				target = target.Add(24 * time.Hour)
			}
			sleepDur := time.Until(target)
			log.Printf("market scheduler: next check at %s (sleeping %s)", target.Format("2006-01-02 15:04"), sleepDur.Round(time.Second))
			time.Sleep(sleepDur)

			date, err := live.LatestTradeDate(tc)
			if err != nil {
				log.Printf("market scheduler: skip — %v", err)
				continue
			}

			if ms.HasSnapshot(date) {
				log.Printf("market scheduler: %s already collected, skip", date)
				continue
			}

			log.Printf("market scheduler: collecting %s…", date)
			snap, err := live.CollectMarketSnapshot(tc, date)
			if err != nil {
				log.Printf("market scheduler: collect failed — %v", err)
				continue
			}
			if err := ms.SaveSnapshot(snap); err != nil {
				log.Printf("market scheduler: save snapshot failed — %v", err)
			}

			if ai.Ready() {
				prompt := formatSnapshotPrompt(snap)
				ch := make(chan string, 64)
				var buf strings.Builder
				go func() {
					_ = ai.CallStream(report.MarketDailySystemPrompt, prompt, ch)
				}()
				for tok := range ch {
					buf.WriteString(tok)
				}
				if buf.Len() > 0 {
					if err := ms.SaveSummary(date, buf.String()); err != nil {
						log.Printf("market scheduler: save summary failed — %v", err)
					} else {
						log.Printf("market scheduler: %s summary saved (%d chars)", date, buf.Len())
					}
				}
			}
		}
	}()
}

func formatSnapshotPrompt(snap *live.MarketSnapshot) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# A股市场日报数据 — %s\n\n", snap.Date))
	sb.WriteString("## 三大指数\n\n| 指数 | 收盘 | 涨跌幅 | 成交额(亿) |\n|------|------|--------|------------|\n")
	for _, idx := range snap.Indices {
		sb.WriteString(fmt.Sprintf("| %s | %.2f | %.2f%% | %.1f |\n", idx.Name, idx.Close, idx.ChgPct, idx.Amount))
	}
	sb.WriteString(fmt.Sprintf("\n## 市场宽度\n- 上涨: %d / 下跌: %d / 平盘: %d\n- 涨停: %d / 跌停: %d\n",
		snap.Breadth.UpCount, snap.Breadth.DownCount, snap.Breadth.FlatCount,
		snap.Breadth.LimitUpCount, snap.Breadth.LimitDownCount))
	sb.WriteString(fmt.Sprintf("\n## 成交量\n- 全市场: %.1f 亿 (较前日 %.2f%%)\n", snap.TotalAmount, snap.AmountChgPct))
	sb.WriteString(fmt.Sprintf("\n## 北向资金\n- 沪股通: %.2f 亿 / 深股通: %.2f 亿 / 合计: %.2f 亿\n",
		snap.NorthFlow.HgtNetBuy, snap.NorthFlow.SgtNetBuy, snap.NorthFlow.TotalNet))
	if len(snap.TopSectors) > 0 {
		sb.WriteString("\n## 领涨板块\n")
		for _, s := range snap.TopSectors {
			sb.WriteString(fmt.Sprintf("- %s: %.2f%%\n", s.Name, s.ChgPct))
		}
	}
	if len(snap.BottomSectors) > 0 {
		sb.WriteString("\n## 领跌板块\n")
		for _, s := range snap.BottomSectors {
			sb.WriteString(fmt.Sprintf("- %s: %.2f%%\n", s.Name, s.ChgPct))
		}
	}
	return sb.String()
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
