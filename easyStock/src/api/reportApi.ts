import { apiGet, getApiBase } from "./client";

export interface Segment {
  name: string;
  revenue: number;
  ratio: number;
}

export interface ReportData {
  stock_code: string;
  stock_name: string;
  year: number;
  revenue: number;
  revenue_yoy: number;
  net_profit: number;
  net_profit_yoy: number;
  net_profit_parent: number;
  gross_margin: number;
  net_margin: number;
  roe: number;
  total_assets: number;
  net_assets: number;
  debt_ratio: number;
  operating_cashflow: number;
  eps: number;
  dividend_per_share: number;
  employee_count: number;
  rd_expense: number;
  segments: Segment[];
  highlights: string;
  risks: string;
  outlook: string;
}

export interface AnalysisResult {
  stock_code: string;
  stock_name: string;
  years: ReportData[];
  summary: string;
}

export interface UploadResponse {
  success: boolean;
  message: string;
  data?: ReportData;
}

export interface ListResponse {
  stock_code: string;
  reports: ReportData[];
}

export function fetchReportList(stockCode: string): Promise<ListResponse> {
  return apiGet<ListResponse>(`/api/reports/${encodeURIComponent(stockCode)}`);
}

export function fetchAnalysis(
  stockCode: string,
  refresh = false
): Promise<AnalysisResult> {
  const qs = refresh ? "?refresh=1" : "";
  return apiGet<AnalysisResult>(
    `/api/reports/${encodeURIComponent(stockCode)}/analysis${qs}`
  );
}

export async function uploadReport(
  stockCode: string,
  stockName: string,
  year: number,
  file: File
): Promise<UploadResponse> {
  const base = getApiBase();
  const fd = new FormData();
  fd.append("stock_code", stockCode);
  fd.append("stock_name", stockName);
  fd.append("year", String(year));
  fd.append("file", file);

  const res = await fetch(`${base}/api/reports/upload`, {
    method: "POST",
    body: fd,
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(text || `HTTP ${res.status}`);
  }
  return res.json() as Promise<UploadResponse>;
}

export async function deleteReport(
  stockCode: string,
  year: number
): Promise<void> {
  const base = getApiBase();
  await fetch(
    `${base}/api/reports/${encodeURIComponent(stockCode)}/${year}`,
    { method: "DELETE" }
  );
}
