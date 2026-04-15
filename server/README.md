# Server (Go)

Backend + Telegram-бот для SaaS-управления аккаунтами, подписками и сессиями.

## Запуск локально

```bash
cd server
go mod tidy
TELEGRAM_BOT_TOKEN=... \
DATABASE_URL=postgres://goodturn:change_me@localhost:5432/goodturn?sslmode=disable \
REDIS_ADDR=localhost:6379 \
WG_ENDPOINT=vpn.example.com:51820 \
WG_SERVER_PUBLIC_KEY=server-public-key-placeholder \
WG_DNS=1.1.1.1 \
go run ./cmd/app
```

## HTTP API

- `GET /healthz`
- `POST /api/v1/sessions/connect`
- `POST /api/v1/sessions/disconnect`
- `POST /api/v1/configs/generate`
- `POST /api/v1/payments/confirm`

Пример `confirm payment`:

```json
{
  "payment_id": 1001
}
```

## Функции текущего этапа

- тарифы и pending-платежи;
- demo-подтверждение платежа и активация подписки на 30 дней;
- очередь напоминаний (`notifications_queue`) за 3/1 день до окончания;
- авто-деактивация конфигов с истекшей подпиской;
- Redis-limiter устройств (TTL 60 сек);
- WireGuard config builder;
- Telegram-команды `/start /subscribe /config /devices /support /mode`.

## Важно

Этот модуль умышленно не реализует функциональность обхода фильтров/белых списков или маскировки трафика под сторонние сервисы.


## Админ-панель

- `GET /admin`
- `GET /admin/users?limit=100`

Если задан `ADMIN_TOKEN`, добавляйте заголовок `X-Admin-Token: <token>`.

- `GET /admin/instances`
- `POST /admin/instances/create`
- `POST /admin/instances/delete`
- `POST /admin/sessions/reset`
- `GET /admin/payments`
- `GET /admin/notifications`
