# order-bot — Docker
#
# Локально:  cp .env.example .env && make start
# С GHCR:    IMAGE=ghcr.io/reqiewu/order-bot:latest make pull start

COMPOSE ?= docker compose
SERVICE ?= order-bot
IMAGE   ?=

.PHONY: help start stop logs clean pull

help:
	@echo "  make start  — собрать и запустить (локальный build)"
	@echo "  make pull   — скачать образ из GHCR (нужен IMAGE=...)"
	@echo "  make stop   — остановить"
	@echo "  make logs   — логи"
	@echo "  make clean  — stop + удалить образ и volume"

start:
	@test -f .env || (echo "Нет .env — скопируй .env.example"; exit 1)
	@if [ -z "$(IMAGE)" ]; then \
		test -d web/dist || (cd web && npm install && npm run build); \
		$(COMPOSE) up -d --build; \
	else \
		IMAGE=$(IMAGE) $(COMPOSE) up -d; \
	fi
	@echo "Mini App :$${MINIAPP_PORT:-8080} · Grafana :3000 (localhost) · логи: make logs"

pull:
	@test -n "$(IMAGE)" || (echo "Задай IMAGE=ghcr.io/reqiewu/order-bot:latest"; exit 1)
	docker pull $(IMAGE)

stop:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f --tail=200 $(SERVICE)

clean: stop
	-$(COMPOSE) down -v --rmi local --remove-orphans
	@echo "Контейнер, образ и volume удалены. .env не трогали."
