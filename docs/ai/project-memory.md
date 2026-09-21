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
- Глобальная SCSS старого слоя стилизует голые теги (`header`, `input`); в новом слое
  стилизовать только классами.
- `@types/react` 18 при React 19: атрибут `inert` не проходит typecheck.
- `transition: width` у контейнера карты не использовать: MapLibre создаётся посреди
  анимации, пропускает первый колбэк ResizeObserver и canvas остаётся узким.
- В скрытой вкладке браузера `requestAnimationFrame` не срабатывает, CSS-анимации
  висят в `running`; скриншот карты бывает пустым до первого движения.

## Почта

- Письма собираются из `notifications/templates/letter.html` (html/template), картинки
  вшиты через embed и уходят вложениями по CID. `cid:` в шаблоне - литерал: подставленный
  через `{{}}` URL с этой схемой html/template заменит на `#ZgotmplZ`.
- PNG баннера и иконок перерисовывает `templates/render_assets.py` (нужны Chrome и Pillow).
  SVG в письмах не используем: Gmail и Outlook его не показывают.
- Postbox SMTP без STARTTLS обрывает соединение до AUTH, поэтому отправитель
  отказывается работать без него.

## Окружение

- macOS: нет `grep -P`, запрещённые символы проверять через `perl -CSD`.
  В zsh шаблоны вида `--include=*.go` брать в кавычки.
