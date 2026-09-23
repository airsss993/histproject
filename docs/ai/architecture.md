# Архитектура

Устойчивая картина: компоненты, потоки, точки входа. Не описание файлов.
Меняется только когда меняется сама схема.

## Компоненты

| Компонент | Где | Роль |
|---|---|---|
| SPA | `frontend/` | карта, "О проекте", вход/регистрация, профиль, панель админа `/admin` |
| API | `backend/cmd/api` | HTTP `/api/v1`, модульный монолит, сборка модулей в `internal/app/modules.go` |
| Worker | `backend/cmd/worker` | задачи Asynq: обработка архива, уведомления |
| Миграции | `backend/cmd/migrate` | Goose; на проде их применяет `deploy.sh`, локально вручную |
| Первый админ | `backend/cmd/bootstrap-admin` | создаёт `super_admin`, требует `--apply` |
| Caddy | `backend/Caddyfile` | единая точка: `/api/*` -> api, `/preview/*` и `/sites/*` -> MinIO, остальное - SPA из `frontend/dist` |
| PostgreSQL | | события, заявки, админы, пользователи, аудит |
| Redis | | сессии, rate limit, очередь Asynq |
| MinIO | | buckets `archives` (ZIP как пришёл) и `sites` (распакованные сайты и превью) |
| headless Chrome | | скриншот превью опубликованного сайта |
| Postbox (SMTP), Telegram | | email автору заявки и админам, сообщения админам в Telegram. Токен бота и получатели - в базе (`notifications.telegram_*`), их меняет супер-админ во вкладке "Бот", env для бота нет |

Модули бекенда: `auth` (проверка роли пользователя), `admins`, `users` (общие аккаунты),
`events`, `submissions`, `sites` (архив -> сайт), `notifications`.

## Потоки

Заявка на событие:

```
SPA форма "Предложить" (только вошедшему посетителю, гостя SPA ведёт на /login и возвращает в форму)
-> POST /api/v1/submissions (multipart, ZIP; без куки histproject_user - 401 до чтения тела)
-> email автора берётся из аккаунта, поле формы игнорируется: по нему профиль /u/:id находит заявки
-> submissions: запись в PostgreSQL (status processing), ZIP в bucket archives
-> Asynq -> worker -> sites: распаковка, файлы в bucket sites, скриншот через Chrome
-> статус new (или отказ при невалидном архиве)
-> админ в /admin: review -> publish (координаты, тип) | reject
-> publish создаёт событие в events
-> notifications -> Postbox SMTP / Telegram
```

Карта:

```
SPA /map -> GET /api/v1/events (постранично, pageSize <= 100, фронт выбирает все страницы)
-> сайт события открывается из bucket sites через Caddy /sites/*
```

Сессия одна: cookie `histproject_user` хранится в Redis. Модуль `auth` проверяет
роль в `auth.admins` по почте вошедшего пользователя. Отдельного входа в панель нет.
У существующих администраторов миграция создаёт профиль пользователя по их почте.

## Frontend

Живой слой: `src/pages/public` и `src/pages/panel`, API-клиенты в их `api.ts`,
UI-примитивы в `src/shared/ui`. Глобального состояния нет: каждая страница держит своё
в `useState`. Старые маршруты `/log`, `/adm`, `/mad`, `/his`, `/mod` редиректят на `/admin`.
