import type {
  PickItem,
  SectorBench,
  SectorRow,
  StockDetail,
} from "@shared/dataset";
import { apiGet, getApiBase } from "./client";

export function fetchPicks(): Promise<PickItem[]> {
  return apiGet<PickItem[]>("/api/picks");
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
  return res.json() as Promise<StockDetail>;
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
