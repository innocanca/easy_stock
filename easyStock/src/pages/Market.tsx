import { useEffect, useRef, useState, useCallback } from "react";
import { getApiBase } from "../api/client";
import { MarkdownBody } from "../components/MarkdownBody";

interface IndexPoint {
  code: string;
  name: string;
  close: number;
  chgPct: number;
  amount: number;
  vol: number;
}

interface MarketBreadth {
  upCount: number;
  downCount: number;
  flatCount: number;
  limitUpCount: number;
  limitDownCount: number;
  total: number;
}

interface NorthFlowData {
  hgtNetBuy: number;
  sgtNetBuy: number;
  totalNet: number;
}

interface SectorChange {
  name: string;
  avgPe: number;
  chgPct: number;
}

interface MarketSnapshot {
  date: string;
  indices: IndexPoint[];
  breadth: MarketBreadth;
  northFlow: NorthFlowData;
  topSectors: SectorChange[];
  bottomSectors: SectorChange[];
  totalAmount: number;
  amountChgPct: number;
}

interface HistoryItem {
  date: string;
  indices: IndexPoint[];
  totalAmount: number;
  amountChgPct: number;
  hasSummary: boolean;
}

function formatDate(d: string) {
  if (d.length !== 8) return d;
  return `${d.slice(0, 4)}-${d.slice(4, 6)}-${d.slice(6, 8)}`;
}

