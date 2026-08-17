# order-bot

[![CI](https://github.com/reqiewu/order-bot/actions/workflows/ci.yml/badge.svg)](https://github.com/reqiewu/order-bot/actions/workflows/ci.yml)
![coverage](.github/badges/coverage.svg)

Личный paper-снайпер **Telegram Gifts**: считает кросс-маркет спред и пишет в личку «купил бы / продал бы». Сам ничего не покупает.

**Portals** · **MRKT** · **Getgems** · **Tonnel** · **Telegram** (Gift Marketplace)

| | |
| :---: | :---: |
| Кто | один оператор |
| Режим | Paper (Confirm / Auto — позже) |
| Матч | коллекция + model + backdrop |
| Триггер | только кросс-маркет; same-market не алертит |
| Poll | ~1с; asks = топ **100** самых дешёвых по слоту |

```text
watch-слоты ──▶ 5 маркетов (poll ~1с) ──▶ топ-100 asks
                                      │
                                      ▼
                         cheap buy vs highest other ask
                                      │
                         + медиана sales на выходе
                                      ▼
                              Telegram paper-алерт
```

---

## Быстрый старт

```bash
cp .env.example .env   # заполни секреты
make start             # Docker: build + up -d
make logs
```

В `.env` минимум:

| Переменная | Зачем |
| :---: | :---: |
| `TELEGRAM_BOT_TOKEN` | бот |
| `OPERATOR_TELEGRAM_ID` | твой numeric id |
| `TOKEN_ENCRYPTION_KEY` | AES для токенов в Bbolt |
| `TELEGRAM_API_ID` / `TELEGRAM_API_HASH` | user-сессия для маркетов |

Дальше:

1. `go run ./cmd/tg-login` → `data/tg.session`
2. `/start` → Mini App
3. Добавить watch-слоты (коллекция обязательна)
4. Для кнопки Mini App в `/start`: `MINIAPP_PUBLIC_URL=https://…` (туннель)

Токены **Getgems / MRKT / Portals / Tonnel** берутся из Telegram-сессии автоматически. Если сессия умерла — `go run ./cmd/tg-login` снова.

Стоп: `make stop`. Снести контейнер + volume: `make clean` (`.env` не трогает).

---

## Маркеты

| Маркет | Asks | Comps (sales) | Откуда креды |
| :---: | :---: | :---: | :---: |
| **Getgems** | да | да | MTProto → `@GetgemsNftBot` |
| **MRKT** | да | да | MTProto → `@mrkt/app` |
| **Portals** | да | да | MTProto → `@portals_market_bot/market` |
| **Tonnel** | да | да | MTProto → `@tonnel_network_bot/gift` |
| **Telegram** | да (только TON-лоты) | пока нет | та же user-сессия |

### Telegram user-сессия

Одна MTProto-сессия даёт Gift Marketplace asks и токены внешних маркетов.

1. `TELEGRAM_API_ID` + `TELEGRAM_API_HASH` с [my.telegram.org/apps](https://my.telegram.org/apps)
2. Логин: `go run ./cmd/tg-login` → `data/tg.session`
3. `make start` монтирует `./data` → `/session`

Если сессия умерла — снова `tg-login`. Env-токены маркетов больше не читаются.

Tonnel ходит с **Chrome TLS**. Если Cloudflare 403 — это IP (VPS/Docker), не ридер.

---

## Make

| Команда | Что делает |
| :---: | :---: |
| `make start` | собрать и запустить |
| `make pull` | скачать образ из GHCR (`IMAGE=…`) |
| `make logs` | логи |
| `make stop` | остановить |
| `make clean` | stop + образ + volume |

Mini App слушает `MINIAPP_PORT` (по умолчанию 8080).

---

## CI / CD

На каждый push / PR в `main`:

| Job | Что делает |
| :---: | :---: |
| **test** | `go vet` + `go test` + обновляет coverage-бейдж |
| **docker** | собирает образ; на `main` пушит в **GHCR** |

Образы:

```text
ghcr.io/reqiewu/order-bot:latest
ghcr.io/reqiewu/order-bot:<short-sha>
```

На VPS (репо private → нужен `read:packages` PAT):

```bash
echo "$GHCR_TOKEN" | docker login ghcr.io -u reqiewu --password-stdin
IMAGE=ghcr.io/reqiewu/order-bot:latest make pull start
```

SSH-деплой из Actions пока не подключён — скажи хост, добавим.

> Если в ветке включены **required signed commits**, автокоммит coverage-бейджа упадёт: либо исключи `.github/badges/` из правила, либо скажи — перенесём бейдж на artifact.

---

## Алерты

В чат только полный сигнал: net после fee проходит и по **лучшему ask** на выходе, и по **медиане sales**. Слабые / no_sales в чат не идут.

Кнопки ведут на лот buy-маркета и на ask sell-маркета. TON/USDT в тексте — справочный курс CMC Gram, на триггер не влияет.

Ёмкость слотов (сколько коллекций/лотов тянет poll): [`docs/listing-capacity.canvas.tsx`](docs/listing-capacity.canvas.tsx).

---

## Mini App без Telegram

Локальная отладка API (в `.env`):

```bash
MINIAPP_DEV=1
MINIAPP_DEV_USER_ID=<твой id>
```
