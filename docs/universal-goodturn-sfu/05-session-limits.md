# Universal GoodTURN SFU — модуль 05 (лимиты устройств + Redis TTL)

## Что реализовано

- Redis-based limiter активных подключений на пользователя с TTL 60 секунд.
- HTTP API для серверной регистрации/завершения сессий:
  - `POST /api/v1/sessions/connect`
  - `POST /api/v1/sessions/disconnect`
- Логика отклонения подключения при превышении `max_devices`.
- Сохранение сессий в PostgreSQL + логирование действий (`session_connected`, `session_disconnected`, `session_rejected`).

## Формат запросов

### Connect

```json
{
  "telegram_id": 123456789,
  "device_id": "desktop-win-01",
  "ip": "10.0.0.10"
}
```

### Disconnect

```json
{
  "telegram_id": 123456789,
  "device_id": "desktop-win-01"
}
```

## Примечание

Модуль реализует бизнес-логику лимитов и контроль активных сессий. Интеграция с реальным tunnel-plane и протоколами транспорта выполняется в следующих этапах.
