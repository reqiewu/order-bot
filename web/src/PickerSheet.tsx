import { useEffect, useMemo, useState, type UIEvent } from 'react';
import {
  backdropGradient,
  formatRarity,
  formatTON,
  giftPreviewURL,
  modelPreviewURL,
  type AttrRow,
  type GiftRow,
} from './api';
import { cachedBackdrop, loadBackdrops } from './backdropCache';

export type GiftSort = 'default' | 'floor' | 'volume';

const PAGE = 24;

function useVisiblePage<T>(items: T[], open: boolean) {
  const [shown, setShown] = useState(PAGE);
  useEffect(() => {
    setShown(PAGE);
  }, [items, open]);
  function onScroll(e: UIEvent<HTMLDivElement>) {
    const el = e.currentTarget;
    if (el.scrollHeight - el.scrollTop - el.clientHeight < 180) {
      setShown((n) => Math.min(items.length, n + PAGE));
    }
  }
  return {
    visible: items.slice(0, shown),
    shown,
    onScroll,
    hasMore: shown < items.length,
  };
}

type GiftPickerProps = {
  open: boolean;
  title: string;
  items: GiftRow[];
  selected: string;
  onClose: () => void;
  onSelect: (name: string) => void;
  onClear?: () => void;
};

export function GiftPicker({ open, title, items, selected, onClose, onSelect, onClear }: GiftPickerProps) {
  const [q, setQ] = useState('');
  const [sort, setSort] = useState<GiftSort>('volume');

  const filtered = useMemo(() => {
    const s = q.trim().toLowerCase();
    let list = items;
    if (s) {
      list = items.filter(
        (it) => it.name.toLowerCase().includes(s) || (it.title || '').toLowerCase().includes(s),
      );
    }
    if (sort === 'default') return list;
    const copy = [...list];
    if (sort === 'floor') {
      copy.sort((a, b) => (a.floor_ton || Number.POSITIVE_INFINITY) - (b.floor_ton || Number.POSITIVE_INFINITY));
    } else {
      copy.sort((a, b) => (b.volume_ton || 0) - (a.volume_ton || 0));
    }
    return copy;
  }, [items, q, sort]);

  const page = useVisiblePage(filtered, open);

  if (!open) return null;
  return (
    <div className="sheet">
      <div className="sheet__bar">
        <button type="button" className="sheet__link" onClick={onClose}>
          Отмена
        </button>
        <div className="sheet__title">{title}</div>
        <button
          type="button"
          className="sheet__link"
          onClick={() => {
            onClear?.();
            onClose();
          }}
        >
          Сброс
        </button>
      </div>
      <div className="sheet__search">
        <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Поиск" />
      </div>
      <div className="sheet__sort">
        <button
          type="button"
          className={sort === 'volume' ? 'is-on' : ''}
          onClick={() => setSort('volume')}
        >
          По обороту
        </button>
        <button
          type="button"
          className={sort === 'floor' ? 'is-on' : ''}
          onClick={() => setSort('floor')}
        >
          По floor
        </button>
        <button
          type="button"
          className={sort === 'default' ? 'is-on' : ''}
          onClick={() => setSort('default')}
        >
          Как в каталоге
        </button>
      </div>
      <div className="sheet__list" onScroll={page.onScroll}>
        {page.visible.map((it) => {
          const active = it.name === selected;
          return (
            <button
              type="button"
              key={it.name}
              className={`sheet__row${active ? ' is-active' : ''}`}
              onClick={() => {
                onSelect(it.name);
                onClose();
              }}
            >
              <img
                className="sheet__thumb"
                src={it.preview_url || giftPreviewURL(it.name)}
                alt=""
                loading="lazy"
              />
              <div className="sheet__main">
                <div className="sheet__name">{it.title || it.name}</div>
              </div>
              <div className="sheet__right">
                <div className="sheet__metrics">
                  <span className="sheet__num">{formatTON(it.floor_ton)}</span>
                  <span className="sheet__meta">{formatTON(it.volume_ton)}</span>
                </div>
                {active ? <span className="sheet__check">✓</span> : null}
              </div>
            </button>
          );
        })}
        {page.hasMore ? <div className="sheet__more">Ещё…</div> : null}
      </div>
    </div>
  );
}

