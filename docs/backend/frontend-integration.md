# Frontend/backend integration (action list for frontend developer)

Keep all existing public/admn components consuming SiteAPI. Replace its backend implementation, not UI call sites where possible. Use a switch between DemoAPI (local demo) and HttpSiteAPI (production). No direct fetch in components.

## Method adaptations

- getServices and getBookings/getClients/getVacations: HTTP gives paginated {items,nextCursor}; adapter returns arrays where existing UI expects arrays (or update admin list views for paging).
- getAvailableSlots(date,id): GET /availability/slots, unwrap slots[].
- getDateStates: unwrap states map; preserve values past/working/full/off/unplanned/vacation/blocked.
- createBookingRequest: preserve promise API and existing form values but generate one cryptographically random Idempotency-Key per submission attempt, reuse key across transport retries; append consentAccepted and consentVersion after client-side consent check; do not use a new key on automatic retry of same request.
- getSettings: public app should consume safe site settings; admin needs separate simple schedule and site settings. Adapter may combine them into the previous flat shape; enforce auth for private fields.
- updateSettings: split appearance PATCH /admin/settings from schedule PUT /admin/schedule/simple with expectedRevision.
- setScheduleMode, setDraftSchedule, setDraftScheduleRange, publishScheduleRange, restoreScheduleVersion: include expectedRevision. When 409 VERSION_CONFLICT, reload schedule and preview; never silently retry a changed schedule with fresh revision.
- Status change methods: map to dedicated POST confirm/cancel/complete/restore. updateBooking is adapter dispatch, not an unrestricted patch.
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
