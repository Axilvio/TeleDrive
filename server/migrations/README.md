# Миграции БД (golang-migrate)

Текущая схема хранится в SQL-миграциях:

- `000001_init.up.sql`
- `000001_init.down.sql`
- `000002_action_logs.up.sql`
- `000002_action_logs.down.sql`
- `000003_configs_mode.up.sql`
- `000003_configs_mode.down.sql`
- `000004_notifications_queue.up.sql`
- `000004_notifications_queue.down.sql`

## Запуск в Docker

```bash
cd docker/universal-goodturn
cp .env.example .env

docker compose run --rm migrate
```

## Откат последней миграции

```bash
cd docker/universal-goodturn

docker compose run --rm migrate down 1
```
