# Идея 01 — реализация на Go + PostgreSQL

**Статус: проект для backend-разработчика, не реализованный сервис.** Первоисточник — [идея 01: назначения](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L26), [отсутствия](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L28-L34), [роли и личный кабинет](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L36-L43), [завершение](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L45-L52). Исполняемая демонстрационная спецификация — [demo-api-spec](demo-api-spec.md), совместный алгоритм — [assignment-engine](assignment-engine.md). Этот документ **изменяет прежнюю архитектуру**, но не правит ветки `main`, `frontend` или `ideas`.

## 1. Решение в одном абзаце

Оставить **один студийный resource** с одним опубликованным расписанием, но добавить **много специалистов и кабинетов внутри студии**. Бронирование `pending_confirmation` может не иметь ни специалиста, ни кабинета; `confirmed` также может быть без них, но **должно иметь явно привязанного клиента**. Публичный API не спрашивает сотрудника. Два конкретно назначенных сеанса не могут конфликтовать по одному специалисту или одному кабинету; несколько неназначенных обязательств должны проходить **общий SAT-поиск распределения** в транзакции. Изменение графиков, назначения, бронирование и доступность используют единые правила.

## 2. Что именно перестаёт быть верным в исходной архитектуре

| Было до идеи 01 | Теперь |
|---|---|
| `resources` воспринимался как одна ограниченная по времени вместимость | `resources` — студия; в ней 1..N `rooms` и 0..N `specialists` |
| `bookings_no_overlap EXCLUDE(resource_id,occupied)` запрещал любые пересечения в студии | Удалён. Два отдельных `EXCLUDE` на `specialist_id` и `room_id` для **закреплённых** заявок; общая вместимость неназначенных обеспечена транзакционным solver |
| Статус `completed` мог ставить администратор | Только назначенный специалист, после окончания, с возможностью отозвать собственную отметку и обязательным аудитом |
| Один admin role в `admin_users` | `staff_accounts` + `account_roles`, один аккаунт с `admin` и `specialist`, привязка к специалисту |
| Один расписанный кабинет без специалистов | Расписание **студии** (старое simple/advanced) + недельный график **каждого** специалиста + overrides + утверждённые отсутствия |
| Публичное бронирование блокируется только по времени одного ресурса | Блокируется, только если новый запрос нельзя **совместно** разместить с действующими обязательствами |

Справочный DDL уже обновлён в [database-schema.sql](../../database-schema.sql); ER и индексы — [er-diagram.md](../../er-diagram.md), [database-indexes.md](../../database-indexes.md).

## 3. Постоянные таблицы и ограничения

**Сохранены:** clients с **неуникальным** телефоном, services, bookings, schedule_state, simple_schedule, studio vacations, advanced published/drafts, events, site settings. Никаких отдельных контактов «записавший человека» нет.

**Добавлены:** `specialists`, `rooms`, `specialist_services`, `specialist_weekly_intervals`, `specialist_day_overrides`, `specialist_override_intervals`, `absence_requests`, `account_roles`, `booking_assignment_events`, `booking_completion_events`, `capacity_exceptions`. `admin_users`/`admin_sessions` преобразованы в `staff_accounts`/`staff_sessions`.

**В bookings добавлены** nullable `specialist_id`, `room_id` и `assignment_updated_at`, `completed_by_specialist_id`, `completed_at`. Два первых всегда **оба NULL или оба NOT NULL**. При назначении админ выбирает сотрудника и при необходимости кабинет выбирается автоматически. Нельзя сохранять «закреплённый кабинет без исполнителя». Две composite FK `(specialist_id,resource_id)` и `(room_id,resource_id)` защищают от назначения ресурса из другой студии. Существующие UUID и снимки услуги/клиента сохраняются.

**Строгие ограничения DB:**
- partial GiST `bookings_specialist_no_overlap`: `(specialist_id, tstzrange(starts_at,blocked_until,'[)'))` для `specialist_id IS NOT NULL` и status pending/confirmed/completed;
- partial GiST `bookings_room_no_overlap`: аналогично для `room_id`;
- `blocked_until = ends_at + buffer_minutes`, `ends_at = starts_at + duration_minutes` — CHECK;
- FK + CHECK состояний, PK/UNIQUE ID, отсутствие удаления связанных историй;
- интервалы недельных и дневных правил не перекрываются внутри специалиста и дня (`EXCLUDE`);
- **невозможно** выразить выполнимость всех неназначенных заявок одной EXCLUDE: это глобальная задача выбора исполнителей и кабинетов; её гарантирует **сервис + PostgreSQL serializing row lock**.

