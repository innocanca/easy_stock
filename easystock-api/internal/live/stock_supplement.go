package live

import (
	"fmt"
	"math"
	"sort"
	"time"

	"easystock/api/internal/tushare"
)

// formatTushareDate 将 YYYYMMDD 转为 YYYY-MM-DD。
func formatTushareDate(s string) string {
	if len(s) != 8 {
		return s
	}
	return s[:4] + "-" + s[4:6] + "-" + s[6:8]
}

// stockShareholdersTab 股东人数（stk_holdernumber）；环比为相对下一期（时间更早一期）。
func stockShareholdersTab(c *tushare.Client, tsCode string, tradeDate string) []map[string]any {
	end, err := time.ParseInLocation("20060102", tradeDate, time.Local)
	if err != nil {
		return nil
	}
	start := end.AddDate(-5, 0, 0)
	resp, err := c.Call("stk_holdernumber", map[string]any{
		"ts_code":    tsCode,
		"start_date": start.Format("20060102"),
		"end_date":   tradeDate,
	}, "ts_code,end_date,holder_num,ann_date")
	if err != nil {
		return nil
	}
	rows, err := tushare.RowsToMaps(resp)
	if err != nil || len(rows) == 0 {
		return nil
	}
	sort.Slice(rows, func(i, j int) bool {
		return tushare.GetString(rows[i], "end_date") > tushare.GetString(rows[j], "end_date")
	})
	if len(rows) > 8 {
		rows = rows[:8]
	}
	out := make([]map[string]any, 0, len(rows))
	for i := range rows {
		ed := tushare.GetString(rows[i], "end_date")
		hn := tushare.GetFloat(rows[i], "holder_num")
		holders := int64(math.Round(hn))
		var changePct float64
		if i+1 < len(rows) {
			prev := tushare.GetFloat(rows[i+1], "holder_num")
			if prev > 0 {
				changePct = (hn - prev) / prev * 100
			}
		}
		out = append(out, map[string]any{
			"end":       formatTushareDate(ed),
			"holders":   holders,
			"changePct": math.Round(changePct*10) / 10,
		})
	}
	return out
}

// stockDividendsTab 分红送股（dividend）。
func stockDividendsTab(c *tushare.Client, tsCode string) []map[string]any {
	resp, err := c.Call("dividend", map[string]any{
		"ts_code": tsCode,
	}, "ts_code,end_date,cash_div,cash_div_tax,stk_div")
	if err != nil {
		return nil
	}
	rows, err := tushare.RowsToMaps(resp)
	if err != nil || len(rows) == 0 {
		return nil
	}
	sort.Slice(rows, func(i, j int) bool {
		return tushare.GetString(rows[i], "end_date") > tushare.GetString(rows[j], "end_date")
	})
	out := make([]map[string]any, 0, 8)
	for _, row := range rows {
		if len(out) >= 8 {
			break
		}
		ed := tushare.GetString(row, "end_date")
		if len(ed) < 4 {
			continue
		}
		cash := tushare.GetFloat(row, "cash_div_tax")
		if cash <= 0 {
			cash = tushare.GetFloat(row, "cash_div")
		}
		if cash <= 0 && tushare.GetFloat(row, "stk_div") <= 0 {
			continue
		}
		year := ed[:4]
		per10 := "—"
		if cash > 0 {
			per10 = fmt.Sprintf("%.2f 元", cash*10)
		} else {
			stk := tushare.GetFloat(row, "stk_div")
			if stk > 0 {
				per10 = fmt.Sprintf("送转 %.2f 股/10股", stk*10)
			}
		}
		out = append(out, map[string]any{
			"year":  year,
			"per10": per10,
			"yield": "—",
		})
	}
	return out
}

// stockFlowsTab 个股资金流向（moneyflow）：净流入、大单+特大单净流入，单位均为「百万元」口径（与前端表头一致）。
func stockFlowsTab(c *tushare.Client, tsCode string, tradeDate string) []map[string]any {
	end, err := time.ParseInLocation("20060102", tradeDate, time.Local)
	if err != nil {
		return nil
	}
	start := end.AddDate(0, 0, -45)
	resp, err := c.Call("moneyflow", map[string]any{
		"ts_code":    tsCode,
		"start_date": start.Format("20060102"),
		"end_date":   tradeDate,
	}, "trade_date,net_mf_amount,buy_lg_amount,sell_lg_amount,buy_elg_amount,sell_elg_amount")
	if err != nil {
		return nil
	}
	rows, err := tushare.RowsToMaps(resp)
	if err != nil || len(rows) == 0 {
		return nil
	}
	sort.Slice(rows, func(i, j int) bool {
		return tushare.GetString(rows[i], "trade_date") > tushare.GetString(rows[j], "trade_date")
	})
	if len(rows) > 12 {
		rows = rows[:12]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		td := tushare.GetString(row, "trade_date")
		net := tushare.GetFloat(row, "net_mf_amount")
		buyLg := tushare.GetFloat(row, "buy_lg_amount")
		sellLg := tushare.GetFloat(row, "sell_lg_amount")
		buyElg := tushare.GetFloat(row, "buy_elg_amount")
		sellElg := tushare.GetFloat(row, "sell_elg_amount")
		lgNet := (buyLg - sellLg) + (buyElg - sellElg)
		// Tushare：金额万元 → 展示「百万」：/100
		mainNet := math.Round(net/100*10) / 10
		north := math.Round(lgNet/100*10) / 10
		out = append(out, map[string]any{
			"date":    formatTushareDate(td),
			"mainNet": mainNet,
			"north":   north,
		})
	}
	return out
}
