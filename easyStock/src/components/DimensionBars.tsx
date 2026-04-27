import type { Dimensions } from "@/data/mock";

const LABELS: { key: keyof Dimensions; short: string }[] = [
  { key: "value", short: "价值" },
  { key: "valuation", short: "估值" },
  { key: "certainty", short: "确定性" },
  { key: "growth", short: "成长性" },
];

export function DimensionBars({ dimensions }: { dimensions: Dimensions }) {
  return (
    <div className="dim-bars">
      {LABELS.map(({ key, short }) => (
        <div key={key} style={{ display: "contents" }}>
          <label>{short}</label>
          <div className="dim-bar-track">
            <div
              className={`dim-bar-fill dim-bar-fill--${key}`}
              style={{ width: `${dimensions[key]}%` }}
            />
          </div>
        </div>
      ))}
    </div>
  );
}