**Законные исключения:** одобренный отпуск, архивирование или ручное назначение с явным предупреждением могут оставить часть прошлых обязательств временно невыполнимыми. Это состояние **не меняет статус** записи и не удаляет её. Периодные предупреждения хранят причинные события, но UI вычисляет актуальную проблему по опубликованным правилам и живым назначениям.

## 4. Полная модель распределения в Go

Предлагаемый пакет `internal/assignment` — чистый алгоритм:
~~~go
type Allocation struct { SpecialistID, RoomID string }
type Job struct {
    BookingID, ServiceID string
    Start, OccupiedUntil time.Time
    Fixed *Allocation
}
type SolveResult struct {
    Feasible bool
    Unknown bool
    Allocations map[string]Allocation // свидетельство, не новое назначение
    ConflictBookingIDs []string
    Reason string
}
func Solve(ctx context.Context, jobs []Job, eligible func(Job) []Allocation,
    limit int) (SolveResult, error) // чистый пакет, без SQL и HTTP
~~~

Реализация должна сначала загрузить **все записи, связанные перекрытиями** (транзитивная компонента), а не только имеющие пересечение с кандидатом. Домены пар строятся из опубликованного студийного расписания, личных интервалов, утверждённых отсутствий, соответствия услуги и активных кабинетов. Закреплённая пара имеет одно допустимое значение; старое проблемное назначение остаётся фактом и порождает предупреждение, не молчаливое перераспределение.

**Порядок поиска:** минимальный текущий домен, forward-checking, проверка неравенства specialist/room для пересекающихся занятых диапазонов, ограничение числа узлов, context cancellation. `UNKNOWN` из-за лимита — отказ новому запросу `CAPACITY_CHECK_LIMIT`, но **не** доказательство `UNSAT`. Доступность в HTML — лишь предпросмотр; транзакционный повтор — обязателен. См. [разбор и тесты](assignment-engine.md).

**Исключения после явного override:** разрешено только косвенно сделать **другие неназначенные** заявки невыполнимыми. Прямые конфликты двух закреплений по room/staff и неподходящий/отсутствующий специалист **всегда** запрещены. `capacity_exceptions` фиксирует пострадавшие booking ID, причинное действие и актор. Публичное создание в связной компоненте с открытым исключением не допускается; соседние не пересекающиеся временные компоненты продолжают работать. Когда администратор исправляет назначения, исключение помечается resolved и предупреждения пересчитываются.

## 5. Конкурентные операции PostgreSQL — обязательный алгоритм

**Важное изменение:** прежнего `SELECT schedule_state FOR SHARE` недостаточно для неназначенных заявок, потому что они **не подпадают под GiST EXCLUDE**. Поэтому **все операции, меняющие вместимость студии**, получают `schedule_state FOR UPDATE` по `resource_id`, а потом делают расчёт и запись. Это сериализует разные Go-инстансы через PostgreSQL. Для небольшой студии одна студийная строка — приемлемый уровень конкуренции, позднее можно оптимизировать по дате без изменения семантики.

### POST /booking-requests

1. Вне транзакции: валидация, нормализация, SHA-256 канонического payload, календарная дата/время в кабинете; rate limit.
2. BEGIN READ COMMITTED; `INSERT idempotency_keys ... ON CONFLICT DO NOTHING`, `SELECT ... FOR UPDATE`; если тот же hash и booking_id — вернуть прежний receipt.
3. `SELECT schedule_state WHERE resource_id=$1 FOR UPDATE`; прочесть `capacity_revision`, опубликованный студийный график, специалистов, квалификации, личные правила, утверждённые отсутствия, кабинеты и все `pending_confirmation/confirmed` в связной компоненте будущего сеанса.
4. Запустить matcher с кандидатом (clientId NULL, specialistId NULL, roomId NULL); при `UNSAT` вернуть `SLOT_UNAVAILABLE`/409, при `UNKNOWN` — `CAPACITY_CHECK_LIMIT`/503 с предложением другого времени. При открытом исключении в компоненте — `CAPACITY_UNRESOLVED`/409.
5. `INSERT bookings` с пустыми назначениями, уже зарезервированным временем, snapshot ФИО/телефона и `pending_confirmation`; `INSERT booking_events`; записать `idempotency_keys.booking_id`; `UPDATE schedule_state SET capacity_revision = capacity_revision+1`; COMMIT.
6. Только после COMMIT вернуть 201; replay — 200. **Ни одна другая транзакция бронирования/назначения не проходит студийную блокировку параллельно**. Частичные ресурсы не резервируются физическими FK до назначения; обязательство на capacity существует за счёт solver и serial lock.

