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

Дальше в личке бота:

1. `/start` → Mini App
2. Вставить токены **MRKT / Portals / Getgems / Tonnel** (пустое имя в `.env` = рынок выкл)
3. Добавить watch-слоты (коллекция обязательна)
4. Для кнопки Mini App в `/start`: `MINIAPP_PUBLIC_URL=https://…` (туннель)

Стоп: `make stop`. Снести контейнер + volume: `make clean` (`.env` не трогает).

---

## Маркеты

| Маркет | Asks | Comps (sales) | Откуда креды |
| :---: | :---: | :---: | :---: |
| **MRKT** | да | да | Mini App / `MRKT_TOKEN` |
| **Portals** | да | да | Mini App / `PORTALS_TOKEN` |
| **Getgems** | да | да | Mini App / `GETGEMS_TOKEN` ([Read API](https://api.getgems.io/public-api/docs)) |
| **Tonnel** | да | да | Mini App / `TONNEL_TOKEN` |
| **Telegram** | да (только TON-лоты) | пока нет | MTProto user session |

### Telegram Gift Marketplace

In-app resale (`payments.getResaleStarGifts`). Paper **buy-кандидат** с дешёвым TON-ask → выход на другие маркеты. Stars-only лоты пропускаются. История продаж — позже.

1. `TELEGRAM_API_ID` + `TELEGRAM_API_HASH` с [my.telegram.org/apps](https://my.telegram.org/apps)
2. Логин (интерактивно): `go run ./cmd/tg-login` → `data/tg.session`
3. `make start` монтирует `./data` → `/session` (`TELEGRAM_SESSION_PATH` в compose)

Выключить: `TELEGRAM_USER_DISABLED=1`.

Пустой токен = рынок выключен (нет poll и sales). Старые имена `PORTALS_TMA` / `GETGEMS_API_KEY` / `TONNEL_INITDATA` ещё читаются.

Tonnel и asks, и comps — только с `TONNEL_TOKEN`. Запросы с **Chrome TLS**. Если Cloudflare всё ещё 403 — это IP (VPS/Docker), не ридер.

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
