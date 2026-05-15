import { useCallback, useEffect, useRef, useState } from "react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { getApiBase } from "@/api/client";

interface SearchHit {
  code: string;
  name: string;
}

function SidebarSearch() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchHit[]>([]);
  const [open, setOpen] = useState(false);
  const [activeIdx, setActiveIdx] = useState(-1);
  const navigate = useNavigate();
  const timerRef = useRef<ReturnType<typeof setTimeout>>();
  const wrapRef = useRef<HTMLDivElement>(null);

  const doSearch = useCallback((q: string) => {
    if (q.trim().length === 0) {
      setResults([]);
      setOpen(false);
      return;
    }
    const base = getApiBase();
    fetch(`${base}/api/search?q=${encodeURIComponent(q)}`)
      .then((r) => r.json())
      .then((list: SearchHit[]) => {
        setResults(list);
        setOpen(list.length > 0);
        setActiveIdx(-1);
      })
      .catch(() => {});
  }, []);

  const handleChange = (val: string) => {
    setQuery(val);
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => doSearch(val), 200);
  };

  const go = (code: string) => {
    setQuery("");
    setResults([]);
    setOpen(false);
    navigate(`/stock/${encodeURIComponent(code)}`);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!open || results.length === 0) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIdx((i) => (i + 1) % results.length);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIdx((i) => (i <= 0 ? results.length - 1 : i - 1));
    } else if (e.key === "Enter" && activeIdx >= 0) {
      e.preventDefault();
      go(results[activeIdx].code);
    } else if (e.key === "Escape") {
      setOpen(false);
    }
  };

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  return (
    <div className="sidebar-search" ref={wrapRef}>
      <input
        type="text"
        className="sidebar-search-input"
        placeholder="搜索股票代码/名称…"
        value={query}
        onChange={(e) => handleChange(e.target.value)}
        onKeyDown={handleKeyDown}
        onFocus={() => results.length > 0 && setOpen(true)}
      />
      {open && (
        <ul className="sidebar-search-dropdown">
          {results.map((r, i) => (
            <li
              key={r.code}
              className={i === activeIdx ? "active" : ""}
              onMouseDown={() => go(r.code)}
              onMouseEnter={() => setActiveIdx(i)}
            >
              <span className="sidebar-search-name">{r.name}</span>
              <span className="sidebar-search-code">{r.code}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

export function Layout() {
  const loc = useLocation();
  const sectorNavActive =
    loc.pathname === "/sector" || loc.pathname.startsWith("/sector/");
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <span className="sidebar-logo">easyStock</span>
          <p className="sidebar-tagline">价值 · 估值 · 成长 · 确定性</p>
        </div>
        <SidebarSearch />
        <nav className="sidebar-nav" aria-label="主导航">
          <NavLink end to="/" className={({ isActive }) => (isActive ? "active" : "")}>
            推荐
          </NavLink>
          <NavLink to="/sector" className={() => (sectorNavActive ? "active" : "")}>
            板块
          </NavLink>
          <NavLink to="/report" className={({ isActive }) => (isActive ? "active" : "")}>
            年报研究
          </NavLink>
          <NavLink to="/watchlist" className={({ isActive }) => (isActive ? "active" : "")}>
            自选股
          </NavLink>
        </nav>
        <div className="sidebar-footer">
          <NavLink to="/chain" className="sidebar-footer-link">产业链（即将上线）</NavLink>
          <p className="sidebar-version">v0.1 · 数据来自 Tushare</p>
        </div>
      </aside>
      <main className="main">
        <Outlet />
      </main>
    </div>
  );
}
