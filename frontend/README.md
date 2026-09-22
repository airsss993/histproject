# Фронтенд "Россия - страна Героев"

Публичная карта событий, заявка на событие, вход автора, профиль и панель модерации.
Vite 4, React 19, TypeScript, React Router, MapLibre GL.

## Запуск

```bash
npm install
npm run dev     # http://localhost:3000
npm run build   # tsc + vite build в dist/
```

Dev-сервер проксирует `/api` на `http://127.0.0.1:8080` - это Caddy из
`backend/docker-compose.dev.yml`. Адрес меняется переменной `VITE_API_PROXY`:

```bash
VITE_API_PROXY=https://russia-heroes.ru npm run dev
```

Клиент ходит в API по пути `${VITE_API_BASE}/api/v1`. По умолчанию `VITE_API_BASE`
пустой, запросы идут на тот же домен.

## Структура

| Путь | Что там |
|---|---|
| `src/app/app.tsx` | точка входа и маршруты |
| `src/pages/public` | карта, заявка, "О проекте", вход и регистрация, профиль, 404; клиент API в `api.ts` |
| `src/pages/panel` | панель модерации `/admin`, свой клиент API в `api.ts` |
| `src/shared/ui` | общие компоненты: кнопки, поля, чипы, аватар |
| `public/` | логотип и границы регионов России (`russia-regions.geojson`) |

Кнопки, поля и чипы берутся из `src/shared/ui`, новых под отдельный экран не заводим.
Цвета, типографика и правила компонентов - в `docs/design-system.html` в корне репозитория.

Линтера и тестов нет, проверка перед коммитом - `npm run build`.
