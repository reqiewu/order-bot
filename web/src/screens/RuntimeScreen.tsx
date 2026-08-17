import { PageHeader } from '../PageHeader';

type Props = {
  pollSec: string;
  minProfit: string;
  minSpreadPct: string;
  setPollSec: (v: string) => void;
  setMinProfit: (v: string) => void;
  setMinSpreadPct: (v: string) => void;
  onSave: () => void;
};

export function RuntimeScreen({
  pollSec,
  minProfit,
  minSpreadPct,
  setPollSec,
  setMinProfit,
  setMinSpreadPct,
  onSave,
}: Props) {
  return (
    <div className="page page--settings">
      <PageHeader title="Настройки" subtitle="Пороги алертов" />

      <div className="set-list">
        <div className="set-row">
          <label className="set-row__label" htmlFor="rt-poll">
            Интервал опроса
          </label>
          <p className="set-row__hint">Как часто бот смотрит рынки.</p>
          <div className="set-row__control">
            <input
              id="rt-poll"
              className="field-input"
              type="number"
              inputMode="decimal"
              value={pollSec}
              onChange={(e) => setPollSec(e.target.value)}
            />
            <span className="set-row__unit">сек</span>
          </div>
        </div>

        <div className="set-row">
          <label className="set-row__label" htmlFor="rt-profit">
            Мин. профит
          </label>
          <p className="set-row__hint">Разница в TON после комиссий.</p>
          <div className="set-row__control">
            <input
              id="rt-profit"
              className="field-input"
              type="number"
              inputMode="decimal"
              value={minProfit}
              onChange={(e) => setMinProfit(e.target.value)}
            />
            <span className="set-row__unit">TON</span>
          </div>
        </div>

        <div className="set-row">
          <label className="set-row__label" htmlFor="rt-spread">
            Мин. спред
          </label>
          <p className="set-row__hint">Минимальная разница цены в процентах.</p>
          <div className="set-row__control">
            <input
              id="rt-spread"
              className="field-input"
              type="number"
              inputMode="decimal"
              value={minSpreadPct}
              onChange={(e) => setMinSpreadPct(e.target.value)}
            />
            <span className="set-row__unit">%</span>
          </div>
        </div>
      </div>

      <div className="create-actions">
        <button type="button" className="primary-btn create-btn" onClick={onSave}>
          Сохранить
        </button>
      </div>
    </div>
  );
}
