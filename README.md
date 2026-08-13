# order-bot

Личный Telegram-бот для кросс-маркет paper-снайпа Telegram Gifts (Portals ↔ MRKT).

Домен: [`CONTEXT.md`](CONTEXT.md). Шаблоны: sibling [`gift-bot`](../gift-bot).

## Статус

Скелет paper-MVP:

- `internal/spread` — NetProfit / trigger / чистка sales (тесты)
- `internal/market` — MRKT + Portals (из gift-bot), полный List до конца пагинации (cap 500 стр.)
- `internal/engine` + `internal/ingress` — горутины → канал → Engine
- `internal/store` — Bbolt whitelist
- `cmd/order-bot` — один процесс, алерты в лог или Telegram

Mini App / HTTPS — следующим этапом (токены пока из `.env`).

## Make (как в gift-bot)

```bash
make setup    # .env из .env.example
make test     # go test ./...
make run      # локально без Docker
make up       # Docker compose up -d --build
make logs
make down
make restart
make clean      # down + локальный образ
make clean-data # down + volume Bbolt
```

Заполни в `.env`: `MRKT_TOKEN`, `PORTALS_TMA`, опционально `TELEGRAM_*` и `WATCH_SLOTS_JSON`.

```bash
WATCH_SLOTS_JSON='[{"collection":"Lunar Snake","model":"Albino"}]'
```
