<div align="center">

<img src="frontend/public/logo.svg" alt="" height="96">

# Россия - страна Героев

Интерактивная карта исторических событий России. Каждое событие на карте - отдельный
сайт, который сделал и прислал участник проекта.

[russia-heroes.ru](https://russia-heroes.ru) · [О проекте](https://airsss993.github.io/histproject/) · [API](backend/docs/openapi.yaml) · [Дизайн-система](docs/design-system.html)

</div>

![Россия - страна Героев: карта событий](docs/banners/hero.jpg)

## Что внутри

- **Карта.** Метки трёх типов: политические и военные события, личности. Фильтр по типу и шкала веков,
  подложка OpenFreeMap, границы регионов из OSM.
- **Страница события** `/event/:id`. Сайт автора в рамке на весь экран, под ним дата, тип,
  автор и описание. Посетители ставят звёзды, уникальные просмотры считаются по активному времени.
- **Заявка.** Вошедший пользователь присылает ZIP со статическим сайтом и ставит точку на карте
  (поиск места через Nominatim идёт с сервера). Worker распаковывает архив в MinIO и снимает
  превью в headless Chrome.
- **Модерация** `/admin`. Очередь заявок, проверка, публикация с координатами или отказ
  с причиной. Супер-админ назначает роли, видит журнал действий и настраивает Telegram-бот
  и почту для уведомлений.
- **Аккаунты.** Регистрация с подтверждением почты, профиль автора `/u/:id` с его событиями.
  Тексты о персональных данных и согласия - в `docs/legal/`.

## Скриншоты

![Карточка события и сайт события](docs/banners/event.jpg)

![Инструкция для авторов и страница о проекте](docs/banners/guide.jpg)

<details>
<summary>Исходные скриншоты</summary>

| Карта | Событие на карте |
|---|---|
| ![Карта событий](docs/screenshots/map.png) | ![Карточка события на карте](docs/screenshots/map-event.png) |
| **Сайт события** | **О проекте** |
| ![Страница события](docs/screenshots/event.png) | ![О проекте](docs/screenshots/about.png) |
| **Как сделать сайт** | **Телефон** |
| ![Гайд для авторов](docs/screenshots/guide.png) | <img src="docs/screenshots/mobile-map.png" alt="Карта на телефоне" width="260"> |

Баннеры собираются из этих кадров: шаблон `docs/banners/banner.html`, рендер `sh docs/banners/render.sh`.

</details>

## Устройство

```mermaid
flowchart LR
  B[Браузер] --> C[Caddy]
  C -->|/api/*| A[API, Go]
  C -->|/sites/*, /preview/*| M[(MinIO)]
  C -->|остальное| S[SPA, frontend/dist]
  A --> P[(PostgreSQL)]
  A --> R[(Redis)]
  A -->|Asynq| W[Worker]
  W --> M
  W --> Ch[headless Chrome]
  W --> N[Postbox SMTP, Telegram]
```

| Каталог | Что это | Стек |
|---|---|---|
| [`frontend/`](frontend/README.md) | SPA: карта, заявка, профиль, панель модерации | Vite 8, React 19, TypeScript, React Router 7, MapLibre GL 6 |
| [`backend/`](backend/README.md) | API, worker обработки архивов, миграции, уведомления | Go 1.26, PostgreSQL 18, Redis, MinIO, Asynq, chromedp |
| [`docs/`](docs) | дизайн-система, документы по ПДн, заметки по архитектуре | |

Потоки данных и роли модулей бекенда описаны в [`docs/ai/architecture.md`](docs/ai/architecture.md).

## Запуск

Только фронтенд, с данными живого сайта:

```bash
cd frontend && npm install && VITE_API_PROXY=https://russia-heroes.ru npm run dev
```

Полный стек в Docker (API на `:7666`, Caddy на `:8080`, фронтенд по умолчанию проксирует `/api` туда же):

```bash
cd backend && cp .env.example .env
docker compose -f docker-compose.dev.yml up -d db redis minio chrome
docker compose -f docker-compose.dev.yml run --rm migrate
docker compose -f docker-compose.dev.yml run --rm bootstrap-admin
docker compose -f docker-compose.dev.yml up -d
```

Переменные окружения и требования к загружаемым архивам - в [`backend/README.md`](backend/README.md).

## Проверки и деплой

- Бекенд: GitHub Actions на каждый push в `backend/` гоняет `go test -race`, `go vet`,
  `golangci-lint` и сборку образа.
- Фронтенд: линтера и тестов нет, перед коммитом `npm run build`.
- Push в `main` = деплой. Workflow `Deploy` заходит на сервер ключом с forced command, тот
  подтягивает `main` и запускает [`deploy.sh`](deploy.sh): сборка фронтенда, образы бекенда,
  миграции, `docker compose up`.

## Лицензия

[Apache 2.0](LICENSE)
