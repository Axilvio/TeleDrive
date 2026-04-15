# Universal GoodTURN SFU — модуль 07 (платежи и подписки)

## Что реализовано

- Абстракция payment service (`internal/payments`):
  - создание pending-платежей по тарифу,
  - подтверждение платежа,
  - активация/продление подписки на 30 дней.
- Telegram flow:
  - создание платежа при выборе тарифа,
  - demo-подтверждение оплаты через callback-кнопку.
- HTTP endpoint:
  - `POST /api/v1/payments/confirm`.
- Subscription job (`internal/subscription`):
  - постановка напоминаний за 3 и 1 день до окончания,
  - деактивация конфигов для истекших подписок.
- Новая таблица `notifications_queue` для очереди напоминаний.

## Примечание

Интеграция с реальными провайдерами Telegram Stars / ЮMoney / Robokassa / USDT выполняется в следующем модуле на базе этого payment abstraction.
