/**
 * Shared demo dataset for frontend + API (replace with DB/Tushare later).
 * Keep in sync when editing mock scenarios.
 */

export type Dimensions = {
  value: number;
  valuation: number;
  certainty: number;
  growth: number;
};

export type PickItem = {
  code: string;
  name: string;
  score: number;
  pe: number;
  roe: number;
  profitGrowthYoy: number;
  dimensions: Dimensions;
  scoreNote: string;
  profitTrend: { q: string; profit: number }[];
  peTrend: { q: string; pe: number }[];
};

export const mockPicks: PickItem[] = [
  {
    code: "600519.SH",
    name: "贵州茅台",
    score: 87,
    pe: 28.4,
    roe: 31.2,
    profitGrowthYoy: 15.3,
    dimensions: { value: 92, valuation: 72, certainty: 90, growth: 78 },
    scoreNote:
      "盈利稳定增长削弱估值压力：近四季利润上行，前瞻 PE 相对自身历史中位仍偏高但可接受。",
    profitTrend: [
      { q: "23Q1", profit: 172 },
      { q: "23Q3", profit: 199 },
      { q: "24Q1", profit: 211 },
      { q: "24Q3", profit: 228 },
    ],
    peTrend: [
      { q: "23Q1", pe: 35 },
      { q: "23Q3", pe: 32 },
      { q: "24Q1", pe: 30 },
      { q: "24Q3", pe: 28.4 },
    ],
  },
  {
    code: "000858.SZ",
    name: "五粮液",
    score: 81,
    pe: 18.2,
    roe: 24.8,
    profitGrowthYoy: 11.8,
    dimensions: { value: 88, valuation: 82, certainty: 82, growth: 72 },
    scoreNote:
      "估值修复空间优于龙头：PE 低于板块均值，利润增速温和，确定性来自品牌与渠道。",
    profitTrend: [
      { q: "23Q1", profit: 108 },
      { q: "23Q3", profit: 118 },
      { q: "24Q1", profit: 122 },
      { q: "24Q3", profit: 131 },
    ],
    peTrend: [
      { q: "23Q1", pe: 22 },
      { q: "23Q3", pe: 20 },
      { q: "24Q1", pe: 19 },
      { q: "24Q3", pe: 18.2 },
    ],
  },
  {
    code: "688981.SH",
    name: "中芯国际",
    score: 74,
    pe: 45.0,
    roe: 6.2,
    profitGrowthYoy: -8.4,
    dimensions: { value: 78, valuation: 58, certainty: 62, growth: 85 },
    scoreNote:
      "成长性叙事（国产替代）拉高预期，短期利润波动放大 PE 不确定性，需跟踪稼动率。",
    profitTrend: [
      { q: "23Q1", profit: 10.2 },
      { q: "23Q3", profit: 9.8 },
      { q: "24Q1", profit: 8.9 },
      { q: "24Q3", profit: 9.1 },
    ],
    peTrend: [
      { q: "23Q1", pe: 42 },
      { q: "23Q3", pe: 48 },
      { q: "24Q1", pe: 52 },
      { q: "24Q3", pe: 45 },
    ],
  },
];

export type StockDetail = {
  code: string;
  name: string;
  sector: string;
  /** URL-safe id aligned with GET /api/sectors/{id} (from API when using Tushare). */
  sectorId?: string;
  sectorAvgPe: number;
  pe: number;
  pePctHistory: number;
  pb: number;
  roe: number;
  roeSeries: { y: string; roe: number }[];
  valueTags: string[];
  valueSummary: string;
  growthKeywords: string[];
  growthSummary: string;
  revenueGrowth: { q: string; pct: number }[];
  financeRows: { label: string; ttm: string; yoy: string }[];
  businessSegments: { name: string; share: number; margin: number }[];
  shareholders: { end: string; holders: number; changePct: number }[];
  dividends: { year: string; per10: string; yield: string }[];
  flows: { date: string; mainNet: number; north: number }[];
  news: { time: string; title: string; major?: boolean }[];
};

