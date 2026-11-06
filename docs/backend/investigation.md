# Frontend audit — 2026-11-06

Actual frontend SHA: 1bdc8c63eb0db12445e4a6dc0bf717967df0d9cf. The main branch has only a README. Investigated demo-api.js, demo-data.js, app.js, admin.js, planner.js, draft-store.js, booking-form.js, calendar-event.js and docs/DEMO_API.md.

| File | Verified role |
|---|---|
| [dist/js/demo-api.js:1–345](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L1-L345) | 57 methods, SiteAPI alias, localStorage key formaTela.demo.v1, JSON migrations v1→v3, validation, scheduling |
| [dist/js/demo-data.js:1–18](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-data.js#L1-L18) | 3 services; default 09:00–21:00, 120-minute step, 0-minute buffer; RUB |
| [dist/app.js:70–116](https://github.com/yomezza/forma.flow/blob/frontend/dist/app.js#L70-L116) | public service/date/slot and site content/settings reads |
| [dist/js/booking-form.js:38–57](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/booking-form.js#L38-L57) | client name, +7 phone, consent, createBookingRequest, handles SLOT_UNAVAILABLE |
| [dist/js/admin.js:1–348](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L1-L348) | bookings/clients/services/schedule/vacations/site settings and demo import-export |
| [dist/js/planner.js:1–185](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/planner.js#L1-L185) | mode switch preview, drafts, selective publication, schedule history and restore |
| [dist/js/draft-store.js:1–13](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/draft-store.js#L1-L13) | unsaved forms in sessionStorage; clients edits kept only in memory |
| [dist/js/calendar-event.js:15–85](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/calendar-event.js#L15-L85) | .ics/Google Calendar/QR created locally; QR contains no customer PII |

Implemented: status transitions, client matching by normalized telephone, fixed booking snapshots, simple/advanced calendar, vacation only in simple, schedule publication, previews, change history (last 30 actions), computed out-of-schedule warning.

Partially implemented / deviations:
1. Web Locks serialize only supporting same-browser tabs; other browsers/devices cannot share localStorage. Use PostgreSQL constraints.
2. Preview and publication are separate unversioned actions. Use expectedRevision and transactional conflict recalculation.
3. Slot timing and past checks use visitor browser timezone. Server must use cabinet IANA timezone.
4. validate checks booking collision but not minimum break in its global final validation. DB reservation range must include buffer.
5. No server admin authentication. The demo exports personal data.
6. Phone is a search key, not proof of identity. Existing client name must not silently be overwritten by any unauthenticated booking.
7. Initial settings may have no timeZone; require configured IANA zone before real bookings.
8. Anonymous createBooking produces confirmed without client; public production API must use createBookingRequest instead.
9. deleteService physically removes services while historical snapshots remain; in SQL prefer archiving for FK integrity.
10. deleteBooking, clearBookings, clearClientsAndBookings, resetDemoData, importData are local demo tools, not suitable production operations.
11. Advanced mode does not honor vacation entries, because effective() uses vacations only for simple; preserve until explicit product decision.

Planned, not implemented: Go backend, PostgreSQL database, admin sessions, cross-device consistency, API idempotency, durable audit, operational backups. See [api-mapping.md](api-mapping.md) for the complete inventory and [the cross-branch source index](frontend-source-links.md) for exact file/line references.

Additional verified code issue: getDateStates(a,b,id) tests (!id || !available(...).length) and therefore reports working dates as full when the optional service id is absent; frontend should correct the adapter semantics rather than copy this expression to Go.

## User-approved divergence from current JavaScript DemoAPI

The current frontend's normalized phone is unique and its findClientByPhone returns a single profile. This is now **superseded as a production requirement**: one number may be shared by multiple clients. The Go/PostgreSQL plan allows duplicate phone values, preserves client UUID identities, and leaves public pending bookings unlinked until an administrator assigns a profile explicitly. This change is architectural documentation only; the frontend source still has the old behavior. No separate contact-person entity is wanted.

These source links target the live `frontend` branch. The audited snapshot is the frontend commit SHA recorded at the top. Architecture changes listed here are **not** edits in the frontend branch.

## Идея 01: обнаруженные пробелы в существующем frontend

[Согласованные сотрудники, кабинеты и роли](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L52) сейчас **отсутствуют** в фактическом JavaScript DemoAPI. `available()` знает лишь студийное расписание и прежнюю единственную вместимость; нет специалистских расписаний, квалификаций, распределителя всех пересекающихся заявок, учётных записей с двойной ролью и собственного специалистского пространства. В Go-проекте `resource_id` теперь означает студию, а старый глобальный `bookings_no_overlap` заменён на два EXCLUDE по назначенным ресурсам + studio FOR UPDATE с совместным solver. Подробнее — [data-model.md](data-model.md), [scheduling-engine.md](scheduling-engine.md) и [OpenAPI](openapi.yaml). Исторические ссылки на frontend описывают старую демо-реализацию, а не готовый backend.
