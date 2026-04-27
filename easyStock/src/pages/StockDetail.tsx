import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type { StockDetail as StockDetailT } from "@shared/dataset";
import { fetchStock } from "@/api/stockApi";
import { sectorNameToId } from "@/utils/sector";

const TABS = [
  { id: "finance", label: "财务概览" },
  { id: "business", label: "主业构成" },
  { id: "holders", label: "股东与分红" },
  { id: "flow", label: "资金与行情" },
  { id: "news", label: "资讯" },
] as const;

type TabId = (typeof TABS)[number]["id"];

export function StockDetail() {
  const { code = "" } = useParams();
  const decoded = decodeURIComponent(code);
  const [stock, setStock] = useState<StockDetailT | null | undefined>(undefined);
  const [err, setErr] = useState<string | null>(null);
  const [tab, setTab] = useState<TabId>("finance");

  useEffect(() => {
    let cancel = false;
    setStock(undefined);
    setErr(null);
    fetchStock(decoded)
      .then((s) => {
        if (!cancel) setStock(s);
      })
      .catch((e: unknown) => {
        if (!cancel) setErr(e instanceof Error ? e.message : "加载失败");
      });
    return () => {
      cancel = true;
    };
  }, [decoded]);

  const sectorRouteId = stock
    ? stock.sectorId && stock.sectorId.length > 0
      ? stock.sectorId
      : sectorNameToId(stock.sector)
    : "liquor";

  const roeSpark = useMemo(() => {
    if (!stock) return null;
    const vals = stock.roeSeries.map((x) => x.roe);
    const max = Math.max(...vals);
    const min = Math.min(...vals);
    const span = max - min || 1;
    return stock.roeSeries.map((x, i) => ({
      ...x,
      h: ((x.roe - min) / span) * 100,
      i,
    }));
  }, [stock]);

  if (err) {
    return (
      <>
        <h1 className="page-title">加载失败</h1>
        <p className="page-sub" style={{ color: "var(--up)" }}>
          {err}
        </p>
        <Link to="/">返回首页</Link>
      </>
    );
  }

  if (stock === undefined) {
    return (
      <>
        <h1 className="page-title">个股</h1>
        <p className="page-sub">加载中…</p>
      </>
    );
  }

  if (stock === null) {
    return (
      <>
        <h1 className="page-title">未找到个股</h1>
        <p className="page-sub">代码：{decoded}</p>
        <Link to="/">返回首页</Link>
      </>
    );
  }

  const peVsSector = ((stock.pe - stock.sectorAvgPe) / stock.sectorAvgPe) * 100;

  return (
    <>
      <div className="stock-summary">
        <div className="stock-summary-top">
          <div>
            <h1>
              {stock.name}{" "}
              <span className="stock-meta">
                <code>{stock.code}</code>
                <Link to={`/sector/${sectorRouteId}`}>板块：{stock.sector}</Link>
              </span>
            </h1>
          </div>
        </div>

        <div className="summary-grid">
          <div className="summary-block">
            <h3>价值</h3>
            <div className="tags">
              {stock.valueTags.map((t) => (
                <span key={t} className="tag">
                  {t}
                </span>
              ))}
            </div>
            <p>{stock.valueSummary}</p>
          </div>
          <div className="summary-block">
            <h3>估值</h3>
            <p className="valuation-line">
              PE <strong>{stock.pe}</strong> · PB {stock.pb}
            </p>
            <p>
              相对自身历史约 <strong>{stock.pePctHistory}%</strong> 分位；较板块 PE 均值{" "}
              <span className={peVsSector > 0 ? "positive" : "negative"}>
                {peVsSector > 0 ? "溢价" : "折价"} {Math.abs(peVsSector).toFixed(1)}%
              </span>
            </p>
          </div>
          <div className="summary-block">
            <h3>确定性（ROE）</h3>
            <p>
              当前 ROE <strong>{stock.roe}%</strong>
            </p>
            {roeSpark && (
              <div className="roe-spark">
                {roeSpark.map((x) => (
                  <span
                    key={x.y}
                    title={`${x.y}: ${x.roe}%`}
                    style={{
                      display: "inline-block",
                      width: 10,
                      height: `${12 + x.h * 0.35}px`,
                      background: "var(--accent)",
                      borderRadius: 2,
                      opacity: 0.6 + x.i * 0.1,
                    }}
                  />
                ))}
                <span className="roe-years">{roeSpark.map((x) => x.y).join(" → ")}</span>
              </div>
            )}
          </div>
          <div className="summary-block">
            <h3>成长性</h3>
            <div className="tags">
              {stock.growthKeywords.map((t) => (
                <span key={t} className="tag">
                  {t}
                </span>
              ))}
            </div>
            <p>{stock.growthSummary}</p>
            <div className="mini-chart-row compact">
              {stock.revenueGrowth.map((g, i) => (
                <div
                  key={g.q}
                  className="mini-bar"
                  style={{
                    height: `${30 + Math.max(0, g.pct + 20)}%`,
                    background: g.pct >= 0 ? "var(--accent)" : "var(--bar-negative)",
                    opacity: 0.65 + i * 0.08,
                  }}
                  title={`${g.q}: ${g.pct}%`}
                />
              ))}
            </div>
          </div>
        </div>
      </div>

      <div className="tabs">
        {TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            className={tab === t.id ? "active" : ""}
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </button>
        ))}
      </div>

      <div className="panel">
        {tab === "finance" && (
          <table>
            <thead>
              <tr>
                <th>指标</th>
                <th>TTM / 当期</th>
                <th>同比</th>
              </tr>
            </thead>
            <tbody>
              {stock.financeRows.map((row) => (
                <tr key={row.label}>
                  <td>{row.label}</td>
                  <td>{row.ttm}</td>
                  <td>{row.yoy}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        {tab === "business" && (
          <table>
            <thead>
              <tr>
                <th>分部</th>
                <th>收入占比</th>
                <th>毛利率（示意）</th>
              </tr>
            </thead>
            <tbody>
              {stock.businessSegments.map((s) => (
                <tr key={s.name}>
                  <td>{s.name}</td>
                  <td>{s.share}%</td>
                  <td>{s.margin}%</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        {tab === "holders" && (
          <>
            <h4 className="panel-subtitle">股东人数</h4>
            <table>
              <thead>
                <tr>
                  <th>截止日</th>
                  <th>人数</th>
                  <th>环比</th>
                </tr>
              </thead>
              <tbody>
                {stock.shareholders.map((s) => (
                  <tr key={s.end}>
                    <td>{s.end}</td>
                    <td>{s.holders.toLocaleString()}</td>
                    <td className={s.changePct >= 0 ? "positive" : "negative"}>
                      {s.changePct >= 0 ? "+" : ""}
                      {s.changePct}%
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            <h4 className="panel-subtitle">分红</h4>
            <table>
              <thead>
                <tr>
                  <th>年度</th>
                  <th>每 10 股</th>
                  <th>股息率（示意）</th>
                </tr>
              </thead>
              <tbody>
                {stock.dividends.map((d) => (
                  <tr key={d.year}>
                    <td>{d.year}</td>
                    <td>{d.per10}</td>
                    <td>{d.yield}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </>
        )}
        {tab === "flow" && (
          <table>
            <thead>
              <tr>
                <th>日期</th>
                <th>主力净流入（百万）</th>
                <th>北向（百万）</th>
              </tr>
            </thead>
            <tbody>
              {stock.flows.map((f) => (
                <tr key={f.date}>
                  <td>{f.date}</td>
                  <td className={f.mainNet >= 0 ? "positive" : "negative"}>{f.mainNet}</td>
                  <td className={f.north >= 0 ? "positive" : "negative"}>{f.north}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        {tab === "news" && (
          <ul className="news-list">
            {stock.news.map((n) => (
              <li key={n.time + n.title}>
                <span className="news-time">{n.time}</span>
                {n.major ? <span className="tag">重大</span> : null}{" "}
                {n.title}
              </li>
            ))}
          </ul>
        )}
      </div>
    </>
  );
}
