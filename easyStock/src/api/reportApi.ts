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

export interface WikiMeta {
  stock_code: string;
  stock_name: string;
  years: number[];
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

export type UploadStreamCallbacks = {
  onStatus?: (msg: string) => void;
  onWikiChunk?: (chunk: string) => void;
  onData?: (data: ReportData) => void;
  onDone?: () => void;
  onError?: (msg: string) => void;
};

function parseSseBlock(block: string): { event: string; data: string } | null {
  const lines = block.split(/\r?\n/).filter((l) => l.length > 0);
  let ev = "";
  const dataLines: string[] = [];
  for (const line of lines) {
    if (line.startsWith("event:")) {
      ev = line.slice(6).trim();
    } else if (line.startsWith("data:")) {
      dataLines.push(line.slice(5).trimStart());
    }
  }
  if (dataLines.length === 0) return null;
  return { event: ev, data: dataLines.join("\n") };
}

export async function uploadReportStream(
  stockCode: string,
  stockName: string,
  year: number,
  file: File,
  cbs: UploadStreamCallbacks
): Promise<void> {
  const base = getApiBase();
  const fd = new FormData();
  fd.append("stock_code", stockCode);
  fd.append("stock_name", stockName);
  fd.append("year", String(year));
  fd.append("file", file);

  const res = await fetch(`${base}/api/reports/upload/stream`, {
    method: "POST",
    body: fd,
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(text || `HTTP ${res.status}`);
  }
  const reader = res.body?.getReader();
  if (!reader) {
    throw new Error("No response body");
  }
  const dec = new TextDecoder();
  let buf = "";
  let streamError = "";

  const handleBlock = (part: string) => {
    const parsed = parseSseBlock(part);
    if (!parsed) return;
    const { event, data } = parsed;
    try {
      if (event === "status" || event === "error" || event === "done") {
        const s = JSON.parse(data) as string;
        if (event === "status") cbs.onStatus?.(s);
        else if (event === "error") {
          streamError = s;
          cbs.onError?.(s);
        } else if (event === "done") cbs.onDone?.();
      } else if (event === "wiki_chunk") {
        const s = JSON.parse(data) as string;
        cbs.onWikiChunk?.(s);
      } else if (event === "data") {
        const row = JSON.parse(data) as ReportData;
        cbs.onData?.(row);
      }
    } catch {
      if (event === "error") {
        streamError = data;
        cbs.onError?.(data);
      }
    }
  };

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += dec.decode(value, { stream: true });
    const parts = buf.split("\n\n");
    buf = parts.pop() ?? "";
    for (const part of parts) {
      handleBlock(part);
    }
  }
  if (buf.trim()) {
    handleBlock(buf);
  }
  if (streamError) {
    throw new Error(streamError);
  }
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

export async function fetchWikiStockCodes(): Promise<string[]> {
  const res = await apiGet<{ stocks: string[] }>("/api/wiki");
  return res.stocks ?? [];
}

export function fetchWikiMeta(stockCode: string): Promise<WikiMeta> {
  return apiGet<WikiMeta>(
    `/api/wiki/${encodeURIComponent(stockCode)}/meta`
  );
}

export async function fetchWikiMerged(stockCode: string): Promise<string> {
  const base = getApiBase();
  const res = await fetch(`${base}/api/wiki/${encodeURIComponent(stockCode)}`);
  if (!res.ok) {
    const t = await res.text().catch(() => "");
    throw new Error(t || `HTTP ${res.status}`);
  }
  return res.text();
}

export async function fetchWikiYear(
  stockCode: string,
  year: number
): Promise<string> {
  const base = getApiBase();
  const res = await fetch(
    `${base}/api/wiki/${encodeURIComponent(stockCode)}/${year}`
  );
  if (!res.ok) {
    const t = await res.text().catch(() => "");
    throw new Error(t || `HTTP ${res.status}`);
  }
  return res.text();
}
