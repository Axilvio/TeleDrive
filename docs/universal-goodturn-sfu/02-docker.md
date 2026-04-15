# Universal GoodTURN SFU — модуль 02 (Docker)

## Что реализовано

- production-ready compose-стек:
  - `server` (Go backend),
  - `postgres` (PostgreSQL 16),
  - `redis` (Redis 7),
  - `prometheus`,
  - `grafana`.
- dev-override compose для локальной разработки с `go run` и bind mount.
- шаблон переменных окружения `.env.example`.
- минимальная конфигурация Prometheus + auto-provision datasource в Grafana.

## Команды запуска

```bash
cd docker/universal-goodturn
cp .env.example .env

docker compose up -d
# или dev-режим
# docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d
```

## Ограничения текущего этапа

- Миграции БД, платежи и бизнес-логика сессий будут добавлены в следующих модулях.
- Метрики `/metrics` должны быть добавлены в backend при интеграции Prometheus middleware.
