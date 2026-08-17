# syntax=docker/dockerfile:1

FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/order-bot ./cmd/order-bot \
    && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/order-bot /app/order-bot
COPY --from=web /web/dist /app/web/dist
COPY --from=build --chown=10001:10001 /out/data /data
USER 10001

ENV BOLT_PATH=/data/order-bot.db
ENV ASSETS_DIR=/data/assets
ENV MINIAPP_STATIC_DIR=/app/web/dist
ENV MINIAPP_PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/order-bot"]
