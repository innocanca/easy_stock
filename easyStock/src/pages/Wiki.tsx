import { useCallback, useEffect, useState } from "react";
import { MarkdownBody } from "@/components/MarkdownBody";
import {
  fetchWikiMeta,
  fetchWikiMerged,
  fetchWikiYear,
  fetchWikiStockCodes,
} from "@/api/reportApi";

export function Wiki() {
  const [stocks, setStocks] = useState<string[]>([]);
  const [selected, setSelected] = useState("");
  const [stockName, setStockName] = useState("");
  const [years, setYears] = useState<number[]>([]);
  const [year, setYear] = useState<number | null>(null);
  const [merged, setMerged] = useState(true);
  const [md, setMd] = useState("");
  const [loading, setLoading] = useState(false);
  const [listLoading, setListLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setListLoading(true);
      setError("");
      try {
        const list = await fetchWikiStockCodes();
        if (cancelled) return;
        setStocks(list);
        setSelected((prev) =>
          prev && list.includes(prev) ? prev : list[0] ?? ""
        );
      } catch (e: unknown) {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : "加载失败");
        }
      } finally {
        if (!cancelled) setListLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const refreshList = async () => {
    setListLoading(true);
    setError("");
    try {
      const list = await fetchWikiStockCodes();
      setStocks(list);
      setSelected((prev) =>
        prev && list.includes(prev) ? prev : list[0] ?? ""
      );
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "加载失败");
    } finally {
      setListLoading(false);
    }
  };

  const loadMerged = useCallback(async (code: string) => {
    setLoading(true);
    setError("");
    setMerged(true);
    setYear(null);
    try {
      const meta = await fetchWikiMeta(code);
      setStockName(meta.stock_name);
      setYears([...meta.years].sort((a, b) => a - b));
      const text = await fetchWikiMerged(code);
      setMd(text);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "加载 Wiki 失败");
      setMd("");
      setYears([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (selected) {
      void loadMerged(selected);
    } else {
      setMd("");
      setYears([]);
      setStockName("");
    }
  }, [selected, loadMerged]);

  const loadYear = async (y: number) => {
    if (!selected) return;
    setLoading(true);
    setMerged(false);
    setYear(y);
    setError("");
    try {
      const text = await fetchWikiYear(selected, y);
      setMd(text);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="wiki-wrap">
      <div className="home-hero">
        <p className="home-eyebrow">KNOWLEDGE BASE</p>
        <h1 className="page-title home-title">知识库 Wiki</h1>
        <p className="page-sub home-desc">
          由年报上传流式生成的 Markdown 知识页，按股票与年份查阅；支持合并视图与分年视图。
        </p>
      </div>

      <div className="wiki-layout">
        <aside className="wiki-sidebar panel">
          <h3 className="wiki-sidebar-title">股票</h3>
          {listLoading && <p className="wiki-muted">加载中…</p>}
          {!listLoading && stocks.length === 0 && (
            <p className="wiki-muted">暂无 Wiki，请先在「年报分析」上传年报。</p>
          )}
          <ul className="wiki-stock-list">
            {stocks.map((code) => (
              <li key={code}>
                <button
                  type="button"
                  className={
                    code === selected ? "wiki-stock-btn active" : "wiki-stock-btn"
                  }
                  onClick={() => setSelected(code)}
                >
                  {code}
                </button>
              </li>
            ))}
          </ul>
          <button type="button" className="report-btn" onClick={() => void refreshList()}>
            刷新列表
          </button>
        </aside>

        <div className="wiki-main">
          {selected && (
            <div className="wiki-toolbar panel">
              <div className="wiki-toolbar-head">
                <h2 className="wiki-stock-title">
                  {stockName || selected}{" "}
                  <span className="wiki-code">{selected}</span>
                </h2>
                <div className="wiki-year-actions">
                  <button
                    type="button"
                    className={
                      merged ? "report-btn report-btn--accent" : "report-btn"
                    }
                    onClick={() => void loadMerged(selected)}
                  >
                    合并视图
                  </button>
                  {years.map((y) => (
                    <button
                      key={y}
                      type="button"
                      className={
                        !merged && year === y
                          ? "report-btn report-btn--accent"
                          : "report-btn"
                      }
                      onClick={() => void loadYear(y)}
                    >
                      {y} 年
                    </button>
                  ))}
                </div>
              </div>
            </div>
          )}

          {error && <div className="report-error">{error}</div>}

          {selected && (
            <div className="panel wiki-md-panel">
              {loading && <p className="wiki-muted">加载中…</p>}
              {!loading && md && <MarkdownBody markdown={md} />}
              {!loading && !md && !error && (
                <p className="wiki-muted">该股票暂无 Wiki 内容。</p>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
