import { useMemo, useState } from 'react';
import {
  backdropGradient,
  formatRarity,
  formatTON,
  giftPreviewURL,
  type AttrRow,
  type GiftRow,
} from './api';

export type GiftSort = 'default' | 'floor' | 'volume';

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
      <div className="sheet__list">
        {filtered.map((it) => {
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
}: AttrPickerProps) {
  const [q, setQ] = useState('');
  const filtered = useMemo(() => {
    const s = q.trim().toLowerCase();
    if (!s) return items;
    return items.filter((it) => it.name.toLowerCase().includes(s));
  }, [items, q]);

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
      <div className="sheet__list">
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
        {filtered.map((it) => {
          const active = it.name === selected;
          const bg = showColors ? backdropGradient(it.center_color, it.edge_color) : undefined;
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
              {showPreview ? (
                <img className="sheet__thumb" src={it.preview_url} alt="" loading="lazy" />
              ) : showColors ? (
                <div
                  className="sheet__thumb"
                  style={bg ? { background: bg } : undefined}
                />
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
      </div>
    </div>
  );
}