export const mockStocks: Record<string, StockDetail> = {
  "600519.SH": {
    code: "600519.SH",
    name: "贵州茅台",
    sector: "白酒",
    sectorAvgPe: 22.5,
    pe: 28.4,
    pePctHistory: 62,
    pb: 9.8,
    roe: 31.2,
    roeSeries: [
      { y: "2020", roe: 31.4 },
      { y: "2021", roe: 29.9 },
      { y: "2022", roe: 30.3 },
      { y: "2023", roe: 31.2 },
    ],
    valueTags: ["强品牌", "稀缺产能", "直营提升"],
    valueSummary:
      "高端白酒龙头，护城河来自品牌与渠道控制力；商业模式简单、现金流充沛。",
    growthKeywords: ["量价平衡", "国际化试点"],
    growthSummary: "中长期看消费升级与产品结构；增速锚定稳健而非爆发。",
    revenueGrowth: [
      { q: "23Q2", pct: 18.0 },
      { q: "23Q4", pct: 17.5 },
      { q: "24Q2", pct: 16.9 },
      { q: "24Q4", pct: 15.8 },
    ],
    financeRows: [
      { label: "营业收入", ttm: "约 1508 亿", yoy: "+15.7%" },
      { label: "归母净利润", ttm: "约 747 亿", yoy: "+15.1%" },
      { label: "毛利率", ttm: "91.9%", yoy: "持平" },
      { label: "资产负债率", ttm: "19.2%", yoy: "-0.8pp" },
      { label: "经营现金流/净利润", ttm: "1.02", yoy: "稳" },
    ],
    businessSegments: [
      { name: "茅台酒", share: 84, margin: 94 },
      { name: "系列酒", share: 12, margin: 72 },
      { name: "其他", share: 4, margin: 45 },
    ],
    shareholders: [
      { end: "2024-09-30", holders: 152300, changePct: -3.2 },
      { end: "2024-06-30", holders: 157400, changePct: 1.1 },
      { end: "2024-03-31", holders: 155700, changePct: -0.5 },
    ],
    dividends: [
      { year: "2023", per10: "308.76 元", yield: "2.1%" },
      { year: "2022", per10: "282.00 元", yield: "2.0%" },
    ],
    flows: [
      { date: "2025-04-25", mainNet: 320, north: 180 },
      { date: "2025-04-24", mainNet: -120, north: 45 },
    ],
    news: [
      { time: "2025-04-20", title: "公司披露年报：净利润同比增长约 15%", major: true },
      { time: "2025-04-08", title: "板块轮动：白酒龙头获北向小幅净流入", major: false },
    ],
  },
  "000858.SZ": {
    code: "000858.SZ",
    name: "五粮液",
    sector: "白酒",
    sectorAvgPe: 22.5,
    pe: 18.2,
    pePctHistory: 48,
    pb: 4.2,
    roe: 24.8,
    roeSeries: [
      { y: "2020", roe: 24.9 },
      { y: "2021", roe: 25.3 },
      { y: "2022", roe: 24.9 },
      { y: "2023", roe: 24.8 },
    ],
    valueTags: ["浓香龙头", "经典大单品"],
    valueSummary: "高端浓香代表，渠道改革与量价策略延续品牌势能。",
    growthKeywords: ["宴席复苏", "经典系列"],
    growthSummary: "增速温和，看点在结构升级与边际利润率。",
    revenueGrowth: [
      { q: "23Q2", pct: 11.2 },
      { q: "23Q4", pct: 12.1 },
      { q: "24Q2", pct: 11.8 },
      { q: "24Q4", pct: 11.5 },
    ],
    financeRows: [
      { label: "营业收入", ttm: "约 830 亿", yoy: "+11.9%" },
      { label: "归母净利润", ttm: "约 305 亿", yoy: "+11.8%" },
      { label: "毛利率", ttm: "76.2%", yoy: "+0.4pp" },
      { label: "资产负债率", ttm: "28.5%", yoy: "-1.2pp" },
      { label: "经营现金流/净利润", ttm: "1.08", yoy: "稳" },
    ],
    businessSegments: [
      { name: "酒类销售", share: 92, margin: 82 },
      { name: "其他", share: 8, margin: 35 },
    ],
    shareholders: [
      { end: "2024-09-30", holders: 482100, changePct: -1.8 },
      { end: "2024-06-30", holders: 490900, changePct: 0.9 },
    ],
    dividends: [
      { year: "2023", per10: "179.82 元", yield: "2.8%" },
      { year: "2022", per10: "149.85 元", yield: "2.5%" },
    ],
    flows: [
      { date: "2025-04-25", mainNet: 85, north: 42 },
      { date: "2025-04-24", mainNet: -40, north: 12 },
    ],
    news: [
      { time: "2025-04-15", title: "公司投资者交流：动销区域分化", major: false },
    ],
  },
  "688981.SH": {
    code: "688981.SH",
    name: "中芯国际",
    sector: "半导体",
    sectorAvgPe: 48.2,
    pe: 45.0,
    pePctHistory: 55,
    pb: 2.1,
    roe: 6.2,
    roeSeries: [
      { y: "2020", roe: 6.5 },
      { y: "2021", roe: 10.2 },
      { y: "2022", roe: 8.9 },
      { y: "2023", roe: 6.2 },
    ],
    valueTags: ["晶圆代工", "国产替代"],
    valueSummary: "大陆先进制程核心资产，资本开支与产能爬坡决定中期弹性。",
    growthKeywords: ["国产替代", "AI 算力链"],
    growthSummary: "叙事来自供应链安全与技术节点突破，利润周期性强。",
    revenueGrowth: [
      { q: "23Q2", pct: -14.2 },
      { q: "23Q4", pct: 3.1 },
      { q: "24Q2", pct: -6.5 },
      { q: "24Q4", pct: 8.2 },
    ],
    financeRows: [
      { label: "营业收入", ttm: "约 473 亿", yoy: "-8.3%" },
      { label: "归母净利润", ttm: "约 41 亿", yoy: "-22.1%" },
      { label: "毛利率", ttm: "21.9%", yoy: "-4.1pp" },
      { label: "资产负债率", ttm: "35.8%", yoy: "+2.2pp" },
      { label: "经营现金流/净利润", ttm: "2.35", yoy: "波动" },
    ],
    businessSegments: [
      { name: "晶圆代工", share: 95, margin: 22 },
      { name: "其他", share: 5, margin: 18 },
    ],
    shareholders: [
      { end: "2024-09-30", holders: 325000, changePct: 4.2 },
      { end: "2024-06-30", holders: 311800, changePct: -2.1 },
    ],
    dividends: [
      { year: "2023", per10: "0.87 元", yield: "0.2%" },
    ],
    flows: [
      { date: "2025-04-25", mainNet: -210, north: -88 },
      { date: "2025-04-24", mainNet: 156, north: 32 },
    ],
    news: [
      { time: "2025-04-10", title: "行业景气：稼动率预期改善但仍需验证", major: true },
    ],
  },
};

