import { useMemo } from 'react';
import {
  backdropGradient,
  giftPreviewURL,
  giftTgsURL,
  modelPreviewURL,
  modelTgsURL,
  type AttrRow,
  type BackdropInfo,
  type GiftRow,
} from '../api';
import { AttrPicker, GiftPicker } from '../PickerSheet';
import { PageHeader } from '../PageHeader';
import { TgsPlayer } from '../TgsPlayer';
import { IconSliders } from '../icons';

type Props = {
  catalogCredit: string;
  gifts: GiftRow[];
  models: AttrRow[];
  backdrops: AttrRow[];
  collection: string;
  model: string;
  backdrop: string;
  catalogBusy: boolean;
  openGift: boolean;
  openModel: boolean;
  openBackdrop: boolean;
  backdropInfo: BackdropInfo | null;
  setCollection: (v: string) => void;
  setModel: (v: string) => void;
  setBackdrop: (v: string) => void;
  setBackdropInfo: (v: BackdropInfo | null) => void;
  setOpenGift: (v: boolean) => void;
  setOpenModel: (v: boolean) => void;
  setOpenBackdrop: (v: boolean) => void;
  onAdd: () => void;
};

export function CreateScreen(props: Props) {
  const {
    catalogCredit,
    gifts,
    models,
    backdrops,
    collection,
    model,
    backdrop,
    catalogBusy,
    openGift,
    openModel,
    openBackdrop,
    backdropInfo,
    setCollection,
    setModel,
    setBackdrop,
    setBackdropInfo,
    setOpenGift,
    setOpenModel,
    setOpenBackdrop,
    onAdd,
  } = props;

  const previewTgs = useMemo(() => {
    if (!collection) return null;
    if (model) return modelTgsURL(collection, model);
    return giftTgsURL(collection);
  }, [collection, model]);

  const previewPng = useMemo(() => {
    if (!collection) return null;
    if (model) return modelPreviewURL(collection, model, 512);
    return giftPreviewURL(collection, 512);
  }, [collection, model]);

  const previewBg = useMemo(() => {
    if (backdropInfo) return backdropGradient(backdropInfo.center_color, backdropInfo.edge_color);
    return undefined;
  }, [backdropInfo]);

  function clearAll() {
    setCollection('');
    setModel('');
    setBackdrop('');
    setBackdropInfo(null);
  }

  return (
    <>
      <div className="page page--tall">
        <PageHeader
          title="Подписка"
          action={
            collection ? (
              <button type="button" className="ghost-btn" onClick={clearAll}>
                Сбросить
              </button>
            ) : null
          }
        />

        <div className="hero" style={previewBg ? { background: previewBg } : undefined}>
          {collection && previewTgs ? (
            <TgsPlayer src={previewTgs} fallbackSrc={previewPng ?? undefined} className="hero__tgs" />
          ) : (
            <div className="hero__empty">
              <div className="hero__icon" aria-hidden>
                <IconSliders />
              </div>
              <div className="hero__title">Задайте фильтры ниже</div>
              <div className="hero__hint">Выберите подарок, модель или фон — превью появится здесь.</div>
            </div>
          )}
          <div className="hero__credit">
            Спасибо{' '}
            <a href="https://t.me/GiftChanges" target="_blank" rel="noreferrer">
              {catalogCredit}
            </a>{' '}
            за API
          </div>
        </div>

        <div className="filter-grid">
          <label className="filter-field">
            <span className="filter-field__label">Подарок</span>
            <button type="button" className="filter-btn" onClick={() => setOpenGift(true)}>
              {collection || 'Все подарки'}
            </button>
          </label>
          <label className="filter-field">
            <span className="filter-field__label">Модель</span>
            <button
              type="button"
              className="filter-btn"
              disabled={!collection}
              onClick={() => setOpenModel(true)}
            >
              {model || (catalogBusy ? 'Загрузка…' : collection ? 'Все модели' : 'Сначала выберите')}
            </button>
          </label>
          <label className="filter-field">
            <span className="filter-field__label">Фон</span>
            <button
              type="button"
              className="filter-btn"
              disabled={!collection}
              onClick={() => setOpenBackdrop(true)}
            >
              {backdrop || (catalogBusy ? 'Загрузка…' : 'Все фоны')}
            </button>
          </label>
        </div>
        <div className="create-actions">
          <button type="button" className="primary-btn create-btn" disabled={!collection} onClick={onAdd}>
            Создать подписку
          </button>
        </div>
      </div>

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
        gift={collection}
        showPreview
        onClose={() => setOpenModel(false)}
        onSelect={(name) => {
          setModel(name);
          setBackdrop('');
        }}
        onClear={() => setModel('')}
        busy={catalogBusy}
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
        busy={catalogBusy}
      />
    </>
  );
}
