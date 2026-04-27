// Package tushare implements the Tushare Pro HTTP API (http://api.tushare.pro).
package tushare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	return &Client{
		Token:   token,
		BaseURL: DefaultBaseURL,
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
		},
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
	req, err := http.NewRequest(http.MethodPost, base, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out APIResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("tushare decode: %w body=%s", err, truncate(string(b), 200))
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("tushare api %s: code=%d msg=%s", apiName, out.Code, out.Msg)
	}
	return &out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
