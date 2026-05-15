import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { DimensionBars } from "@/components/DimensionBars";
import { TrendSpark } from "@/components/TrendSpark";
import { MarkdownBody } from "@/components/MarkdownBody";
import { getApiBase } from "@/api/client";
import { fetchPicks, fetchPickStyles, type PicksPageResponse, type PickStyleInfo } from "@/api/stockApi";

/** 与后端默认一致：500 亿人民币（万元） */
const MIN_MV_WAN = 5_000_000;

const PAGE_SIZE_OPTIONS = [8, 12, 16, 24] as const;

const SCORE_PRESETS = [
  { label: "不限", min: 1 },
  { label: "≥ 60", min: 60 },
  { label: "≥ 65", min: 65 },
  { label: "≥ 70", min: 70 },
  { label: "≥ 75", min: 75 },
  { label: "≥ 80", min: 80 },
  { label: "≥ 85", min: 85 },
  { label: "≥ 90", min: 90 },
] as const;

export function Home() {
  const [data, setData] = useState<PicksPageResponse | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(12);
  const [scoreMin, setScoreMin] = useState(1);
  const [style, setStyle] = useState("balanced");
  const [styles, setStyles] = useState<PickStyleInfo[]>([]);
  const [aiMd, setAiMd] = useState("");
  const [aiLoading, setAiLoading] = useState(false);
  const [aiStatus, setAiStatus] = useState("");
  const [aiError, setAiError] = useState("");
  const aiAbortRef = useRef<AbortController | null>(null);

  const startAiRecommend = useCallback(() => {
    aiAbortRef.current?.abort();
    setAiMd("");
    setAiError("");
    setAiLoading(true);
    setAiStatus("正在获取候选股票池…");

    const ctrl = new AbortController();
    aiAbortRef.current = ctrl;
    const base = getApiBase();
    const url = `${base}/api/picks/ai-recommend?style=${encodeURIComponent(style)}`;

    fetch(url, { signal: ctrl.signal })
      .then(async (res) => {
        if (!res.ok || !res.body) throw new Error(`HTTP ${res.status}`);
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";
        let md = "";
        let currentEvent = "";

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split("\n");
          buffer = lines.pop() ?? "";
          for (const line of lines) {
            if (line.startsWith("event: ")) {
              currentEvent = line.slice(7).trim();
            } else if (line.startsWith("data: ") && currentEvent) {
              try {
                const payload = JSON.parse(line.slice(6)) as string;
                if (currentEvent === "chunk") {
                  md += payload;
                  setAiMd(md);
                } else if (currentEvent === "status") {
                  setAiStatus(payload);
                } else if (currentEvent === "error") {
                  setAiError(payload);
                }
              } catch { /* skip malformed */ }
              currentEvent = "";
            }
          }
        }
        setAiLoading(false);
        setAiStatus("");
      })
      .catch((e: unknown) => {
        if ((e as Error).name !== "AbortError") {
          setAiError(e instanceof Error ? e.message : "请求失败");
        }
        setAiLoading(false);
      });
  }, [style]);

  useEffect(() => {
    fetchPickStyles()
      .then((s) => setStyles(Array.isArray(s) ? s : []))
      .catch(() => {});
  }, []);

  useEffect(() => {
    let cancel = false;
    setErr(null);
    fetchPicks({
      page,
      pageSize,
      minMvWan: MIN_MV_WAN,
      scoreMin,
      scoreMax: 99,
      style,
    })
      .then((d) => {
        if (cancel) return;
        setData(d);
        const ps = d.pageSize > 0 ? d.pageSize : pageSize;
        const maxPage =
          Number.isFinite(ps) && ps >= 1 ? Math.max(1, Math.ceil(d.total / ps)) : 1;
        if (Number.isFinite(maxPage) && d.page > maxPage) {
          setPage(maxPage);
        }
      })
      .catch((e: unknown) => {
        if (!cancel) setErr(e instanceof Error ? e.message : "加载失败");
      });
    return () => {
      cancel = true;
    };
  }, [page, pageSize, scoreMin, style]);

  const totalPages = useMemo(() => {
    if (!data) return 1;
    const ps = data.pageSize > 0 ? data.pageSize : pageSize;
    if (!Number.isFinite(ps) || ps < 1) return 1;
    const n = Math.ceil(data.total / ps);
    return Number.isFinite(n) && n >= 1 ? n : 1;
  }, [data, pageSize]);

  const pageOptions = useMemo(() => {
    return Array.from({ length: totalPages }, (_, i) => i + 1);
  }, [totalPages]);

  function onStyleChange(id: string) {
    setStyle(id);
    setPage(1);
    setData(null);
  }

  function onScorePreset(min: number) {
    setScoreMin(min);
    setPage(1);
  }

  function onPageSizeChange(n: number) {
    setPageSize(n);
    setPage(1);
  }

  function onPageSelect(p: number) {
    setPage(p);
  }

  if (err) {
    return (
      <div className="home-wrap">
        <header className="home-hero">
          <p className="home-eyebrow">今日精选</p>
          <h1 className="page-title home-title">推荐</h1>
        </header>
        <div className="home-error-panel" role="alert">
          <p className="home-error-title">暂时无法加载列表</p>
          <p className="home-error-detail">{err}</p>
          <p className="home-error-hint">
            请确认已启动 easystock-api（默认端口 4000），并已配置{" "}
            <code className="home-inline-code">TUSHARE_TOKEN</code>；本地开发需与 Vite 代理一致，或设置{" "}
            <code className="home-inline-code">VITE_API_URL</code>。
          </p>
          <button type="button" className="home-retry-btn" onClick={() => window.location.reload()}>
            刷新重试
          </button>
        </div>
      </div>
    );
  }

  if (data === null) {
    return (
      <div className="home-wrap">
        <header className="home-hero">
          <p className="home-eyebrow">今日精选</p>
          <h1 className="page-title home-title">推荐</h1>
          <p className="page-sub home-desc">正在拉取候选池与评分…</p>
        </header>
        <div className="pick-grid">
          {[0, 1, 2, 3, 4, 5].map((k) => (
            <div key={k} className="pick-card pick-card--skeleton" aria-hidden />
          ))}
        </div>
      </div>
    );
  }

  const items = Array.isArray(data.items) ? data.items : [];

  return (
    <div className="home-wrap">
      <header className="home-hero">
        <p className="home-eyebrow">今日精选</p>
        <h1 className="page-title home-title">推荐</h1>
        <p className="page-sub home-desc home-desc--muted">
          市值 ≥ 500 亿人民币 · 按综合评分从高到低排序 · 可筛选分数并分页浏览
        </p>
      </header>

      {styles.length > 0 && (
        <div className="style-tabs">
          {styles.map((s) => (
            <button
              key={s.id}
              type="button"
              className={`style-tab${style === s.id ? " style-tab--active" : ""}`}
              onClick={() => onStyleChange(s.id)}
              title={s.desc}
            >
              {s.label}
            </button>
          ))}
          <span className="style-desc">
            {styles.find((s) => s.id === style)?.desc ?? ""}
          </span>
          <button
            type="button"
            className="ai-recommend-btn"
            onClick={startAiRecommend}
            disabled={aiLoading}
          >
            {aiLoading ? "AI 分析中…" : "AI 智选"}
          </button>
        </div>
      )}

      {(aiMd || aiLoading || aiError) && (
        <div className="ai-recommend-panel">
          <div className="ai-recommend-header">
            <h3 className="ai-recommend-title">AI 智能选股</h3>
            {!aiLoading && aiMd && (
              <button type="button" className="ai-recommend-close" onClick={() => { setAiMd(""); setAiError(""); }}>
                收起
              </button>
            )}
          </div>
          {aiStatus && <p className="ai-recommend-status">{aiStatus}</p>}
          {aiError && <p className="ai-recommend-error">{aiError}</p>}
          {aiMd && <MarkdownBody markdown={aiMd} className="ai-recommend-body" />}
        </div>
      )}

      <div className="home-picks-toolbar">
        <div className="home-picks-toolbar-row">
          <label className="home-picks-field">
            <span className="home-picks-label">综合分</span>
            <select
              className="home-picks-select"
              value={scoreMin}
              onChange={(e) => onScorePreset(Number(e.target.value))}
              aria-label="筛选综合评分下限"
            >
              {SCORE_PRESETS.map((p) => (
                <option key={p.min} value={p.min}>
                  {p.label}
                </option>
              ))}
            </select>
          </label>
          <label className="home-picks-field">
            <span className="home-picks-label">每页</span>
            <select
              className="home-picks-select"
              value={pageSize}
              onChange={(e) => onPageSizeChange(Number(e.target.value))}
              aria-label="每页条数"
            >
              {PAGE_SIZE_OPTIONS.map((n) => (
                <option key={n} value={n}>
                  {n} 条
                </option>
              ))}
            </select>
          </label>
          <label className="home-picks-field">
            <span className="home-picks-label">页码</span>
            <select
              className="home-picks-select"
              value={Math.min(page, totalPages)}
              onChange={(e) => onPageSelect(Number(e.target.value))}
              aria-label="选择页码"
              disabled={totalPages <= 1}
            >
              {pageOptions.map((p) => (
                <option key={p} value={p}>
                  第 {p} / {totalPages} 页
                </option>
              ))}
            </select>
          </label>
        </div>
        <p className="home-picks-meta">
          共 <strong>{data.total}</strong> 只股票符合条件
          {items.length > 0 ? (
            <>
              ，本页展示第 {(page - 1) * pageSize + 1}–{(page - 1) * pageSize + items.length} 名
            </>
          ) : null}
        </p>
      </div>

      {items.length === 0 ? (
        <p className="home-picks-empty">当前筛选条件下暂无股票，请放宽综合分筛选。</p>
      ) : (
        <div className="pick-grid">
          {items.map((item, index) => {
            const rank = (page - 1) * pageSize + index + 1;
            return (
              <article
                key={item.code}
                className={`pick-card${rank <= 3 ? ` pick-card--top pick-card--top-${rank}` : ""}`}
              >
                <div className="pick-card-glow" aria-hidden />
                <div className="pick-card-inner">
                  <header className="pick-card-head">
                    <span className="pick-rank" title={`排名第 ${rank} 位（全表排序）`}>
                      {rank}
                    </span>
                    <div className="pick-card-title-block">
                      <h2 className="pick-stock-name">
                        <Link to={`/stock/${encodeURIComponent(item.code)}`}>{item.name}</Link>
                      </h2>
                      <span className="pick-code-pill">{item.code}</span>
                    </div>
                    <div className="pick-score-wrap" title="综合分 · 满分 100">
                      <span className="pick-score-ring">
                        <span className="pick-score-value">{item.score}</span>
                        <span className="pick-score-unit">分</span>
                      </span>
                    </div>
                  </header>

                  <div className="pick-stat-chips">
                    <div className="pick-chip">
                      <span className="pick-chip-label">PE</span>
                      <span className="pick-chip-val">{item.pe}</span>
                    </div>
                    <div className="pick-chip">
                      <span className="pick-chip-label">ROE</span>
                      <span className="pick-chip-val">{item.roe}%</span>
                    </div>
                    <div className="pick-chip">
                      <span className="pick-chip-label">利润同比</span>
                      <span
                        className={`pick-chip-val${item.profitGrowthYoy >= 0 ? " pick-chip-val--up" : " pick-chip-val--down"}`}
                      >
                        {item.profitGrowthYoy >= 0 ? "+" : ""}
                        {item.profitGrowthYoy}%
                      </span>
                    </div>
                  </div>

                  <div className="pick-dim-section">
                    <p className="pick-section-label">四维雷达</p>
                    <DimensionBars dimensions={item.dimensions} />
                  </div>

                  <p className="pick-note-preview">{item.scoreNote}</p>

                  <details className="pick-expand pick-expand--styled">
                    <summary>
                      <span className="pick-expand-label">展开 · 盈利与 PE 走势</span>
                      <span className="pick-expand-icon" aria-hidden />
                    </summary>
                    <div className="pick-expand-body">
                      <TrendSpark item={item} />
                      <Link className="pick-detail-cta" to={`/stock/${encodeURIComponent(item.code)}`}>
                        查看个股详情
                        <span aria-hidden className="pick-detail-cta-arrow">
                          →
                        </span>
                      </Link>
                    </div>
                  </details>
                </div>
              </article>
            );
          })}
        </div>
      )}
    </div>
  );
}
