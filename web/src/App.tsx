import { useCallback, useEffect, useState } from 'react';
import { Text } from '@telegram-apps/telegram-ui';
import { IconGear, IconList, IconPlus, IconStatus } from './icons';
import {
  addSlot,
  deleteSlot,
  getBackdropInfo,
  getGift,
  getRuntime,
  getTokens,
  listAllBackdrops,
  listGifts,
  listSlots,
  putRuntime,
  type AttrRow,
  type BackdropInfo,
  type GiftRow,
  type TokenStatus,
  type WatchSlot,
} from './api';
import { CreateScreen } from './screens/CreateScreen';
import { RuntimeScreen } from './screens/RuntimeScreen';
import { SubscriptionsScreen } from './screens/SubscriptionsScreen';
import { TokensScreen } from './screens/TokensScreen';
import { rememberAll, rememberBackdrop } from './backdropCache';
import './App.css';

type TabId = 'create' | 'subs' | 'status' | 'settings';
type TokenField = 'mrkt' | 'portals' | 'getgems' | 'tonnel';
type ToastTone = 'ok' | 'remove';
type ToastState = { text: string; tone: ToastTone };

const CHECKED_KEY = 'order-bot-token-checked';

function loadChecked(): Partial<Record<TokenField, number>> {
  try {
    const raw = localStorage.getItem(CHECKED_KEY);
    return raw ? (JSON.parse(raw) as Partial<Record<TokenField, number>>) : {};
  } catch {
    return {};
  }
}

function stampChecked(prev: Partial<Record<TokenField, number>>): Partial<Record<TokenField, number>> {
  const now = Date.now();
  const next = { ...prev, mrkt: now, portals: now, getgems: now, tonnel: now };
  localStorage.setItem(CHECKED_KEY, JSON.stringify(next));
  return next;
}

