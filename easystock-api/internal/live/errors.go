package live

import "errors"

// ErrStockNotFound 表示该股票在 Tushare 当日无 daily_basic 等“可识别为不存在/无数据”的情况。
var ErrStockNotFound = errors.New("stock not found")

// ErrSectorNotFound 表示板块 id 无法解析或当前行情下无该行业聚合。
var ErrSectorNotFound = errors.New("sector not found")
