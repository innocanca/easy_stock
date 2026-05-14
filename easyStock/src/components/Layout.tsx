import { NavLink, Outlet, useLocation } from "react-router-dom";

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
        <nav className="sidebar-nav" aria-label="主导航">
          <NavLink end to="/" className={({ isActive }) => (isActive ? "active" : "")}>
            推荐
          </NavLink>
          <NavLink to="/sector" className={() => (sectorNavActive ? "active" : "")}>
            板块
          </NavLink>
          <NavLink to="/chain" className={({ isActive }) => (isActive ? "active" : "")}>
            产业链
          </NavLink>
          <NavLink to="/report" className={({ isActive }) => (isActive ? "active" : "")}>
            年报分析
          </NavLink>
          <NavLink to="/wiki" className={({ isActive }) => (isActive ? "active" : "")}>
            知识库
          </NavLink>
        </nav>
      </aside>
      <main className="main">
        <Outlet />
      </main>
    </div>
  );
}
