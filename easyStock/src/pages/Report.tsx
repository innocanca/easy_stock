import { useEffect, useRef, useState } from "react";
import { uploadReportStream } from "@/api/reportApi";
import { MarkdownBody } from "@/components/MarkdownBody";

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

  useEffect(() => {
    const el = mdScrollRef.current;
    if (!el || uploading === null) return;
    el.scrollTop = el.scrollHeight;
  }, [streamMd, uploading]);

  const handleUpload = async () => {
    const files = fileRef.current?.files;
    if (!files || files.length === 0) return;
    setUploading(uploadYear);
    setError("");
    setStreamStatus("");
    setStreamMd("");
    try {
      await uploadReportStream(stockCode, stockName, uploadYear, files[0], {
        onStatus: (m) => setStreamStatus(m),
        onWikiChunk: (c) => setStreamMd((w) => w + c),
        onDone: () => setStreamStatus((s) => (s ? `${s} · 已完成` : "已完成")),
        onError: (m) => setError(m),
      });
      if (fileRef.current) fileRef.current.value = "";
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "上传失败");
    } finally {
      setUploading(null);
    }
  };

  const clearResult = () => {
    setStreamMd("");
    setStreamStatus("");
    setError("");
  };

  const yearOptions = Array.from({ length: 12 }, (_, i) => 2015 + i);
  const hasStreamContent = streamMd.trim().length > 0;

  return (
    <div className="report-wrap">
      <div className="home-hero">
        <p className="home-eyebrow">ANNUAL REPORT</p>
        <h1 className="page-title home-title">年报分析</h1>
        <p className="page-sub home-desc">
          上传年度报告 PDF，下方以 Markdown 实时渲染核心摘要；不保存财务结构化数据，摘要会写入知识库便于查阅。
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

        <div className="report-upload-row">
          <label>年份</label>
          <select value={uploadYear} onChange={(e) => setUploadYear(+e.target.value)}>
            {yearOptions.map((y) => (
              <option key={y} value={y}>
                {y}
              </option>
            ))}
          </select>
          <input ref={fileRef} type="file" accept=".pdf" />
          <button
            className="report-btn report-btn--primary"
            onClick={handleUpload}
            disabled={uploading !== null}
          >
            {uploading !== null ? `分析中 (${uploading})…` : "上传并分析"}
          </button>
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
    </div>
  );
}
