import { useEffect, useState } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { fetchSectorList } from "@/api/stockApi";
import { Layout } from "@/components/Layout";
import { Chain } from "@/pages/Chain";
import { Home } from "@/pages/Home";
import { Report } from "@/pages/Report";
import { Wiki } from "@/pages/Wiki";
import { Sector } from "@/pages/Sector";
import { StockDetail } from "@/pages/StockDetail";
import { Watchlist } from "@/pages/Watchlist";
import Market from "@/pages/Market";

function SectorIndexRedirect() {
  const [target, setTarget] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    fetchSectorList()
      .then((list) => {
        if (list.length > 0) setTarget(`/sector/${encodeURIComponent(list[0].id)}`);
        else setFailed(true);
      })
      .catch(() => setFailed(true));
  }, []);

  if (failed) return <Navigate to="/" replace />;
  if (!target) {
    return (
      <>
        <h1 className="page-title">板块</h1>
        <p className="page-sub">加载中…</p>
      </>
    );
  }
  return <Navigate to={target} replace />;
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<Home />} />
          <Route path="stock/:code" element={<StockDetail />} />
          <Route path="sector" element={<SectorIndexRedirect />} />
          <Route path="sector/:id" element={<Sector />} />
          <Route path="chain" element={<Chain />} />
          <Route path="report" element={<Report />} />
          <Route path="wiki" element={<Wiki />} />
          <Route path="watchlist" element={<Watchlist />} />
          <Route path="market" element={<Market />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
