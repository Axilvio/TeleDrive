# Universal GoodTURN SFU — модуль 04 (Telegram-бот)

## Что реализовано

- Полноценный command flow:
  - `/start`
  - `/subscribe`
  - `/config`
  - `/devices`
  - `/support`
- Inline callbacks:
  - выбор тарифа (`buy_basic`, `buy_standard`, `buy_pro`)
  - `disconnect_all`
  - `show_account`
- Логирование действий пользователя в PostgreSQL (`action_logs`).
- Создание pending-платежа в таблице `payments` при выборе тарифа.
- Получение/создание активного конфига пользователя из таблицы `configs`.
- Просмотр активных сессий из таблицы `sessions`.

## Базовые ограничения текущего этапа

- Подключение реальных payment providers будет добавлено в следующем модуле.
- Выдача QR и export-конфигов в файл будет добавлена в модуле управления конфигами.
