# Universal GoodTURN SFU — модуль 01

## Важно
Этот документ описывает **безопасный и легитимный** каркас проекта (SaaS-управление подписками, аккаунтами, конфигами, мониторингом).
Функции обхода фильтров, маскировки трафика под сторонние сервисы и обхода политик доступа намеренно исключены.

## Предлагаемая структура монорепозитория

```text
.
├── server/
│   ├── cmd/app/
│   │   └── main.go
│   ├── internal/
│   │   ├── bot/
│   │   │   └── bot.go
│   │   ├── config/
│   │   │   └── config.go
│   │   └── http/
│   │       └── server.go
│   ├── go.mod
│   └── README.md
├── client-tauri/
├── client-android/
├── client-ios/
├── docker/
└── docs/
    └── universal-goodturn-sfu/
        └── 01-project-structure.md
```

## Границы модуля

- Базовый Go backend (health endpoint + graceful shutdown).
- Telegram-бот как основной интерфейс (команды `/start`, `/subscribe`, `/config`, `/devices`, `/support`).
- Конфигурация из ENV.
- Подготовка к следующим модулям: БД, платежи, сессии, админка.
