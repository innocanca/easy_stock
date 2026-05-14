package report

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type AIClient struct {
	Mode    string // "http" or "cursor"
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client

	CursorAPIKey string
	CursorModel  string
	NodeBin      string
	ScriptDir    string
}

func NewAIClient() *AIClient {
	c := &AIClient{
		HTTP: &http.Client{Timeout: 180 * time.Second},
	}

	c.CursorAPIKey = strings.TrimSpace(os.Getenv("CURSOR_API_KEY"))
	c.CursorModel = strings.TrimSpace(os.Getenv("CURSOR_MODEL"))
	if c.CursorModel == "" {
		c.CursorModel = "claude-sonnet-4-6"
	}
	c.NodeBin = strings.TrimSpace(os.Getenv("NODE_BIN"))
	if c.NodeBin == "" {
		c.NodeBin = "node"
	}

	c.APIKey = strings.TrimSpace(os.Getenv("AI_API_KEY"))
	c.BaseURL = strings.TrimSpace(os.Getenv("AI_API_URL"))
	if c.BaseURL == "" {
		c.BaseURL = "https://api.deepseek.com"
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	c.Model = strings.TrimSpace(os.Getenv("AI_MODEL"))
	if c.Model == "" {
		c.Model = "deepseek-chat"
	}

	if c.CursorAPIKey != "" {
		c.Mode = "cursor"
		c.ScriptDir = findScriptDir()
	} else if c.APIKey != "" {
		c.Mode = "http"
	}

	return c
}

func findScriptDir() string {
	candidates := []string{
		"scripts",
		"easystock-api/scripts",
	}
	for _, d := range candidates {
		if _, err := os.Stat(filepath.Join(d, "cursor-analyze.mjs")); err == nil {
			abs, _ := filepath.Abs(d)
			return abs
		}
	}
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "scripts")
}

func (c *AIClient) Ready() bool {
	return c.Mode != ""
}

func (c *AIClient) ProviderInfo() string {
	if c.Mode == "cursor" {
		return fmt.Sprintf("cursor-sdk (model=%s)", c.CursorModel)
	}
	return fmt.Sprintf("http (model=%s, url=%s)", c.Model, c.BaseURL)
}

// ---- Cursor SDK mode ----

type cursorInput struct {
	System string `json:"system"`
	User   string `json:"user"`
	Model  string `json:"model"`
}

func (c *AIClient) buildCursorCmd(ctx context.Context, system, user string, stream bool) (*exec.Cmd, string, error) {
	input := cursorInput{System: system, User: user, Model: c.CursorModel}
	inputJSON, _ := json.Marshal(input)

	tmpFile, err := os.CreateTemp("", "cursor-input-*.json")
	if err != nil {
		return nil, "", fmt.Errorf("create temp: %w", err)
	}
	if _, err := tmpFile.Write(inputJSON); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, "", err
	}
	tmpFile.Close()

	scriptPath := filepath.Join(c.ScriptDir, "cursor-analyze.mjs")
	args := []string{scriptPath, tmpFile.Name()}
	if stream {
		args = append(args, "--stream")
	}

	cmd := exec.CommandContext(ctx, c.NodeBin, args...)
	cmd.Env = append(os.Environ(), "CURSOR_API_KEY="+c.CursorAPIKey)
	return cmd, tmpFile.Name(), nil
}

