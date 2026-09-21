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
| Postbox (SMTP), Telegram | | email автору заявки и админам, сообщение в чат админов |

Модули бекенда: `auth` (сессии админов), `admins`, `users` (публичные пользователи),
`events`, `submissions`, `sites` (архив -> сайт), `notifications`.

## Потоки

Заявка на событие:

```
SPA форма "Предложить"
-> POST /api/v1/submissions (multipart, ZIP)
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

Сессии: две независимые cookie - `histproject_session` (админы, модуль `auth`) и
`histproject_user` (пользователи, модуль `users`). Обе хранятся в Redis.

## Frontend

Живой слой: `src/pages/public` и `src/pages/panel`, API-клиенты в их `api.ts`,
UI-примитивы в `src/shared/ui`. Старый учебный слой `src/entities`, `src/features`
подключён только через `src/app/store.ts` и ходит в несуществующее старое API.
