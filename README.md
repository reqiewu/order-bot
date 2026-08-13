# order-bot

Личный Telegram-бот для кросс-маркет paper-снайпа Telegram Gifts (Portals ↔ MRKT ↔ Getgems).

## Make (Docker)

```bash
cp .env.example .env   # один раз
make start             # up -d --build
make logs
make stop
make clean             # stop + образ + volume
```

В `.env`: `TELEGRAM_BOT_TOKEN`, `OPERATOR_TELEGRAM_ID`, `TOKEN_ENCRYPTION_KEY`.  
Токены MRKT/Portals и watch-слоты — в **Mini App**.  
Getgems: `GETGEMS_API_KEY` (Read API, без Mini App).  
Ёмкость слотов (нагрузочный прогон): `docs/listing-capacity.canvas.tsx`.  
Для кнопки Mini App в `/start`: `MINIAPP_PUBLIC_URL=https://…` (туннель).

Локальная отладка API без Telegram (в `.env`):

```
MINIAPP_DEV=1
MINIAPP_DEV_USER_ID=<твой id>
```
