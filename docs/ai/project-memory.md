# Память проекта

Практические факты, которые трудно быстро достать из кода: ловушки, неочевидные
зависимости, особенности запуска и тестов. Не журнал изменений.
Запись устарела - исправить или удалить, а не дописывать новую рядом.

## Запуск

- Dev API снаружи: Caddy `http://127.0.0.1:8080` (прокси Vite `/api` смотрит туда же,
  переопределяется `VITE_API_PROXY`), сам API - `:7666`. Проверять API curl'ом через
  `127.0.0.1:8080/api/v1/...`: через dev-сервер Vite приходит не JSON.
- `docker compose -f docker-compose.dev.yml build` без `--profile tools` не пересобирает
  `migrate` и `bootstrap-admin`.
- Без Docker API и worker запускаются `go run`, но в `.env` hostnames `db`, `redis`,
  `minio`, `chrome` меняются на `localhost`.

## Тесты

- Интеграционные тесты (`*_integration_test.go`) скипаются, если не заданы переменные
  `TEST_DATABASE_URL`, `TEST_*_DATABASE_URL`, `TEST_REDIS_ADDRESS`, `TEST_APP_*`.
  В CI они не заданы, то есть CI гоняет только unit-тесты.
- Имя тестовой базы обязано оканчиваться на `_test`, иначе тест отказывается работать.
- Во фронте тестов и линтера нет; единственная автоматическая проверка - `npm run build`
  (`tsc` + `vite build`).

## Frontend

- maplibre-gl держим на 5: в 6 нет default export, а отдельный воркер ломается при
  pre-bundling Vite 4.
- `transition: width` у контейнера карты не использовать: MapLibre создаётся посреди
  анимации, пропускает первый колбэк ResizeObserver и canvas остаётся узким.
- В скрытой вкладке браузера `requestAnimationFrame` не срабатывает, CSS-анимации
  висят в `running`; скриншот карты бывает пустым до первого движения.
- CSS-классы не начинать с `ad-`, `ads-`, `banner`: косметические фильтры блокировщиков
  (uBlock, AdBlock) прячут `.ad-top`, `.ad-card` и им подобные через `display: none`, и у
  модератора с блокировщиком панель была пустой. Панель поэтому на префиксе `mod-`.

## Почта

- Письма собираются из `notifications/templates/letter.html` (html/template), картинки
  вшиты через embed и уходят вложениями по CID. `cid:` в шаблоне - литерал: подставленный
  через `{{}}` URL с этой схемой html/template заменит на `#ZgotmplZ`.
- PNG баннера и иконок перерисовывает `templates/render_assets.py` (нужны Chrome и Pillow).
  SVG в письмах не используем: Gmail и Outlook его не показывают.
- Postbox SMTP без STARTTLS обрывает соединение до AUTH, поэтому отправитель
  отказывается работать без него.

## Прод

- Сервер `185.113.11.122`, код в `/opt/histproject`, `.env` только там. Push в `main` = деплой
  (`.github/workflows/deploy.yml`, без проверок). Ключ Actions в `authorized_keys` ограничен
  forced command: `git reset --hard origin/main` и `deploy.sh`. Правки руками в `/opt/histproject` затрутся.
- На том же сервере живёт личный VPN (3x-ui, VLESS Reality). 443 держит nginx stream
  (`/etc/nginx/stream.conf`): SNI `russia-heroes.ru` -> Caddy `127.0.0.1:8443`, остальное -> xray
  `127.0.0.1:4443`. Обоим уходит PROXY protocol, без него все клиенты API делят один rate limit.
  Новый домен или поддомен сайта добавлять и в `map` nginx, иначе он уйдёт в VPN.
- `APP_ENV=development`, пока нет ключей Postbox: `production` без них не стартует. Бот Telegram от env
  не зависит, он настраивается в панели.

## Telegram-бот

- Доступ к уведомлениям выдаётся из списка "Написали боту", а он берётся из `getUpdates`: Telegram
  хранит апдейты сутки и отдаёт их, только пока у бота нет webhook. Поставите webhook этому же боту -
  список кандидатов сломается (409 от Telegram).
- Журнал доставки ведётся по каждому чату (`event_key` с суффиксом `:<chat_id>`), чтобы повтор после сбоя
  одного получателя не слал дубль остальным.

## Окружение

- macOS: нет `grep -P`, запрещённые символы проверять через `perl -CSD`.
  В zsh шаблоны вида `--include=*.go` брать в кавычки.
