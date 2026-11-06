# PostgreSQL: операции, блокировки и транзакции

Актуально для идеи [01-specialists](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L52). Основной [DDL](database-schema.sql), [общая доступность](scheduling-engine.md), [контракты Go](go-service-contracts.md).

## Главная гарантия

`resources` — студия, **не** единственный кабинет. PostgreSQL запрещает двойное закрепление через два partial GiST EXCLUDE: `bookings_specialist_no_overlap` и `bookings_room_no_overlap`. Эти ограничения не охватывают заявки с `specialist_id=NULL, room_id=NULL`. Поэтому **каждая меняющая вместимость транзакция** сериализуется через `SELECT ... FROM schedule_state WHERE resource_id=$1 FOR UPDATE`, после чего вызывается общий solver размещения. `FOR SHARE` больше не подходит. Эта гарантия действует между экземплярами Go.

Порядок блокировок: при публичном POST сначала `idempotency_keys`, затем `schedule_state FOR UPDATE`, далее изменяемые booking/специалист/кабинет. У прочих команд сначала прочитать неизменный `resource_id`, затем studio-lock, потом `booking FOR UPDATE`. Избегать инверсии порядка и обращений к внешним API внутри tx. `READ COMMITTED` допустим при обязательном studio-lock для всех capacity writes; EXCLUDE страхует прямые коллизии.

## Публичная заявка — POST /api/v1/booking-requests

1. Проверить данные, обязательное согласие, нормализовать +7 телефон и UTC момент по IANA TZ студии, получить idempotency hash; клиент карточкой **не определяется**.
2. BEGIN; вставить idempotency key с ON CONFLICT DO NOTHING, SELECT FOR UPDATE. При повторе с тем же payload вернуть прежнюю receipt; с новым payload — 409.
3. SELECT schedule_state FOR UPDATE. Получить активный опубликованный режим, услугу, рабочие интервалы специалистов, разрешённые услуги, утверждённые отсутствия, активные кабинеты и пересекающиеся pending/confirmed обязательства.
4. Построить транзитивную компоненту пересечений с новым кандидатом; проверить отсутствие открытой capacity-аномалии в этой компоненте; запустить solver. SAT → продолжить; UNSAT → `SLOT_UNAVAILABLE` (409); UNKNOWN/лимит → `CAPACITY_CHECK_LIMIT` (503) без резервирования.
5. INSERT booking `pending_confirmation`, `client_id=NULL`, `specialist_id=NULL`, `room_id=NULL` с контактным snapshot и consent; INSERT booking_events; связать idempotency key; повысить `capacity_revision` студии; COMMIT.
6. Отдать 201 либо 200 при повторе, только публичную receipt без исполнителей и телефонов.

При одной комнате два пересекающихся запроса не пройдут вместе, хотя оба могут физически иметь NULL assignment в таблице; solver и DB lock дают инвариант. Две комнаты и два совместимых специалиста допускают параллельные сеансы.

## Назначение/переназначение — PUT /admin/bookings/{id}/assignment

Предпросмотр (без записи) возвращает `capacityRevision`, допустимую пару и конкретные ID косвенно пострадавших неназначенных заявок. Сервер при команде **повторно** выполняет его после `schedule_state FOR UPDATE`, проверяя `expectedCapacityRevision`. Затем `booking FOR UPDATE` и статус pending/confirmed. `roomId:null` означает автоматический подбор активной комнаты в устойчивом порядке. Проверки: услуга, смена, отсутствие, уже занятые конкретные specialist/room. Прямые нарушения всегда 409. Только вытеснение других **неназначенных** допускает `acknowledgedBookingIds` со строгим равенством актуальному набору.

Обновить `bookings.specialist_id,room_id,assignment_updated_at`, append `booking_assignment_events` с before/after, append `audit_log`, `capacity_exceptions` для подтверждённого косвенного ущерба, повысить revision; COMMIT. SQLSTATE 23P01 → 409. Статус подтверждения не меняется автоматически. Прежние назначенные записи нельзя молча переставлять ради SAT.

## Привязка клиентской карточки и статусы

`PUT /admin/bookings/{id}/client`: `booking FOR UPDATE`, SELECT выбранного client UUID либо INSERT нового (телефон не уникален), UPDATE `client_id`, audit; snapshots ФИО/телефона не переписывать. `pending_confirmation → confirmed` разрешено администратору при `client_id IS NOT NULL`, **без обязательного specialist_id**. `cancelled → pending_confirmation` вызывает общую проверку capacity и EXCLUDE; отмена освобождает время. Все capacity changes используют studio lock.

`confirmed → completed`: только текущий назначенный специалист (связь authenticated account → specialist UUID), только когда `now >= ends_at`; `booking FOR UPDATE`, UPDATE `completed_by_specialist_id/completed_at,status`, INSERT `booking_completion_events`, `booking_events`, `audit_log`; COMMIT. Специалист, который завершил, может в любой момент отменить **свою** отметку: `completed → confirmed`, новый event `undone`, прежняя история сохраняется. Админ сам не завершает.

## Утверждение отсутствия, редакция графика, архив

