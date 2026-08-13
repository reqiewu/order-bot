# order-bot — Docker / локальная разработка
#
# 1) один раз:  make setup   → заполни .env
# 2) запуск:    make up
# 3) логи:      make logs
# 4) стоп:      make down
#
# Без Mini App пока: токены и WATCH_SLOTS_JSON в .env.
# Порты наружу не обязательны (бот сам ходит в Telegram/MRKT/Portals).

COMPOSE ?= docker compose
SERVICE ?= order-bot

.PHONY: help setup tidy test run build up down restart logs ps status shell clean clean-data

help:
	@echo "Цели:"
	@echo "  make setup      — создать .env из .env.example"
	@echo "  make tidy       — go mod tidy"
	@echo "  make test       — go test ./..."
	@echo "  make run        — локально: go run ./cmd/order-bot"
	@echo "  make build      — собрать Docker-образ"
	@echo "  make up         — собрать и запустить в фоне"
	@echo "  make logs       — логи сервиса"
	@echo "  make down       — остановить"
	@echo "  make restart    — перезапуск"
	@echo "  make ps         — статус контейнера"
	@echo "  make shell      — shell внутри контейнера"
	@echo "  make clean      — down + удалить локальный образ (.env не трогает)"
	@echo "  make clean-data — down + удалить volume с Bbolt (.env не трогает)"

setup:
	@test -f .env || cp .env.example .env
	@echo "Готово. Открой .env и заполни:"
	@echo "  TELEGRAM_BOT_TOKEN, OPERATOR_TELEGRAM_ID"
	@echo "  MRKT_TOKEN, PORTALS_TMA"
	@echo "  WATCH_SLOTS_JSON (опционально)"
	@echo "Потом: make up   или   make run"

tidy:
	go mod tidy

test:
	go test ./...

run:
	@test -f .env || (echo "Нет .env — сначала: make setup"; exit 1)
	set -a; . ./.env; set +a; go run ./cmd/order-bot

build:
	$(COMPOSE) build

up:
	@test -f .env || (echo "Нет .env — сначала: make setup"; exit 1)
	$(COMPOSE) up -d --build
	@echo ""
	@echo "order-bot запущен (paper)."
	@echo "Логи: make logs"

down:
	$(COMPOSE) down

restart:
	$(COMPOSE) restart $(SERVICE)

logs:
	$(COMPOSE) logs -f --tail=200 $(SERVICE)

ps status:
	$(COMPOSE) ps

shell:
	$(COMPOSE) exec $(SERVICE) sh || $(COMPOSE) exec $(SERVICE) /bin/sh || true

clean: down
	-$(COMPOSE) down --rmi local --remove-orphans
	@echo "Контейнер/локальный образ остановлены. .env не трогали."

clean-data: down
	-$(COMPOSE) down -v
	@echo "Volume с данными (Bbolt) удалён. Дальше: make up"
