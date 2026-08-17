import { useEffect, useState } from 'react';
import { backdropGradient, giftPreviewURL, modelPreviewURL, type WatchSlot } from '../api';
import { cachedBackdrop, loadBackdrop } from '../backdropCache';
import { PageHeader } from '../PageHeader';

type Props = {
  slots: WatchSlot[];
  onDelete: (s: WatchSlot) => void;
  onCreate: () => void;
};

function slotCountLabel(n: number): string {
  const n10 = n % 10;
  const n100 = n % 100;
  if (n10 === 1 && n100 !== 11) return `${n} подписка`;
  if (n10 >= 2 && n10 <= 4 && (n100 < 12 || n100 > 14)) return `${n} подписки`;
  return `${n} подписок`;
}

export function SubscriptionsScreen({ slots, onDelete, onCreate }: Props) {
  const n = slots.length;
  const [, setPaint] = useState(0);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      for (const s of slots) {
        if (!s.collection || !s.backdrop) continue;
        await loadBackdrop(s.collection, s.backdrop);
        if (cancelled) return;
        setPaint((x) => x + 1);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [slots]);

  return (
    <div className="page">
      <PageHeader title="Подписки" subtitle={n > 0 ? slotCountLabel(n) : undefined} />

      {n === 0 ? (
        <div className="empty">
          <div className="empty__title">Пока пусто</div>
          <p className="empty__hint">Следите за подарком, моделью или фоном — бот будет ловить листинги на рынках.</p>
          <button type="button" className="primary-btn" onClick={onCreate}>
            Создать подписку
          </button>
        </div>
      ) : (
        <ul className="sub-list" aria-label="Подписки">
          {slots.map((s, i) => {
            const coll = s.collection || '';
            const mod = s.model || '';
            const bg = s.backdrop || '';
            const id = s.id || `${coll}|${mod}|${bg}`;
            const png = mod ? modelPreviewURL(coll, mod, 128) : giftPreviewURL(coll, 128);
            const info = coll && bg ? cachedBackdrop(coll, bg) : undefined;
            const fill = backdropGradient(info?.center_color, info?.edge_color);
            return (
              <li className="status-row" key={id} style={{ ['--i' as string]: i }}>
                {coll ? (
                  <div className="tgs-thumb" style={fill ? { background: fill } : undefined}>
                    <img src={png} alt="" />
                  </div>
                ) : null}
                <div className="status-row__main">
                  <div className="status-row__name">{coll || id}</div>
                  <div className="meta-tags">
                    <span className="tag">{mod || 'все модели'}</span>
                    <span className="tag">{bg || 'все фоны'}</span>
                  </div>
                </div>
                <button
                  type="button"
                  className="row-action"
                  aria-label={`Удалить подписку ${coll || id}`}
                  onClick={() => onDelete(s)}
                >
                  Удалить
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
