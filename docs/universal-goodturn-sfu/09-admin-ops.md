# Universal GoodTURN SFU — модуль 09 (admin operations)

## Что реализовано

- Расширение админ-панели:
  - просмотр и CRUD server instances,
  - ручной reset активных сессий по `user_id`,
  - просмотр платежей,
  - просмотр очереди уведомлений.
- HTMX partial endpoints:
  - `GET /admin/instances`
  - `POST /admin/instances/create`
  - `POST /admin/instances/delete`
  - `POST /admin/sessions/reset`
  - `GET /admin/payments`
  - `GET /admin/notifications`

## Примечание

Это операционный слой для поддержки. Следующий этап — фильтры, поиск и действия bulk-уровня.
