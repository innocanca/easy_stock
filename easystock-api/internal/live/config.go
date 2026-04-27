package live

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func PickLimit() int {
	const def = 15
	s := strings.TrimSpace(os.Getenv("TUSHARE_PICK_LIMIT"))
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	if n > 50 {
		return 50
	}
	return n
}

func PicksCacheTTL() time.Duration {
	const def = 5 * time.Minute
	s := strings.TrimSpace(os.Getenv("TUSHARE_PICKS_CACHE_MINUTES"))
	if s == "" {
		return def
	}
	m, err := strconv.Atoi(s)
	if err != nil || m < 1 {
		return def
	}
	return time.Duration(m) * time.Minute
}

// SectorCacheTTL 板块列表与详情聚合缓存。
func SectorCacheTTL() time.Duration {
	const def = 15 * time.Minute
	s := strings.TrimSpace(os.Getenv("TUSHARE_SECTOR_CACHE_MINUTES"))
	if s == "" {
		return def
	}
	m, err := strconv.Atoi(s)
	if err != nil || m < 1 {
		return def
	}
	return time.Duration(m) * time.Minute
}

// MinStocksPerIndustry 行业至少多少只股票才进入板块列表。
func MinStocksPerIndustry() int {
	const def = 5
	s := strings.TrimSpace(os.Getenv("TUSHARE_SECTOR_MIN_STOCKS"))
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}

// MaxSectors 板块列表最多条数（按行业名字符串排序后截断）。
func MaxSectors() int {
	const def = 80
	s := strings.TrimSpace(os.Getenv("TUSHARE_SECTOR_MAX"))
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	if n > 500 {
		return 500
	}
	return n
}

// TradeDate returns YYYYMMDD from TUSHARE_TRADE_DATE, or empty for auto (latest open day).
func TradeDate() string {
	return strings.TrimSpace(os.Getenv("TUSHARE_TRADE_DATE"))
}
