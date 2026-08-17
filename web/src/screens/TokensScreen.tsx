import { useEffect, useState } from 'react';
import { PageHeader } from '../PageHeader';
import { formatAgo, marketStatus, marketTone, type TokenFields } from '../tokenStatus';

function StatusRow({
  name,
  ok,
  set,
  live,
  stored,
  session,
  checked,
  now,
}: {
  name: string;
  ok?: boolean;
  set?: boolean;
  live?: boolean;
  stored?: boolean;
  session?: boolean;
  checked?: number;
  now: number;
}) {
  const tone = marketTone(set, ok, live, stored, session);
  const label = marketStatus(set, ok, live, stored, session);
  return (
    <div className="status-row">
      <span className={`status-dot status-dot--${tone}`} aria-hidden />
      <div className="status-row__main">
        <div className="status-row__name">{name}</div>
      </div>
      <div className={`status-row__right status-row__right--${tone}`}>
        <span>{label}</span>
        {tone === 'ok' && checked ? <span className="status-row__ago">{formatAgo(checked, now)}</span> : null}
      </div>
    </div>
  );
}

export function TokensScreen(props: TokenFields) {
  const { tokens, checkedAt } = props;
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 5000);
    return () => window.clearInterval(id);
  }, []);

  const sessionDown = tokens?.tg_session === false;
  const problems = [
    tokens?.getgems_ok,
    tokens?.mrkt_ok,
    tokens?.portals_ok,
    tokens?.tonnel_ok,
  ].filter((ok) => ok === false).length;

  return (
    <div className="page">
      <PageHeader title="Статусы" subtitle="Состояние сессий" />

      {sessionDown || problems > 0 ? (
        <div className="status-banner" role="status" aria-atomic="true">
          <span className="status-dot status-dot--warn" aria-hidden />
          {sessionDown ? 'Обновите сессию' : `${problems} сессий с проблемами`}
        </div>
      ) : (
        <div className="status-banner status-banner--ok" role="status" aria-atomic="true">
          <span className="status-dot status-dot--ok" aria-hidden />
          Все сессии в порядке
        </div>
      )}

      <div className="block-label">Сессии</div>
      <div className="card">
        <StatusRow
          name="GetGems"
          ok={tokens?.getgems_ok}
          set={tokens?.getgems_set}
          live={tokens?.getgems_live}
          stored={tokens?.getgems_stored}
          session={tokens?.tg_session}
          checked={checkedAt.getgems}
          now={now}
        />
        <StatusRow
          name="MRKT"
          ok={tokens?.mrkt_ok}
          set={tokens?.mrkt_set}
          live={tokens?.mrkt_live}
          stored={tokens?.mrkt_stored}
          session={tokens?.tg_session}
          checked={checkedAt.mrkt}
          now={now}
        />
        <StatusRow
          name="Portals"
          ok={tokens?.portals_ok}
          set={tokens?.portals_set}
          live={tokens?.portals_live}
          stored={tokens?.portals_stored}
          session={tokens?.tg_session}
          checked={checkedAt.portals}
          now={now}
        />
        <StatusRow
          name="Tonnel"
          ok={tokens?.tonnel_ok}
          set={tokens?.tonnel_set}
          live={tokens?.tonnel_live}
          stored={tokens?.tonnel_stored}
          session={tokens?.tg_session}
          checked={checkedAt.tonnel}
          now={now}
        />
      </div>
    </div>
  );
}
