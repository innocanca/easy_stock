import { useCallback, useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import {
  fetchWikiMerged,
  fetchWikiMeta,
  fetchWikiYear,
  uploadReportStream,
} from "@/api/reportApi";
import { getApiBase } from "@/api/client";
import { MarkdownBody } from "@/components/MarkdownBody";
import { Spinner } from "@/components/Spinner";

type Tab = "cninfo" | "upload" | "wiki";

interface CninfoReport {
  year: number;
  title: string;
  date: string;
  download_url: string;
  size_mb: string;
}

function parseSseBlock(block: string): { event: string; data: string } | null {
  const lines = block.split(/\r?\n/).filter((l) => l.length > 0);
  let ev = "";
  const dataLines: string[] = [];
  for (const line of lines) {
    if (line.startsWith("event:")) ev = line.slice(6).trim();
    else if (line.startsWith("data:")) dataLines.push(line.slice(5).trimStart());
  }
  if (dataLines.length === 0) return null;
  return { event: ev, data: dataLines.join("\n") };
}

export function Report() {
  const [searchParams] = useSearchParams();
  const [stockCode, setStockCode] = useState(searchParams.get("code") || "000858.SZ");
  const [stockName, setStockName] = useState(searchParams.get("name") || "五粮液");
  const [uploading, setUploading] = useState<number | null>(null);
  const [error, setError] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);
  const [uploadYear, setUploadYear] = useState(2024);
  const [streamStatus, setStreamStatus] = useState("");
  const [streamMd, setStreamMd] = useState("");
  const mdScrollRef = useRef<HTMLDivElement>(null);
  const [dragOver, setDragOver] = useState(false);
  const [droppedFile, setDroppedFile] = useState<File | null>(null);
  const [tab, setTab] = useState<Tab>("cninfo");

  // cninfo state
  const [cninfoReports, setCninfoReports] = useState<CninfoReport[]>([]);
  const [cninfoLoading, setCninfoLoading] = useState(false);
  const [cninfoAnalyzing, setCninfoAnalyzing] = useState<number | null>(null);

  // wiki state
  const [wikiMd, setWikiMd] = useState("");
  const [wikiLoading, setWikiLoading] = useState(false);
  const [wikiYears, setWikiYears] = useState<number[]>([]);
  const [wikiYear, setWikiYear] = useState<number | null>(null);
  const [wikiMerged, setWikiMerged] = useState(true);

  useEffect(() => {
    const el = mdScrollRef.current;
    if (!el || (uploading === null && cninfoAnalyzing === null)) return;
    el.scrollTop = el.scrollHeight;
  }, [streamMd, uploading, cninfoAnalyzing]);

  // Auto-search cninfo when stock code changes on the cninfo tab
  const searchCninfo = useCallback(async (code: string, name: string) => {
    if (!code) return;
    setCninfoLoading(true);
    setError("");
    try {
      const base = getApiBase();
      const qs = new URLSearchParams({ code, name });
      const res = await fetch(`${base}/api/cninfo/reports?${qs}`);
      if (!res.ok) {
        const t = await res.text().catch(() => "");
        throw new Error(t || `HTTP ${res.status}`);
      }
      const list = (await res.json()) as CninfoReport[];
      setCninfoReports(list ?? []);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "查询失败");
      setCninfoReports([]);
    } finally {
      setCninfoLoading(false);
    }
  }, []);

  useEffect(() => {
    if (tab === "cninfo" && stockCode) {
      void searchCninfo(stockCode, stockName);
    }
  }, [tab, stockCode, stockName, searchCninfo]);

  const handleCninfoAnalyze = async (report: CninfoReport) => {
    setCninfoAnalyzing(report.year);
    setError("");
    setStreamStatus("");
    setStreamMd("");
    try {
      const base = getApiBase();
      const res = await fetch(`${base}/api/cninfo/analyze`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          stock_code: stockCode,
          stock_name: stockName,
          year: report.year,
          download_url: report.download_url,
        }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);

      const reader = res.body?.getReader();
      if (!reader) throw new Error("No response body");
      const dec = new TextDecoder();
      let buf = "";

      const handleBlock = (block: string) => {
        const parsed = parseSseBlock(block);
        if (!parsed) return;
        const { event, data } = parsed;
        try {
          if (event === "status") {
            setStreamStatus(JSON.parse(data) as string);
          } else if (event === "wiki_chunk") {
            const chunk = JSON.parse(data) as string;
            setStreamMd((prev) => prev + chunk);
          } else if (event === "error") {
            setError(JSON.parse(data) as string);
          } else if (event === "done") {
            setStreamStatus((s) => (s ? `${s} · 已完成` : "已完成"));
          }
        } catch { /* skip */ }
      };

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += dec.decode(value, { stream: true });
        const parts = buf.split("\n\n");
        buf = parts.pop() ?? "";
        for (const part of parts) handleBlock(part);
      }
      if (buf.trim()) handleBlock(buf);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "分析失败");
    } finally {
      setCninfoAnalyzing(null);
    }
  };

  // Upload handlers
  const getUploadFile = (): File | null => {
    if (droppedFile) return droppedFile;
    const files = fileRef.current?.files;
    return files && files.length > 0 ? files[0] : null;
  };

  const handleUpload = async () => {
    const file = getUploadFile();
    if (!file) return;
    setUploading(uploadYear);
    setError("");
    setStreamStatus("");
    setStreamMd("");
    try {
      await uploadReportStream(stockCode, stockName, uploadYear, file, {
        onStatus: (m) => setStreamStatus(m),
        onWikiChunk: (c) => setStreamMd((w) => w + c),
        onDone: () => setStreamStatus((s) => (s ? `${s} · 已完成` : "已完成")),
        onError: (m) => setError(m),
      });
      setDroppedFile(null);
      if (fileRef.current) fileRef.current.value = "";
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "上传失败");
    } finally {
      setUploading(null);
    }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    const file = e.dataTransfer.files[0];
    if (file && file.name.endsWith(".pdf")) {
      setDroppedFile(file);
      const yearMatch = file.name.match(/(20[12]\d)/);
      if (yearMatch) setUploadYear(parseInt(yearMatch[1], 10));
    }
  };

  const clearResult = () => {
    setStreamMd("");
    setStreamStatus("");
    setError("");
  };

  // Wiki handlers
  const loadWiki = useCallback(async (code: string, yr?: number) => {
    setWikiLoading(true);
    try {
      const meta = await fetchWikiMeta(code);
      setWikiYears([...meta.years].sort((a, b) => a - b));
      if (yr) {
        setWikiMerged(false);
        setWikiYear(yr);
        setWikiMd(await fetchWikiYear(code, yr));
      } else {
        setWikiMerged(true);
        setWikiYear(null);
        setWikiMd(await fetchWikiMerged(code));
      }
    } catch {
      setWikiMd("");
      setWikiYears([]);
    } finally {
      setWikiLoading(false);
    }
  }, []);

  useEffect(() => {
    if (tab === "wiki" && stockCode) void loadWiki(stockCode);
  }, [tab, stockCode, loadWiki]);

  const hasStreamContent = streamMd.trim().length > 0;
  const isAnalyzing = uploading !== null || cninfoAnalyzing !== null;

  const tabs: { key: Tab; label: string }[] = [
    { key: "cninfo", label: "巨潮年报" },
    { key: "upload", label: "本地上传" },
    { key: "wiki", label: "知识库" },
  ];

  return (
    <div className="report-wrap">
      <div className="home-hero">
        <p className="home-eyebrow">ANNUAL REPORT</p>
        <h1 className="page-title home-title">年报研究</h1>
        <p className="page-sub home-desc">
          从巨潮资讯网一键获取年报并 AI 分析，或本地上传 PDF；摘要写入知识库便于查阅。
        </p>
      </div>

      <div className="report-controls">
        <div className="report-stock-input">
          <label>股票代码</label>
          <input
            type="text"
            value={stockCode}
            onChange={(e) => setStockCode(e.target.value.trim())}
            placeholder="000858.SZ"
          />
          <label>股票名称</label>
          <input
            type="text"
            value={stockName}
            onChange={(e) => setStockName(e.target.value.trim())}
            placeholder="五粮液"
          />
        </div>
      </div>

      <div className="tabs">
        {tabs.map((t) => (
          <button
            key={t.key}
            type="button"
            className={tab === t.key ? "active" : ""}
            onClick={() => setTab(t.key)}
          >
            {t.label}
          </button>
        ))}
      </div>

      {error && <div className="report-error">{error}</div>}

      {/* ===== 巨潮年报 Tab ===== */}
      {tab === "cninfo" && (
        <>
          <div className="panel" style={{ marginBottom: "1rem" }}>
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: "0.75rem" }}>
              <h3 style={{ margin: 0, fontSize: "0.95rem" }}>
                {stockName} 可用年报（来自巨潮资讯网）
              </h3>
              <button
                className="report-btn"
                onClick={() => searchCninfo(stockCode, stockName)}
                disabled={cninfoLoading}
              >
                {cninfoLoading ? "查询中…" : "刷新"}
              </button>
            </div>
            {cninfoLoading && <Spinner text="正在查询巨潮资讯网…" />}
            {!cninfoLoading && cninfoReports.length === 0 && (
              <p style={{ color: "var(--muted)", fontSize: "0.88rem" }}>
                未找到年报，请确认股票代码正确（如 000858.SZ）
              </p>
            )}
            {!cninfoLoading && cninfoReports.length > 0 && (
              <table>
                <thead>
                  <tr>
                    <th>年份</th>
                    <th>报告标题</th>
                    <th>发布日期</th>
                    <th>大小</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {cninfoReports.map((r) => (
                    <tr key={r.year}>
                      <td><strong>{r.year}</strong></td>
                      <td style={{ fontSize: "0.82rem" }}>{r.title}</td>
                      <td>{r.date}</td>
                      <td>{r.size_mb} MB</td>
                      <td>
                        <button
                          className="report-btn report-btn--primary"
                          style={{ fontSize: "0.78rem", padding: "0.3rem 0.6rem" }}
                          onClick={() => handleCninfoAnalyze(r)}
                          disabled={isAnalyzing}
                        >
                          {cninfoAnalyzing === r.year ? "分析中…" : "AI 分析"}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          {(hasStreamContent || streamStatus) && (
            <div className="panel report-stream-panel">
              <div className="report-stream-head">
                <h3>分析结果{cninfoAnalyzing ? ` (${cninfoAnalyzing}年)` : ""}</h3>
                <button type="button" className="report-btn" onClick={clearResult} disabled={!hasStreamContent && !streamStatus}>
                  清空
                </button>
              </div>
              <p className="report-stream-status">{streamStatus || "处理中…"}</p>
              <div className="report-stream-md" ref={mdScrollRef}>
                {hasStreamContent ? (
                  <MarkdownBody markdown={streamMd} />
                ) : (
                  <p className="wiki-muted">分析内容将在此流式显示…</p>
                )}
              </div>
            </div>
          )}
        </>
      )}

      {/* ===== 本地上传 Tab ===== */}
      {tab === "upload" && (
        <>
          <div className="report-controls">
            <div className="report-upload-row">
              <label>年份</label>
              <select value={uploadYear} onChange={(e) => setUploadYear(+e.target.value)}>
                {Array.from({ length: 12 }, (_, i) => 2015 + i).map((y) => (
                  <option key={y} value={y}>{y}</option>
                ))}
              </select>
              <button
                className="report-btn report-btn--primary"
                onClick={handleUpload}
                disabled={uploading !== null || (!droppedFile && !(fileRef.current?.files?.length))}
              >
                {uploading !== null ? `分析中 (${uploading})…` : "上传并分析"}
              </button>
            </div>

            <div
              className={`drop-zone${dragOver ? " drag-over" : ""}`}
              onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
              onDragLeave={() => setDragOver(false)}
              onDrop={handleDrop}
              onClick={() => fileRef.current?.click()}
            >
              <input
                ref={fileRef}
                type="file"
                accept=".pdf"
                style={{ display: "none" }}
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  if (f) {
                    setDroppedFile(f);
                    const m = f.name.match(/(20[12]\d)/);
                    if (m) setUploadYear(parseInt(m[1], 10));
                  }
                }}
              />
              <div className="drop-zone-icon">📄</div>
              <p className="drop-zone-text">
                {droppedFile ? "" : "拖拽PDF年报到此处，或点击选择文件"}
              </p>
              {droppedFile && <p className="drop-zone-file">{droppedFile.name}</p>}
              <p className="drop-zone-hint">支持 .pdf 格式，文件名含年份可自动识别</p>
            </div>
          </div>

          <div className="panel report-stream-panel">
            <div className="report-stream-head">
              <h3>分析结果</h3>
              <button type="button" className="report-btn" onClick={clearResult} disabled={!hasStreamContent && !streamStatus}>
                清空
              </button>
            </div>
            <p className="report-stream-status">
              {uploading !== null
                ? streamStatus || "正在处理…"
                : streamStatus || "上传 PDF 后点击「上传并分析」，AI 以 Markdown 流式输出分析摘要。"}
            </p>
            <div className="report-stream-md" ref={mdScrollRef}>
              {hasStreamContent ? (
                <MarkdownBody markdown={streamMd} />
              ) : (
                <p className="wiki-muted">摘要将在此流式显示…</p>
              )}
            </div>
          </div>
        </>
      )}

      {/* ===== 知识库 Tab ===== */}
      {tab === "wiki" && (
        <div className="report-wiki-wrap">
          <div className="wiki-year-actions" style={{ marginBottom: "1rem" }}>
            <button
              type="button"
              className={wikiMerged ? "report-btn report-btn--accent" : "report-btn"}
              onClick={() => void loadWiki(stockCode)}
            >
              合并视图
            </button>
            {wikiYears.map((y) => (
              <button
                key={y}
                type="button"
                className={!wikiMerged && wikiYear === y ? "report-btn report-btn--accent" : "report-btn"}
                onClick={() => void loadWiki(stockCode, y)}
              >
                {y}年
              </button>
            ))}
          </div>
          {wikiLoading && <Spinner text="加载知识库…" />}
          {!wikiLoading && wikiMd && (
            <div className="panel wiki-md-panel">
              <MarkdownBody markdown={wikiMd} />
            </div>
          )}
          {!wikiLoading && !wikiMd && (
            <p style={{ color: "var(--muted)", fontSize: "0.88rem" }}>
              暂无知识库内容，请先分析年报后查看。
            </p>
          )}
        </div>
      )}
    </div>
  );
}
