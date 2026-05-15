import type {
  PickItem,
  SectorBench,
  SectorRow,
  StockDetail,
} from "@shared/dataset";
import { apiGet, getApiBase } from "./client";

export interface PickStyleInfo {
  id: string;
  label: string;
  desc: string;
}

export type PicksQuery = {
  page?: number;
  pageSize?: number;
  /** 万元，默认 5_000_000 = 500 亿人民币 */
  minMvWan?: number;
  scoreMin?: number;
  scoreMax?: number;
  style?: string;
};

export type PicksPageResponse = {
  items: PickItem[];
  total: number;
  page: number;
  pageSize: number;
};

/**
 * 兼容新版 `{ items, total, page, pageSize }` 与旧版「纯数组」响应，避免 `items` 为空导致页面崩溃。
 */
export async function fetchPicks(q: PicksQuery = {}): Promise<PicksPageResponse> {
  const params = new URLSearchParams();
  if (q.page != null) params.set("page", String(q.page));
  if (q.pageSize != null) params.set("page_size", String(q.pageSize));
  if (q.minMvWan != null) params.set("min_mv_wan", String(q.minMvWan));
  if (q.scoreMin != null) params.set("score_min", String(q.scoreMin));
  if (q.scoreMax != null) params.set("score_max", String(q.scoreMax));
  if (q.style) params.set("style", q.style);
  const qs = params.toString();
  const raw = await apiGet<PicksPageResponse | PickItem[]>(`/api/picks${qs ? `?${qs}` : ""}`);

  if (Array.isArray(raw)) {
    const len = raw.length;
    return {
      items: raw,
      total: len,
      page: q.page ?? 1,
      pageSize: len > 0 ? len : (q.pageSize ?? 12),
    };
  }

  const items = Array.isArray(raw.items) ? raw.items : [];
  const total = typeof raw.total === "number" && Number.isFinite(raw.total) ? raw.total : items.length;
  const page = typeof raw.page === "number" && raw.page >= 1 ? raw.page : (q.page ?? 1);
  let ps =
    typeof raw.pageSize === "number" && raw.pageSize > 0 ? raw.pageSize : (q.pageSize ?? 12);
  if (!Number.isFinite(ps) || ps < 1) {
    ps = 12;
  }
  return { items, total, page, pageSize: ps };
}

export function fetchPickStyles(): Promise<PickStyleInfo[]> {
  return apiGet<PickStyleInfo[]>("/api/picks/styles");
}

/** 补齐数组与数值，避免个股页因字段缺失运行时崩溃（白屏）。 */
function normalizeStockDetail(raw: Partial<StockDetail> | null | undefined): StockDetail | null {
  if (!raw || typeof raw !== "object") return null;
  const code = String(raw.code ?? "").trim();
  if (!code) return null;

  const sectorAvgPe =
    typeof raw.sectorAvgPe === "number" && Number.isFinite(raw.sectorAvgPe) ? raw.sectorAvgPe : 20;

  const roeSeries = Array.isArray(raw.roeSeries)
    ? raw.roeSeries.map((x) => ({
        y: String(x?.y ?? ""),
        roe: typeof x?.roe === "number" && Number.isFinite(x.roe) ? x.roe : Number(x?.roe) || 0,
      }))
    : [];

  const shareholders = Array.isArray(raw.shareholders)
    ? raw.shareholders.map((s) => ({
        end: String(s?.end ?? ""),
        holders:
          typeof s?.holders === "number" && Number.isFinite(s.holders)
            ? s.holders
            : Math.round(Number(s?.holders) || 0),
        changePct:
          typeof s?.changePct === "number" && Number.isFinite(s.changePct)
            ? s.changePct
            : Number(s?.changePct) || 0,
      }))
    : [];

  const flows = Array.isArray(raw.flows)
    ? raw.flows.map((f) => ({
        date: String(f?.date ?? ""),
        mainNet:
          typeof f?.mainNet === "number" && Number.isFinite(f.mainNet)
            ? f.mainNet
            : Number(f?.mainNet) || 0,
        north:
          typeof f?.north === "number" && Number.isFinite(f.north) ? f.north : Number(f?.north) || 0,
      }))
    : [];

  return {
    code,
    name: String(raw.name ?? code),
    sector: String(raw.sector ?? "—"),
    sectorId: raw.sectorId,
    sectorAvgPe,
    pe: typeof raw.pe === "number" && Number.isFinite(raw.pe) ? raw.pe : Number(raw.pe) || 0,
    pePctHistory:
      typeof raw.pePctHistory === "number" && Number.isFinite(raw.pePctHistory)
        ? raw.pePctHistory
        : Number(raw.pePctHistory) || 50,
    pb: typeof raw.pb === "number" && Number.isFinite(raw.pb) ? raw.pb : Number(raw.pb) || 0,
    roe: typeof raw.roe === "number" && Number.isFinite(raw.roe) ? raw.roe : Number(raw.roe) || 0,
    roeSeries,
    valueTags: Array.isArray(raw.valueTags) ? raw.valueTags : [],
    valueSummary: String(raw.valueSummary ?? ""),
    growthKeywords: Array.isArray(raw.growthKeywords) ? raw.growthKeywords : [],
    growthSummary: String(raw.growthSummary ?? ""),
    revenueGrowth: Array.isArray(raw.revenueGrowth) ? raw.revenueGrowth : [],
    financeRows: Array.isArray(raw.financeRows) ? raw.financeRows : [],
    businessSegments: Array.isArray(raw.businessSegments) ? raw.businessSegments : [],
    shareholders,
    dividends: Array.isArray(raw.dividends) ? raw.dividends : [],
    flows,
    news: Array.isArray(raw.news) ? raw.news : [],
  };
}

export async function fetchStock(code: string): Promise<StockDetail | null> {
  const base = getApiBase();
  const url = `${base}/api/stocks/${encodeURIComponent(code)}`;
  const res = await fetch(url);
  if (res.status === 404) return null;
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(text || `HTTP ${res.status}`);
  }
  const raw = (await res.json()) as Partial<StockDetail>;
  return normalizeStockDetail(raw);
}

export interface PeHistoryPoint {
  date: string;
  pe: number;
}

export async function fetchPeHistory(code: string): Promise<PeHistoryPoint[]> {
  return apiGet<PeHistoryPoint[]>(`/api/stocks/${encodeURIComponent(code)}/pe-history`);
}

export function fetchSectorList(): Promise<SectorBench[]> {
  return apiGet<SectorBench[]>("/api/sectors");
}

export type SectorDetailResponse = {
  sector: SectorBench | null;
  stocks: SectorRow[];
  news: { time: string; title: string }[];
};

export function fetchSectorDetail(id: string): Promise<SectorDetailResponse> {
  return apiGet<SectorDetailResponse>(`/api/sectors/${encodeURIComponent(id)}`);
}