### PUT /admin/bookings/{id}/assignment

1. Предпросмотр GET/POST вычисляет revision, прямые/косвенные конфликты и конкретные affected IDs, **не резервирует** ресурсы.
2. BEGIN; по `booking_id` узнать `resource_id` без блокировки (resource_id неизменен), `schedule_state FOR UPDATE`; сверить `expectedCapacityRevision`; затем `SELECT booking FOR UPDATE`.
3. Проверить статус и целевую пару; при `roomId=NULL` выбрать лучший совместимый активный кабинет по `sort_order,id`, с min косвенного ущерба. **Нельзя** выбрать архивного/отсутствующего/некомпетентного специалиста, занятую пару/кабинет. Пересчитать глобальный matcher с учётом **фиксированных старых назначений** и новых.
4. Если косвенно пострадали только неназначенные booking IDs — вернуть preview при отсутствии `acknowledgedBookingIds`; после явного подтверждения проверить **точное совпадение** этих IDs, внести причинные записи `capacity_exceptions`. Если прямой конфликт — 409 без override.
5. UPDATE `bookings.specialist_id/room_id/assignment_updated_at`. Отдельно INSERT `booking_assignment_events` (before/after, actor, affected IDs, override), INSERT `audit_log`; bump capacity_revision; COMMIT. GiST EXCLUDE при попытке двойного закрепления гарантирует 23P01→409 даже при обходе алгоритма.

**Другие операции под тем же studio-lock:** `cancelBooking`, `restoreBooking`, перенос времени (если будет), `setSpecialistServices`, `setSpecialistSchedule`, `updateRoom.isActive`, `archiveSpecialist`, `approveAbsence`, `publishScheduleRange`, `setScheduleMode`, изменение перерыва и студийных часов. Изменения только карточки клиента не требуют studio capacity lock. Порядок блокировок: idempotency (если операция с ключом) → schedule_state FOR UPDATE → booking FOR UPDATE → affected staff/room rows. Лишние чтения внутри транзакции ограничивать запросами по bounded time window. Не делать сетевые вызовы под блокировкой.

**Внимание:** транзакции могут законно оставлять уже существующие назначения «вне личного расписания» после admin-approved leave; FK/EXCLUDE защищают **прямое двойное закрепление**, но «прошлый отпуск» должен сохраняться как вычисляемое предупреждение.

## 6. Статусы, личная завершённость и аудит

- `pending_confirmation → confirmed` — admin после привязки `client_id`; **специалист не обязателен**.
- `confirmed → completed` — только текущий назначенный специалист, `now >= ends_at` с часовым поясом кабинета уже учтённым в UTC; при атомарном переходе установить `completed_by_specialist_id`, `completed_at`, запись `booking_completion_events(action='completed')`, booking_events, audit_log. Переход завершающего admin недопустим без переключения в specialist роль.
- `completed → confirmed` — только тот же специалист, чей `completed_by_specialist_id`, сколько бы времени ни прошло; удалить `completed_at` (но оставить `completed_by_specialist_id` как последнего автора), INSERT `booking_completion_events(action='undone')` + booking_events + audit_log. Прежняя история неизменна. Требуется проверка, что ни новая назначенная бронь, ни действующая студийная вместимость не конфликтуют; в практике старое прошедшее время не бронируется публично, но exclusion защищает повтор.
- `confirmed → cancelled`, `pending → cancelled` — admin; `cancelled → pending_confirmation` — admin после solver и EXCLUDE. Отмена освобождает capacity, но сохраняет историю/назначение; при restore проверять текущую пару или сначала явно сбрасывать назначение с аудитом.
- Переназначать `completed` и `cancelled` нельзя. При архивации или утверждении отсутствия сохранить ссылку на специалиста в старой брони и отправить предупреждение `REQUIRES_REASSIGNMENT`.
- Прежний специалист видит в своём журнале **только событие** до/после собственного назначения, но не чужую активную запись или телефон. Все PII в audit/details минимальны.

