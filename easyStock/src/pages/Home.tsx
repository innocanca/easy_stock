import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { DimensionBars } from "@/components/DimensionBars";
import { TrendSpark } from "@/components/TrendSpark";
import type { PickItem } from "@shared/dataset";
import { fetchPicks } from "@/api/stockApi";

export function Home() {
  const [items, setItems] = useState<PickItem[] | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    let cancel = false;
    fetchPicks()
      .then((d) => {
        if (!cancel) setItems(d);
      })
      .catch((e: unknown) => {
        if (!cancel) setErr(e instanceof Error ? e.message : "加载失败");
      });
    return () => {
      cancel = true;
    };
  }, []);

  if (err) {
    return (
      <div className="home-wrap">
        <header className="home-hero">
          <p className="home-eyebrow">今日精选</p>
          <h1 className="page-title home-title">推荐组合</h1>
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

  if (items === null) {
    return (
      <div className="home-wrap">
        <header className="home-hero">
          <p className="home-eyebrow">今日精选</p>
          <h1 className="page-title home-title">推荐组合</h1>
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

  return (
    <div className="home-wrap">
      <header className="home-hero">
        <p className="home-eyebrow">今日精选</p>
        <h1 className="page-title home-title">推荐组合</h1>
        <p className="page-sub home-desc">
          按<strong>四维</strong>（价值 · 估值 · 确定性 · 成长）与盈利、PE 变化综合打分；数据由 Tushare 经 API 实时聚合。
        </p>
      </header>

      <div className="pick-grid">
        {items.map((item, index) => {
          const rank = index + 1;
          return (
            <article
              key={item.code}
              className={`pick-card${rank <= 3 ? ` pick-card--top pick-card--top-${rank}` : ""}`}
            >
              <div className="pick-card-glow" aria-hidden />
              <div className="pick-card-inner">
                <header className="pick-card-head">
                  <span className="pick-rank" title={`排名第 ${rank} 位`}>
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
    </div>
  );
}
