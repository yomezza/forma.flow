# Публичная доступность и движок размещения на Go

Источники: [опубликованное студийное расписание](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L122-L138), [новая идея](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L25). Правила проекта: [business-rules.md](business-rules.md) и [database-operations.md](database-operations.md).

## Входы

`AvailableSlots(ctx, resourceID, date, serviceID)`: active service и duration, IANA zone студии, опубликованные studio simple/advanced интервалы (никаких drafts), шаг/перерыв, все активные кабинеты, active специалистов и `specialist_services`, их недельные интервалы и overrides, утверждённые отсутствия, pending/confirmed заявки с закреплёнными `specialist_id/room_id` либо NULL, и открытые capacity exceptions. Зафиксированный `Clock` используется для отсечения прошлого; дата — локальная дата студии, starts_at/blocked_until — UTC.

## Генерация слотов

1. Определить студийный режим. Simple: workDays, blockedDates, vacations; advanced: только published working/off, unplanned закрыт. Личные отпуска **дополнительно** ограничивают конкретных специалистов.
2. Для каждой смены студии генерировать времена по step; кандидат обязан целиком помещаться в одном опубликованном интервале и быть в будущем. Преобразовать по IANA TZ и отвергать DST gap/fold.
3. Получить личные доступные интервалы каждого специалиста (week или date override) минус **approved** absence. Кандидат подходит только квалифицированному активному специалисту.
4. Сформировать задачи с интервалами занятости `[starts_at,blocked_until)` и доменами пар `(specialistId,roomId)`. У закреплённой заявки домен — только её текущая пара: solver не вправе незаметно переназначать. Все связные по пересечениям задачи должны иметь совместное распределение. Проверять **транзитивную** компоненту кандидата.
5. Чистый `internal/assignment` реализует backtracking с minimum remaining values, forward-checking, deterministic ordering, лимит узлов и `context.Context`. Возвращает SAT/UNSAT/UNKNOWN. SAT → показать слот, UNSAT → скрыть слот, UNKNOWN → не обещать его и отдать ограничение расчёта.
6. Если компонента кандидата пересекается с открытой capacity exception (результат разрешённого админом ущерба), не выдавать новые брони туда до разрешения; независимые компоненты остаются доступными.
7. В POST повторить проверку в транзакции под `schedule_state FOR UPDATE`. Предпросмотр доступности не резервирует ничто; создание pending резервирует **вместимость**.

### Важные границы

- 1 специалист, 1 кабинет → в 10:00 и 10:30 два сеанса одновременно невозможны.
- 2 кабинета и 2 специалиста → два сеанса в 10:00 возможны при совместимой квалификации.
- S1 умеет A/B, S2 только A, одновременно A и B → A должен получить S2, B → S1. Жадный выбор S1 для A привёл бы к ложному отказу.
- Длительность 60, break 15: занятость до 11:15, следующий старт ровно 11:15 допустим.
- A пересекает B, B пересекает C, A не пересекает C → одна связная компонента проверки.
- Pending и confirmed участвуют; cancelled исключён; completed остаётся историческим фактом, не обязателен для будущей публичной вместимости (DB EXCLUDE сохраняет его диапазон).
- При устаревшем графике старая бронь сохраняется с `OUTSIDE_SCHEDULE`/ `REQUIRES_REASSIGNMENT`, но **для новой заявки** нельзя использовать недоступного закреплённого специалиста как свободный слот.

## Go-граница

~~~go
type Allocation struct { SpecialistID, RoomID string }
type AssignmentJob struct {
    BookingID, ServiceID string
    StartsAt, BlockedUntil time.Time
    Fixed *Allocation // nil у неназначенных
}
type SolveResult struct {
    Feasible, Unknown bool
    Witness map[string]Allocation
    Reason string
    AffectedBookingIDs []string
}
func Solve(ctx context.Context, jobs []AssignmentJob,
    domains map[string][]Allocation, nodeLimit int) (SolveResult,error)
~~~

Пакет solver **не импортирует** pgx и HTTP, получает все данные на вход. UNKNOWN из-за time/node limit — не SAT и не доказанный UNSAT; новый POST отклоняется с `CAPACITY_CHECK_LIMIT` 503, а GET не заявляет доступность. Админский preview может отдельно найти минимальные ID косвенно пострадавших **неназначенных** задач; прямой конфликт ресурсов и статусы игнорировать нельзя.

Пакет `schedule` строит временные кандидаты и вычисляет дату `past/working/full/off/unplanned/vacation/blocked`, пакет `assignment` проверяет общую совместимость. Публичная API не отдаёт witness, ФИО, specialist ID и room ID. Случайные SQL запросы на каждый слот запрещены — данные пакетно подгружаются по диапазону с верхними лимитами.

Модульные и PostgreSQL-тесты: [go-testing.md](go-testing.md).