export type SectorBench = {
  id: string;
  name: string;
  avgPe: number;
  avgRoe: number;
  revGrowth: number;
  vsMarketPe: number;
};

export const mockSectors: SectorBench[] = [
  { id: "liquor", name: "白酒", avgPe: 22.5, avgRoe: 24.1, revGrowth: 12.4, vsMarketPe: 1.15 },
  { id: "semi", name: "半导体", avgPe: 48.2, avgRoe: 8.2, revGrowth: 18.9, vsMarketPe: 1.85 },
  { id: "bank", name: "银行", avgPe: 5.4, avgRoe: 10.5, revGrowth: 3.2, vsMarketPe: 0.42 },
];

export type SectorRow = {
  code: string;
  name: string;
  pe: number;
  roe: number;
  vsSectorPe: number;
};

export const mockSectorStocks: Record<string, SectorRow[]> = {
  liquor: [
    { code: "600519.SH", name: "贵州茅台", pe: 28.4, roe: 31.2, vsSectorPe: 0.26 },
    { code: "000858.SZ", name: "五粮液", pe: 18.2, roe: 24.8, vsSectorPe: -0.19 },
    { code: "000568.SZ", name: "泸州老窖", pe: 16.1, roe: 28.5, vsSectorPe: -0.28 },
  ],
  semi: [
    { code: "688981.SH", name: "中芯国际", pe: 45.0, roe: 6.2, vsSectorPe: -0.06 },
  ],
  bank: [
    { code: "601398.SH", name: "工商银行", pe: 5.2, roe: 10.1, vsSectorPe: -0.04 },
  ],
};

export const mockSectorNews: Record<string, { time: string; title: string }[]> = {
  liquor: [
    { time: "2025-04-22", title: "白酒板块：旺季动销数据分化，龙头韧性延续" },
  ],
  semi: [{ time: "2025-04-21", title: "半导体景气跟踪：设备材料订单环比改善" }],
  bank: [{ time: "2025-04-18", title: "银行股：息差压力边际缓和仍待观察" }],
};
