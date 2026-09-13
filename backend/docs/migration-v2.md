# V2 migration runbook

## Текущая блокировка

На 2026-09-13 хост `10.3.13.202` идет через `utun4`, TCP-порты 22, 5432, 6379, 9000 и 9010 принимают соединение. SSH сбрасывает соединение на `kex_exchange_identification`, PostgreSQL не отвечает на protocol handshake. Live migration не запускать, пока SSH не покажет реальный compose project и volumes.

## Правила

- Все команды ниже выполняются на локальном Mac, если не указан другой host.
- `DATABASE_URL`, MinIO access key и MinIO secret key берутся из server secret storage. Не писать их в shell history, чат и файлы репозитория.
- Backup и inventory хранятся вне репозитория.
- API и worker останавливаются до snapshot и не запускаются до переключения на новый API.
- Migration из `migrations/v2` forward-only. Git revert не откатывает схему PostgreSQL.

## 1. Доступ

Локальный Mac:

```bash
route -n get 10.3.13.202 | grep interface
nc -z -w 2 10.3.13.202 22
ssh -o BatchMode=yes -o ConnectTimeout=5 airsss@10.3.13.202 'hostname'
```

Продолжать, только если SSH завершился с кодом 0. На сервере сначала снять read-only состояние:

Сервер `10.3.13.202`:

```bash
hostname
docker ps --format '{{.Names}}\t{{.Image}}\t{{.Status}}'
docker volume ls
```

Не запускать `docker compose down -v` и не удалять volumes.

## 2. Maintenance window

На сервере остановить только legacy API и worker через фактический compose project. Имена services берутся из `docker compose config --services`, а не из этого runbook.

Сервер `10.3.13.202`, каталог compose project:

```bash
docker compose config --services
read -r API_SERVICE WORKER_SERVICE
docker compose stop "$API_SERVICE" "$WORKER_SERVICE"
docker compose ps
```

PostgreSQL, Redis и MinIO не останавливать.

## 3. PostgreSQL backup

Локальный Mac:

```bash
BACKUP_DIR="$HOME/histproject-backups/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"
read -r PGHOST
read -r PGPORT
read -r PGUSER
read -r PGDATABASE
read -r -s PGPASSWORD
read -r -s DATABASE_URL
export PGHOST PGPORT PGUSER PGDATABASE PGPASSWORD DATABASE_URL
pg_dump --format=custom --file="$BACKUP_DIR/postgres.dump"
pg_restore --list "$BACKUP_DIR/postgres.dump" > "$BACKUP_DIR/postgres.list"
shasum -a 256 "$BACKUP_DIR/postgres.dump" > "$BACKUP_DIR/postgres.dump.sha256"
```

Сохранить baseline counts под стабильными именами:

Локальный Mac:

```bash
psql -X -v ON_ERROR_STOP=1 -AtF $'\t' > "$BACKUP_DIR/counts-before.tsv" <<'SQL'
SELECT 'admins', COUNT(*) FROM public.admins
UNION ALL SELECT 'legacy_sessions', COUNT(*) FROM public.admin_sessions
UNION ALL SELECT 'submissions', COUNT(*) FROM public.requests
UNION ALL SELECT 'events', COUNT(*) FROM public.objects
UNION ALL SELECT 'audit_log', COUNT(*) FROM public.request_audit_log
UNION ALL SELECT 'telegram_subscriptions', COUNT(*) FROM public.bot_subscriptions
ORDER BY 1;
SQL
```

## 4. MinIO inventory

`mc alias set` запросит access key и secret key интерактивно.

Локальный Mac:

```bash
mc alias set histproject-backup http://10.3.13.202:9000 --api S3v4
for bucket in requests sites; do
  mc stat "histproject-backup/$bucket"
  mc find "histproject-backup/$bucket" --print '{}\t{size}\t{time}'
done | LC_ALL=C sort > "$BACKUP_DIR/minio-objects.tsv"
wc -l "$BACKUP_DIR/minio-objects.tsv" > "$BACKUP_DIR/minio-object-count.txt"
shasum -a 256 "$BACKUP_DIR/minio-objects.tsv" > "$BACKUP_DIR/minio-objects.tsv.sha256"
mc alias rm histproject-backup
```

Миграция PostgreSQL не изменяет MinIO bucket и objects.

## 5. Apply

Код должен быть в коммите, где `go test -race ./...`, integration test и `go vet ./...` прошли. Перед apply проверить, что `$BACKUP_DIR/postgres.dump`, `$BACKUP_DIR/counts-before.tsv` и `$BACKUP_DIR/minio-objects.tsv` существуют и не пусты.

Локальный Mac, корень `backend`:

```bash
test -s "$BACKUP_DIR/postgres.dump"
test -s "$BACKUP_DIR/counts-before.tsv"
test -f "$BACKUP_DIR/minio-objects.tsv"
MIGRATION_TIMEOUT=5m go run ./cmd/migrate --apply
```

## 6. Post-migration checks

Локальный Mac:

```bash
psql -X -v ON_ERROR_STOP=1 -AtF $'\t' > "$BACKUP_DIR/counts-after.tsv" <<'SQL'
SELECT 'admins', COUNT(*) FROM auth.admins
UNION ALL SELECT 'legacy_sessions', COUNT(*) FROM auth.legacy_sessions
UNION ALL SELECT 'submissions', COUNT(*) FROM submissions.submissions
UNION ALL SELECT 'events', COUNT(*) FROM events.events
UNION ALL SELECT 'audit_log', COUNT(*) FROM submissions.audit_log
UNION ALL SELECT 'telegram_subscriptions', COUNT(*) FROM notifications.telegram_subscriptions
ORDER BY 1;
SQL
diff -u "$BACKUP_DIR/counts-before.tsv" "$BACKUP_DIR/counts-after.tsv"
psql -X -v ON_ERROR_STOP=1 -Atc "SELECT COUNT(*) FROM pg_constraint c JOIN pg_class t ON t.oid = c.conrelid JOIN pg_namespace child ON child.oid = t.relnamespace JOIN pg_class p ON p.oid = c.confrelid JOIN pg_namespace parent ON parent.oid = p.relnamespace WHERE c.contype = 'f' AND child.nspname IN ('auth','submissions','events','notifications') AND child.nspname <> parent.nspname"
```

Последняя команда должна вернуть `0`. После сравнения counts проверить `/healthz`, `/readyz`, login и чтение событий на новом API. Legacy API после V2 migration не запускать.

## 7. Аварийное восстановление

Если post-migration checks не прошли, не запускать API и worker. Создать новую пустую PostgreSQL database, восстановить `$BACKUP_DIR/postgres.dump` через `pg_restore`, повторить baseline counts и только после этого переключить `DATABASE_URL`. Не восстанавливать dump поверх частично измененной database.
