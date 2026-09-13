# Переписывание backend в модульный монолит

## Цель

Заменить текущий backend на тестируемый модульный монолит на Go, сохранив данные PostgreSQL и объекты MinIO. Совместимость со старым HTTP API не требуется.

## Границы

- Вся новая реализация живет в `backend/`.
- Старые пакеты удаляются после переноса поведения и тестов, а не остаются второй архитектурой.
- Данные PostgreSQL и MinIO не удаляются и не перенося вручную.
- Все изменения схемы выполняются forward-only миграциями Goose.
- Перед первой миграцией живой системы создаются backup PostgreSQL и реестр ключей MinIO.
- Секреты хранятся только в env или в системе секретов. Логи и HTTP-ответы не содержат токены, пароли и ключи.

## Стек

- Go 1.26.
- Стандартные `net/http`, `http.ServeMux`, `log/slog` и `encoding/json`.
- PostgreSQL через `pgx/v5` и `pgxpool`.
- Redis для сессий, rate limiting и фоновой очереди.
- MinIO для ZIP-архивов, превью и опубликованных сайтов.
- Goose для миграций.
- Asynq остается как очередь, пока не появится измеримая причина ее заменить.

Gin, `sqlx`, `lib/pq`, JWT и глобальные клиенты удаляются после переноса зависящего от них кода.

## Структура

```text
backend/
  cmd/
    api/
    worker/
  internal/
    app/
    platform/
      config/
      database/
      httpserver/
      logging/
      queue/
      storage/
    modules/
      auth/
      admins/
      events/
      submissions/
      sites/
      notifications/
  migrations/
  docs/
    openapi.yaml
```

`cmd/api` поднимает HTTP API. `cmd/worker` обрабатывает архивы, создает скриншоты и отправляет уведомления. Оба процесса используют одни модули и platform-адаптеры.

## Границы модулей

### auth

- Вход, выход, проверка сессии и middleware авторизации.
- Непрозрачный случайный session token в `HttpOnly`, `Secure`, `SameSite=Lax` cookie.
- Redis хранит сессию с TTL. Logout удаляет ее немедленно.
- Пароли проверяются по существующим bcrypt-хешам. Перехеширование не требуется.

### admins

- Список, создание, смена роли и удаление администраторов.
- Роли: `admin` и `super_admin`.
- Создание первого `super_admin` выносится в отдельную CLI-команду. API не генерирует учетную запись незаметно при старте.

### events

- Публичный список и карточка опубликованного события.
- Фильтры по типам и датам, пагинация и стабильная сортировка.
- Координаты хранятся числами и валидируются на границе HTTP и БД.
- Тип события передается стабильным кодом, а не номером без контракта.

### submissions

- Прием заявки и ZIP-архива.
- Статусы: `processing`, `new`, `review`, `rejected`, `published`.
- Взятие заявки на проверку, отклонение и публикация оформляются как явные переходы состояния.
- Каждое админское действие записывается в аудит в той же транзакции.
- Публикация передает модулю events снимок данных через экспортируемый интерфейс. Модуль events не читает таблицы submissions.

### sites

- Проверка ZIP-архива, защита от zip slip и zip bomb, распаковка и загрузка в MinIO.
- Лимиты задают размер входного архива, суммарный распакованный размер, число файлов и размер одного файла.
- Обязателен `index.html` в корне. Символические ссылки, абсолютные пути и выход из корня отклоняются.
- Скриншот создается в изолированном Chrome с timeout и отменой контекста.
- Ключи существующих объектов MinIO не меняются на первом релизе.

### notifications

- Отправка email и Telegram после создания или смены статуса заявки.
- Ошибка внешнего канала не откатывает уже завершенную бизнес-транзакцию.
- Повторы выполняются фоновой очередью. Операции идемпотентны.

## Взаимодействие модулей

- Модуль читает и изменяет только свои таблицы.
- Между модулями нет SQL JOIN и foreign key.
- Синхронная команда передается через узкий Go-интерфейс, объявленный потребителем.
- Долгая или повторяемая операция передается через Redis-очередь.
- Межмодульный DTO содержит снимок нужных данных, а не ссылку на чужую модель.

## HTTP API

Новый API имеет префикс `/api/v1`. Формат ошибки един для всех endpoints:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "Неверные данны",
    "fields": {
      "eventDate": "Используйте YYYY-MM-DD"
    }
  }
}
```

Ошибка содержит безопасное сообщение для клиента. Внутренняя причина пишется в лог с request ID.

Первый контракт включает:

- `GET /healthz`
- `GET /readyz`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/session`
- `GET /api/v1/events`
- `GET /api/v1/events/{id}`
- `POST /api/v1/submissions`
- `GET /api/v1/admin/submissions`
- `GET /api/v1/admin/submissions/{id}`
- `POST /api/v1/admin/submissions/{id}/review`
- `POST /api/v1/admin/submissions/{id}/reject`
- `POST /api/v1/admin/submissions/{id}/publish`
- `PATCH /api/v1/admin/events/{id}/coordinates`
- `DELETE /api/v1/admin/events/{id}`
- `GET /api/v1/admin/admins`
- `POST /api/v1/admin/admins`
- `PATCH /api/v1/admin/admins/{id}/role`
- `DELETE /api/v1/admin/admins/{id}`
- `GET /api/v1/admin/audit-log`

