import React from 'react';
import ReactDOM from 'react-dom/client';
import { AppRoot } from '@telegram-apps/telegram-ui';
import '@telegram-apps/telegram-ui/dist/styles.css';
import { App } from './App';

window.Telegram?.WebApp?.ready();
window.Telegram?.WebApp?.expand();

function syncViewport() {
  const h = window.Telegram?.WebApp?.viewportStableHeight || window.innerHeight;
  document.documentElement.style.setProperty('--app-h', `${h}px`);
}
syncViewport();
window.Telegram?.WebApp?.onEvent?.('viewportChanged', syncViewport);
window.addEventListener('resize', syncViewport);

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AppRoot>
      <App />
    </AppRoot>
  </React.StrictMode>,
);
