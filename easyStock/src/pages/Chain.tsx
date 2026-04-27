import { Link } from "react-router-dom";

export function Chain() {
  return (
    <>
      <h1 className="page-title">产业链分析</h1>
      <p className="page-sub">上下游结构化展示（占位）。</p>
      <div className="chain-placeholder">
        <h2>即将接入</h2>
        <p>
          计划支持上下游节点图或「上游 / 本公司 / 下游」三段式列表。
          <br />
          数据接入前保留此占位，避免误以为功能异常。
        </p>
        <Link to="/">返回推荐首页</Link>
      </div>
    </>
  );
}
