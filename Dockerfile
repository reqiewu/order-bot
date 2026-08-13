# syntax=docker/dockerfile:1

FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/order-bot ./cmd/order-bot

FROM alpine:3.21
RUN adduser -D -u 10001 orderbot \
	&& mkdir -p /data \
	&& chown orderbot:orderbot /data
COPY --from=build /out/order-bot /app/order-bot
COPY --from=build /src/web/dist /app/web/dist
USER orderbot
WORKDIR /app
ENV BOLT_PATH=/data/order-bot.db
ENV MINIAPP_STATIC_DIR=/app/web/dist
ENV MINIAPP_ADDR=:8080
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/app/order-bot"]
