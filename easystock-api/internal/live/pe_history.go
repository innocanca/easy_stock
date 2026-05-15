package live

import (
	"encoding/json"
	"fmt"
	"math"

	"easystock/api/internal/tushare"
)

type PeHistoryPoint struct {
	Date string  `json:"date"`
	Pe   float64 `json:"pe"`
}

// PeHistoryJSON returns up to ~750 trading days (about 3 years) of PE TTM history.
func PeHistoryJSON(c *tushare.Client, tsCode string) ([]byte, error) {
	resp, err := c.Call("daily_basic", map[string]any{
		"ts_code": tsCode,
	}, "ts_code,trade_date,pe_ttm")
	if err != nil {
		return nil, fmt.Errorf("daily_basic pe_history: %w", err)
	}
	rows, err := tushare.RowsToMaps(resp)
	if err != nil {
		return nil, err
	}

	var points []PeHistoryPoint
	for _, row := range rows {
		pe := tushare.GetFloat(row, "pe_ttm")
		if pe <= 0 || pe > 1000 {
			continue
		}
		d := tushare.GetString(row, "trade_date")
		if len(d) < 8 {
			continue
		}
		points = append(points, PeHistoryPoint{
			Date: d[:4] + "-" + d[4:6] + "-" + d[6:8],
			Pe:   math.Round(pe*100) / 100,
		})
	}

	// Tushare returns newest first; reverse for chronological order
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}

	// Keep last 750 points
	if len(points) > 750 {
		points = points[len(points)-750:]
	}

	return json.Marshal(points)
}
