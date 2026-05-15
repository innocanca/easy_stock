import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Spinner } from "@/components/Spinner";
import { fetchStock } from "@/api/stockApi";
import type { StockDetail } from "@shared/dataset";
import {
  getWatchlist,
  removeFromWatchlist,
  type WatchlistItem,
} from "@/utils/watchlist";

interface StockCard {
  item: WatchlistItem;
  detail: StockDetail | null;
  loading: boolean;
}

function PeBar({ pct }: { pct: number }) {
  const clamped = Math.max(0, Math.min(100, pct));
  const color = clamped < 30 ? "var(--down, #22c55e)" : clamped < 70 ? "var(--accent)" : "var(--up, #ef4444)";
  return (
    <div className="wl-pe-bar">
      <div className="wl-pe-bar-fill" style={{ width: `${clamped}%`, background: color }} />
    </div>
  );
}

export function Watchlist() {
  const [cards, setCards] = useState<StockCard[]>([]);
  const [compareSet, setCompareSet] = useState<Set<string>>(new Set());

  useEffect(() => {
    const list = getWatchlist();
    const initial: StockCard[] = list.map((item) => ({
      item,
      detail: null,
      loading: true,
    }));
    setCards(initial);

    list.forEach((item) => {
      fetchStock(item.code)
        .then((detail) => {
          setCards((prev) =>
            prev.map((c) =>
              c.item.code === item.code ? { ...c, detail, loading: false } : c
            )
          );
        })
        .catch(() => {
          setCards((prev) =>
            prev.map((c) =>
              c.item.code === item.code ? { ...c, loading: false } : c
            )
          );
        });
    });
  }, []);

  const handleRemove = (code: string) => {
    removeFromWatchlist(code);
    setCards((prev) => prev.filter((c) => c.item.code !== code));
    setCompareSet((prev) => {
      const next = new Set(prev);
      next.delete(code);
      return next;
    });
  };

  const toggleCompare = (code: string) => {
    setCompareSet((prev) => {
      const next = new Set(prev);
      if (next.has(code)) next.delete(code);
      else if (next.size < 3) next.add(code);
      return next;
    });
  };

  const compareCards = cards.filter((c) => compareSet.has(c.item.code) && c.detail);

  return (
    <div className="home-wrap">
      <header className="home-hero">
        <p className="home-eyebrow">WATCHLIST</p>
        <h1 className="page-title home-title">自选股</h1>
        <p className="page-sub home-desc home-desc--muted">
          管理你关注的个股，勾选最多 3 只可横向对比
        </p>
      </header>

      {cards.length === 0 && (
        <div className="watchlist-empty">
          <div className="wl-empty-icon">&#9734;</div>
          <p className="wl-empty-title">暂无自选股</p>
          <p className="wl-empty-hint">在个股详情页或 AI 智选中点击「加入自选」添加</p>
          <Link to="/" className="wl-empty-cta">去推荐页看看 &rarr;</Link>
        </div>
      )}

      {compareCards.length >= 2 && (
        <div className="wl-compare-panel">
          <h3 className="wl-compare-title">
            个股对比
            <span className="wl-compare-count">{compareCards.length} 只</span>
          </h3>
          <div className="compare-table-wrap">
            <table className="compare-table">
              <thead>
                <tr>
                  <th>指标</th>
                  {compareCards.map((c) => (
                    <th key={c.item.code}>
                      <Link to={`/stock/${encodeURIComponent(c.item.code)}`}>{c.detail!.name}</Link>
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {[
                  { label: "PE (TTM)", key: "pe" },
                  { label: "PB", key: "pb" },
                  { label: "ROE", key: "roe", suffix: "%" },
                  { label: "板块均 PE", key: "sectorAvgPe" },
                  { label: "PE 历史分位", key: "pePctHistory", suffix: "%" },
                  { label: "行业", key: "sector" },
                ].map((row) => (
                  <tr key={row.key}>
                    <td>{row.label}</td>
                    {compareCards.map((c) => {
                      const v = (c.detail as Record<string, unknown>)?.[row.key];
                      return <td key={c.item.code}>{v != null ? `${v}${row.suffix ?? ""}` : "—"}</td>;
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {compareSet.size > 0 && compareSet.size < 2 && (
        <div className="wl-compare-hint">
          再选 {2 - compareSet.size} 只即可对比
        </div>
      )}

      <div className="watchlist-grid">
        {cards.map((c) => {
          const d = c.detail;
          const isCompare = compareSet.has(c.item.code);
          return (
            <article key={c.item.code} className={`wl-card${isCompare ? " wl-card--selected" : ""}`}>
              <div className="wl-card-top">
                <Link to={`/stock/${encodeURIComponent(c.item.code)}`} className="wl-card-name">
                  {c.item.name}
                </Link>
                <span className="wl-card-code">{c.item.code}</span>
              </div>

              {c.loading ? (
                <div className="wl-card-loading"><Spinner text="" /></div>
              ) : d ? (
                <>
                  <div className="wl-card-sector">{d.sector}</div>
                  <div className="wl-card-metrics">
                    <div className="wl-metric">
                      <span className="wl-metric-label">PE</span>
                      <span className="wl-metric-val">{d.pe}</span>
                    </div>
                    <div className="wl-metric">
                      <span className="wl-metric-label">PB</span>
                      <span className="wl-metric-val">{d.pb}</span>
                    </div>
                    <div className="wl-metric">
                      <span className="wl-metric-label">ROE</span>
                      <span className="wl-metric-val">{d.roe}%</span>
                    </div>
                  </div>
                  <div className="wl-card-pe-row">
                    <span className="wl-pe-label">PE 历史分位</span>
                    <span className="wl-pe-val">{d.pePctHistory}%</span>
                  </div>
                  <PeBar pct={d.pePctHistory} />
                  {d.roeSeries && d.roeSeries.length > 0 && (
                    <div className="wl-card-roe-series">
                      {d.roeSeries.map((r) => (
                        <div key={r.y} className="wl-roe-item">
                          <span className="wl-roe-year">{r.y}</span>
                          <span className="wl-roe-val">{r.roe}%</span>
                        </div>
                      ))}
                    </div>
                  )}
                </>
              ) : (
                <p className="wl-card-err">数据加载失败</p>
              )}

              <div className="wl-card-actions">
                <button
                  type="button"
                  className={`wl-action-btn wl-action-btn--compare${isCompare ? " wl-action-btn--active" : ""}`}
                  onClick={() => toggleCompare(c.item.code)}
                  disabled={!isCompare && compareSet.size >= 3}
                >
                  {isCompare ? "取消对比" : "加入对比"}
                </button>
                <button
                  type="button"
                  className="wl-action-btn wl-action-btn--remove"
                  onClick={() => handleRemove(c.item.code)}
                >
                  移除
                </button>
              </div>
            </article>
          );
        })}
      </div>
    </div>
  );
}
