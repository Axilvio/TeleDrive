# Universal GoodTURN SFU — модуль 06 (генерация WireGuard-конфигов)

## Что реализовано

- Генерация приватного ключа WireGuard (32 bytes, base64) при первом запросе конфига.
- Хранение ключа в таблице `configs` и возврат готового клиентского `.conf`.
- Поддержка режимов маршрутизации:
  - `full_tunnel`
  - `split_tunnel`
  - `per_app`
- Новый API endpoint:
  - `POST /api/v1/configs/generate`
- Telegram-команда `/mode` для выбора режима через inline-кнопки.

## ENV для рендера конфига

- `WG_ENDPOINT`
- `WG_SERVER_PUBLIC_KEY`
- `WG_DNS`

## Ограничения

- QR-рендер и скачивание файла будет добавлено в следующем модуле.
