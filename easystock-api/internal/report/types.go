package report

type ReportData struct {
	StockCode       string    `json:"stock_code"`
	StockName       string    `json:"stock_name"`
	Year            int       `json:"year"`
	Revenue         float64   `json:"revenue"`
	RevenueYoY      float64   `json:"revenue_yoy"`
	NetProfit       float64   `json:"net_profit"`
	NetProfitYoY    float64   `json:"net_profit_yoy"`
	NetProfitParent float64   `json:"net_profit_parent"`
	GrossMargin     float64   `json:"gross_margin"`
	NetMargin       float64   `json:"net_margin"`
	ROE             float64   `json:"roe"`
	TotalAssets     float64   `json:"total_assets"`
	NetAssets       float64   `json:"net_assets"`
	DebtRatio       float64   `json:"debt_ratio"`
	OperatingCashFlow float64 `json:"operating_cashflow"`
	EPS             float64   `json:"eps"`
	DividendPerShare float64  `json:"dividend_per_share"`
	EmployeeCount   int       `json:"employee_count"`
	RDExpense       float64   `json:"rd_expense"`
	Segments        []Segment `json:"segments"`
	Highlights      string    `json:"highlights"`
	Risks           string    `json:"risks"`
	Outlook         string    `json:"outlook"`
}

type Segment struct {
	Name    string  `json:"name"`
	Revenue float64 `json:"revenue"`
	Ratio   float64 `json:"ratio"`
}

type AnalysisResult struct {
	StockCode string       `json:"stock_code"`
	StockName string       `json:"stock_name"`
	Years     []ReportData `json:"years"`
	Summary   string       `json:"summary"`
}

type UploadResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    *ReportData `json:"data,omitempty"`
}

type ListResponse struct {
	StockCode string       `json:"stock_code"`
	Reports   []ReportData `json:"reports"`
}
