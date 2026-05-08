import type { PickItem } from "@shared/dataset";

function SparkBars({
  values,
  color,
}: {
  values: number[];
  color: string;
}) {
  const max = Math.max(...values, 1);
  const min = Math.min(...values, 0);
  const span = max - min || 1;
  return (
    <div className="mini-chart-row">
      {values.map((v, i) => {
        const h = ((v - min) / span) * 100;
        return (
          <div
            key={i}
            className="mini-bar"
            style={{
              height: `${Math.max(12, h)}%`,
              background: color,
              opacity: 0.75 + i * 0.05,
            }}
          />
        );
      })}
    </div>
  );
}

export function TrendSpark({ item }: { item: PickItem }) {
  const profits = item.profitTrend.map((x) => x.profit);
  const pes = item.peTrend.map((x) => x.pe);
  return (
    <div className="spark-wrap">
      <div className="spark-box">
        <h4>利润趋势（最近四季，亿元）</h4>
        <SparkBars values={profits} color="var(--accent)" />
        <div className="spark-caption">{item.profitTrend.map((x) => x.q).join(" → ")}</div>
      </div>
      <div className="spark-box">
        <h4>PE 变化</h4>
        <SparkBars values={pes} color="var(--chart-secondary)" />
        <div className="spark-caption">{item.peTrend.map((x) => x.q).join(" → ")}</div>
      </div>
    </div>
  );
}