func (c *AIClient) callCursor(system, user string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd, tmpPath, err := c.buildCursorCmd(ctx, system, user, false)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpPath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("report/ai: calling cursor-sdk (%s)", c.CursorModel)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("cursor-sdk failed: %w\nstderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func (c *AIClient) callCursorStream(system, user string, ch chan<- string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd, tmpPath, err := c.buildCursorCmd(ctx, system, user, true)
	if err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	go func() { _, _ = io.Copy(&stderrBuf, stderrPipe) }()

	log.Printf("report/ai: calling cursor-sdk stream (%s)", c.CursorModel)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	rd := bufio.NewReader(stdout)
	buf := make([]byte, 4096)
	for {
		n, readErr := rd.Read(buf)
		if n > 0 {
			ch <- string(buf[:n])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = cmd.Process.Kill()
			return fmt.Errorf("read stdout: %w", readErr)
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("cursor-sdk failed: %w\nstderr: %s", err, stderrBuf.String())
	}
	return nil
}

// ---- HTTP API mode (DeepSeek / OpenAI compatible) ----

type chatReq struct {
	Model          string      `json:"model"`
	Messages       []chatMsg   `json:"messages"`
	Temperature    float64     `json:"temperature"`
	Stream         bool        `json:"stream,omitempty"`
	ResponseFormat *respFormat `json:"response_format,omitempty"`
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type respFormat struct {
	Type string `json:"type"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func (c *AIClient) callHTTP(system, user string, jsonMode bool) (string, error) {
	body := chatReq{
		Model:       c.Model,
		Temperature: 0.1,
		Messages: []chatMsg{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	if jsonMode {
		body.ResponseFormat = &respFormat{Type: "json_object"}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, truncBody(respBody))
	}

	var cr chatResp
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}
	return strings.TrimSpace(cr.Choices[0].Message.Content), nil
}

func (c *AIClient) callHTTPStream(system, user string, ch chan<- string) error {
	body := chatReq{
		Model:       c.Model,
		Temperature: 0.1,
		Stream:      true,
		Messages: []chatMsg{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, truncBody(b))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk streamChunk
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				ch <- choice.Delta.Content
			}
		}
	}
	return scanner.Err()
}

// ---- Streaming public methods ----

// CallStream invokes AI with streaming, sending chunks to ch. Closes ch when done.
func (c *AIClient) CallStream(system, user string, ch chan<- string) error {
	defer close(ch)
	switch c.Mode {
	case "cursor":
		return c.callCursorStream(system, user, ch)
	case "http":
		return c.callHTTPStream(system, user, ch)
	default:
		return fmt.Errorf("no AI provider configured")
	}
}

// ---- Public methods ----

const extractPrompt = `你是一个专业的财务分析师。请从以下年度报告文本中提取关键财务数据，并以严格的JSON格式返回。

所有金额单位统一为亿元（人民币），比率统一为小数（如15%用0.15表示）。
如果某个指标在报告中未明确提到，请设为0。

请提取以下字段：
- revenue: 营业收入（亿元）
- revenue_yoy: 营收同比增长率（小数）
- net_profit: 净利润（亿元）
- net_profit_yoy: 净利润同比增长率（小数）
- net_profit_parent: 归属于母公司股东的净利润（亿元）
- gross_margin: 毛利率（小数）
- net_margin: 净利率（小数）
- roe: 加权平均净资产收益率（小数）
- total_assets: 总资产（亿元）
- net_assets: 归属于母公司股东的净资产（亿元）
- debt_ratio: 资产负债率（小数）
- operating_cashflow: 经营活动产生的现金流量净额（亿元）
- eps: 基本每股收益（元/股）
- dividend_per_share: 每股现金分红（元，含税）
- employee_count: 在职员工总数（整数）
- rd_expense: 研发投入（亿元）
- segments: 主营业务分产品/品类收入构成数组，每项 {"name":"产品名","revenue":亿元,"ratio":占比小数}
- highlights: 2-3句话总结本年度经营亮点
- risks: 2-3句话总结主要风险
- outlook: 2-3句话总结未来展望

请只返回一个JSON对象，不要包含任何其他文字或Markdown标记。`

func (c *AIClient) ExtractFinancials(pdfText string, stockCode, stockName string, year int) (*ReportData, error) {
	userMsg := fmt.Sprintf("以下是 %s（%s）%d年年度报告的文本内容：\n\n%s", stockName, stockCode, year, pdfText)

	var content string
	var err error

	switch c.Mode {
	case "cursor":
		content, err = c.callCursor(extractPrompt, userMsg)
	case "http":
		content, err = c.callHTTP(extractPrompt, userMsg, true)
	default:
		return nil, fmt.Errorf("no AI provider configured")
	}
	if err != nil {
		return nil, err
	}

	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	content = extractJSONObject(content)

	var data ReportData
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return nil, fmt.Errorf("parse JSON: %w\nraw: %s", err, truncBody([]byte(content)))
	}
	data.StockCode = stockCode
	data.StockName = stockName
	data.Year = year
	return &data, nil
}

const multiYearPrompt = `你是一位资深证券分析师。请基于以下多年财务数据，撰写一份全面的企业发展分析报告。

分析要求：
1. 整体发展轨迹：概述企业在这些年间的发展阶段和关键转折点
2. 收入与利润分析：分析营收和利润的增长趋势、增速变化
3. 盈利能力演变：分析毛利率、净利率、ROE的变化趋势及原因
4. 财务健康度：评估资产负债率、现金流的稳健性
5. 业务结构变化：分析产品/业务线收入构成的变化
6. 股东回报：评价分红政策和每股收益变化
7. 风险与展望：综合判断企业未来发展前景

请用中文撰写，条理清晰，数据翔实，约800-1200字。直接返回分析文本，不要包含JSON格式。`

func (c *AIClient) MultiYearAnalysis(reports []ReportData) (string, error) {
	dataJSON, _ := json.MarshalIndent(reports, "", "  ")
	userMsg := fmt.Sprintf("以下是 %s（%s）%d年至%d年的年度财务数据：\n\n%s",
		reports[0].StockName, reports[0].StockCode,
		reports[0].Year, reports[len(reports)-1].Year,
		string(dataJSON))

	switch c.Mode {
	case "cursor":
		return c.callCursor(multiYearPrompt, userMsg)
	case "http":
		return c.callHTTP(multiYearPrompt, userMsg, false)
	default:
		return "", fmt.Errorf("no AI provider configured")
	}
}

// ---- Wiki generation ----

const wikiPrompt = `你是一位资深证券分析师，同时也是一位出色的知识整理专家。请将以下年度报告内容整理成一份结构清晰的Markdown知识Wiki页面。

要求：
1. 使用Markdown格式，结构清晰，包含以下章节：
   - ## 公司概况（简介、主营业务、行业地位）
   - ## 经营分析（本年度经营情况、重大事项、战略进展）
   - ## 财务分析（收入利润、盈利能力、现金流、资产负债）
   - ## 业务构成（各产品/业务线收入占比及变化）
   - ## 风险因素（主要风险及应对措施）
   - ## 未来展望（发展规划、增长驱动力）
2. 数据准确，引用报告中的关键数字
3. 语言简洁专业，适合快速查阅
4. 在每个章节内使用要点列表或简短段落

直接返回Markdown内容，不要用代码块包裹。`

const wikiPromptWithContext = `你是一位资深证券分析师，同时也是一位出色的知识整理专家。请将以下年度报告内容整理成一份结构清晰的Markdown知识Wiki页面。

你已经有该公司以往年份的知识库内容（见下方"已有知识库"部分），请在生成本年度Wiki时：
1. 与历史数据做对比分析，标注关键指标的同比变化
2. 延续已有知识库的分析框架和关注点
3. 如发现趋势性变化或重大转折，特别标注

Wiki格式要求：
1. 使用Markdown格式，结构清晰，包含以下章节：
   - ## 公司概况（简介、主营业务、行业地位）
   - ## 经营分析（本年度经营情况、重大事项、战略进展，与往年对比）
   - ## 财务分析（收入利润、盈利能力、现金流、资产负债，标注同比变化）
   - ## 业务构成（各产品/业务线收入占比及变化趋势）
   - ## 风险因素（主要风险及应对措施，与往年对比是否有新增风险）
   - ## 未来展望（发展规划、增长驱动力）
2. 数据准确，引用报告中的关键数字
3. 语言简洁专业，适合快速查阅

直接返回Markdown内容，不要用代码块包裹。`

func (c *AIClient) WikiUserMsg(pdfText, stockName, stockCode string, year int, existingWiki string) string {
	var sb strings.Builder
	if existingWiki != "" {
		sb.WriteString("## 已有知识库\n\n")
		sb.WriteString(existingWiki)
		sb.WriteString("\n\n---\n\n")
	}
	sb.WriteString(fmt.Sprintf("## %s（%s）%d年年度报告\n\n", stockName, stockCode, year))
	sb.WriteString(pdfText)
	return sb.String()
}

func (c *AIClient) WikiSystemPrompt(hasContext bool) string {
	if hasContext {
		return wikiPromptWithContext
	}
	return wikiPrompt
}

// ---- Utilities ----

func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return s
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if esc {
			esc = false
			continue
		}
		if c == '\\' && inStr {
			esc = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}

func truncBody(b []byte) string {
	s := string(b)
	if len(s) > 500 {
		return s[:500] + "..."
	}
	return s
}
