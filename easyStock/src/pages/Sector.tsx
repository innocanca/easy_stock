import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type { SectorBench, SectorRow } from "@shared/dataset";
import { fetchSectorDetail, fetchSectorList } from "@/api/stockApi";
import { Spinner } from "@/components/Spinner";

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

function sectorBenchPeScore(s: SectorBench) {
  return Number.isFinite(s.avgPe) && s.avgPe > 0 ? s.avgPe : Number.POSITIVE_INFINITY;
}

/** 平方根刻度 + 最小柱高：压低极端高 PE 对纵轴的挤压，让左侧低 PE 柱子仍可辨认 */
function peToBarHeight(pe: number, maxPe: number, innerH: number, minPx: number): number {
  if (!(pe > 0) || !(maxPe > 0)) return minPx;
  const sqrtH = Math.sqrt(pe / maxPe) * innerH;
  return Math.min(innerH, Math.max(minPx, sqrtH));
}

function sectorChartLabelText(name: string, slotW: number): string {
  const maxChars = slotW >= 52 ? 12 : slotW >= 44 ? 10 : slotW >= 36 ? 8 : 6;
  if (name.length <= maxChars) return name;
  return `${name.slice(0, Math.max(2, maxChars - 1))}…`;
}

/** 各板块平均市盈率柱状图：横轴为行业板块，按平均 PE 从低到高排列 */
function SectorsAvgPeBarChart({
  sectors,
  activeId,
}: {
  sectors: SectorBench[];
  activeId: string;
}) {
  /** bottom 留足斜排文字空间，避免裁切 */
  const padding = { top: 12, right: 10, bottom: 62, left: 40 };
  /** 略增高绘图区，低 PE 柱子更清晰 */
  const innerH = 168;
  const minBarPx = 18;
  /**
   * 固定柱间距：不因板块数量变多而把柱子压扁挤在一屏。
   * 图会变宽，靠横向滚动浏览（单屏约可见 ~14～18 根，视窗口宽度而定）。
   */
  const slotW = 54;
  const barW = 38;
  const innerW = sectors.length * slotW;
  const W = padding.left + innerW + padding.right;
  const H = padding.top + innerH + padding.bottom;

  const maxPe = useMemo(() => {
    let m = 0;
    for (const s of sectors) {
      if (Number.isFinite(s.avgPe) && s.avgPe > 0) m = Math.max(m, s.avgPe);
    }
    return m > 0 ? m : 1;
  }, [sectors]);

  const meanPe = useMemo(() => {
    const vals = sectors.map((s) => s.avgPe).filter((p) => Number.isFinite(p) && p > 0);
    if (vals.length === 0) return null;
    return vals.reduce((a, b) => a + b, 0) / vals.length;
  }, [sectors]);

  const meanY =
    meanPe !== null && meanPe > 0
      ? padding.top + innerH - Math.sqrt(meanPe / maxPe) * innerH
      : null;

  const baselineY = padding.top + innerH;
  /** 轴下方标签旋转中心：略低于轴线，斜角略浅更易读 */
  const labelPivotY = baselineY + 18;

  return (
    <div className="sector-pe-chart-wrap">
      {sectors.length > 10 ? (
        <p className="sector-pe-chart-density-hint">
          共 {sectors.length} 个板块 · 横向滑动查看，避免一屏过挤
        </p>
      ) : null}
      <div className="sector-pe-chart-scroll">
        <svg
          className="sector-pe-chart-svg"
          width={W}
          height={H}
          viewBox={`0 0 ${W} ${H}`}
          role="img"
          aria-label="各板块平均市盈率柱状图，按平均市盈率从低到高排列"
        >
          <text
            x={padding.left - 6}
            y={padding.top + 10}
            textAnchor="end"
            className="sector-pe-chart-axis"
          >
            {maxPe.toFixed(1)}
          </text>
          <text
            x={padding.left - 6}
            y={padding.top + 22}
            textAnchor="end"
            className="sector-pe-chart-axis-note"
          >
            √刻度
          </text>
          <text
            x={padding.left - 6}
            y={baselineY}
            textAnchor="end"
            className="sector-pe-chart-axis"
          >
            0
          </text>
          <line
            x1={padding.left}
            y1={baselineY}
            x2={padding.left + innerW}
            y2={baselineY}
            className="sector-pe-chart-baseline"
          />
          {meanY !== null ? (
            <g>
              <line
                x1={padding.left}
                y1={meanY}
                x2={padding.left + innerW}
                y2={meanY}
                className="sector-pe-chart-refline"
              />
              <text
                x={padding.left + innerW - 2}
                y={meanY - 4}
                textAnchor="end"
                className="sector-pe-chart-reflabel"
              >
                板块均值（算术平均）{meanPe !== null ? meanPe.toFixed(1) : ""}
              </text>
            </g>
          ) : null}
          {sectors.map((s, i) => {
            const cx = padding.left + i * slotW + slotW / 2;
            const xBar = cx - barW / 2;
            const valid = Number.isFinite(s.avgPe) && s.avgPe > 0;
            const h = valid ? peToBarHeight(s.avgPe, maxPe, innerH, minBarPx) : 4;
            const y = baselineY - h;
            const rankClass =
              i === 0 ? "sector-pe-chart-bar--r1" : i === 1 ? "sector-pe-chart-bar--r2" : i === 2 ? "sector-pe-chart-bar--r3" : "";
            const activeClass = s.id === activeId ? " sector-pe-chart-bar--active" : "";

            return (
              <g key={s.id}>
                <Link to={`/sector/${encodeURIComponent(s.id)}`}>
                  <title>
                    {s.name} · 平均 PE {valid ? s.avgPe : "—"}
                  </title>
                  <rect
                    x={xBar}
                    y={y}
                    width={barW}
                    height={Math.max(h, 2)}
                    rx={3}
                    className={`sector-pe-chart-bar ${rankClass}${!valid ? " sector-pe-chart-bar--na" : ""}${activeClass}`}
                  />
                </Link>
                <text
                  x={cx}
                  y={labelPivotY}
                  textAnchor="middle"
                  dominantBaseline="middle"
                  transform={`rotate(-38 ${cx} ${labelPivotY})`}
                  className="sector-pe-chart-label"
                >
                  {sectorChartLabelText(s.name, slotW)}
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    </div>
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

  /** 板块列表按平均 PE 由低到高（无效 PE 的板块排在末尾） */
  const sortedSectorsByPeAsc = useMemo(() => {
    if (!sectors) return [];
    return [...sectors].sort((a, b) => {
      const d = sectorBenchPeScore(a) - sectorBenchPeScore(b);
      if (d !== 0) return d;
      return a.id.localeCompare(b.id);
    });
  }, [sectors]);

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
    return <Spinner text="加载板块数据…" />;
  }

  const sector = bench;

  return (
    <>
      <h1 className="page-title">板块</h1>
      <p className="page-sub">各行业平均 PE 对比；详情页展示行业内个股相对偏离（数据来自 API）。</p>

      {sortedSectorsByPeAsc.length > 0 ? (
        <div className="sector-pe-sort-block sector-board-pe-overview">
          <p className="section-label sector-pe-sort-heading">
            <IconPeAscending className="sector-pe-sort-icon" />
            <span>板块平均 PE（由低到高）</span>
            <span className="sector-pe-sort-hint">柱高为 √ 比例（悬停见 PE）；左低右高</span>
          </p>
          <SectorsAvgPeBarChart sectors={sortedSectorsByPeAsc} activeId={id} />
        </div>
      ) : null}

      <div className="sector-layout">
        <nav className="sector-list" aria-label="板块切换（按平均 PE 从低到高）">
          {sortedSectorsByPeAsc.map((s) => (
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
                  <div className="value">{sector.avgRoe >= 0 ? `${sector.avgRoe}%` : "—"}</div>
                </div>
                <div className="bench-card">
                  <div className="label">营收增速（示意）</div>
                  <div className="value">{sector.revGrowth >= 0 ? `${sector.revGrowth}%` : "—"}</div>
                </div>
                <div className="bench-card">
                  <div className="label">相对全市场 PE</div>
                  <div className="value">{sector.vsMarketPe}x</div>
                </div>
              </div>

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
                        <td>{r.roe >= 0 ? `${r.roe}%` : "—"}</td>
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
