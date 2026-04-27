package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"easystock/api/internal/live"
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

	mux.HandleFunc("GET /api/picks", func(w http.ResponseWriter, r *http.Request) {
		if tc == nil {
			writeError(w, http.StatusServiceUnavailable, "TUSHARE_TOKEN is required (mock fallback disabled)")
			return
		}
		b, err := live.PicksJSON(tc)
		if err != nil {
			log.Printf("tushare picks: %v", err)
			writeError(w, http.StatusBadGateway, err.Error())
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
			writeError(w, http.StatusBadGateway, err.Error())
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
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, b)
	})

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

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
