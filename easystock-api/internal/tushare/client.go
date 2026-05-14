// Package tushare implements the Tushare Pro HTTP API (https://api.tushare.pro or http).
package tushare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "http://api.tushare.pro"

// Client calls Tushare Pro. Token from https://tushare.pro — keep server-side only.
type Client struct {
	Token   string
	BaseURL string
	HTTP    *http.Client
}

func NewClient(token string) *Client {
	if token == "" {
		token = os.Getenv("TUSHARE_TOKEN")
	}
	base := strings.TrimSpace(os.Getenv("TUSHARE_BASE_URL"))
	if base == "" {
		base = DefaultBaseURL
	}
	return &Client{
		Token:   token,
		BaseURL: base,
		HTTP:    newHTTPClientForTushare(5 * time.Second),
	}
}

// newHTTPClientForTushare builds a client with the same dial/proxy/TLS rules; per-request ceiling is d.
func newHTTPClientForTushare(d time.Duration) *http.Client {
	if d <= 0 {
		d = 5 * time.Second
	}
	dialer := &net.Dialer{
		Timeout:   d,
		KeepAlive: 30 * time.Second,
	}
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			n := network
			if strings.TrimSpace(os.Getenv("TUSHARE_PREFER_IPV4")) == "1" {
				switch network {
				case "tcp", "tcp4", "tcp6":
					n = "tcp4"
				}
			}
			return dialer.DialContext(ctx, n, addr)
		},
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   d,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: d,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   d,
	}
}

type apiRequest struct {
	APIName string         `json:"api_name"`
	Token   string         `json:"token"`
	Params  map[string]any `json:"params,omitempty"`
	Fields  string         `json:"fields,omitempty"`
}

// APIResponse is Tushare JSON envelope.
type APIResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data *struct {
		Fields []string        `json:"fields"`
		Items  [][]interface{} `json:"items"`
	} `json:"data"`
}

// Call invokes a Tushare interface. fields can be empty to get default columns.
func (c *Client) Call(apiName string, params map[string]any, fields string) (*APIResponse, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("tushare: empty token")
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	body := apiRequest{
		APIName: apiName,
		Token:   c.Token,
		Params:  params,
		Fields:  fields,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	max := callMaxRetries()
	var lastErr error
	for attempt := 0; attempt < max; attempt++ {
		if attempt > 0 {
			time.Sleep(200 * time.Millisecond)
		}
		out, retry, err := c.doOneCall(base, apiName, raw)
		if err != nil && !retry {
			return nil, err
		}
		if err == nil {
			return out, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("tushare: %s: exhausted retries", apiName)
}

func callMaxRetries() int {
	s := strings.TrimSpace(os.Getenv("TUSHARE_HTTP_RETRIES"))
	if s == "" {
		return 2
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 2
	}
	if n > 8 {
		return 8
	}
	return n
}

// doOneCall returns (response, retry, err). retry is true for transient network / gateway failures.
func (c *Client) doOneCall(base, apiName string, raw []byte) (*APIResponse, bool, error) {
	req, err := http.NewRequest(http.MethodPost, base, bytes.NewReader(raw))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	b, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, true, err
	}
	if resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("tushare http %d: %s", resp.StatusCode, truncate(string(b), 200))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("tushare http %d: %s", resp.StatusCode, truncate(string(b), 200))
	}
	s := strings.TrimSpace(string(b))
	if strings.HasPrefix(s, "<") {
		return nil, true, fmt.Errorf("tushare non-json body: %s", truncate(s, 200))
	}
	var out APIResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, true, fmt.Errorf("tushare decode: %w body=%s", err, truncate(s, 200))
	}
	if out.Code != 0 {
		return nil, false, fmt.Errorf("tushare api %s: code=%d msg=%s", apiName, out.Code, out.Msg)
	}
	return &out, false, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
