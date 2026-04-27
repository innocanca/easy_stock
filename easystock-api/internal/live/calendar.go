package live

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"easystock/api/internal/tushare"
)

// LatestTradeDate returns the most recent open trading day on SSE calendar (covers A-share calendar for most uses).
func LatestTradeDate(c *tushare.Client) (string, error) {
	if d := TradeDate(); d != "" {
		return d, nil
	}
	end := time.Now().Format("20060102")
	start := time.Now().AddDate(0, 0, -30).Format("20060102")
	resp, err := c.Call("trade_cal", map[string]any{
		"exchange":   "SSE",
		"start_date": start,
		"end_date":   end,
		"is_open":    "1",
	}, "cal_date")
	if err != nil {
		// Optional fallback for restricted积分 / network
		if fb := strings.TrimSpace(os.Getenv("TUSHARE_TRADE_DATE_FALLBACK")); fb != "" {
			return fb, nil
		}
		return "", err
	}
	rows, err := tushare.RowsToMaps(resp)
	if err != nil || len(rows) == 0 {
		return "", fmt.Errorf("trade_cal: no open days")
	}
	var dates []string
	for _, r := range rows {
		d := tushare.GetString(r, "cal_date")
		if len(d) == 8 {
			dates = append(dates, d)
		}
	}
	if len(dates) == 0 {
		return "", fmt.Errorf("trade_cal: empty cal_date")
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	return dates[0], nil
}
