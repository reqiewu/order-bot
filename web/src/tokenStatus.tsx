import type { TokenStatus } from './api';

export function marketStatus(
  set: boolean | undefined,
  ok: boolean | undefined,
  live?: boolean,
  stored?: boolean,
  session?: boolean,
): string {
  if (ok === true) return 'Работает';
  if (session === false || ok === false) return 'Обновите сессию';
  if (set || live || stored) return 'Не проверен';
  return 'Обновите сессию';
}

export function formatAgo(at: number | undefined, now: number): string {
  if (!at) return '';
  const s = Math.max(0, Math.round((now - at) / 1000));
  if (s < 8) return 'только что';
  if (s < 60) return `${s} с назад`;
  const m = Math.round(s / 60);
  if (m < 60) return `${m} мин назад`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h} ч назад`;
  return `${Math.round(h / 24)} д назад`;
}

export function marketTone(
  set: boolean | undefined,
  ok: boolean | undefined,
  live?: boolean,
  stored?: boolean,
  session?: boolean,
): 'ok' | 'warn' | 'mute' {
  if (ok === true) return 'ok';
  if (ok === false || session === false) return 'warn';
  if (live || stored || set) return 'mute';
  return 'mute';
}

export function TokenMark({ ok }: { ok?: boolean }) {
  if (ok === true) {
    return (
      <span className="token-mark token-mark--ok" aria-label="ok">
        ✓
      </span>
    );
  }
  if (ok === false) {
    return (
      <span className="token-mark token-mark--bad" aria-label="bad">
        ✗
      </span>
    );
  }
  return (
    <span className="token-mark token-mark--unknown" aria-label="unknown">
      ·
    </span>
  );
}

export type TokenFields = {
  tokens: TokenStatus | null;
  checkedAt: Partial<Record<'mrkt' | 'portals' | 'getgems' | 'tonnel', number>>;
};
