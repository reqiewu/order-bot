function authHeaders(): HeadersInit {
  const h: Record<string, string> = { 'Content-Type': 'application/json' };
  const initData = window.Telegram?.WebApp?.initData;
  if (initData) {
    h['X-Telegram-Init-Data'] = initData;
  }
  const devId = import.meta.env.VITE_DEV_TELEGRAM_ID as string | undefined;
  if (devId) {
    h['X-Dev-Telegram-Id'] = String(devId);
  }
  return h;
}

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: { ...authHeaders(), ...(init?.headers ?? {}) },
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    const msg = (data as { error?: string }).error ?? res.statusText;
    throw new Error(msg);
  }
  return data as T;
}

export type TokenStatus = {
  mrkt_set: boolean;
  portals_set: boolean;
  getgems_set?: boolean;
  tonnel_set?: boolean;
  mrkt_live?: boolean;
  portals_live?: boolean;
  getgems_live?: boolean;
  tonnel_live?: boolean;
  mrkt_stored?: boolean;
  portals_stored?: boolean;
  getgems_stored?: boolean;
  tonnel_stored?: boolean;
  mrkt_ok?: boolean;
  portals_ok?: boolean;
  getgems_ok?: boolean;
  tonnel_ok?: boolean;
  mrkt_error?: string;
  portals_error?: string;
  getgems_error?: string;
  tonnel_error?: string;
  tg_session?: boolean;
};

export type Runtime = {
  poll_interval_sec: number;
  min_profit_ton: number;
  min_spread_bps: number;
  min_spread_pct: number;
};

export type WatchSlot = {
  id?: string;
  collection: string;
  model?: string;
  backdrop?: string;
};

export function getTokens(probe = false) {
  const qs = probe ? '?probe=1' : '';
  return api<TokenStatus>(`/api/settings/tokens${qs}`);
}

export type TokenBody = {
  mrkt_token?: string;
  portals_tma?: string;
  getgems_api_key?: string;
  tonnel_initdata?: string;
};

export function putTokens(body: TokenBody) {
  return api<TokenStatus & { ok: boolean }>('/api/settings/tokens', {
    method: 'PUT',
    body: JSON.stringify(body),
  });
}

export function probeTokens(body?: TokenBody) {
  return api<TokenStatus & { ok: boolean }>('/api/settings/tokens/probe', {
    method: 'POST',
    body: JSON.stringify(body ?? {}),
  });
}

