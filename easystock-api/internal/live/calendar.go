package live

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"easystock/api/internal/tushare"
)

// LatestTradeDate returns the most recent open trading day on SSE calendar
// whose daily_basic data is likely available. Tushare publishes daily data
// after ~18:00 CST, so before that cutoff we exclude today.
func LatestTradeDate(c *tushare.Client) (string, error) {
	if d := TradeDate(); d != "" {
		return d, nil
	}

	cst := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cst)

	// Before 18:00 CST the current day's data is not yet published;
	// use yesterday as the upper bound so we get the most recent day with data.
	end := now
	if now.Hour() < 18 {
		end = now.AddDate(0, 0, -1)
	}

	endStr := end.Format("20060102")
	startStr := end.AddDate(0, 0, -30).Format("20060102")

	resp, err := c.Call("trade_cal", map[string]any{
		"exchange":   "SSE",
		"start_date": startStr,
		"end_date":   endStr,
		"is_open":    "1",
	}, "cal_date")
	if err != nil {
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
