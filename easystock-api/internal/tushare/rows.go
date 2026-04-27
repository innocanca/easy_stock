package tushare

import (
	"fmt"
	"strconv"
)

// RowsToMaps converts Tushare data.fields + data.items into []map[string]interface{} with native types approximated.
func RowsToMaps(resp *APIResponse) ([]map[string]interface{}, error) {
	if resp.Data == nil {
		return nil, fmt.Errorf("tushare: empty data")
	}
	fields := resp.Data.Fields
	out := make([]map[string]interface{}, 0, len(resp.Data.Items))
	for _, row := range resp.Data.Items {
		m := make(map[string]interface{}, len(fields))
		for i, name := range fields {
			if i < len(row) {
				m[name] = row[i]
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func GetString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return fmt.Sprint(x)
	}
}

func GetFloat(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	default:
		return 0
	}
}