export default function Market() {
  const [snapshot, setSnapshot] = useState<MarketSnapshot | null>(null);
  const [summary, setSummary] = useState("");
  const [history, setHistory] = useState<HistoryItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");
  const [selectedDate, setSelectedDate] = useState("");
  const summaryRef = useRef("");

  const fetchHistory = useCallback(async () => {
    try {
      const res = await fetch(`${getApiBase()}/api/market/history?days=30`);
      if (res.ok) {
        const data: HistoryItem[] = await res.json();
        setHistory(data);
      }
    } catch {
      /* ignore */
    }
  }, []);

  useEffect(() => {
    fetchHistory();
  }, [fetchHistory]);

  const loadDaily = useCallback(async (date: string) => {
    try {
      const res = await fetch(`${getApiBase()}/api/market/daily?date=${date}`);
      if (!res.ok) return false;
      const data = await res.json();
      if (data.snapshot) {
        setSnapshot(data.snapshot);
        setSummary(data.summary || "");
        setSelectedDate(date);
        return true;
      }
    } catch {
      /* ignore */
    }
    return false;
  }, []);

  const collectToday = useCallback(() => {
    setLoading(true);
    setError("");
    setStatus("正在采集…");
    setSnapshot(null);
    setSummary("");
    summaryRef.current = "";

    const es = new EventSource(`${getApiBase()}/api/market/collect`);
    es.addEventListener("status", (e: MessageEvent) => {
      setStatus(JSON.parse(e.data));
    });
    es.addEventListener("snapshot", (e: MessageEvent) => {
      const snap: MarketSnapshot = JSON.parse(e.data);
      setSnapshot(snap);
      setSelectedDate(snap.date);
    });
    es.addEventListener("chunk", (e: MessageEvent) => {
      const tok = JSON.parse(e.data) as string;
      summaryRef.current += tok;
      setSummary(summaryRef.current);
    });
    es.addEventListener("error", (e: MessageEvent) => {
      setError(JSON.parse(e.data));
      setLoading(false);
      es.close();
    });
    es.addEventListener("done", () => {
      setLoading(false);
      setStatus("");
      es.close();
      fetchHistory();
    });
    es.onerror = () => {
      setLoading(false);
      setError((prev) =>
        prev ||
          "连接已断开（常见于 AI 首段输出较慢或开发代理超时）。请重试；若反复出现，可在 .env 设置 VITE_API_URL=http://127.0.0.1:4000 直连后端。",
      );
      es.close();
    };
  }, [fetchHistory]);

  const pctColor = (v: number) =>
    v > 0 ? "var(--c-up, #e74c3c)" : v < 0 ? "var(--c-down, #27ae60)" : "var(--text-secondary)";

  const pctSign = (v: number) => (v > 0 ? "+" : "");

  return (
    <div className="market-page">
      <div className="market-header">
        <h1>大盘追踪</h1>
        <button
          className="btn-collect"
          onClick={collectToday}
          disabled={loading}
        >
          {loading ? "采集中…" : "采集今日数据"}
        </button>
      </div>

      {status && <div className="market-status">{status}</div>}
      {error && <div className="market-error">{error}</div>}

      {/* Index Cards */}
      {snapshot && (
        <div className="market-indices">
          {snapshot.indices.map((idx) => (
            <div
              key={idx.code}
              className="index-card"
              style={{ borderColor: pctColor(idx.chgPct) }}
            >
              <div className="index-name">{idx.name}</div>
              <div className="index-close" style={{ color: pctColor(idx.chgPct) }}>
                {idx.close.toFixed(2)}
              </div>
              <div className="index-pct" style={{ color: pctColor(idx.chgPct) }}>
                {pctSign(idx.chgPct)}{idx.chgPct.toFixed(2)}%
              </div>
              <div className="index-amount">成交 {idx.amount.toFixed(0)} 亿</div>
            </div>
          ))}
        </div>
      )}

      {/* Breadth + North + Volume */}
      {snapshot && (
        <div className="market-stats-grid">
          <div className="stat-card">
            <h3>涨跌统计</h3>
            <div className="breadth-bar">
              <div
                className="breadth-up"
                style={{
                  width: `${(snapshot.breadth.upCount / Math.max(snapshot.breadth.total, 1)) * 100}%`,
                }}
              >
                涨 {snapshot.breadth.upCount}
              </div>
              <div
                className="breadth-flat"
                style={{
                  width: `${(snapshot.breadth.flatCount / Math.max(snapshot.breadth.total, 1)) * 100}%`,
                }}
              />
              <div
                className="breadth-down"
                style={{
                  width: `${(snapshot.breadth.downCount / Math.max(snapshot.breadth.total, 1)) * 100}%`,
                }}
              >
                跌 {snapshot.breadth.downCount}
              </div>
            </div>
            <div className="breadth-limits">
              <span className="limit-up">涨停 {snapshot.breadth.limitUpCount}</span>
              <span className="limit-down">跌停 {snapshot.breadth.limitDownCount}</span>
            </div>
          </div>

          <div className="stat-card">
            <h3>北向资金</h3>
            <div className="north-flow-total" style={{ color: pctColor(snapshot.northFlow.totalNet) }}>
              {pctSign(snapshot.northFlow.totalNet)}{snapshot.northFlow.totalNet.toFixed(2)} 亿
            </div>
            <div className="north-flow-detail">
              <span>沪股通 {pctSign(snapshot.northFlow.hgtNetBuy)}{snapshot.northFlow.hgtNetBuy.toFixed(2)}亿</span>
              <span>深股通 {pctSign(snapshot.northFlow.sgtNetBuy)}{snapshot.northFlow.sgtNetBuy.toFixed(2)}亿</span>
            </div>
          </div>

          <div className="stat-card">
            <h3>量能</h3>
            <div className="volume-total">{snapshot.totalAmount.toFixed(0)} 亿</div>
            <div
              className="volume-chg"
              style={{ color: pctColor(snapshot.amountChgPct) }}
            >
              较前日 {pctSign(snapshot.amountChgPct)}{snapshot.amountChgPct.toFixed(2)}%
            </div>
          </div>
        </div>
      )}

      {/* Sector performance */}
      {snapshot && (snapshot.topSectors?.length > 0 || snapshot.bottomSectors?.length > 0) && (
        <div className="market-sectors-row">
          {snapshot.topSectors?.length > 0 && (
            <div className="sector-col">
              <h3>领涨板块</h3>
              <div className="sector-list">
                {snapshot.topSectors.map((s) => (
                  <div key={s.name} className="sector-item sector-up">
                    <span className="sector-name">{s.name}</span>
                    <span className="sector-pct" style={{ color: pctColor(s.chgPct) }}>
                      {pctSign(s.chgPct)}{s.chgPct.toFixed(2)}%
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
          {snapshot.bottomSectors?.length > 0 && (
            <div className="sector-col">
              <h3>领跌板块</h3>
              <div className="sector-list">
                {snapshot.bottomSectors.map((s) => (
                  <div key={s.name} className="sector-item sector-down">
                    <span className="sector-name">{s.name}</span>
                    <span className="sector-pct" style={{ color: pctColor(s.chgPct) }}>
                      {pctSign(s.chgPct)}{s.chgPct.toFixed(2)}%
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* AI Summary */}
      {summary && (
        <div className="market-ai-summary">
          <h2>AI 市场日报</h2>
          <MarkdownBody markdown={summary} />
        </div>
      )}

      {/* History sidebar */}
      {history.length > 0 && (
        <div className="market-history">
          <h3>历史日报</h3>
          <div className="history-list">
            {history.map((h) => {
              const sh = h.indices?.[0];
              return (
                <div
                  key={h.date}
                  className={`history-item${selectedDate === h.date ? " active" : ""}`}
                  onClick={() => loadDaily(h.date)}
                >
                  <span className="history-date">{formatDate(h.date)}</span>
                  {sh && (
                    <span className="history-pct" style={{ color: pctColor(sh.chgPct) }}>
                      {pctSign(sh.chgPct)}{sh.chgPct.toFixed(2)}%
                    </span>
                  )}
                  {h.hasSummary && <span className="history-ai-badge">AI</span>}
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
