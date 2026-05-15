export function Spinner({ text = "加载中…" }: { text?: string }) {
  return (
    <div className="spinner-wrap">
      <div className="spinner" />
      {text && <p className="spinner-text">{text}</p>}
    </div>
  );
}
