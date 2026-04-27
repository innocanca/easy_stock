/**
 * API base URL:
 * - Empty: same-origin `/api/*` (Vite dev proxy → backend, or nginx in Docker).
 * - Set VITE_API_URL=http://localhost:4000 if you run frontend without proxy.
 */
export function getApiBase(): string {
  const raw = import.meta.env.VITE_API_URL;
  if (raw === undefined || raw === null) return "";
  const s = String(raw).trim();
  return s.replace(/\/$/, "");
}

export async function apiGet<T>(path: string): Promise<T> {
  const base = getApiBase();
  const url = `${base}${path.startsWith("/") ? path : `/${path}`}`;
  const res = await fetch(url);
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`);
  }
  return res.json() as Promise<T>;
}
