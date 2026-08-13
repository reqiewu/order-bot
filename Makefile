# order-bot — Docker
#
# Один раз: cp .env.example .env && (cd web && npm i && npm run build)
# Дальше:   make start → make logs → make stop

COMPOSE ?= docker compose
SERVICE ?= order-bot

.PHONY: help start stop logs clean

help:
	@echo "  make start  — собрать и запустить"
	@echo "  make stop   — остановить"
	@echo "  make logs   — логи"
	@echo "  make clean  — stop + удалить образ и volume"

start:
	@test -f .env || (echo "Нет .env — скопируй .env.example"; exit 1)
	@test -d web/dist || (cd web && npm install && npm run build)
	$(COMPOSE) up -d --build
	@echo "Mini App :$${MINIAPP_PORT:-8080} · логи: make logs"

stop:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f --tail=200 $(SERVICE)

clean: stop
	-$(COMPOSE) down -v --rmi local --remove-orphans
	@echo "Контейнер, образ и volume удалены. .env не трогали."
