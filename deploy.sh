#!/bin/sh
# Сборка и перезапуск прода. Код уже подтянут: git reset делает forced command
# в authorized_keys сервера до запуска этого файла.
set -eu
cd "$(dirname "$0")"

docker run --rm -v "$PWD/frontend:/app" -w /app node:20-alpine \
	sh -c 'npm ci --no-audit --no-fund && npm run build'

cd backend
compose="docker compose -f docker-compose.prod.yml"
$compose build
$compose run --rm migrate
$compose up -d --remove-orphans
docker image prune -f >/dev/null
