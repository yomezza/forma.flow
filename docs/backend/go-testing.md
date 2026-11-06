# Go testing strategy and critical cases

Use go test standard table tests for pure domain/schedule engine; httptest for HTTP; PostgreSQL 16–18 ephemeral integration DB per suite for constraints and transactions. Never rely only on mocks for concurrent correctness.

## Unit / table-driven examples

~~~go
package schedule
import ("testing"; "time")
func TestOverlap(t *testing.T) {
    base := time.Date(2026,11,10,9,0,0,0,time.UTC)
    cases := []struct{name string; a0,a1,b0,b1 int; want bool}{
        {"overlap",0,75,60,120,true},
        {"touching boundary",0,60,60,120,false},
        {"buffer overlap",0,75,60,120,true},
        {"disjoint",0,60,90,150,false},
    }
    for _,tt:=range cases { t.Run(tt.name,func(t *testing.T) {
        a0:=base.Add(time.Duration(tt.a0)*time.Minute)
        a1:=base.Add(time.Duration(tt.a1)*time.Minute)
        b0:=base.Add(time.Duration(tt.b0)*time.Minute)
        b1:=base.Add(time.Duration(tt.b1)*time.Minute)
        if got:=Overlaps(a0,a1,b0,b1);got!=tt.want {t.Fatalf("got %v want %v",got,tt.want)}
    })}
}
~~~

Test matrix:
- Simple 09:00–12:00, step 30, duration 60, no bookings: 09:00,09:30,10:00,10:30,11:00.
- 10:00–11:00 occupied with break 0: 09:00 and 11:00 still allowed, 09:30/10:00/10:30 blocked.
- Same booked span with break 15: 09:00 and 11:00 also blocked because buffer makes intervals intersect.
- Work intervals [09:00,10:00) and [11:00,12:00), duration 90: zero slots, do not combine intervals.
- Advanced without published day: unplanned, no slots even when simple weekly day is open.
- Advanced draft working not published: no public slots; publish one date only changes that date.
- BlockedDate or vacation in simple: no slots; two inclusive vacation periods touching same day are rejected.
- 2026-03-29 Europe/Berlin at 02:30 local: gap rejected; 2026-10-25 at 02:30: fold rejected. Other normal dates unaffected.
- Date/current time cutoff uses cabinet IANA zone and injected Clock, not host process location.
- Pending and confirmed reserve; cancelled frees; completed remains protected as historical event.
- Phone normalizes from 8/+7/10 digits but is NOT a unique client identifier. Multiple clients (even identical fullName and phone) have separate UUIDs and histories. Public booking sets client_id=NULL; admin candidate search returns all matches, explicit linking precedes confirmation. Malicious or mistaken same-phone input must never auto-link or overwrite a client.
- Status: each allowed transition, all forbidden pairs, cancelled restore collision, status no-op.
- Change active mode retains independent settings, preview conflicts, out-of-schedule warnings don't change status.
- Publish range with stale expectedRevision returns 409 and writes nothing; restore history goes to draft only.
- Data validation: overlapping intervals, bad step, invalid date (Feb 30), start>=end, negative price, invalid phone, missing consent/version.

## PostgreSQL concurrency integration

Start 20 goroutines with separate pgxpool connections + barrier, unique idempotency keys, same resource/date/time; expect exactly 1 committed booking, 19 SLOT_UNAVAILABLE due to the locked global matcher; SQLSTATE 23P01 is tested separately for two concurrent **assigned** bookings sharing a specialist or room. Repeat across two independent server instances. Same key and identical body requested by 20 goroutines returns same booking ID exactly once; same key changed payload => IDEMPOTENCY_CONFLICT. Force rollback after booking INSERT: no partial booking/idempotency rows. Force rollback after new-client INSERT during admin assignment: no orphan profile and no linked booking. Verify cancelled time reusable and restore conflict rejected. Validate exclusion with a different start but overlapping buffer, not just identical start.

## HTTP tests

httptest.NewRecorder / httptest.NewServer: JSON decode and body limit, auth cookie/CSRF on all admin writes, 401/403/422/409 envelope, 201/200 idempotency, no PII in public GET, pagination bounds and cursor, correct request timeouts and request ID. Build fixed-clock golden tests for OpenAPI examples.

## Migration and operations tests

Apply goose up into empty database, replay up no-op, validate indices/FKs/exclusion, test down only on throwaway DB. Verify backup restoration and representative 100k booking rows EXPLAIN plans. Test concurrent publish vs booking (schedule_state lock) and serializing conflicting vacation/schedule changes. Run go test -race ./... for code races, but do not confuse this with SQL concurrency tests.

