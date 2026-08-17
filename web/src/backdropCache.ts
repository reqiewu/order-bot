import { getBackdropInfo, type BackdropInfo } from './api';

const cache = new Map<string, BackdropInfo>();
const inflight = new Map<string, Promise<BackdropInfo | null>>();

function key(gift: string, name: string): string {
  return `${gift}\0${name}`;
}

export function rememberAll(list: BackdropInfo[]) {
  for (const info of list) {
    if (!info?.name) continue;
    cache.set(key('*', info.name), info);
  }
}

export function cachedBackdrop(gift: string, name: string): BackdropInfo | undefined {
  if (!name) return undefined;
  if (gift) {
    const hit = cache.get(key(gift, name));
    if (hit) return hit;
  }
  return cache.get(key('*', name));
}

export function rememberBackdrop(gift: string, info: BackdropInfo) {
  if (!gift || !info?.name) return;
  cache.set(key(gift, info.name), info);
}

export function loadBackdrop(gift: string, name: string): Promise<BackdropInfo | null> {
  if (!gift || !name) return Promise.resolve(null);
  const k = key(gift, name);
  const hit = cache.get(k);
  if (hit) return Promise.resolve(hit);
  const pending = inflight.get(k);
  if (pending) return pending;
  const p = getBackdropInfo(gift, name)
    .then((info) => {
      cache.set(k, info);
      inflight.delete(k);
      return info;
    })
    .catch(() => {
      inflight.delete(k);
      return null;
    });
  inflight.set(k, p);
  return p;
}

export async function loadBackdrops(
  gift: string,
  names: string[],
  onEach?: () => void,
): Promise<void> {
  const unique = [...new Set(names.map((n) => n.trim()).filter(Boolean))];
  const chunk = 6;
  for (let i = 0; i < unique.length; i += chunk) {
    await Promise.all(
      unique.slice(i, i + chunk).map(async (n) => {
        await loadBackdrop(gift, n);
        onEach?.();
      }),
    );
  }
}
