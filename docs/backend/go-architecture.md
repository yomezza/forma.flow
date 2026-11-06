# Go-архитектура FORMA TELA — модульный монолит

Актуальная модель учитывает [идею 01: специалисты и кабинеты](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L52) и [сохранение нескольких клиентов на одном телефоне](business-rules.md). Старый подход «один resource = один исполнитель/кабинет» заменён: `resources` — студия, `specialists` и `rooms` — независимые сущности внутри неё.

~~~text
backend/
  cmd/server/main.go
  internal/
    config/
    domain/                    # типы, состояния, ошибки, значения времени
    catalog/                   # услуги и их архивирование
    client/                    # независимые UUID-карточки, общий телефон возможен
    booking/                   # заявка, client link, assignment, lifecycle, history
    assignment/                # чистый SAT/UNSAT/UNKNOWN solver
    specialist/                # персонал, квалификации, рабочая неделя, overrides
    room/                      # кабинеты и доступность
    absence/                   # заявки на отсутствие, approve/reject/withdraw
    schedule/                  # опубликованный график студии, кандидаты слотов
    auth/                      # аккаунты, роли, sessions, password lifecycle
    admin/                     # dashboard, блок внимания, site settings
    storage/postgres/          # pgxpool, queries, UnitOfWork
    transport/http/            # net/http, handlers, DTO, middleware, errors
  migrations/                  # goose
  api/openapi.yaml
  tests/integration/
  go.mod
~~~

## Границы модулей

HTTP handlers только декодируют JSON, обеспечивают авторизацию, вызывают сервис и формируют HTTP-ответ. `booking` координирует жизненный цикл и транзакции; `assignment` возвращает свидетельство совместной размещаемости и не импортирует pgx; `schedule` строит студийные слоты и поставляет графики; `specialist/absence/room` реализуют административные use cases. Инфраструктурные SQL-запросы сосредоточены в `storage/postgres`. `cmd/server` связывает зависимости. Интерфейсы вводить только там, где они нужны потребителю для тестирования или замены реализации.

## Корректность при параллельных запросах

Любая capacity-changing операция (создание/восстановление/отмена брони, назначение, перенос, смена графика/квалификаций, одобрение отсутствия, архивирование, room activation) выполняется в транзакции с `SELECT schedule_state FOR UPDATE` по ID студии. Это **не FOR SHARE**: несколько неназначенных заявок с NULL специалистом/кабинетом невозможно защитить GiST EXCLUDE без общей сериализации. Внутри блокировки загрузить полный затронутый набор, вызвать `assignment.Solve`, затем записать изменения, повысить capacity_revision и commit. Два EXCLUDE по specialist_id и room_id дополнительно запрещают двойное закрепление даже при ошибке сервиса. Ни `sync.Mutex`, ни один предварительный GET слотов не дают межинстансной гарантии.

Порядок локов: для idempotent POST — idempotency row, затем studio row, затем booking и прочие изменяемые rows; остальные команды — studio row, затем booking, затем staff/room. Внешние HTTP вызовы в tx запрещены. `context.Context` первым параметром, deadlines, `errors.As` для SQLSTATE 23P01/23505/40001/40P01, `log/slog` без PII.

## Время, безопасность, deployment

Время — UTC timestamptz в БД, локальная дата/час относительно IANA resource.TimeZone. Переходы DST валидируются явно. Для password использовать Argon2id, аккаунты `staff_accounts` с отдельным `account_roles`; один аккаунт может быть одновременно admin и specialist. **Активный интерфейсный режим роли не является авторизацией**. Персональному кабинету выдавать только профильные DTO без телефонов.

Конфигурация: DATABASE_URL, HTTP_ADDR, PUBLIC_ORIGIN, COOKIE_SECURE, IANA studio TZ, pool/statement/lock timeouts, solver budget, параметры сессий. Отдельный миграционный этап при деплое, readiness и graceful shutdown. Никакой необходимости в микросервисах, ORM, Redis или Node runtime для этого MVP.

Соседние документы: [Go типы](go-domain-models.md), [контракты](go-service-contracts.md), [SQL](database-schema.sql), [транзакции](database-operations.md), [тесты](go-testing.md).