## 7. Auth и privacy

`staff_accounts` (бывшие `admin_users`) + `staff_sessions` (бывшие `admin_sessions`) + `account_roles`: у одного UUID несколько ролей `owner|admin|specialist`. Ссылка `specialists.account_id` позволяет конкретному специалисту читать только `booking.specialist_id===myProfileID`. Серверная роль выбирается из реально назначенных account_roles; заголовок `X-Active-Role` — интерфейсное предпочтение, **не** полномочие. Любой `/specialist/*` endpoint дополнительно сверяет свой profile UUID и strip PII при сериализации.

Администратор создаёт сотрудника/аккаунт, сервер выдаёт временный одноразовый секрет защищённым способом, хранит только Argon2id хеш, флаг `must_change_password`; при первом входе доступны только смена пароля и выход. HTTP cookie: Secure, HttpOnly, SameSite и CSRF для writes, ограничение попыток, истечение сессии, инвалидация при архивации/блокировке. События invite/сброса парольного доступа аудируются без пароля в логах. В localStorage настоящих паролей нет.

Специалисту отдаются **новые DTO** `SpecialistBookingSummary` только с `id,clientName,serviceName,date,time,durationMinutes,status,roomName`. Никаких `clientPhone`, `phoneNormalized`, `clientId`, полномочий клиентов и booking exports. Админ видит полную карточку. Публичные slots и booking receipt не содержат сведения о специалисте или кабинете.

## 8. Пакеты Go

~~~text
backend/internal/
  domain/                  # IDs, status, value types, intervals
  assignment/{solver.go,domains.go,conflicts.go}  # чистый совместный поиск
  specialist/{service.go,repository.go}           # профиль, квалификации, графики
  room/{service.go,repository.go}                 # кабинеты
  absence/{service.go,repository.go}              # заявки и одобрение
  booking/{service.go,repository.go}              # запись, назначение, завершение
  schedule/{engine.go,service.go}                 # published studio rules + candidate times
  auth/{service.go,roles.go}                      # аккаунты, роли, временный пароль
  storage/postgres/                               # pgx, transactions, SQL
  transport/http/                                 # net/http, DTO, role middleware, errors
~~~

~~~go
type AssignmentPreview struct {
    Allowed bool
    RequiresConfirmation bool
    CapacityRevision int64
    SpecialistID, RoomID string
    AffectedBookingIDs []string
    Reason string
}
type AssignBookingCommand struct {
    BookingID, SpecialistID string
    RoomID *string
    ExpectedCapacityRevision int64
    AcknowledgedBookingIDs []string
    ActorAccountID string
}
type AbsenceDecision struct {
    RequestID, ActorAccountID string
    ExpectedCapacityRevision int64
    AcknowledgedBookingIDs []string
    RejectionReason string
}
~~~

Use `context.Context`, typed `errors.Is/As`, one transaction boundary per command, injectable clock for time comparisons. `solver` has no pgx import and is unit-tested independently. Source [demo logic](assignment-engine.md) defines behavior for the frontend and the Go implementation.

## 9. OpenAPI, tests and rollout

[OpenAPI 3.1](../../openapi.yaml) and [API map](api-mapping.md) contain proposed routes for all important new actions: `/admin/specialists`, `/admin/rooms`, `/admin/absence-requests`, `/admin/bookings/{id}/assignment`, `/specialist/me/*`. All admin routes are authenticated and CSRF-protected, specialist routes are scoped to own profile. Full tests: [go-testing.md](../../go-testing.md), and [idea 01 test matrix](go-backend-tests.md).

**Deployment:** migrate database forward, preserve UUIDs and original bookings, rename existing admin account/session tables rather than create duplicate accounts, seed one real room and one specialist (without automatically pretending to know who completed historical work), then deploy backend and update frontend adapter. Do not start public booking using the old resource-wide EXCLUDE logic or without the serialized capacity check. [Migration plan](postgres-migration-guide.md).

## 10. Explicitly deferred business questions

Before production: actual IANA TZ, distribution of staff across locations (MVP one studio), acceptable solver load budget and cache/queue behavior, consent and retention policy, advanced-mode vacations. None is a reason to replace agreed status/assignment separation or share-phone client rule.
