const KEY = "easystock_watchlist";

export interface WatchlistItem {
  code: string;
  name: string;
}

export function getWatchlist(): WatchlistItem[] {
  try {
    const raw = localStorage.getItem(KEY);
    return raw ? (JSON.parse(raw) as WatchlistItem[]) : [];
  } catch {
    return [];
  }
}

export function saveWatchlist(list: WatchlistItem[]) {
  localStorage.setItem(KEY, JSON.stringify(list));
}

export function addToWatchlist(item: WatchlistItem) {
  const list = getWatchlist();
  if (!list.some((w) => w.code === item.code)) {
    list.push(item);
    saveWatchlist(list);
  }
}

export function removeFromWatchlist(code: string) {
  const list = getWatchlist().filter((w) => w.code !== code);
  saveWatchlist(list);
}

export function toggleWatchlist(item: WatchlistItem): boolean {
  const list = getWatchlist();
  const idx = list.findIndex((w) => w.code === item.code);
  if (idx >= 0) {
    list.splice(idx, 1);
    saveWatchlist(list);
    return false;
  }
  list.push(item);
  saveWatchlist(list);
  return true;
}

export function isInWatchlist(code: string): boolean {
  return getWatchlist().some((w) => w.code === code);
}
