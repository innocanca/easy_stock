package tushare

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// LikelyAwaitingHeadersTimeout reports whether err is typical of "no response headers in time"
// (Client.Timeout, context deadline, dial stall, etc.).
func LikelyAwaitingHeadersTimeout(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "timeout") ||
		strings.Contains(s, "deadline exceeded") ||
		strings.Contains(s, "awaiting response headers") ||
		strings.Contains(s, "i/o timeout")
}

func (c *Client) effectiveBaseURL() string {
	b := strings.TrimSpace(c.BaseURL)
	if b == "" {
		return DefaultBaseURL
	}
	return b
}

// EffectiveBaseURL returns the POST base URL (after env/default).
func (c *Client) EffectiveBaseURL() string {
	return c.effectiveBaseURL()
}

// PingTradeCal performs a single POST to trade_cal (minimal date range), using one HTTP client
// with per-request ceiling `timeout` (no Call() retries). Use to verify reachability vs a heavy API.
func (c *Client) PingTradeCal(timeout time.Duration) (elapsed time.Duration, err error) {
	if c.Token == "" {
		return 0, fmt.Errorf("tushare: empty token")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	base := c.effectiveBaseURL()
	end := time.Now().Format("20060102")
	start := time.Now().AddDate(0, 0, -7).Format("20060102")
	body := apiRequest{
		APIName: "trade_cal",
		Token:   c.Token,
		Params: map[string]any{
			"exchange": "SSE", "start_date": start, "end_date": end, "is_open": "1",
		},
		Fields: "cal_date",
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	probe := &Client{
		Token:   c.Token,
		BaseURL: c.BaseURL,
		HTTP:    newHTTPClientForTushare(timeout),
	}
	t0 := time.Now()
	_, _, err = probe.doOneCall(base, "trade_cal", raw)
	return time.Since(t0), err
}

// FormatErrWithTimeoutProbe appends a one-shot trade_cal probe when the error looks like a header/timeout issue.
func (c *Client) FormatErrWithTimeoutProbe(original error) string {
	if original == nil {
		return ""
	}
	if !LikelyAwaitingHeadersTimeout(original) {
		return original.Error()
	}
	took, perr := c.PingTradeCal(10 * time.Second)
	base := c.effectiveBaseURL()
	ms := took.Milliseconds()
	if perr != nil {
		return fmt.Sprintf("%s；[探测] trade_cal 单次请求(10s 内)仍失败: %v（多为到 %s 的网络/代理 HTTP_PROXY）", original.Error(), perr, base)
	}
	return fmt.Sprintf("%s；[探测] trade_cal 在 %dms 内成功（%s 可达）；本次更可能是该接口数据量大、限流或并发", original.Error(), ms, base)
}