/** Strip quotes / Bearer / tma before save. */
export function normalizeMRKTPaste(raw: string): string {
  let s = raw.trim().replace(/^["']|["']$/g, '').trim();
  if (/^bearer\s+/i.test(s)) s = s.replace(/^bearer\s+/i, '').trim();
  return s;
}

export function normalizePortalsPaste(raw: string): string {
  let s = raw.trim().replace(/^["']|["']$/g, '').trim();
  if (/^authorization:\s*/i.test(s)) s = s.replace(/^authorization:\s*/i, '').trim();
  if (/^tma\s+/i.test(s)) s = s.replace(/^tma\s+/i, '').trim();
  return s;
}

export function normalizeGetgemsPaste(raw: string): string {
  return normalizeMRKTPaste(raw);
}

export function normalizeTonnelPaste(raw: string): string {
  return normalizePortalsPaste(raw);
}

export function getRuntime() {
  return api<Runtime>('/api/settings/runtime');
}

export function putRuntime(body: Partial<{
  poll_interval_sec: number;
  min_profit_ton: number;
  min_spread_pct: number;
}>) {
  return api<{ ok: boolean }>('/api/settings/runtime', {
    method: 'PUT',
    body: JSON.stringify(body),
  });
}

export function listSlots() {
  return api<{ slots: WatchSlot[] }>('/api/slots');
}

export function addSlot(slot: WatchSlot) {
  return api<{ ok: boolean; slots: WatchSlot[] }>('/api/slots', {
    method: 'POST',
    body: JSON.stringify(slot),
  });
}

export function deleteSlot(slot: WatchSlot) {
  return api<{ ok: boolean; slots: WatchSlot[] }>('/api/slots', {
    method: 'DELETE',
    body: JSON.stringify(slot),
  });
}

export type GiftRow = {
  name: string;
  title: string;
  floor_ton: number;
  volume_ton: number;
  preview_url?: string;
};

export type AttrRow = {
  name: string;
  title?: string;
  rarity: number;
  preview_url?: string;
  center_color?: string;
  edge_color?: string;
  pattern_color?: string;
  text_color?: string;
};

export type BackdropInfo = {
  name: string;
  center_color: string;
  edge_color: string;
  pattern_color: string;
  text_color: string;
};

export function listGifts(q = '') {
  const qs = q ? `?q=${encodeURIComponent(q)}` : '';
  return api<{ items: GiftRow[]; credit?: string }>(`/api/catalog/gifts${qs}`);
}

export function getGift(gift: string) {
  return api<{
    name: string;
    models: AttrRow[];
    backdrops: AttrRow[];
    credit?: string;
  }>(`/api/catalog/gifts/${encodeURIComponent(gift)}`);
}

export function listModels(gift: string, q = '') {
  const qs = q ? `?q=${encodeURIComponent(q)}` : '';
  return api<{ items: AttrRow[]; total: number }>(
    `/api/catalog/gifts/${encodeURIComponent(gift)}/models${qs}`,
  );
}

export function listBackdrops(gift: string, model = '', q = '') {
  const params = new URLSearchParams();
  if (model) params.set('model', model);
  if (q) params.set('q', q);
  const qs = params.toString() ? `?${params}` : '';
  return api<{ items: AttrRow[]; total: number }>(
    `/api/catalog/gifts/${encodeURIComponent(gift)}/backdrops${qs}`,
  );
}

export function getBackdropInfo(gift: string, backdrop: string) {
  return api<BackdropInfo>(
    `/api/catalog/gifts/${encodeURIComponent(gift)}/backdrops/${encodeURIComponent(backdrop)}`,
  );
}

export function listAllBackdrops() {
  return api<{ items: BackdropInfo[]; credit?: string }>('/api/catalog/backdrops');
}

function assetSlug(s: string): string {
  return (s || '').replace(/[^a-zA-Z0-9._'-]/g, '');
}

export function giftPreviewURL(gift: string, size = 128): string {
  return `/api/assets/original/${encodeURIComponent(assetSlug(gift))}.png?size=${size}`;
}

export function modelPreviewURL(gift: string, model: string, size = 128): string {
  return `/api/assets/model/${encodeURIComponent(assetSlug(gift))}/${encodeURIComponent(assetSlug(model))}.png?size=${size}`;
}

export function giftTgsURL(gift: string): string {
  return `/api/assets/original/${encodeURIComponent(assetSlug(gift))}.tgs`;
}

export function modelTgsURL(gift: string, model: string): string {
  return `/api/assets/model/${encodeURIComponent(assetSlug(gift))}/${encodeURIComponent(assetSlug(model))}.tgs`;
}

export function formatTON(n: number): string {
  if (!n || n <= 0) return '—';
  if (n >= 1000) return n.toLocaleString('ru-RU', { maximumFractionDigits: 0 });
  if (n >= 10) return n.toLocaleString('ru-RU', { maximumFractionDigits: 1 });
  return n.toLocaleString('ru-RU', { maximumFractionDigits: 2 });
}

export function formatRarity(r: number): string {
  if (!r || r <= 0) return '';
  const v = Number.isInteger(r) ? String(r) : r.toFixed(1).replace(/\.0$/, '');
  return `${v}%`;
}

export function backdropGradient(center?: string, edge?: string): string | undefined {
  if (!center) return undefined;
  if (!edge) return center;
  return `linear-gradient(145deg, ${center}, ${edge})`;
}
