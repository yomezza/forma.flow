# Запуск Go-backend FORMA TELA

Требуется Go 1.27.4 и PostgreSQL 16–18. Для тестов контракта дополнительно
Python 3.12+, PyYAML и jsonschema; в runtime Python не используется.

## Локальный запуск

Из каталога `backend/`:

```sh
cp .env.example .env
# Заполните .env, особенно STUDIO_TIME_ZONE и DB_PASSWORD.
# Для DB_PASSWORD в compose используйте случайную URL-safe строку.
set -a
. ./.env
set +a

docker compose up -d db
make build
make migrate

export BOOTSTRAP_EMAIL='owner@example.com'
# Файл с реальным паролем (12–1024 символа и перевод строки), права 0600.
# Пароль читается из stdin, не передаётся аргументом и не печатается.
bin/bootstrap < /path/to/protected-owner-password.txt
bin/server
```

Bootstrap выполняется один раз, атомарно создаёт студию с указанным IANA TZ,
один кабинет и owner/admin аккаунт. Не создаёт вымышленных специалистов,
каталог услуг или открытые рабочие дни. После входа настройте реальные услуги,
специалистов, их квалификации/неделю и студийный график через admin API.
До этого публичная запись закрыта. HTTP-сервер откажется стартовать без
актуальных миграций, bootstrap или при несовпадении TZ в конфигурации и БД.

`GET /healthz` — процесс жив. `GET /readyz` — PostgreSQL доступен.
`SIGTERM`/`SIGINT` прекращают приём новых соединений и дают время завершить запросы.

Для контейнерного сервера после bootstrap:

```sh
docker compose build backend
docker compose up -d backend
```

Вместо Go-команд можно запускать миграции и bootstrap из того же образа:
`docker compose run --rm --entrypoint /app/manage backend up` и
`docker compose run --rm -e BOOTSTRAP_EMAIL --entrypoint /app/bootstrap backend < /path/to/protected-owner-password.txt`.
Не запускайте эти команды параллельно с первым bootstrap.
Если сборка использует корпоративный TLS proxy, Dockerfile поддерживает
BuildKit secret `proxy_ca`; не отключайте проверку TLS.

## Конфигурация

| Переменная | Значение |
|---|---|
| `DATABASE_URL` | PostgreSQL URL, production с TLS и отдельной прикладной ролью |
| `HTTP_ADDR` | По умолчанию `:8080` |
| `PUBLIC_ORIGIN` | Точный origin frontend, без пути и завершающего `/` |
| `STUDIO_TIME_ZONE` | Обязательный фактический IANA TZ; не TZ браузера/сервера |
| `COOKIE_SECURE` | `true` по умолчанию; `false` только для локального HTTP |
| `DB_POOL_MAX` | 20 соединений по умолчанию |
| `SOLVER_MAX_NODES` | 100000 узлов на расчёт |
| `SOLVER_MAX_COMPONENT` | 100 заданий на компоненту |

Сессии живут 12 часов. SQL statement timeout — 5 секунд, lock timeout — 3,
HTTP use case deadline — 12. Выборки доступности ограничены 62 днями, редакции
расписания — 366. Batch snapshot имеет защитный предел 5000 строк на набор;
при превышении возвращается 503, данные не записываются наугад.
Списки используют SQL LIMIT/OFFSET с непрозрачным cursor, `limit` 1–100.
Публичные заявки ограничены 30 запросами/минуту на IP/процесс; вход — 8.
Для общей границы нескольких публичных инстансов настройте rate limit ingress.
Forwarded-заголовки не принимаются за доказательство IP; прямое подключение или
доверенный ingress должны корректно передавать адрес на транспортном уровне.

## Миграции и резервные копии

`make migrate` применяет goose Up под advisory session lock. HTTP-процессы
сами не меняют схему. `make status` показывает состояние. Существующие применённые
миграции не редактировать. Down допускается только при `MIGRATION_ALLOW_DOWN=true`
в одноразовой БД с именем `forma_test_*`; никогда не используется при штатном старте.

Production backup: зашифрованный `pg_dump -Fc` или PostgreSQL backup/PITR с
ограниченным доступом. Восстановление сначала в отдельную БД через `pg_restore
--exit-on-error`, затем проверка схемы, числа записей, FK, EXCLUDE и replay
идемпотентного запроса. Не запускайте production down для восстановления.
Тестовый restore drill действительно выполняет pg_dump/pg_restore в отдельную БД.

## Проверки

Для тестов нужна отдельная PostgreSQL-инстанция; тестовый пользователь должен
создавать/удалять временные БД и устанавливать btree_gist. Это не production URL.

```sh
python3 -m venv .venv
.venv/bin/pip install -r tests/requirements.txt
export PATH="$PWD/.venv/bin:$PATH"

docker run -d --name forma-test-postgres \
  -e POSTGRES_USER=forma -e POSTGRES_PASSWORD=local-test-only \
  -e POSTGRES_DB=forma -p 127.0.0.1:55432:5432 postgres:16-bookworm
export INTEGRATION_DATABASE_URL='postgres://forma:local-test-only@127.0.0.1:55432/forma?sslmode=disable'
export PG_TEST_CONTAINER=forma-test-postgres
make check
make integration
make race
```

Полный suite: 20 конкурентных запросов между HTTP-инстансами и отдельными
OS-процессами; 1/2 кабинета; идемпотентность; общий телефон; privacy/RBAC;
назначения/косвенное вытеснение; завершение/undo; отсутствие; публикация;
откаты; up/down/up; backup restore; 100000 записей и EXPLAIN; все 79 операций
OpenAPI с независимой JSON Schema validation. Без `INTEGRATION_DATABASE_URL`
интеграционные тесты пропускаются, это не проверка готовности выпуска.
Для restore drill необходим `PG_TEST_CONTAINER`. Синтетические данные тестов
не копируются в рабочую студию.

```sh
go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
govulncheck ./...
make generate  # после согласованного изменения api/openapi.yaml
```

CI запускает проверки на PostgreSQL 16 и 18, включая `-race` и restore drill.
Frontend в этой задаче не меняется; инструкции адаптера —
[frontend-handoff.md](../docs/implementation/frontend-handoff.md).