type AttrPickerProps = {
  open: boolean;
  title: string;
  items: AttrRow[];
  selected: string;
  onClose: () => void;
  onSelect: (name: string) => void;
  onClear?: () => void;
  showPreview?: boolean;
  showColors?: boolean;
  gift?: string;
  busy?: boolean;
};

export function AttrPicker({
  open,
  title,
  items,
  selected,
  onClose,
  onSelect,
  onClear,
  showPreview,
  showColors,
  gift,
  busy,
}: AttrPickerProps) {
  const [q, setQ] = useState('');
  const [, setPaint] = useState(0);
  const filtered = useMemo(() => {
    const s = q.trim().toLowerCase();
    if (!s) return items;
    return items.filter((it) => it.name.toLowerCase().includes(s));
  }, [items, q]);
  const page = useVisiblePage(filtered, open);

  useEffect(() => {
    if (!open || !showColors || !gift) return;
    const missing = filtered
      .slice(0, page.shown)
      .filter((it) => !it.center_color && !cachedBackdrop(gift, it.name))
      .map((it) => it.name);
    if (missing.length === 0) return;
    let cancelled = false;
    void loadBackdrops(gift, missing, () => {
      if (!cancelled) setPaint((n) => n + 1);
    });
    return () => {
      cancelled = true;
    };
  }, [open, showColors, gift, page.shown, filtered]);

  if (!open) return null;
  return (
    <div className="sheet">
      <div className="sheet__bar">
        <button type="button" className="sheet__link" onClick={onClose}>
          Отмена
        </button>
        <div className="sheet__title">{title}</div>
        <button
          type="button"
          className="sheet__link"
          onClick={() => {
            onClear?.();
            onClose();
          }}
        >
          Сброс
        </button>
      </div>
      <div className="sheet__search">
        <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Поиск" />
      </div>
      <div className="sheet__list" onScroll={page.onScroll}>
        <button
          type="button"
          className={`sheet__row${!selected ? ' is-active' : ''}`}
          onClick={() => {
            onSelect('');
            onClose();
          }}
        >
          <div className="sheet__thumb sheet__thumb--empty" />
          <div className="sheet__main">
            <div className="sheet__name">Любая</div>
          </div>
          {!selected ? <span className="sheet__check">✓</span> : null}
        </button>
        {busy && items.length === 0 ? (
          <div className="sheet__more">Загрузка…</div>
        ) : null}
        {page.visible.map((it) => {
          const active = it.name === selected;
          const info = showColors && gift ? cachedBackdrop(gift, it.name) : undefined;
          const bg = showColors
            ? backdropGradient(info?.center_color || it.center_color, info?.edge_color || it.edge_color)
            : undefined;
          return (
            <button
              type="button"
              key={it.name}
              className={`sheet__row${active ? ' is-active' : ''}`}
              onClick={() => {
                onSelect(it.name);
                onClose();
              }}
            >
              {showPreview && (gift || it.preview_url) ? (
                <img
                  className="sheet__thumb"
                  src={it.preview_url || (gift ? modelPreviewURL(gift, it.name, 128) : '')}
                  alt=""
                  loading="lazy"
                />
              ) : showColors ? (
                <div className="sheet__thumb" style={bg ? { background: bg } : undefined} />
              ) : (
                <div className="sheet__thumb sheet__thumb--empty" />
              )}
              <div className="sheet__main">
                <div className="sheet__name">{it.title || it.name}</div>
              </div>
              <div className="sheet__right">
                <span className="sheet__rarity">{formatRarity(it.rarity)}</span>
                {active ? <span className="sheet__check">✓</span> : null}
              </div>
            </button>
          );
        })}
        {page.hasMore ? <div className="sheet__more">Ещё…</div> : null}
      </div>
    </div>
  );
}
