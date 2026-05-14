package live

import (
	"fmt"

	"easystock/api/internal/tushare"
)

const dailyBasicMaxBatch = 6000

// DailyBasicByTradeDate 拉取指定交易日全市场 daily_basic。
// 官方说明单次最多 6000 条；全市场股票数可能超过 6000，需 offset 分页。
// 若账号积分较低，单次可能只返回少量记录——属 Tushare 权限限制，非本服务截断。
func DailyBasicByTradeDate(c *tushare.Client, tradeDate string, fields string) ([]map[string]interface{}, error) {
	params := map[string]any{
		"ts_code":    "",
		"trade_date": tradeDate,
	}
	resp, err := c.Call("daily_basic", params, fields)
	if err != nil {
		return nil, fmt.Errorf("daily_basic: %w", err)
	}
	rows0, err := tushare.RowsToMaps(resp)
	if err != nil {
		return nil, err
	}
	if len(rows0) == 0 {
		return nil, fmt.Errorf("daily_basic: empty rows for trade_date=%s", tradeDate)
	}

	seen := make(map[string]struct{}, len(rows0)*2)
	out := make([]map[string]interface{}, 0, len(rows0)*2)
	addUnique := func(rows []map[string]interface{}) int {
		added := 0
		for _, row := range rows {
			code := tushare.GetString(row, "ts_code")
			if code == "" {
				continue
			}
			if _, ok := seen[code]; ok {
				continue
			}
			seen[code] = struct{}{}
			out = append(out, row)
			added++
		}
		return added
	}
	addUnique(rows0)

	if len(rows0) < dailyBasicMaxBatch {
		return out, nil
	}

	for offset := dailyBasicMaxBatch; ; offset += dailyBasicMaxBatch {
		params2 := map[string]any{
			"ts_code":    "",
			"trade_date": tradeDate,
			"offset":     offset,
			"limit":      dailyBasicMaxBatch,
		}
		resp2, err := c.Call("daily_basic", params2, fields)
		if err != nil {
			break
		}
		rows, err := tushare.RowsToMaps(resp2)
		if err != nil || len(rows) == 0 {
			break
		}
		added := addUnique(rows)
		if len(rows) < dailyBasicMaxBatch {
			break
		}
		if added == 0 {
			break
		}
	}

	return out, nil
}
