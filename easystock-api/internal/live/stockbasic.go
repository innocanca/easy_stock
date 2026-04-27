package live

import (
	"sync"

	"easystock/api/internal/tushare"
)

type StockBasicRow struct {
	Name     string
	Industry string
	Market   string
}

var (
	sbMu     sync.Mutex
	sbLoaded bool
	sbByCode map[string]StockBasicRow
)

// StockBasicMap loads stock_basic once (listed A-shares) and caches ts_code → name/industry.
func StockBasicMap(c *tushare.Client) (map[string]StockBasicRow, error) {
	sbMu.Lock()
	defer sbMu.Unlock()
	if sbLoaded && sbByCode != nil {
		return sbByCode, nil
	}
	resp, err := c.Call("stock_basic", map[string]any{
		"list_status": "L",
	}, "ts_code,name,industry,market")
	if err != nil {
		return nil, err
	}
	rows, err := tushare.RowsToMaps(resp)
	if err != nil {
		return nil, err
	}
	m := make(map[string]StockBasicRow, len(rows))
	for _, r := range rows {
		code := tushare.GetString(r, "ts_code")
		if code == "" {
			continue
		}
		m[code] = StockBasicRow{
			Name:     tushare.GetString(r, "name"),
			Industry: tushare.GetString(r, "industry"),
			Market:   tushare.GetString(r, "market"),
		}
	}
	sbByCode = m
	sbLoaded = true
	return sbByCode, nil
}

func ResetStockBasicCacheForTests() {
	sbMu.Lock()
	defer sbMu.Unlock()
	sbLoaded = false
	sbByCode = nil
}

// Must have stock row for picks name
func stockName(m map[string]StockBasicRow, tsCode string) string {
	r, ok := m[tsCode]
	if !ok || r.Name == "" {
		return tsCode
	}
	return r.Name
}
