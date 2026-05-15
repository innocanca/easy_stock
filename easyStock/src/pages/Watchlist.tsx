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
    <div>
      <div className="home-hero">
        <p className="home-eyebrow">WATCHLIST</p>
        <h1 className="page-title home-title">自选股</h1>
        <p className="page-sub home-desc">
          管理你关注的个股，支持横向对比
        </p>
      </div>

      {cards.length === 0 && (
        <div className="watchlist-empty">
          <p>暂无自选股</p>
          <p>在个股详情页点击「加入自选」添加</p>
          <Link to="/" className="report-btn" style={{ textDecoration: "none" }}>
            去推荐页看看 →
          </Link>
        </div>
      )}

      {compareCards.length >= 2 && (
        <div className="panel" style={{ marginBottom: "1.5rem" }}>
          <h3 style={{ margin: "0 0 0.75rem", fontSize: "0.95rem" }}>
            个股对比（{compareCards.length}只）
          </h3>
          <div className="compare-table-wrap">
            <table className="compare-table">
              <thead>
                <tr>
                  <th>指标</th>
                  {compareCards.map((c) => (
                    <th key={c.item.code}>{c.detail!.name}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td>PE (TTM)</td>
                  {compareCards.map((c) => (
                    <td key={c.item.code}>{c.detail!.pe}</td>
                  ))}
                </tr>
                <tr>
                  <td>PB</td>
                  {compareCards.map((c) => (
                    <td key={c.item.code}>{c.detail!.pb}</td>
                  ))}
                </tr>
                <tr>
                  <td>ROE</td>
                  {compareCards.map((c) => (
                    <td key={c.item.code}>{c.detail!.roe}%</td>
                  ))}
                </tr>
                <tr>
                  <td>板块均PE</td>
                  {compareCards.map((c) => (
                    <td key={c.item.code}>{c.detail!.sectorAvgPe || "—"}</td>
                  ))}
                </tr>
                <tr>
                  <td>PE历史分位</td>
                  {compareCards.map((c) => (
                    <td key={c.item.code}>{c.detail!.pePctHistory}%</td>
                  ))}
                </tr>
                <tr>
                  <td>行业</td>
                  {compareCards.map((c) => (
                    <td key={c.item.code}>{c.detail!.sector}</td>
                  ))}
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      )}

      {compareSet.size > 0 && compareSet.size < 2 && (
        <div className="compare-bar">
          <span style={{ fontSize: "0.82rem", color: "var(--muted)" }}>
            再选 {2 - compareSet.size} 只股票即可对比
          </span>
        </div>
      )}

      <div className="watchlist-grid">
        {cards.map((c) => (
          <div key={c.item.code} className="watchlist-card">
            <div className="watchlist-card-header">
              <Link
                to={`/stock/${encodeURIComponent(c.item.code)}`}
                className="watchlist-card-name"
                style={{ textDecoration: "none", color: "var(--text)" }}
              >
                {c.item.name}
              </Link>
              <span className="watchlist-card-code">{c.item.code}</span>
            </div>
            {c.loading ? (
              <Spinner text="" />
            ) : c.detail ? (
              <div className="watchlist-card-stats">
                <div className="watchlist-card-stat">
                  <label>PE</label> {c.detail.pe}
                </div>
                <div className="watchlist-card-stat">
                  <label>PB</label> {c.detail.pb}
                </div>
                <div className="watchlist-card-stat">
                  <label>ROE</label> {c.detail.roe}%
                </div>
              </div>
            ) : (
              <p style={{ fontSize: "0.82rem", color: "var(--muted)" }}>数据加载失败</p>
            )}
            <div className="watchlist-card-actions">
              <button
                className={`btn-star${compareSet.has(c.item.code) ? " active" : ""}`}
                onClick={() => toggleCompare(c.item.code)}
                disabled={!compareSet.has(c.item.code) && compareSet.size >= 3}
              >
                {compareSet.has(c.item.code) ? "取消对比" : "加入对比"}
              </button>
              <button
                className="btn-star"
                onClick={() => handleRemove(c.item.code)}
                style={{ color: "var(--up)" }}
              >
                移除
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