Специалист создаёт/отзывает только собственную `pending` absence request — публичная вместимость от этого не меняется. Админ `approveAbsence` в tx: studio-lock, проверка pending, проверка пересечений и подтверждённых ID, `approved`, audit и актуализация `capacity_exceptions`; уже назначенные сеансы остаются со статусом, но отмечаются `REQUIRES_REASSIGNMENT`. `rejectAbsence` требует причины. Архив/отключение кабинета или смена специализаций/личного графика выполняются под студийной блокировкой, оставляют историю и пересчитывают предупреждения. Публикация студийного режима использует **две ревизии**: `schedule_state.revision` для правил и `capacity_revision` для доступности.

## SQL чтения

~~~sql
-- Активные обязательства в окне; загружать одним набором для построения компонент
SELECT id, service_id, specialist_id, room_id, starts_at, blocked_until, status
FROM bookings
WHERE resource_id=$1 AND starts_at < $3 AND blocked_until > $2
 AND status IN ('pending_confirmation','confirmed')
ORDER BY starts_at,id;

-- Заявки конкретного специалиста; не отдавать client_phone в DTO
SELECT id,client_name,service_name,local_date,local_time,status,room_id
FROM bookings
WHERE specialist_id=$1 AND status IN ('pending_confirmation','confirmed','completed')
ORDER BY starts_at,id;

-- Поиск всех карточек по одному телефону, без автоматического выбора
SELECT id,full_name,phone_normalized FROM clients
WHERE phone_normalized=$1 ORDER BY full_name,id;
~~~

Интервалы `[starts_at, blocked_until)` учитывают буфер. При выборе компоненты извлекать все связанные записи (в том числе перекрывающиеся через цепочку); один ограниченный запрос + расширение диапазона до замыкания либо запрос всех записей ограниченной даты с проверкой в Go, **не** только тех, кто непосредственно пересекает кандидата. Угрожающие PII операции не логировать сырыми запросами.

[Тесты конкурентного бронирования](go-testing.md), [миграции](migrations.md).

## Capacity-changing transactions — idea 01

**Critical correction:** selecting `schedule_state FOR SHARE` followed by checking the booking's individual time is **unsafe** after adding unassigned bookings. Two transactions could both accept one available slot while their NULL assignments bypass GiST. For every capacity-changing action acquire `schedule_state(resource_id) FOR UPDATE`, compute the **joint** SAT assignment and then INSERT/UPDATE before COMMIT. `READ COMMITTED` is acceptable only under this lock discipline (and DB exclusion constraints); skip-lock paths are forbidden. Serialize creation, cancellation, restore, assignment/reassignment, specialist service change, specialist week/overrides, absence approval, room activation, studio schedule publication and step/break changes. Increment `capacity_revision` with every such mutation.

**Public POST:** idempotency row first → studio FOR UPDATE → read candidate service, published studio day, room/staff domain, all intersecting pending+confirmed bookings (transitive component) → solver SAT required → INSERT `client_id=NULL,specialist_id=NULL,room_id=NULL` → booking_events/idempotency row update/revision bump → COMMIT. On UNSAT return 409 SLOT_UNAVAILABLE; on solver budget/timeout return 503 CAPACITY_CHECK_LIMIT. The public receipt does not expose matched specialist IDs.

**Admin assignment:** preview returns chosen specialist/room, `capacityRevision`, directly disallowed reasons and indirectly affected unassigned booking IDs. Actual PUT checks expectedCapacityRevision under studio FOR UPDATE, then booking FOR UPDATE; re-evaluates eligibility and solver. Direct busy/off/incompetent/room conflict cannot be overridden (DB 23P01 → 409). If only other **unassigned** requests are indirectly displaced, require explicit acknowledgment of exact affected IDs; persist `booking_assignment_events`, `capacity_exceptions`, audit, assignment columns and bump revision. Assignment does **not** change booking status. Existing historical bookings stay linked when a resource becomes inactive.

**Specialist absence approval / archive:** same studio lock, re-evaluate impacted assigned bookings, preserve assignments, store warnings and audit; admin may confirm documented conflicts without automatically cancelling/rescheduling. Pending absence does not restrict availability. **Completion:** SELECT booking FOR UPDATE and verify assigned specialist, `confirmed` status and `now>=ends_at`; update status, completed_by, completed_at, completion_events/booking_events/audit in one transaction. Undo requires same recorded completer, returns confirmed and creates separate audit/event. Completed remains protected by resource GiST.

**SQL access patterns:** `SELECT ... FROM bookings WHERE resource_id=$1 AND starts_at<$windowEnd AND blocked_until>$windowStart AND status IN ('pending_confirmation','confirmed')`; join/read active specialists, service qualifications, weekly and day-override intervals, approved absences and active rooms in **bounded batches**, not per candidate. `SELECT id FROM schedule_state WHERE resource_id=$1 FOR UPDATE` is the serialization point; transaction commits after all changed tables and audit rows. See [SQL schema](database-schema.sql) and [solver](scheduling-engine.md).


## Идея 01: источник и приоритет

Согласованные требования: [назначения и вместимость](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L26), [личные графики и отсутствия](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L28-L34), [роли](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L36-L43), [завершение и аудит](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L45-L52). Текущий frontend не реализует эти правила; Go/PostgreSQL-проект в данном документе — целевое состояние.
