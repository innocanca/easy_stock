import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type { SectorBench, SectorRow } from "@shared/dataset";
import { fetchSectorDetail, fetchSectorList } from "@/api/stockApi";

function IconPeAscending({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <path d="M4 20h4v-8H4v8zm6 0h4V4h-4v16zm6 0h4v-11h-4v11z" fill="currentColor" opacity=".35" />
      <path d="M4 14h4V4H4v10zm6 6h4v-6h-4v6zm6-11h4V4h-4v5z" fill="currentColor" />
    </svg>
  );
}

export function Sector() {
  const { id = "" } = useParams();

  const [sectors, setSectors] = useState<SectorBench[] | null>(null);
  const [stocks, setStocks] = useState<SectorRow[]>([]);
  const [news, setNews] = useState<{ time: string; title: string }[]>([]);
  const [bench, setBench] = useState<SectorBench | null>(null);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    let cancel = false;
    fetchSectorList()
      .then((list) => {
        if (!cancel) setSectors(list);
      })
      .catch((e: unknown) => {
        if (!cancel) setErr(e instanceof Error ? e.message : "加载板块列表失败");
      });
    return () => {
      cancel = true;
    };
  }, []);

  useEffect(() => {
    let cancel = false;
    setLoading(true);
    setErr(null);
    setBench(null);
    setStocks([]);
    setNews([]);
    fetchSectorDetail(id)
      .then((d) => {
        if (cancel) return;
        setBench(d.sector);
        setStocks(d.stocks ?? []);
        setNews(d.news ?? []);
      })
      .catch((e: unknown) => {
        if (!cancel) setErr(e instanceof Error ? e.message : "加载板块详情失败");
      })
      .finally(() => {
        if (!cancel) setLoading(false);
      });
    return () => {
      cancel = true;
    };
  }, [id]);

  const sortedRows = useMemo(() => {
    return [...stocks].sort((a, b) => b.vsSectorPe - a.vsSectorPe);
  }, [stocks]);

  /** PE 数值由低到高（无效或缺失 PE 排在末尾） */
  const sortedByPeAsc = useMemo(() => {
    const score = (r: SectorRow) =>
      Number.isFinite(r.pe) && r.pe > 0 ? r.pe : Number.POSITIVE_INFINITY;
    return [...stocks].sort((a, b) => {
      const d = score(a) - score(b);
      if (d !== 0) return d;
      return a.code.localeCompare(b.code);
    });
  }, [stocks]);

  if (err && sectors === null) {
    return (
      <>
        <h1 className="page-title">板块</h1>
        <p className="page-sub" style={{ color: "var(--up)" }}>
          {err}
        </p>
        <Link to="/">返回首页</Link>
      </>
    );
  }

  if (sectors === null) {
    return (
      <>
        <h1 className="page-title">板块</h1>
        <p className="page-sub">加载中…</p>
      </>
    );
  }

  const sector = bench;

  return (
    <>
      <h1 className="page-title">板块</h1>
      <p className="page-sub">均值基准 + 行业内个股相对偏离（数据来自 API）。</p>

      <div className="sector-layout">
        <nav className="sector-list" aria-label="板块切换">
          {sectors.map((s) => (
            <Link key={s.id} to={`/sector/${s.id}`} className={s.id === id ? "active" : ""}>
              {s.name}
            </Link>
          ))}
        </nav>

        <div>
          {loading && <p className="page-sub">加载详情…</p>}
          {err && !loading && (
            <p className="page-sub" style={{ color: "var(--up)" }}>
              {err}
            </p>
          )}
          {!sector && !loading && !err ? (
            <p className="empty-state">未找到该板块。</p>
          ) : sector ? (
            <>
              <h2 className="sector-detail-title">{sector.name}</h2>
              <div className="bench-grid">
                <div className="bench-card">
                  <div className="label">平均 PE</div>
                  <div className="value">{sector.avgPe}</div>
                </div>
                <div className="bench-card">
                  <div className="label">平均 ROE</div>
                  <div className="value">{sector.avgRoe}%</div>
                </div>
                <div className="bench-card">
                  <div className="label">营收增速（示意）</div>
                  <div className="value">{sector.revGrowth}%</div>
                </div>
                <div className="bench-card">
                  <div className="label">相对全市场 PE</div>
                  <div className="value">{sector.vsMarketPe}x</div>
                </div>
              </div>

              {sortedByPeAsc.length > 0 ? (
                <div className="sector-pe-sort-block">
                  <p className="section-label sector-pe-sort-heading">
                    <IconPeAscending className="sector-pe-sort-icon" />
                    <span>PE 由低到高</span>
                    <span className="sector-pe-sort-hint">左侧估值更低 → 右侧更高</span>
                  </p>
                  <div className="sector-pe-strip-scroll">
                    <ul className="sector-pe-strip" aria-label="行业内按市盈率从低到高">
                      {sortedByPeAsc.map((r, i) => (
                        <li key={r.code} className="sector-pe-strip-item">
                          <Link
                            className={`sector-pe-chip${i < 3 ? ` sector-pe-chip--rank-${i + 1}` : ""}`}
                            to={`/stock/${encodeURIComponent(r.code)}`}
                            title={`${r.name} · PE ${Number.isFinite(r.pe) && r.pe > 0 ? r.pe : "—"}`}
                          >
                            <span className="sector-pe-chip-rank" aria-hidden>
                              {i + 1}
                            </span>
                            <span className="sector-pe-chip-text">
                              <span className="sector-pe-chip-name">{r.name}</span>
                              <span className="sector-pe-chip-pe">
                                PE{" "}
                                <strong>
                                  {Number.isFinite(r.pe) && r.pe > 0 ? r.pe : "—"}
                                </strong>
                              </span>
                            </span>
                          </Link>
                          {i < sortedByPeAsc.length - 1 ? (
                            <span className="sector-pe-strip-arrow" aria-hidden>
                              <svg width="12" height="12" viewBox="0 0 24 24" fill="none">
                                <path
                                  d="M9 6l6 6-6 6"
                                  stroke="currentColor"
                                  strokeWidth="2.2"
                                  strokeLinecap="round"
                                  strokeLinejoin="round"
                                />
                              </svg>
                            </span>
                          ) : null}
                        </li>
                      ))}
                    </ul>
                  </div>
                </div>
              ) : null}

              <p className="section-label">个股 vs 板块（按相对 PE 偏离排序）</p>
              <div className="panel table-only" style={{ marginBottom: "1.25rem" }}>
                <table>
                  <thead>
                    <tr>
                      <th>代码</th>
                      <th>名称</th>
                      <th>PE</th>
                      <th>ROE</th>
                      <th>相对板块 PE</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sortedRows.map((r) => (
                      <tr key={r.code}>
                        <td>
                          <Link to={`/stock/${encodeURIComponent(r.code)}`}>{r.code}</Link>
                        </td>
                        <td>
                          <Link to={`/stock/${encodeURIComponent(r.code)}`}>{r.name}</Link>
                        </td>
                        <td>{r.pe}</td>
                        <td>{r.roe}%</td>
                        <td className={r.vsSectorPe >= 0 ? "positive" : "negative"}>
                          {r.vsSectorPe >= 0 ? "+" : ""}
                          {(r.vsSectorPe * 100).toFixed(0)}%
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <p className="section-label">行业资讯</p>
              <div className="panel">
                <ul className="news-list">
                  {news.map((n) => (
                    <li key={n.time + n.title}>
                      <span className="news-time">{n.time}</span>
                      {n.title}
                    </li>
                  ))}
                </ul>
              </div>
            </>
          ) : null}
        </div>
      </div>
    </>
  );
}
