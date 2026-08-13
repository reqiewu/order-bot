import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Cell,
  Input,
  List,
  Section,
  Spinner,
  Text,
} from '@telegram-apps/telegram-ui';
import {
  addSlot,
  backdropGradient,
  deleteSlot,
  getBackdropInfo,
  getRuntime,
  getTokens,
  listBackdrops,
  listGifts,
  listModels,
  listSlots,
  modelPreviewURL,
  giftPreviewURL,
  normalizeMRKTPaste,
  normalizePortalsPaste,
  probeTokens,
  putRuntime,
  putTokens,
  type AttrRow,
  type BackdropInfo,
  type GiftRow,
  type Runtime,
  type TokenStatus,
  type WatchSlot,
} from './api';
import { AttrPicker, GiftPicker } from './PickerSheet';
import './App.css';

export function App() {
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [tokens, setTokens] = useState<TokenStatus | null>(null);
  const [runtime, setRuntime] = useState<Runtime | null>(null);
  const [slots, setSlots] = useState<WatchSlot[]>([]);
  const [catalogCredit, setCatalogCredit] = useState('@GiftChanges');

  const [mrkt, setMrkt] = useState('');
  const [portals, setPortals] = useState('');
  const [pollSec, setPollSec] = useState('90');
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

  const [tokenBusy, setTokenBusy] = useState(false);

  const reload = useCallback(async () => {
    setErr(null);
    const [t, r, s, g] = await Promise.all([
      getTokens(true),
      getRuntime(),
      listSlots(),
      listGifts(),
    ]);
    setTokens(t);
    setRuntime(r);
    setSlots(s.slots ?? []);
    setPollSec(String(r.poll_interval_sec));
    setMinProfit(String(r.min_profit_ton));
    setMinSpreadPct(String(r.min_spread_pct));
    setGifts(g.items ?? []);
    if (g.credit) setCatalogCredit(g.credit);
  }, []);

  useEffect(() => {
    reload()
      .catch((e: Error) => setErr(e.message))
      .finally(() => setLoading(false));
  }, [reload]);

  useEffect(() => {
    if (!collection) {
      setModels([]);
      setBackdrops([]);
      return;
    }
    let cancelled = false;
    setCatalogBusy(true);
    void (async () => {
      try {
        const m = await listModels(collection);
        if (cancelled) return;
        setModels(m.items ?? []);
        const b = await listBackdrops(collection, model);
        if (cancelled) return;
        setBackdrops(b.items ?? []);
      } catch (e) {
        if (!cancelled) setErr((e as Error).message);
      } finally {
        if (!cancelled) setCatalogBusy(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [collection, model]);

  useEffect(() => {
    if (!collection || !backdrop) {
      setBackdropInfo(null);
      return;
    }
    const fromList = backdrops.find((b) => b.name === backdrop);
    if (fromList?.center_color) {
      setBackdropInfo({
        name: fromList.name,
        center_color: fromList.center_color,
        edge_color: fromList.edge_color || fromList.center_color,
        pattern_color: fromList.pattern_color || '',
        text_color: fromList.text_color || '',
      });
      return;
    }
    let cancelled = false;
    void getBackdropInfo(collection, backdrop)
      .then((info) => {
        if (!cancelled) setBackdropInfo(info);
      })
      .catch(() => {
        if (!cancelled) setBackdropInfo(null);
      });
    return () => {
      cancelled = true;
    };
  }, [collection, backdrop, backdrops]);

  const previewUrl = useMemo(() => {
    if (!collection) return null;
    if (model) return modelPreviewURL(collection, model, 256);
    return giftPreviewURL(collection, 256);
  }, [collection, model]);

  const previewBg = useMemo(
    () => backdropGradient(backdropInfo?.center_color, backdropInfo?.edge_color),
    [backdropInfo],
  );

  const selectedBackdropColors = useMemo(() => {
    if (!backdrop) return undefined;
    if (backdropInfo) return backdropGradient(backdropInfo.center_color, backdropInfo.edge_color);
    const row = backdrops.find((b) => b.name === backdrop);
    return backdropGradient(row?.center_color, row?.edge_color);
  }, [backdrop, backdropInfo, backdrops]);

  async function saveTokens() {
    setErr(null);
    setTokenBusy(true);
    try {
      const body: { mrkt_token?: string; portals_tma?: string } = {};
      const m = normalizeMRKTPaste(mrkt);
      const p = normalizePortalsPaste(portals);
      if (m) body.mrkt_token = m;
      if (p) body.portals_tma = p;
      if (!body.mrkt_token && !body.portals_tma) {
        setErr('Вставь хотя бы один токен');
        return;
      }
      const t = await putTokens(body);
      setTokens(await getTokens(true));
      setMrkt('');
      setPortals('');
      if (t.mrkt_ok === false || t.portals_ok === false) {
        setErr('Сохранено, но проверка не прошла — см. статус');
      }
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setTokenBusy(false);
    }
  }

  async function checkTokens() {
    setErr(null);
    setTokenBusy(true);
    const prevMrktSet = tokens?.mrkt_set ?? false;
    const prevPortalsSet = tokens?.portals_set ?? false;
    try {
      const body: { mrkt_token?: string; portals_tma?: string } = {};
      const m = normalizeMRKTPaste(mrkt);
      const p = normalizePortalsPaste(portals);
      if (m) body.mrkt_token = m;
      if (p) body.portals_tma = p;
      const res = await probeTokens(Object.keys(body).length ? body : undefined);
      setTokens({
        mrkt_set: res.mrkt_set ?? prevMrktSet,
        portals_set: res.portals_set ?? prevPortalsSet,
        mrkt_ok: res.mrkt_ok,
        portals_ok: res.portals_ok,
        mrkt_error: res.mrkt_error,
        portals_error: res.portals_error,
      });
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setTokenBusy(false);
    }
  }

  function tokenSubtitle(
    set: boolean | undefined,
    ok: boolean | undefined,
    live?: boolean,
    stored?: boolean,
    error?: string,
  ): string {
    if (ok === true) return 'токен ок';
    if (ok === false) return error ? `ошибка: ${error}` : 'токен недействителен';
    if (live || stored || set) return 'не проверен';
    return 'не задан';
  }

  function TokenMark({ ok }: { ok?: boolean }) {
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

  async function saveRuntime() {
    setErr(null);
    try {
      await putRuntime({
        poll_interval_sec: Number(pollSec),
        min_profit_ton: Number(minProfit),
        min_spread_pct: Number(minSpreadPct),
      });
      await reload();
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
    } catch (e) {
      setErr((e as Error).message);
    }
  }

  async function onDelete(slot: WatchSlot) {
    setErr(null);
    try {
      const res = await deleteSlot(slot);
      setSlots(res.slots ?? []);
    } catch (e) {
      setErr((e as Error).message);
    }
  }

  if (loading) {
    return (
      <div style={{ padding: 24, display: 'flex', justifyContent: 'center' }}>
        <Spinner size="m" />
      </div>
    );
  }

  return (
    <List>
      <Section header="order-bot">
        <Cell subtitle="Paper снайп · Portals ↔ MRKT">
          <Text weight="2">Настройки</Text>
        </Cell>
        {err ? (
          <Cell>
            <Text style={{ color: 'var(--tgui--destructive_text_color)' }}>{err}</Text>
          </Cell>
        ) : null}
      </Section>

      <Section header="Токены" footer="MRKT: UUID access_token, не JWT. Portals: initData из Mini App.">
        <Cell
          after={<TokenMark ok={tokens?.mrkt_ok} />}
          subtitle={tokenSubtitle(
            tokens?.mrkt_set,
            tokens?.mrkt_ok,
            tokens?.mrkt_live,
            tokens?.mrkt_stored,
            tokens?.mrkt_error,
          )}
        >
          MRKT
        </Cell>
        <Input
          header="MRKT access_token"
          type="password"
          autoComplete="off"
          placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
          value={mrkt}
          onChange={(e) => setMrkt(e.target.value)}
          onPaste={(e) => {
            const text = e.clipboardData.getData('text');
            if (!text) return;
            e.preventDefault();
            setMrkt(normalizeMRKTPaste(text));
          }}
        />
        <Cell
          after={<TokenMark ok={tokens?.portals_ok} />}
          subtitle={tokenSubtitle(
            tokens?.portals_set,
            tokens?.portals_ok,
            tokens?.portals_live,
            tokens?.portals_stored,
            tokens?.portals_error,
          )}
        >
          Portals TMA
        </Cell>
        <Input
          header="Portals initData"
          type="password"
          autoComplete="off"
          placeholder="user=%7B%22id%22%3A…&auth_date=…&hash=…"
          value={portals}
          onChange={(e) => setPortals(e.target.value)}
          onPaste={(e) => {
            const text = e.clipboardData.getData('text');
            if (!text) return;
            e.preventDefault();
            setPortals(normalizePortalsPaste(text));
          }}
        />
        <div style={{ padding: '8px 16px 16px', display: 'flex', gap: 8, flexDirection: 'column' }}>
          <Button stretched size="l" disabled={tokenBusy} onClick={saveTokens}>
            {tokenBusy ? 'Проверяем…' : 'Сохранить токены'}
          </Button>
          <Button stretched size="l" mode="bezeled" disabled={tokenBusy} onClick={checkTokens}>
            Проверить сейчас
          </Button>
        </div>
      </Section>

      <Section header="Пороги">
        <Input
          header="Интервал опроса (сек)"
          type="number"
          value={pollSec}
          onChange={(e) => setPollSec(e.target.value)}
        />
        <Input
          header="Мин. профит (TON)"
          type="number"
          value={minProfit}
          onChange={(e) => setMinProfit(e.target.value)}
        />
        <Input
          header="Мин. спред (%)"
          type="number"
          value={minSpreadPct}
          onChange={(e) => setMinSpreadPct(e.target.value)}
        />
        <div style={{ padding: '8px 16px 16px' }}>
          <Button stretched size="l" mode="bezeled" onClick={saveRuntime}>
            Сохранить пороги
          </Button>
        </div>
        {runtime ? (
          <Cell subtitle={`сейчас: ${runtime.poll_interval_sec}s · ${runtime.min_profit_ton} TON · ${runtime.min_spread_pct}%`}>
            Активные
          </Cell>
        ) : null}
      </Section>

      <Section header="Watch slots" footer={`Каталог подарков: ${catalogCredit} · floor/vol: MRKT`}>
        {collection ? (
          <div className="preview-card" style={previewBg ? { background: previewBg } : undefined}>
            <button
              type="button"
              className="preview-card__x"
              onClick={() => {
                setCollection('');
                setModel('');
                setBackdrop('');
                setBackdropInfo(null);
              }}
            >
              ×
            </button>
            <img src={previewUrl ?? undefined} alt="" className="preview-card__img" />
            <a
              className="preview-card__credit"
              href="https://t.me/GiftChanges"
              target="_blank"
              rel="noreferrer"
            >
              Спасибо {catalogCredit} за API
            </a>
          </div>
        ) : null}

        <button type="button" className="pick-btn" onClick={() => setOpenGift(true)}>
          <span className="pick-btn__left">
            {collection ? (
              <img className="pick-btn__thumb" src={giftPreviewURL(collection)} alt="" />
            ) : (
              <span className="pick-btn__thumb pick-btn__thumb--empty" />
            )}
            <span>Подарок *</span>
          </span>
          <strong>{collection || 'Выбрать…'}</strong>
        </button>
        <button
          type="button"
          className="pick-btn"
          disabled={!collection || catalogBusy}
          onClick={() => setOpenModel(true)}
        >
          <span className="pick-btn__left">
            {collection && model ? (
              <img className="pick-btn__thumb" src={modelPreviewURL(collection, model, 128)} alt="" />
            ) : (
              <span className="pick-btn__thumb pick-btn__thumb--empty" />
            )}
            <span>Модель</span>
          </span>
          <strong>{model || 'Любая'}</strong>
        </button>
        <button
          type="button"
          className="pick-btn"
          disabled={!collection || catalogBusy}
          onClick={() => setOpenBackdrop(true)}
        >
          <span className="pick-btn__left">
            <span
              className={`pick-btn__thumb${selectedBackdropColors ? '' : ' pick-btn__thumb--empty'}`}
              style={selectedBackdropColors ? { background: selectedBackdropColors } : undefined}
            />
            <span>Фон</span>
          </span>
          <strong>{backdrop || 'Любой'}</strong>
        </button>

        <div style={{ padding: '8px 16px 16px' }}>
          <Button stretched size="l" disabled={!collection} onClick={onAddSlot}>
            Добавить слот
          </Button>
        </div>
        {slots.map((s) => {
          const collection = s.collection || '';
          const model = s.model || '';
          const backdrop = s.backdrop || '';
          const id = s.id || `${collection}|${model}|${backdrop}`;
          const label = [collection, model, backdrop].filter(Boolean).join(' · ') || id;
          const thumb = model
            ? modelPreviewURL(collection, model, 128)
            : giftPreviewURL(collection, 128);
          return (
            <Cell
              key={id}
              before={
                collection ? <img className="slot-thumb" src={thumb} alt="" /> : undefined
              }
              after={
                <Button size="s" mode="outline" onClick={() => onDelete(s)}>
                  Удалить
                </Button>
              }
            >
              {label}
            </Cell>
          );
        })}
        {slots.length === 0 ? (
          <Cell>
            <Text>Слотов пока нет</Text>
          </Cell>
        ) : null}
      </Section>

      <GiftPicker
        open={openGift}
        title="Подарки"
        items={gifts}
        selected={collection}
        onClose={() => setOpenGift(false)}
        onSelect={(name) => {
          setCollection(name);
          setModel('');
          setBackdrop('');
        }}
        onClear={() => {
          setCollection('');
          setModel('');
          setBackdrop('');
        }}
      />
      <AttrPicker
        open={openModel}
        title="Модели"
        items={models}
        selected={model}
        showPreview
        onClose={() => setOpenModel(false)}
        onSelect={(name) => {
          setModel(name);
          setBackdrop('');
        }}
        onClear={() => setModel('')}
      />
      <AttrPicker
        open={openBackdrop}
        title="Фоны"
        items={backdrops}
        selected={backdrop}
        showColors
        onClose={() => setOpenBackdrop(false)}
        onSelect={setBackdrop}
        onClear={() => setBackdrop('')}
      />
    </List>
  );
}