Контракт описывается в `backend/docs/openapi.yaml` до подключения нового фронта.

## HTTP middleware

Порядок middleware:

1. Request ID.
2. Structured access log.
3. Recovery.
4. Security headers.
5. CORS с точным сравнением origin.
6. Лимит размера body.
7. Rate limit для login и публичной формы.
8. Auth и role check на защищенных routes.

Сервер задает `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout` и выполняет graceful shutdown по `SIGINT` и `SIGTERM`.

## Данные и миграция

Целевые PostgreSQL schemas:

- `auth`: администраторы.
- `submissions`: заявки и аудит их модерации.
- `events`: опубликованные события.
- `notifications`: подписки и журнал доставки.

Существующие rows переносятся без смены ID. Миграция:

1. Создает schemas и новые колонки.
2. Заполняет стабильные коды статусов и типов событий.
3. Проверяет число rows, диапазоны ID, NULL и дубли.
4. Удаляет только межмодульные foreign key, не удаляя колонки и rows.
5. Переносит и переименовывает таблицы в модульных schemas.
6. Добавляет новые NOT NULL и CHECK constraints только после успешного backfill.

Активные JWT-сессии не переносятся. После cutover администраторы входят заново, но их учетные записи и хеши паролей сохраняются.

## Надежность

- Каждый внешний вызов и SQL-операция получают `context.Context` с timeout.
- В SQL-циклах нет запросов. Списки загружаются одним запросом или batch-операцией.
- Повторная обработка job не создает дубли сайтов, событий и уведомлений.
- Ошибки транспорта, валидации, авторизации, конфликта состояния и внутренние ошибки имеют разные стабильные коды.

## Наблюдаемость

- В production `slog` пишет JSON, локально пишет читаемый text.
- Каждая HTTP-запись содержит request ID, method, route, status, duration и response bytes.
- Логи job содержат job ID, submission ID, attempt и duration.
- `/healthz` отвечает 200, если процесс жив.
- `/readyz` отвечает 200 только при доступных PostgreSQL, Redis и MinIO.

## Конфигурация

- Парсинг env реализован как чистая функция и покрыт table-driven тестами.
- Локальные несекретные значения имеют явные defaults.
- Production не стартует без адресов зависимостей, credentials, cookie settings и разрешенных origins.
- Названия env едины для local, CI и production. Выбор между `PG_DSN`, `PG_DSN_DEV` и `PG_DSN_PROD` удаляется в пользу одного `DATABASE_URL`.

## Тесты и quality gates

- Unit-тесты покрывают бизнес-правила, парсинг конфига, middleware и валидацию ZIP.
- HTTP-тесты используют `httptest` и проверяют status, JSON и headers.
- Integration-тесты репозиториев работают с настоящим PostgreSQL в Docker.
- Миграции проверяются на пустой БД и на копии текущей схемы.
- Обязательные команды: `go test -race ./...`, `go vet ./...`, `go build ./cmd/api ./cmd/worker` и `golangci-lint run`.

## Docker и запуск

- Multi-stage Dockerfile с версией Go, зафиксированной до patch.
- Отдельные containers `api` и `worker`.
- PostgreSQL, Redis и MinIO имеют healthcheck и persistent volumes.
- `api` и `worker` зависят от healthy dependencies.
- Каждый core container имеет `restart: always`.
- PostgreSQL, Redis, MinIO и Chrome не публикуют порты на production host без явной необходимости.
- Все services работают в отдельной internal bridge network.

## Порядок реализации

1. Каркас platform: конфиг, логи, HTTP-сервер, middleware, health checks и graceful shutdown.
2. Подключение PostgreSQL, Redis и MinIO через явные адаптеры.
3. Миграция схемы и проверки сохранности данных.
4. auth и admins.
5. events.
6. submissions и аудит.
7. sites и worker.
8. notifications.
9. OpenAPI, Docker Compose и корневой CI.
10. Прогон на копии production-данных, backup и cutover.

Каждый этап заканчивается тестами и отдельным conventional commit. Непроверенный код не пушится.

## Не входит в первую версию

- Обратная совместимость с `/api` без версии.
- Перенос старых активных сессий.
- Kubernetes, distributed tracing и отдельные microservices.
- Замена Redis, MinIO, PostgreSQL, Resend или Telegram без доказанной проблемы.
