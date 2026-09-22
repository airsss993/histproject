# histproject

Моно-репозиторий проекта "Россия - страна Героев": интерактивная карта исторических
объектов с админ-панелью и заявками от пользователей.

| Каталог | Что это | Стек |
|---|---|---|
| `frontend/` | SPA с картой, формой заявки и панелями админа/модератора | Vite, React 19, TypeScript, React Router, MapLibre GL |
| `backend/` | API, обработка загружаемых архивов, уведомления | Go 1.26, PostgreSQL, MinIO, Redis, Docker |

## Запуск

Фронтенд:

```bash
cd frontend && npm install && npm run dev
```

Бекенд: `backend/README.md`, переменные окружения - `backend/.env.example`.

## API

Контракт - `backend/docs/openapi.yaml`.
