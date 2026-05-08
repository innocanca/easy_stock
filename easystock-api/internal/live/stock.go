package live

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"easystock/api/internal/tushare"
)

// StockDetailJSON builds one stock payload aligned with easyStock StockDetail type.
func StockDetailJSON(c *tushare.Client, tsCode string) ([]byte, error) {
	date, err := LatestTradeDate(c)
	if err != nil {
		return nil, err
	}
	dbResp, err := c.Call("daily_basic", map[string]any{
		"ts_code":    tsCode,
		"trade_date": date,
	}, "ts_code,trade_date,close,pe_ttm,pb,total_mv,circ_mv,turnover_rate")
	if err != nil {
		return nil, fmt.Errorf("daily_basic: %w", err)
	}
	dbRows, err := tushare.RowsToMaps(dbResp)
	if err != nil {
		return nil, fmt.Errorf("daily_basic: %w", err)
	}
	if len(dbRows) == 0 {
		return nil, ErrStockNotFound
	}
	db := dbRows[0]
	pe := tushare.GetFloat(db, "pe_ttm")
	pb := tushare.GetFloat(db, "pb")

	names, err := StockBasicMap(c)
	if err != nil {
		return nil, err
	}
	sb := names[tsCode]
	sector := sb.Industry
	sectorID := SectorIDFromIndustry(sb.Industry)
	if sector == "" {
		sector = "—"
	}
	name := sb.Name
	if name == "" {
		name = tsCode
	}

	var roe float64
	var roeSeries []map[string]any
	var latestFi map[string]any

	fiResp, fiErr := c.Call("fina_indicator", map[string]any{
		"ts_code": tsCode,
	}, "ts_code,end_date,roe,grossprofit_margin,debt_to_assets,netprofit_margin")
	if fiErr == nil {
		fiRows, err := tushare.RowsToMaps(fiResp)
		if err == nil && len(fiRows) > 0 {
			sort.Slice(fiRows, func(i, j int) bool {
				return tushare.GetString(fiRows[i], "end_date") > tushare.GetString(fiRows[j], "end_date")
			})
			latestFi = fiRows[0]
			roe = tushare.GetFloat(latestFi, "roe")
			for _, row := range fiRows {
				if len(roeSeries) >= 5 {
					break
				}
				ed := tushare.GetString(row, "end_date")
				if len(ed) < 4 {
					continue
				}
				y := ed[:4]
				r := tushare.GetFloat(row, "roe")
				roeSeries = append(roeSeries, map[string]any{"y": y, "roe": math.Round(r*10) / 10})
			}
		}
	}

	tags := []string{"Tushare"}
	if sector != "" && sector != "—" {
		tags = append(tags, sector)
	}

	financeRows := []map[string]string{
		{"label": "收盘价(当日)", "ttm": fmt.Sprintf("%.2f", tushare.GetFloat(db, "close")), "yoy": "—"},
		{"label": "PE(ttm)", "ttm": fmt.Sprintf("%.2f", pe), "yoy": "—"},
		{"label": "PB", "ttm": fmt.Sprintf("%.2f", pb), "yoy": "—"},
		{"label": "总市值(万元)", "ttm": fmt.Sprintf("%.0f", tushare.GetFloat(db, "total_mv")), "yoy": "—"},
	}
	if latestFi != nil {
		financeRows = append(financeRows,
			map[string]string{"label": "ROE(财报)", "ttm": fmt.Sprintf("%.2f%%", tushare.GetFloat(latestFi, "roe")), "yoy": tushare.GetString(latestFi, "end_date")},
			map[string]string{"label": "毛利率", "ttm": fmt.Sprintf("%.2f%%", tushare.GetFloat(latestFi, "grossprofit_margin")), "yoy": "—"},
			map[string]string{"label": "资产负债率", "ttm": fmt.Sprintf("%.2f%%", tushare.GetFloat(latestFi, "debt_to_assets")), "yoy": "—"},
			map[string]string{"label": "净利率", "ttm": fmt.Sprintf("%.2f%%", tushare.GetFloat(latestFi, "netprofit_margin")), "yoy": "—"},
		)
	}

	sh := stockShareholdersTab(c, tsCode, date)
	if sh == nil {
		sh = []map[string]any{}
	}
	div := stockDividendsTab(c, tsCode)
	if div == nil {
		div = []map[string]any{}
	}
	fl := stockFlowsTab(c, tsCode, date)
	if fl == nil {
		fl = []map[string]any{}
	}

	detail := map[string]any{
		"code":             tsCode,
		"name":             name,
		"sector":           sector,
		"sectorId":         sectorID,
		"sectorAvgPe":      20.0,
		"pe":               math.Round(pe*10) / 10,
		"pePctHistory":     50,
		"pb":               math.Round(pb*100) / 100,
		"roe":              math.Round(roe*10) / 10,
		"roeSeries":        roeSeries,
		"valueTags":        tags,
		"valueSummary":     fmt.Sprintf("行情来自 Tushare daily_basic（%s），财务指标来自 fina_indicator（如有）。行业：%s。", date, sector),
		"growthKeywords":   []string{"数据接入"},
		"growthSummary":    "可继续接入业绩预告、研报摘要等。",
		"revenueGrowth":    []map[string]any{},
		"financeRows":      financeRows,
		"businessSegments": []map[string]any{},
		"shareholders":     sh,
		"dividends":        div,
		"flows":            fl,
		"news":             []map[string]any{},
	}

	return json.Marshal(detail)
}