export function App() {
  const [tab, setTab] = useState<TabId>('create');
  const [toast, setToast] = useState<ToastState | null>(null);
  const [toastKey, setToastKey] = useState(0);
  const [err, setErr] = useState<string | null>(null);
  const [tokens, setTokens] = useState<TokenStatus | null>(null);
  const [checkedAt, setCheckedAt] = useState<Partial<Record<TokenField, number>>>(loadChecked);
  const [slots, setSlots] = useState<WatchSlot[]>([]);
  const [catalogCredit, setCatalogCredit] = useState('@GiftChanges');

  const [pollSec, setPollSec] = useState('1');
  const [minProfit, setMinProfit] = useState('0.1');
  const [minSpreadPct, setMinSpreadPct] = useState('5');

  const [gifts, setGifts] = useState<GiftRow[]>([]);
  const [models, setModels] = useState<AttrRow[]>([]);
  const [backdrops, setBackdrops] = useState<AttrRow[]>([]);
  const [collection, setCollection] = useState('');
  const [model, setModel] = useState('');
  const [backdrop, setBackdrop] = useState('');
  const [catalogBusy, setCatalogBusy] = useState(false);

  const [openGift, setOpenGift] = useState(false);
  const [openModel, setOpenModel] = useState(false);
  const [openBackdrop, setOpenBackdrop] = useState(false);
  const [backdropInfo, setBackdropInfo] = useState<BackdropInfo | null>(null);

  const reload = useCallback(async () => {
    setErr(null);
    const [t, r, s, g, backs] = await Promise.all([
      getTokens(true),
      getRuntime(),
      listSlots(),
      listGifts(),
      listAllBackdrops().catch(() => ({ items: [] as BackdropInfo[] })),
    ]);
    setTokens(t);
    setCheckedAt(stampChecked(loadChecked()));
    setSlots(s.slots ?? []);
    setPollSec(String(r.poll_interval_sec));
    setMinProfit(String(r.min_profit_ton));
    setMinSpreadPct(String(r.min_spread_pct));
    setGifts(g.items ?? []);
    if (g.credit) setCatalogCredit(g.credit);
    rememberAll(backs.items ?? []);
  }, []);

  useEffect(() => {
    reload().catch((e: Error) => setErr(e.message));
  }, [reload]);

  useEffect(() => {
    const probe = () =>
      getTokens(true)
        .then((t) => {
          setTokens(t);
          setCheckedAt(stampChecked(loadChecked()));
        })
        .catch(() => {
          /* keep last known status */
        });
    const id = window.setInterval(() => {
      void probe();
    }, 45_000);
    return () => window.clearInterval(id);
  }, []);

  useEffect(() => {
    if (!toast) return;
    const t = window.setTimeout(() => setToast(null), 1400);
    return () => window.clearTimeout(t);
  }, [toast, toastKey]);

  useEffect(() => {
    if (!collection) {
      setModels([]);
      setBackdrops([]);
      setCatalogBusy(false);
      return;
    }
    let cancelled = false;
    setCatalogBusy(true);
    void getGift(collection)
      .then((g) => {
        if (cancelled) return;
        setModels(g.models ?? []);
        setBackdrops(g.backdrops ?? []);
      })
      .catch((e: Error) => {
        if (!cancelled) setErr(e.message);
      })
      .finally(() => {
        if (!cancelled) setCatalogBusy(false);
      });
    return () => {
      cancelled = true;
    };
  }, [collection]);

  useEffect(() => {
    if (!collection || !backdrop) {
      setBackdropInfo(null);
      return;
    }
    const fromList = backdrops.find((b) => b.name === backdrop);
    if (fromList?.center_color) {
      const info = {
        name: fromList.name,
        center_color: fromList.center_color,
        edge_color: fromList.edge_color || fromList.center_color,
        pattern_color: fromList.pattern_color || '',
        text_color: fromList.text_color || '',
      };
      rememberBackdrop(collection, info);
      setBackdropInfo(info);
      return;
    }
    let cancelled = false;
    void getBackdropInfo(collection, backdrop)
      .then((info) => {
        if (!cancelled) {
          rememberBackdrop(collection, info);
          setBackdropInfo(info);
        }
      })
      .catch(() => {
        if (!cancelled) setBackdropInfo(null);
      });
    return () => {
      cancelled = true;
    };
  }, [collection, backdrop, backdrops]);

  async function saveRuntime() {
    setErr(null);
    try {
      await putRuntime({
        poll_interval_sec: Number(pollSec),
        min_profit_ton: Number(minProfit),
        min_spread_pct: Number(minSpreadPct),
      });
      setToast({ text: 'Настройки сохранены', tone: 'ok' });
      setToastKey((k) => k + 1);
    } catch (e) {
      setErr((e as Error).message);
    }
  }

  async function onAddSlot() {
    setErr(null);
    try {
      const res = await addSlot({
        collection: collection.trim(),
        model: model.trim() || undefined,
        backdrop: backdrop.trim() || undefined,
      });
      setSlots(res.slots ?? []);
      setCollection('');
      setModel('');
      setBackdrop('');
      setToast({ text: 'Подписка создана', tone: 'ok' });
      setToastKey((k) => k + 1);
    } catch (e) {
      setErr((e as Error).message);
    }
  }

  async function onDelete(slot: WatchSlot) {
    setErr(null);
    try {
      const res = await deleteSlot(slot);
      setSlots(res.slots ?? []);
      setToast({ text: 'Подписка удалена', tone: 'remove' });
      setToastKey((k) => k + 1);
    } catch (e) {
      setErr((e as Error).message);
    }
  }

  return (
    <div className="app-shell">
      {err ? (
        <div className="app-err">
          <Text style={{ color: 'var(--tgui--destructive_text_color)' }}>{err}</Text>
        </div>
      ) : null}

      <div className="app-shell__body">
        {tab === 'create' ? (
          <CreateScreen
            catalogCredit={catalogCredit}
            gifts={gifts}
            models={models}
            backdrops={backdrops}
            collection={collection}
            model={model}
            backdrop={backdrop}
            catalogBusy={catalogBusy}
            openGift={openGift}
            openModel={openModel}
            openBackdrop={openBackdrop}
            backdropInfo={backdropInfo}
            setCollection={setCollection}
            setModel={setModel}
            setBackdrop={setBackdrop}
            setBackdropInfo={setBackdropInfo}
            setOpenGift={setOpenGift}
            setOpenModel={setOpenModel}
            setOpenBackdrop={setOpenBackdrop}
            onAdd={onAddSlot}
          />
        ) : null}
        {tab === 'subs' ? (
          <SubscriptionsScreen
            slots={slots}
            onDelete={onDelete}
            onCreate={() => {
              setToast(null);
              setTab('create');
            }}
          />
        ) : null}
        {tab === 'status' ? (
          <TokensScreen tokens={tokens} checkedAt={checkedAt} />
        ) : null}
        {tab === 'settings' ? (
          <RuntimeScreen
            pollSec={pollSec}
            minProfit={minProfit}
            minSpreadPct={minSpreadPct}
            setPollSec={setPollSec}
            setMinProfit={setMinProfit}
            setMinSpreadPct={setMinSpreadPct}
            onSave={saveRuntime}
          />
        ) : null}
      </div>

      {toast ? (
        <div
          key={toastKey}
          className={`toast toast--${toast.tone}`}
          role="status"
          onClick={() => setToast(null)}
        >
          {toast.text}
        </div>
      ) : null}

      <nav className="dock" aria-label="Разделы">
        <button
          type="button"
          className={tab === 'create' ? 'is-on' : ''}
          aria-current={tab === 'create' ? 'page' : undefined}
          onClick={() => {
            setToast(null);
            setTab('create');
          }}
        >
          <span className="dock__icon">
            <IconPlus />
          </span>
          Создать
        </button>
        <button
          type="button"
          className={tab === 'subs' ? 'is-on' : ''}
          aria-current={tab === 'subs' ? 'page' : undefined}
          onClick={() => {
            setToast(null);
            setTab('subs');
          }}
        >
          <span className="dock__icon">
            <IconList />
          </span>
          Подписки
        </button>
        <button
          type="button"
          className={tab === 'status' ? 'is-on' : ''}
          aria-current={tab === 'status' ? 'page' : undefined}
          onClick={() => {
            setToast(null);
            setTab('status');
          }}
        >
          <span className="dock__icon">
            <IconStatus />
          </span>
          Статусы
        </button>
        <button
          type="button"
          className={tab === 'settings' ? 'is-on' : ''}
          aria-current={tab === 'settings' ? 'page' : undefined}
          onClick={() => {
            setToast(null);
            setTab('settings');
          }}
        >
          <span className="dock__icon">
            <IconGear />
          </span>
          Настройки
        </button>
      </nav>
    </div>
  );
}
