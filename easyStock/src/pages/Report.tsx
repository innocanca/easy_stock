import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Bar,
  CartesianGrid,
  ComposedChart,
  Legend,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  deleteReport,
  fetchAnalysis,
  fetchReportList,
  uploadReportStream,
  type AnalysisResult,
  type ReportData,
} from "@/api/reportApi";
import { MarkdownBody } from "@/components/MarkdownBody";

const COLORS = {
  revenue: "#2563eb",
  profit: "#059669",
  margin: "#7c3aed",
  roe: "#ea580c",
  cashflow: "#0891b2",
  debt: "#dc2626",
  asset: "#6366f1",
  dividend: "#d97706",
};

const fmt = (v: number, suffix = "") => {
  if (v === 0) return "-";
  return v.toFixed(2) + suffix;
};

const fmtPct = (v: number) => {
  if (v === 0) return "-";
  return (v * 100).toFixed(1) + "%";
};

type Tab = "overview" | "growth" | "profit" | "structure" | "insight";

export function Report() {
  const [stockCode, setStockCode] = useState("000858.SZ");
  const [stockName, setStockName] = useState("五粮液");
  const [reports, setReports] = useState<ReportData[]>([]);
  const [analysis, setAnalysis] = useState<AnalysisResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [uploading, setUploading] = useState<number | null>(null);
  const [error, setError] = useState("");
  const [tab, setTab] = useState<Tab>("overview");
  const [analysisLoading, setAnalysisLoading] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);
  const [uploadYear, setUploadYear] = useState(2024);
  const [streamOpen, setStreamOpen] = useState(false);
  const [streamStatus, setStreamStatus] = useState("");
  const [streamWiki, setStreamWiki] = useState("");

  const loadReports = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const res = await fetchReportList(stockCode);
      setReports(res.reports);
      if (res.reports.length > 0) {
        setStockName(res.reports[0].stock_name);
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [stockCode]);

  useEffect(() => {
    loadReports();
  }, [loadReports]);

  const handleUpload = async () => {
    const files = fileRef.current?.files;
    if (!files || files.length === 0) return;
    setUploading(uploadYear);
    setError("");
    setStreamOpen(true);
    setStreamStatus("");
    setStreamWiki("");
    try {
      await uploadReportStream(stockCode, stockName, uploadYear, files[0], {
        onStatus: (m) => setStreamStatus(m),
        onWikiChunk: (c) => setStreamWiki((w) => w + c),
        onData: (data) => {
          setReports((prev) => {
            const filtered = prev.filter((r) => r.year !== data.year);
            return [...filtered, data].sort((a, b) => a.year - b.year);
          });
        },
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

  const handleDelete = async (year: number) => {
    if (!confirm(`确认删除 ${year} 年报数据？`)) return;
    await deleteReport(stockCode, year);
    setReports((prev) => prev.filter((r) => r.year !== year));
    setAnalysis(null);
  };

  const handleAnalyze = async (refresh = false) => {
    if (reports.length < 2) {
      setError("至少需要2年数据才能生成综合分析");
      return;
    }
    setAnalysisLoading(true);
    setError("");
    try {
      const res = await fetchAnalysis(stockCode, refresh);
      setAnalysis(res);
      setTab("insight");
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "分析失败");
    } finally {
      setAnalysisLoading(false);
    }
  };

  const years = useMemo(() => reports.map((r) => r.year), [reports]);
  const existingYears = new Set(years);

  const chartData = useMemo(
    () =>
      reports.map((r) => ({
        year: String(r.year),
        revenue: +r.revenue.toFixed(2),
        net_profit: +r.net_profit.toFixed(2),
        net_profit_parent: +r.net_profit_parent.toFixed(2),
        operating_cashflow: +r.operating_cashflow.toFixed(2),
        gross_margin: +(r.gross_margin * 100).toFixed(1),
        net_margin: +(r.net_margin * 100).toFixed(1),
        roe: +(r.roe * 100).toFixed(1),
        debt_ratio: +(r.debt_ratio * 100).toFixed(1),
        total_assets: +r.total_assets.toFixed(2),
        net_assets: +r.net_assets.toFixed(2),
        eps: +r.eps.toFixed(2),
        dividend: +r.dividend_per_share.toFixed(2),
        revenue_yoy: +(r.revenue_yoy * 100).toFixed(1),
        profit_yoy: +(r.net_profit_yoy * 100).toFixed(1),
      })),
    [reports]
  );

  const tabs: { key: Tab; label: string }[] = [
    { key: "overview", label: "数据总览" },
    { key: "growth", label: "收入与利润" },
    { key: "profit", label: "盈利能力" },
    { key: "structure", label: "财务结构" },
    { key: "insight", label: "综合分析" },
  ];

  return (
    <div className="report-wrap">
      <div className="home-hero">
        <p className="home-eyebrow">ANNUAL REPORT</p>
        <h1 className="page-title home-title">年报分析</h1>
        <p className="page-sub home-desc">
          上传年度报告PDF，AI自动提取财务数据，多维度分析企业发展变化
        </p>
      </div>

      {/* Stock selector & upload */}
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
          <button className="report-btn" onClick={loadReports} disabled={loading}>
            {loading ? "加载中…" : "刷新"}
          </button>
        </div>

        <div className="report-upload-row">
          <label>年份</label>
          <select
            value={uploadYear}
            onChange={(e) => setUploadYear(+e.target.value)}
          >
            {Array.from({ length: 12 }, (_, i) => 2015 + i).map((y) => (
              <option key={y} value={y}>
                {y}
                {existingYears.has(y) ? " ✓" : ""}
              </option>
            ))}
          </select>
          <input ref={fileRef} type="file" accept=".pdf" />
          <button
            className="report-btn report-btn--primary"
            onClick={handleUpload}
            disabled={uploading !== null}
          >
            {uploading !== null
              ? `分析中 (${uploading})…`
              : "上传并分析"}
          </button>
        </div>
      </div>

      {error && <div className="report-error">{error}</div>}

      {streamOpen && (
        <div
          className="report-stream-backdrop"
          role="presentation"
          onClick={() => setStreamOpen(false)}
        >
          <div
            className="report-stream-modal panel"
            role="dialog"
            aria-labelledby="report-stream-title"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="report-stream-head">
              <h3 id="report-stream-title">流式分析</h3>
              <button
                type="button"
                className="report-btn"
                onClick={() => setStreamOpen(false)}
              >
                关闭
              </button>
            </div>
            <p className="report-stream-status">{streamStatus || "准备中…"}</p>
            <div className="report-stream-md">
              {streamWiki ? (
                <MarkdownBody markdown={streamWiki} />
              ) : (
                <p className="wiki-muted">Wiki 内容将在此实时显示…</p>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Uploaded years */}
      {reports.length > 0 && (
        <div className="report-year-chips">
          <span className="report-year-label">已分析年份：</span>
          {reports.map((r) => (
            <span key={r.year} className="report-year-chip">
              {r.year}
              <button
                className="report-year-del"
                onClick={() => handleDelete(r.year)}
                title="删除"
              >
                ×
              </button>
            </span>
          ))}
          <button
            className="report-btn report-btn--accent"
            onClick={() => handleAnalyze(false)}
            disabled={analysisLoading || reports.length < 2}
          >
            {analysisLoading ? "分析中…" : "生成综合分析"}
          </button>
        </div>
      )}

      {/* Tabs & Content */}
      {reports.length > 0 && (
        <>
          <div className="tabs report-tabs">
            {tabs.map((t) => (
              <button
                key={t.key}
                className={tab === t.key ? "active" : ""}
                onClick={() => setTab(t.key)}
              >
                {t.label}
              </button>
            ))}
          </div>

          {tab === "overview" && <OverviewTable reports={reports} />}
          {tab === "growth" && <GrowthCharts data={chartData} />}
          {tab === "profit" && <ProfitCharts data={chartData} />}
          {tab === "structure" && <StructureCharts data={chartData} reports={reports} />}
          {tab === "insight" && (
            <InsightPanel
              reports={reports}
              analysis={analysis}
              loading={analysisLoading}
              onRefresh={() => handleAnalyze(true)}
            />
          )}
        </>
      )}

      {reports.length === 0 && !loading && (
        <div className="report-empty">
          <h3>暂无年报数据</h3>
          <p>请上传年度报告PDF文件，支持2015-2026年</p>
        </div>
      )}
    </div>
  );
}

function OverviewTable({ reports }: { reports: ReportData[] }) {
  return (
    <div className="panel report-panel">
      <div className="report-table-scroll">
        <table className="report-table">
          <thead>
            <tr>
              <th>指标</th>
              {reports.map((r) => (
                <th key={r.year}>{r.year}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            <tr>
              <td className="report-metric-name">营业收入 (亿)</td>
              {reports.map((r) => <td key={r.year}>{fmt(r.revenue)}</td>)}
            </tr>
            <tr>
              <td className="report-metric-name">营收增速</td>
              {reports.map((r) => (
                <td key={r.year} className={r.revenue_yoy >= 0 ? "positive" : "negative"}>
                  {fmtPct(r.revenue_yoy)}
                </td>
              ))}
            </tr>
            <tr>
              <td className="report-metric-name">净利润 (亿)</td>
              {reports.map((r) => <td key={r.year}>{fmt(r.net_profit)}</td>)}
            </tr>
            <tr>
              <td className="report-metric-name">净利润增速</td>
              {reports.map((r) => (
                <td key={r.year} className={r.net_profit_yoy >= 0 ? "positive" : "negative"}>
                  {fmtPct(r.net_profit_yoy)}
                </td>
              ))}
            </tr>
            <tr>
              <td className="report-metric-name">归母净利润 (亿)</td>
              {reports.map((r) => <td key={r.year}>{fmt(r.net_profit_parent)}</td>)}
            </tr>
            <tr>
              <td className="report-metric-name">毛利率</td>
              {reports.map((r) => <td key={r.year}>{fmtPct(r.gross_margin)}</td>)}
            </tr>
            <tr>
              <td className="report-metric-name">净利率</td>
              {reports.map((r) => <td key={r.year}>{fmtPct(r.net_margin)}</td>)}
            </tr>
            <tr>
              <td className="report-metric-name">ROE</td>
              {reports.map((r) => <td key={r.year}>{fmtPct(r.roe)}</td>)}
            </tr>
            <tr>
              <td className="report-metric-name">总资产 (亿)</td>
              {reports.map((r) => <td key={r.year}>{fmt(r.total_assets)}</td>)}
            </tr>
            <tr>
              <td className="report-metric-name">净资产 (亿)</td>
              {reports.map((r) => <td key={r.year}>{fmt(r.net_assets)}</td>)}
            </tr>
            <tr>
              <td className="report-metric-name">资产负债率</td>
              {reports.map((r) => <td key={r.year}>{fmtPct(r.debt_ratio)}</td>)}
            </tr>
            <tr>
              <td className="report-metric-name">经营现金流 (亿)</td>
              {reports.map((r) => <td key={r.year}>{fmt(r.operating_cashflow)}</td>)}
            </tr>
            <tr>
              <td className="report-metric-name">EPS (元)</td>
              {reports.map((r) => <td key={r.year}>{fmt(r.eps)}</td>)}
            </tr>
            <tr>
              <td className="report-metric-name">每股分红 (元)</td>
              {reports.map((r) => <td key={r.year}>{fmt(r.dividend_per_share)}</td>)}
            </tr>
            <tr>
              <td className="report-metric-name">员工人数</td>
              {reports.map((r) => (
                <td key={r.year}>{r.employee_count > 0 ? r.employee_count.toLocaleString() : "-"}</td>
              ))}
            </tr>
            <tr>
              <td className="report-metric-name">研发投入 (亿)</td>
              {reports.map((r) => <td key={r.year}>{fmt(r.rd_expense)}</td>)}
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  );
}

type ChartRow = Record<string, string | number>;

function GrowthCharts({ data }: { data: ChartRow[] }) {
  return (
    <div className="report-chart-grid">
      <div className="panel report-chart-panel">
        <h3 className="report-chart-title">营业收入与净利润 (亿元)</h3>
        <ResponsiveContainer width="100%" height={320}>
          <ComposedChart data={data} margin={{ top: 5, right: 20, bottom: 5, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
            <XAxis dataKey="year" tick={{ fontSize: 12 }} />
            <YAxis tick={{ fontSize: 12 }} />
            <Tooltip />
            <Legend />
            <Bar dataKey="revenue" name="营业收入" fill={COLORS.revenue} radius={[4, 4, 0, 0]} barSize={32} />
            <Bar dataKey="net_profit" name="净利润" fill={COLORS.profit} radius={[4, 4, 0, 0]} barSize={32} />
            <Line dataKey="net_profit_parent" name="归母净利润" stroke={COLORS.margin} strokeWidth={2} dot={{ r: 4 }} />
          </ComposedChart>
        </ResponsiveContainer>
      </div>

      <div className="panel report-chart-panel">
        <h3 className="report-chart-title">增长率 (%)</h3>
        <ResponsiveContainer width="100%" height={320}>
          <ComposedChart data={data} margin={{ top: 5, right: 20, bottom: 5, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
            <XAxis dataKey="year" tick={{ fontSize: 12 }} />
            <YAxis tick={{ fontSize: 12 }} unit="%" />
            <Tooltip />
            <Legend />
            <Bar dataKey="revenue_yoy" name="营收增速" fill={COLORS.revenue} radius={[4, 4, 0, 0]} barSize={28} />
            <Bar dataKey="profit_yoy" name="净利润增速" fill={COLORS.profit} radius={[4, 4, 0, 0]} barSize={28} />
          </ComposedChart>
        </ResponsiveContainer>
      </div>

      <div className="panel report-chart-panel">
        <h3 className="report-chart-title">经营现金流 (亿元)</h3>
        <ResponsiveContainer width="100%" height={320}>
          <ComposedChart data={data} margin={{ top: 5, right: 20, bottom: 5, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
            <XAxis dataKey="year" tick={{ fontSize: 12 }} />
            <YAxis tick={{ fontSize: 12 }} />
            <Tooltip />
            <Bar dataKey="operating_cashflow" name="经营现金流" fill={COLORS.cashflow} radius={[4, 4, 0, 0]} barSize={36} />
          </ComposedChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

function ProfitCharts({ data }: { data: ChartRow[] }) {
  return (
    <div className="report-chart-grid">
      <div className="panel report-chart-panel">
        <h3 className="report-chart-title">盈利能力趋势 (%)</h3>
        <ResponsiveContainer width="100%" height={340}>
          <ComposedChart data={data} margin={{ top: 5, right: 20, bottom: 5, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
            <XAxis dataKey="year" tick={{ fontSize: 12 }} />
            <YAxis tick={{ fontSize: 12 }} unit="%" />
            <Tooltip />
            <Legend />
            <Line dataKey="gross_margin" name="毛利率" stroke={COLORS.revenue} strokeWidth={2.5} dot={{ r: 5 }} />
            <Line dataKey="net_margin" name="净利率" stroke={COLORS.profit} strokeWidth={2.5} dot={{ r: 5 }} />
            <Line dataKey="roe" name="ROE" stroke={COLORS.roe} strokeWidth={2.5} dot={{ r: 5 }} />
          </ComposedChart>
        </ResponsiveContainer>
      </div>

      <div className="panel report-chart-panel">
        <h3 className="report-chart-title">EPS与分红 (元/股)</h3>
        <ResponsiveContainer width="100%" height={340}>
          <ComposedChart data={data} margin={{ top: 5, right: 20, bottom: 5, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
            <XAxis dataKey="year" tick={{ fontSize: 12 }} />
            <YAxis tick={{ fontSize: 12 }} />
            <Tooltip />
            <Legend />
            <Bar dataKey="eps" name="每股收益" fill={COLORS.revenue} radius={[4, 4, 0, 0]} barSize={28} />
            <Bar dataKey="dividend" name="每股分红" fill={COLORS.dividend} radius={[4, 4, 0, 0]} barSize={28} />
          </ComposedChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

function StructureCharts({
  data,
  reports,
}: {
  data: ChartRow[];
  reports: ReportData[];
}) {
  return (
    <div className="report-chart-grid">
      <div className="panel report-chart-panel">
        <h3 className="report-chart-title">资产规模 (亿元)</h3>
        <ResponsiveContainer width="100%" height={340}>
          <ComposedChart data={data} margin={{ top: 5, right: 20, bottom: 5, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
            <XAxis dataKey="year" tick={{ fontSize: 12 }} />
            <YAxis tick={{ fontSize: 12 }} />
            <Tooltip />
            <Legend />
            <Bar dataKey="total_assets" name="总资产" fill={COLORS.asset} radius={[4, 4, 0, 0]} barSize={28} />
            <Bar dataKey="net_assets" name="净资产" fill={COLORS.profit} radius={[4, 4, 0, 0]} barSize={28} />
          </ComposedChart>
        </ResponsiveContainer>
      </div>

      <div className="panel report-chart-panel">
        <h3 className="report-chart-title">资产负债率 (%)</h3>
        <ResponsiveContainer width="100%" height={340}>
          <ComposedChart data={data} margin={{ top: 5, right: 20, bottom: 5, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
            <XAxis dataKey="year" tick={{ fontSize: 12 }} />
            <YAxis tick={{ fontSize: 12 }} unit="%" domain={[0, 60]} />
            <Tooltip />
            <Bar dataKey="debt_ratio" name="资产负债率" fill={COLORS.debt} radius={[4, 4, 0, 0]} barSize={36} />
          </ComposedChart>
        </ResponsiveContainer>
      </div>

      {/* Per-year segment breakdown */}
      <div className="panel report-chart-panel report-chart-panel--wide">
        <h3 className="report-chart-title">主营业务构成</h3>
        <div className="report-segments-grid">
          {reports
            .filter((r) => r.segments && r.segments.length > 0)
            .map((r) => (
              <div key={r.year} className="report-segment-card">
                <h4>{r.year}年</h4>
                {r.segments.map((s, i) => (
                  <div key={i} className="report-seg-row">
                    <span className="report-seg-name">{s.name}</span>
                    <div className="report-seg-bar-track">
                      <div
                        className="report-seg-bar-fill"
                        style={{ width: `${Math.min(s.ratio * 100, 100)}%` }}
                      />
                    </div>
                    <span className="report-seg-val">
                      {fmt(s.revenue)}亿 ({(s.ratio * 100).toFixed(1)}%)
                    </span>
                  </div>
                ))}
              </div>
            ))}
        </div>
      </div>
    </div>
  );
}

function InsightPanel({
  reports,
  analysis,
  loading,
  onRefresh,
}: {
  reports: ReportData[];
  analysis: AnalysisResult | null;
  loading: boolean;
  onRefresh: () => void;
}) {
  return (
    <div className="report-insight-wrap">
      {/* Per-year highlights */}
      <div className="panel report-panel">
        <h3 className="report-chart-title">各年度要点</h3>
        <div className="report-yearly-insights">
          {reports.map((r) => (
            <div key={r.year} className="report-year-insight">
              <h4>{r.year}年</h4>
              {r.highlights && (
                <div className="report-insight-block">
                  <span className="report-insight-tag report-insight-tag--highlight">亮点</span>
                  <p>{r.highlights}</p>
                </div>
              )}
              {r.risks && (
                <div className="report-insight-block">
                  <span className="report-insight-tag report-insight-tag--risk">风险</span>
                  <p>{r.risks}</p>
                </div>
              )}
              {r.outlook && (
                <div className="report-insight-block">
                  <span className="report-insight-tag report-insight-tag--outlook">展望</span>
                  <p>{r.outlook}</p>
                </div>
              )}
            </div>
          ))}
        </div>
      </div>

      {/* AI comprehensive analysis */}
      <div className="panel report-panel">
        <div className="report-analysis-header">
          <h3 className="report-chart-title">AI 综合分析</h3>
          <button
            className="report-btn report-btn--accent"
            onClick={onRefresh}
            disabled={loading || reports.length < 2}
          >
            {loading ? "生成中…" : "重新生成"}
          </button>
        </div>
        {loading && (
          <div className="report-analysis-loading">
            <div className="report-spinner" />
            <p>AI正在分析多年财务数据，请稍候…</p>
          </div>
        )}
        {!loading && analysis?.summary && (
          <div className="report-analysis-content">
            {analysis.summary.split("\n").map((line, i) =>
              line.trim() ? <p key={i}>{line}</p> : <br key={i} />
            )}
          </div>
        )}
        {!loading && !analysis?.summary && reports.length >= 2 && (
          <p className="report-analysis-hint">
            点击"生成综合分析"按钮，AI将基于所有年份数据生成全面的发展分析报告
          </p>
        )}
        {!loading && reports.length < 2 && (
          <p className="report-analysis-hint">至少需要上传2年年报数据才能生成综合分析</p>
        )}
      </div>
    </div>
  );
}
