import { NavLink, Outlet, useLocation } from "react-router-dom";

export function Layout() {
  const loc = useLocation();
  const sectorNavActive =
    loc.pathname === "/sector" || loc.pathname.startsWith("/sector/");
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-brand">
          easyStock
          <span>价值 · 估值 · 成长 · 确定性</span>
        </div>
        <nav className="sidebar-nav">
          <NavLink end to="/" className={({ isActive }) => (isActive ? "active" : "")}>
            推荐组合
          </NavLink>
          <NavLink to="/sector" className={() => (sectorNavActive ? "active" : "")}>
            板块
          </NavLink>
          <NavLink to="/chain" className={({ isActive }) => (isActive ? "active" : "")}>
            产业链
          </NavLink>
        </nav>
        <p className="sidebar-footnote">
          本地开发：先启动 easystock-api（Go，:4000），再运行 npm run dev；前端通过 /api 代理拉取数据。
        </p>
      </aside>
      <main className="main">
        <Outlet />
      </main>
    </div>
  );
}
