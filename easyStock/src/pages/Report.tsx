import { useCallback, useEffect, useRef, useState } from "react";
import {
  fetchWikiMerged,
  fetchWikiMeta,
  fetchWikiYear,
  uploadReportStream,
} from "@/api/reportApi";
import { MarkdownBody } from "@/components/MarkdownBody";
import { Spinner } from "@/components/Spinner";

type Tab = "upload" | "wiki";

export function Report() {
  const [stockCode, setStockCode] = useState("000858.SZ");
  const [stockName, setStockName] = useState("五粮液");
  const [uploading, setUploading] = useState<number | null>(null);
  const [error, setError] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);
  const [uploadYear, setUploadYear] = useState(2024);
  const [streamStatus, setStreamStatus] = useState("");
  const [streamMd, setStreamMd] = useState("");
  const mdScrollRef = useRef<HTMLDivElement>(null);
  const [dragOver, setDragOver] = useState(false);
  const [droppedFile, setDroppedFile] = useState<File | null>(null);
  const [tab, setTab] = useState<Tab>("upload");

  const [wikiMd, setWikiMd] = useState("");
  const [wikiLoading, setWikiLoading] = useState(false);
  const [wikiYears, setWikiYears] = useState<number[]>([]);
  const [wikiYear, setWikiYear] = useState<number | null>(null);
  const [wikiMerged, setWikiMerged] = useState(true);

  useEffect(() => {
    const el = mdScrollRef.current;
    if (!el || uploading === null) return;
    el.scrollTop = el.scrollHeight;
  }, [streamMd, uploading]);

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

  const loadWiki = useCallback(async (code: string, yr?: number) => {
    setWikiLoading(true);
    try {
      const meta = await fetchWikiMeta(code);
      setWikiYears([...meta.years].sort((a, b) => a - b));
      if (yr) {
        setWikiMerged(false);
        setWikiYear(yr);
        const text = await fetchWikiYear(code, yr);
        setWikiMd(text);
      } else {
        setWikiMerged(true);
        setWikiYear(null);
        const text = await fetchWikiMerged(code);
        setWikiMd(text);
      }
    } catch {
      setWikiMd("");
      setWikiYears([]);
    } finally {
      setWikiLoading(false);
    }
  }, []);

  useEffect(() => {
    if (tab === "wiki" && stockCode) {
      void loadWiki(stockCode);
    }
  }, [tab, stockCode, loadWiki]);

  const yearOptions = Array.from({ length: 12 }, (_, i) => 2015 + i);
  const hasStreamContent = streamMd.trim().length > 0;

  const tabs: { key: Tab; label: string }[] = [
    { key: "upload", label: "年报上传" },
    { key: "wiki", label: "知识库" },
  ];

  return (
    <div className="report-wrap">
      <div className="home-hero">
        <p className="home-eyebrow">ANNUAL REPORT</p>
        <h1 className="page-title home-title">年报研究</h1>
        <p className="page-sub home-desc">
          上传年度报告 PDF，AI 实时流式输出核心摘要；摘要会写入知识库便于查阅。
        </p>
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

      {tab === "upload" && (
        <>
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

            <div className="report-upload-row">
              <label>年份</label>
              <select value={uploadYear} onChange={(e) => setUploadYear(+e.target.value)}>
                {yearOptions.map((y) => (
                  <option key={y} value={y}>
                    {y}
                  </option>
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
              {droppedFile && (
                <p className="drop-zone-file">{droppedFile.name}</p>
              )}
              <p className="drop-zone-hint">支持 .pdf 格式，文件名含年份可自动识别</p>
            </div>
          </div>

          {error && <div className="report-error">{error}</div>}

          <div className="panel report-stream-panel">
            <div className="report-stream-head">
              <h3>分析结果</h3>
              <button
                type="button"
                className="report-btn"
                onClick={clearResult}
                disabled={!hasStreamContent && !streamStatus}
              >
                清空
              </button>
            </div>
            <p className="report-stream-status">
              {uploading !== null
                ? streamStatus || "正在处理…"
                : streamStatus ||
                  "上传 PDF 后点击「上传并分析」，模型以 Markdown 流式输出；下方使用 Markdown 渲染（标题、列表、加粗等）。"}
            </p>
            <div className="report-stream-md" ref={mdScrollRef}>
              {hasStreamContent ? (
                <MarkdownBody markdown={streamMd} />
              ) : (
                <p className="wiki-muted">摘要将在此流式显示并实时解析为 Markdown…</p>
              )}
            </div>
          </div>
        </>
      )}

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
              暂无知识库内容，请先上传年报并等待 Wiki 生成。
            </p>
          )}
        </div>
      )}
    </div>
  );
}
