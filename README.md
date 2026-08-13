# order-bot

[![CI](https://github.com/reqiewu/order-bot/actions/workflows/ci.yml/badge.svg)](https://github.com/reqiewu/order-bot/actions/workflows/ci.yml)
![coverage](.github/badges/coverage.svg)

Личный paper-снайпер **Telegram Gifts**: считает кросс-маркет спред и пишет в личку «купил бы / продал бы». Сам ничего не покупает.

**Portals** · **MRKT** · **Getgems** · **Tonnel**

| | |
| :---: | :---: |
| Кто | один оператор |
| Режим | Paper (Confirm / Auto — позже) |
| Матч | коллекция + model + backdrop |
| Триггер | только кросс-маркет; same-market не алертит |

```text
watch-слоты ──▶ 4 маркета (poll) ──▶ книги asks
                                      │
                                      ▼
                         cheap buy vs highest other ask
                                      │
                         + медиана sales на выходе
                                      ▼
                              Telegram paper-алерт
                         [ Купить MRKT ] [ Купить Tonnel ]
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
2. Вставить **MRKT** / **Portals** токены
3. Добавить watch-слоты (коллекция обязательна)
4. Для кнопки Mini App в `/start`: `MINIAPP_PUBLIC_URL=https://…` (туннель)

Стоп: `make stop`. Снести контейнер + volume: `make clean` (`.env` не трогает).

---

## Маркеты

| Маркет | Asks | Comps (sales) | Откуда креды |
| :---: | :---: | :---: | :---: |
| **MRKT** | да | да | Mini App / `MRKT_TOKEN` |
| **Portals** | да | да | Mini App / `PORTALS_TMA` |
| **Getgems** | да | да | `GETGEMS_API_KEY` ([Read API](https://api.getgems.io/public-api/docs)) |
| **Tonnel** | да, без логина | `TONNEL_INITDATA` | `.env`; выключить: `TONNEL_DISABLED=1` |

Tonnel ходит с **Chrome TLS** (не обычный Go `net/http`). Если Cloudflare всё ещё 403 — это IP (VPS/Docker), не ридер.

---

## Make

| Команда | Что делает |
| :---: | :---: |
| `make start` | собрать и запустить |
| `make pull` | скачать образ из GHCR (`IMAGE=…`) |
| `make logs` | логи |
| `make stop` | остановить |
| `make clean` | stop + образ + volume |

Mini App слушает `:8080` (`MINIAPP_PORT`).

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
