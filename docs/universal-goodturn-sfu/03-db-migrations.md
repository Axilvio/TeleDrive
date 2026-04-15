# Universal GoodTURN SFU — модуль 03 (PostgreSQL schema + migrations)

## Что реализовано

- SQL-схема PostgreSQL в `golang-migrate` формате:
  - `users`
  - `configs`
  - `sessions`
  - `payments`
  - `server_instances`
- Внешние ключи, ограничения (`CHECK`), индексы под рабочие запросы.
- Partial unique index для контроля одного активного сеанса на пару `(user_id, device_id)`.
- One-shot сервис `migrate` в Docker Compose (`profile: tools`) для применения миграций.

## Команды

```bash
cd docker/universal-goodturn
cp .env.example .env

# применить миграции
 docker compose --profile tools run --rm migrate

# откатить 1 миграцию
 docker compose --profile tools run --rm migrate down 1
```

## Примечания

- Счётчики realtime-устройств в Redis (TTL 60 сек) и бизнес-логика лимитов тарифов будут реализованы в следующем модуле серверной логики.