Additional DB invariant tests: INSERT with ends_at inconsistent with duration_minutes must fail CHECK; INSERT with blocked_until inconsistent with buffer_minutes must fail; exactly one non-cancelled booking may occupy an overlapping span after race.

## Tests: shared family phone

1. Clients A and B share +79991234567; two INSERTs succeed, different UUIDs.
2. Public booking1 (Ivan) and booking2 (Maria) use the same phone but different times; client_id=NULL for both, status pending_confirmation, slots reserved.
3. Admin candidate GET finds both profiles and returns array/page; public endpoint never lists profiles.
4. Explicitly link booking1→A, booking2→B; each GET /admin/clients/{id}/bookings shows only that person's history, including cancelled entries.
5. Confirm pending without client link returns 409 CLIENT_LINK_REQUIRED, after assignment it succeeds.
6. Cancel unlinked pending succeeds; restore rechecks slot and remains pending.
7. Create profile C with identical name and phone to profile A succeeds; no automatic merge or dedup.
8. Audit log records original/new UUID on corrected association. Studio-locked solver prevents excess unassigned reservations, independent of client_id; GiST EXCLUDE prevents direct double assignments.

## Идея 01 — обязательные Go/PostgreSQL тесты

Требования: [идея 01](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L52). Без этих тестов нельзя утверждать, что несколько специалистов поддерживаются корректно.

| Кейс | Условия | Ожидаемый результат |
|---|---|---|
| Одна студия, один кабинет | 20 одновременных POST на 10:00; 1 специалист, 1 кабинет | 1 успех, 19 SLOT_UNAVAILABLE; не только для одного Go-инстанса |
| Несколько кабинетов | 2 специалиста и 2 кабинета, оба свободны | 2 параллельных сеанса в одно время допустимы; третий нет |
| Глобальная задача вместо greedy | A,B услуги; S1 умеет A+B, S2 только A; параллельно заявлены A и B | SAT (S1→B, S2→A) даже если greedy ошибочно назначил бы S1→A |
| Неназначенные пересечения | 2 заявки с NULL specialist_id/room_id в 1 кабинет | Вторая отклонена solver под studio FOR UPDATE, хотя DB EXCLUDE не покрывает NULL |
| Занятый назначенный специалист | один сотрудник на двух пересекающихся назначениях, разные кабинеты | DB 23P01 от specialist EXCLUDE, 409 |
| Один назначенный кабинет | 2 специалиста в одном кабинете одновременно | DB 23P01 от room EXCLUDE, 409 |
| Связная компонента | A пересекает B, B пересекает C, A не пересекает C | solver учитывает все три, а не только непосредственных соседей |
| Косвенное вытеснение | админ закрепляет A за S1, заявка B без исполнителя остаётся без решения | preview требует подтверждение ID B; после подтверждения сохранить предупреждение, status B неизменён; новые слоты в той компоненте недоступны |
| Прямой конфликт | два закрепления на одного специалиста/кабинет | нельзя override даже с подтверждением |
| Отпуск специалиста | pending absence vs approved absence с назначенным сеансом | pending не влияет, approved оставляет старый booking и выдаёт REQUIRES_REASSIGNMENT |
| Двойная роль аккаунта | admin + specialist в одном аккаунте | admin не завершает сеанс; specialist-контекст делает это лишь для своего назначенного сеанса |
| Раннее завершение | confirmed, но now<ends_at | 409 NOT_ENDED_YET |
| Отмена завершения | completed автором S1; повторный вход через месяц | S1 может вернуть confirmed с новым completion event; S2 и admin не могут |
| Приватность | GET /specialist/me/bookings и истории переназначений | нет clientPhone, clientId, чужих/неназначенных bookings даже в JSON дополнительных полях |
| Общий телефон | два клиента с одинаковым телефоном | два разных client UUID; специалист не видит телефоны |
| Гонки с публикацией | POST booking параллельно с publish/approveAbsence/archive | studio FOR UPDATE сериализует; либо SAT до изменения, либо корректный конфликт после |
| Лимит matcher | искусственно сложный компонент, deadline/cap | UNKNOWN→CAPACITY_CHECK_LIMIT; нельзя принимать заявку наугад |

Тестировать в PostgreSQL реальный `schedule_state.capacity_revision` на всех влияющих на вместимость операциях; попытаться умышленно обойти проверку с READ COMMITTED+FOR SHARE — тест должен выявить небезопасность старого поведения. Проверить миграцию admin_users→staff_accounts с сохранением FK и ролей, а также ссылки на completed/archived персонал. Применять `go test -race` для Go гонок **отдельно** от DB многопроцессной конкуренции.
