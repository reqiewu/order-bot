# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/order-bot ./cmd/order-bot

FROM alpine:3.21
RUN adduser -D -u 10001 orderbot \
	&& mkdir -p /data \
	&& chown orderbot:orderbot /data
COPY --from=build /out/order-bot /app/order-bot
USER orderbot
WORKDIR /app
ENV BOLT_PATH=/data/order-bot.db
VOLUME ["/data"]
ENTRYPOINT ["/app/order-bot"]
