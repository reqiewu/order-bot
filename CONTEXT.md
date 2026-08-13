# order-bot

Личный Telegram-бот для **кросс-маркет снайпа** Telegram Gifts (NFT/TON): paper-алерты «купил бы / продал бы», затем semi-auto → full auto. Не мультипользовательский продукт. Парсеры и паттерны — из sibling **`gift-bot`** (`../gift-bot`); секреты не копировать.

## Language — Product

**OrderBot**:
Один оператор (`telegram_user_id` allowlist). Режим исполнения: **Paper → Confirm → Auto** (C→B→A). v1 = **Paper** only.
_Avoid_: публичный SaaS; full auto в первом релизе; seed/lite-client до этапа B/A

**Paper**:
Считает спред и шлёт алерт без покупки: buy-маркет + цена, sell-маркет + цена (ask / undercut), net после fee.
_Avoid_: слабые / `no_sales` / «почти спред» алерты в чат

**Confirm / Auto** (later):
Confirm — алерт + OK перед buy. Auto — без OK. Продажа в v1-стратегии всё равно ручная по подсказке.
_Avoid_: автолистинг продажи в paper-MVP

**Signal (v1)**:
Только **кросс-маркет** Portals ↔ MRKT. Same-market / отскок к floor — не buy-триггер.
_Avoid_: Fragment, Telegram Stars, Getgems, Tonnel в v1 (позже как quotes/buy)

## Language — Match & markets

**MODEL_BG**:
Ключ матча: **коллекция + model + backdrop**. Pattern (узор) в v1 не в ключе.
_Avoid_: матч только по `#`; матч только по коллекции для P&L

**COLL_FLOOR**:
Мин. ask коллекции — **бейдж** в алерте, не цена выхода и не buy-триггер.
_Avoid_: NetProfit от floor коллекции

**Buy venues (v1)**:
Portals + MRKT (исполнение позже; в paper — кандидаты на покупку).

**Quote venues (v1)**:
Те же Portals + MRKT. Getgems / Tonnel — после ридеров; fee-модель уже заложена как у Portals/MRKT.

**WatchSlot (whitelist)**:
Слот мониторинга: **коллекция обязательна**; model и backdrop опциональны. Управление — в Mini App (после этапа ядра — временно `.env`/локальный конфиг).
_Avoid_: парсить весь маркет без whitelist

## Language — Math & fees

**Money**:
Расчёты в **uint64 nanoTON** (1 TON = 1e9). Без float в ядре спреда.

**Cross fees (Portals/MRKT, обе стороны)**:
Покупка: цена витрины (buy-fee «включена» в ask).  
Продажа: **5%** с SellPrice.  
Перенос: **0.3 TON** вывод + **0.25 TON** transfer.  
Gas reserve: **0.05 TON**.

`Net = SellPrice × 0.95 − BuyPrice − 0.3 − 0.25 − 0.05`

Same-market (не кросс): только 5% с продажи — в v1 сигналах не используем.

**Trigger**:
`Net ≥ max(TargetMinProfit, 5% × BuyPrice)` с дефолтом **TargetMinProfit = 0.1 TON**. Оба порога крутятся в Mini App.
Полный триггер только если проходит и якорь **best ask**, и якорь **медиана sales**.

**SellPrice (ask)**:
Триггер от **лучшего ask** на маркете выхода по MODEL_BG. В алерте также **undercut** (best − шаг).

**SellPrice (sales)**:
Медиана **чистых** sales по MODEL_BG на маркете выхода.

## Language — Liquidity & sales hygiene

**Sales window**:
До **72ч**, запрос **on-demand** по кандидату (кэш по MODEL_BG). Не качать все sales маркета.

**Outlier filter (B)**:
1) грубая медиана `M0` по сырым sales в окне;  
2) оставить `P ∈ [0.7×M0, 1.3×M0]`;  
3) и `P ∈ [0.5×A, 1.5×A]` где `A` = best ask того же ключа;  
4) медиана по оставшимся; нужно **≥ 5** иначе полного алерта нет.

**Alert policy**:
В чат только полные сигналы. Пустые / WEAK — максимум DEBUG-лог.
_Avoid_: спам «ликвиднось слабая»

## Language — Ingress & engine

**Workers**:
Две горутины (Portals, MRKT) → типизированный `chan MarketEvent` → один **Engine**. Thread-safe на границе канала.

**Listings snapshot**:
Полный стакан по каждому WatchSlot до конца пагинации (**без partial**). Replace-snapshot на тике. Дефолт интервала **90с** (в Mini App), слоты через общий rate-limit. Safety cap страниц — только защита от бесконечного цикла; обрыв ≠ «ок, partial алерт».

**Sales fetch**:
Только для ключей-кандидатов (предварительный спред по asks), не в каждом listing-тике.

**Book**:
Горячий стакан + кэш sales — **в RAM**. После рестарта — bootstrap заново.

**Dedup**:
По `buy_market + listing_id`: алерт если новый и ок, или был плохой → стал ок. Если уже алертили как ок — **скип**, даже если цена улучшилась. Лот исчез — сброс записи.

## Language — UI & auth

**Telegram bot**:
Только push: торговые алерты + «токен протух». Не место для вставки секретов в проде.

**Mini App**:
Токены MRKT/Portals, WatchSlots, пороги, interval. Паттерн UI — как gift-bot (TelegramUI), отдельный app order-bot.
_Avoid_: общий Mini App с gift-bot

**Tokens**:
Ручная вставка в Mini App (локально на старте — `.env`). Авторефреш Portals TMA на VPS **не** в v1. Протух → DM, парсинг/триггеры с sales не врут.
_Avoid_: headless Telegram для TMA в MVP

**Allowlist**:
Один `telegram_user_id`. Остальным бот не отвечает.

## Language — Storage & ship

**Bbolt**:
Файл `/data/order-bot.db` (Docker volume): токены (at rest encrypted), whitelist, settings, alert dedup. Не стакан.

**Binary**:
Один процесс: Engine + bot API + (позже) HTTP Mini App.

**Dev order**:
1) ядро + `.env` + алерты локально;  
2) Mini App через HTTPS-туннель;  
3) VPS + Caddy/nginx LE или Cloudflare Tunnel.

**Code reuse**:
Скопировать/адаптировать `gift-bot` market readers (MRKT, Portals), rate-limit/429 retry Portals. Не шарить `.env`/БД с gift-bot.

## Out of v1

Getgems/Tonnel/Fragment/Stars parsers; auto-buy / lite-client / seed; same-market buy; wash-trade graph по адресам; delta-API вместо replace-snapshot; multi-tenant.
