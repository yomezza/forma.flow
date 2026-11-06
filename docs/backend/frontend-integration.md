# Frontend/backend integration (action list for frontend developer)

Keep all existing public/admin components consuming SiteAPI. Replace its backend implementation, not UI call sites where possible. Use a switch between DemoAPI (local demo) and HttpSiteAPI (production). No direct fetch in components.

## Method adaptations

- getServices and getBookings/getClients/getVacations: HTTP gives paginated {items,nextCursor}; adapter returns arrays where existing UI expects arrays (or update admin list views for paging).
- The production admin client lookup must be plural findClientsByPhone(phone); GET /admin/clients/by-phone returns 0..N profiles, not one chosen by matching phone.
- getAvailableSlots(date,id): GET /availability/slots, unwrap slots[].
- getDateStates: unwrap states map; preserve values past/working/full/off/unplanned/vacation/blocked.
- createBookingRequest: preserve promise API and existing form values but generate one cryptographically random Idempotency-Key per submission attempt, reuse key across transport retries; append consentAccepted and consentVersion after client-side consent check; do not use a new key on automatic retry of same request.
- getSettings: public app should consume safe site settings; admin needs separate simple schedule and site settings. Adapter may combine them into the previous flat shape; enforce auth for private fields.
- updateSettings: split appearance PATCH /admin/settings from schedule PUT /admin/schedule/simple with expectedRevision.
- setScheduleMode, setDraftSchedule, setDraftScheduleRange, publishScheduleRange, restoreScheduleVersion: include expectedRevision. When 409 VERSION_CONFLICT, reload schedule and preview; never silently retry a changed schedule with fresh revision.
- Status change methods: map to dedicated POST confirm/cancel/complete/restore. updateBooking is adapter dispatch, not an unrestricted patch. Before confirm, admin must explicitly link to existing client or create+link via PUT /admin/bookings/{id}/client; 409 CLIENT_LINK_REQUIRED means profile isn't assigned.
- deleteService becomes archive isActive=false; message in admin UI should reflect that.
- exportData is owner-only with explicit warning and authenticated export. importData/resetDemoData/clearBookings/clearClientsAndBookings/deleteBooking/resetServices/resetSettings are demo-only; hide or disable them in production mode and provide explanation.
- getWarning is localStorage-only warning; in HTTP mode provide empty warning or network-specific notice; not a server endpoint.

## Network UX contract

All HTTP errors convert to Error with code, message and optional details. Show distinct offline/timeout errors and allow user retry; when SLOT_UNAVAILABLE refresh slots and clear selected time as current form does. Disable submit while request in flight but keep idempotency token on ambiguous network failure. Never assume previous availability GET means booking is guaranteed.

Use AbortController for component unmount/navigation; avoid cancelling a booking after the server may have committed without retaining idempotency key. Repeated clicks must use the same key, not generate new requests. Date/calendar timezone shown to visitor is cabinet timezone, not automatically user local. Reuse existing client-side QR/.ics calendar generation, preserving no-PII links; no public read-by-booking-id endpoint.

Authentication: build an actual admin login screen and session expiry UX. CSRF token via authenticated session endpoint; send header on mutation. The current footer button only switches views and is NOT a secure admin gate. Keep AdminDraftStore unsaved form state and don't let a stale draft overwrite a newer schedule revision.

## Source mismatches to fix in frontend branch only

1. Demo getDateStates when serviceId omitted currently marks working days as full because expression includes !id; consider conditional handling.
2. Browser timezone is used for local date and slot cutoff; HTTP adapter must use server date states.
3. Existing admin export/import JSON contains personal data; production import must be removed from UI.
4. Demo service deletion is physical; production action is archive.
5. If admin uses getBookings/getClients with large datasets, replace all-at-once rendering with server pagination.
6. Session login/logout and forbidden error handling require new UI; no changes are made in this architecture branch to frontend.

## Admin UX task: several clients sharing one phone

A new request shows submitted name/telephone, remains reserved and unlinked (clientId:null). The admin should view all possible phone matches (possibly none; list IDs/names/history); **never automatically select a profile**, not even the only hit. Provide “Связать с существующим клиентом” or “Создать нового клиента” to associate with the booking; confirm only after assignment. Cancellation works without assigning a profile. Changing an accidental link is allowed only through an audited administrator action. Do not rewrite the booking's submitted name and phone. The public form stays unchanged and there is NO separate contact-person entity. Frontend source files in frontend branch are not edited here.

## Cross-branch evidence for the frontend developer

- [Current `SiteAPI` alias](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L344-L345) and [frontend API reference](https://github.com/yomezza/forma.flow/blob/frontend/docs/DEMO_API.md#L1-L108).
- [Public date/slot consumers](https://github.com/yomezza/forma.flow/blob/frontend/dist/app.js#L70-L116); [booking form network/error UX](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/booking-form.js#L38-L56).
- [Administrative list, client profile and booking confirmation](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L125-L177).
- [Planner previews and publication](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/planner.js#L143-L157); [unsaved form store](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/draft-store.js#L1-L13).
- [Existing calendar event exporter](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/calendar-event.js#L41-L85).

These links open source in `frontend` but **this document only describes required future frontend work**; it does not change that branch.

## Идея 01 — новый production HttpSiteAPI и интерфейсы

Привести [DemoAPI-спецификацию идеи](features/01-specialists/demo-api-spec.md) к реальным HTTP handler из [OpenAPI](openapi.yaml). Изменения фронтенда отдельно, в другой ветке:

- Публичная форма остаётся **без выбора специалиста**; вызывается тот же `createBookingRequest`, однако backend запускает joint capacity solver вместо сравнения по одному ресурсу.
- Admin: список персонала/кабинетов, личная неделя сотрудника, назначение/переназначение, предупреждение о косвенно вытесненных неназначенных заявках и точное подтверждение конфликтов, очередь отсутствий, «Требуют внимания».
- Specialist: отдельное рабочее пространство, роль в аккаунте, личные назначения (pending и confirmed в том числе), календарь, свои заявки на отсутствие и события переназначений, завершение/undo. Номер клиента никогда не приходит в ответах specialist.
- Admin должен перестать вызывать старый `completeBooking`: переход в completed только из specialist-role, после окончания сеанса, отдельный endpoint.
- `clientId` привязывается админом отдельно от `specialistId/roomId`; подтверждённая заявка без специалиста разрешена.
- `expectedCapacityRevision` передавать после preview, 409 заставляет обновить картину, никаких автоматических повторов с изменившимся списком конфликтов.

[Точные утверждённые правила](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L52), [методы в REST](api-mapping.md).
