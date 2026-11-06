# Verified cross-branch references: frontend → Go architecture

This page deliberately links from the **`backend-architecture` branch to the `frontend` branch** using complete GitHub URLs (`/blob/frontend/…#Lx-Ly`). A relative Markdown path here would resolve in `backend-architecture` and could point to the wrong file or a missing file.

**Checked frontend head:** [`1bdc8c63eb0db12445e4a6dc0bf717967df0d9cf`](https://github.com/yomezza/forma.flow/tree/1bdc8c63eb0db12445e4a6dc0bf717967df0d9cf), on 2026-11-06. Branch links below deliberately follow `frontend` as it evolves. For a fixed historical copy, use a permalink to the commit SHA instead of `frontend`. Line anchors may move after future frontend edits.

The links document **current JavaScript code**, not an implemented backend. The architecture in this branch is the target specification; whenever it differs from the current demo, it must be marked explicitly in [investigation](investigation.md), [API mapping](api-mapping.md), or [frontend integration](frontend-integration.md).

## Entry points and contract

- [README.md:11–29](https://github.com/yomezza/forma.flow/blob/frontend/README.md#L11-L29) — Frontend README: actual module map.
- [docs/DEMO_API.md:1–12](https://github.com/yomezza/forma.flow/blob/frontend/docs/DEMO_API.md#L1-L12) — Documented frontend DemoAPI contract and its stated limits.
- [dist/js/demo-api.js:1–15](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L1-L15) — JavaScript facade, storage key and helpers.
- [dist/js/demo-api.js:246–345](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L246-L345) — Actual exported DemoAPI methods and SiteAPI alias.
- [dist/js/demo-data.js:1–18](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-data.js#L1-L18) — Local demo defaults: services, RUB prices, weekly calendar.

## Booking and clients

- [dist/app.js:70–116](https://github.com/yomezza/forma.flow/blob/frontend/dist/app.js#L70-L116) — Public selection and slot/date API calls.
- [dist/js/booking-form.js:38–57](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/booking-form.js#L38-L57) — Booking form: consent, +7 phone, POST adapter call, conflict UX.
- [dist/js/demo-api.js:25–43](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L25-L43) — Demo client phone normalization + uniqueness assumption.
- [dist/js/demo-api.js:171–186](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L171-L186) — Demo validate: phone uniqueness and booking overlap.
- [dist/js/demo-api.js:303–323](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L303-L323) — Demo client search/updates + auto-match on booking.
- [dist/js/demo-api.js:43–56](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L43-L56) — Booking states and reservations.
- [dist/js/admin.js:125–149](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L125-L149) — Admin booking list/actions, currently direct confirmation.
- [dist/js/admin.js:153–178](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L153-L178) — Admin client profile, history and search.

## Scheduling

- [dist/js/demo-api.js:99–140](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L99-L140) — Simple/advanced rule selection, holiday/blocked priorities.
- [dist/js/demo-api.js:226–244](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L226-L244) — Slot generation and browser-clock cutoff.
- [dist/js/demo-api.js:253–272](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L253-L272) — Date states and schedule mode preview.
- [dist/js/demo-api.js:273–290](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L273-L290) — Advanced drafts, selective publish and history restore.
- [dist/js/planner.js:143–181](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/planner.js#L143-L181) — Planner: previews, publish call, schedule history UI.
- [dist/js/admin.js:217–281](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L217-L281) — Simple weekly settings and vacations admin UI.

## Site settings, demo maintenance, safety

- [dist/js/demo-api.js:333–343](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L333-L343) — Demo data export/import/reset.
- [dist/js/admin.js:312–329](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L312-L329) — Admin export/import and dangerous reset controls.
- [dist/js/draft-store.js:1–13](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/draft-store.js#L1-L13) — Unsaved form drafts in tab session, sensitive edits in memory.
- [dist/js/admin.js:1–24](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L1-L24) — Admin-only navigation is UI, not authentication.
- [dist/js/calendar-event.js:15–85](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/calendar-event.js#L15-L85) — Calendar event local time, .ics, Google link, non-PII event transfer.
- [dist/assets/brand/logos/logo-01.svg](https://github.com/yomezza/forma.flow/blob/frontend/dist/assets/brand/logos/logo-01.svg) — Site branding visual symbols / logo asset.
- [docs/DESIGN_SYSTEM.md](https://github.com/yomezza/forma.flow/blob/frontend/docs/DESIGN_SYSTEM.md) — Frontend visual philosophy and design notes.

## Known non-equivalences (do not treat source as production contract)

- **Phone and identity:** demo requires phone uniqueness and auto-attaches clients; the accepted Go design allows many profiles per phone and requires explicit admin association. Source: [dist/js/demo-api.js:38–43](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L38-L43) and [dist/js/demo-api.js:303–323](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L303-L323). Target: [business rules](business-rules.md), [SQL schema](database-schema.sql), [frontend tasks](frontend-integration.md).
- **Time:** demo's availability uses the visitor's browser clock; Go uses the cabinet's IANA time zone. Source: [dist/js/demo-api.js:226–239](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L226-L239).
- **Publication:** demo previews and publishes without `expectedRevision`; Go must reject stale revisions. Source: [dist/js/planner.js:153–157](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/planner.js#L153-L157).
- **Auth and deletion:** demo admin button, exports and hard deletion are local-only; Go requires authenticated operations, auditing and no production reset endpoints. Source: [dist/js/admin.js:1–24](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L1-L24) and [dist/js/admin.js:312–329](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L312-L329).

## Update procedure

Before changing any architecture requirement, review the latest `frontend` head and confirm that these branch links still point to the intended definitions. If the corresponding function moved, update anchors. Never change files in `main` or `frontend` from this branch. Do not silently interpret a documented target behavior as already implemented in DemoAPI.

### Working example: moving branch vs fixed permalink

- [Current `frontend` code for `createBookingRequest`](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L314-L321) — follows subsequent pushes to the frontend branch, so anchors should be rechecked.
- [The same code frozen at audited commit](https://github.com/yomezza/forma.flow/blob/1bdc8c63eb0db12445e4a6dc0bf717967df0d9cf/dist/js/demo-api.js#L314-L321) — does not move when new frontend commits arrive.

Both links navigate directly to code in another Git branch/commit, without copying the JavaScript into the architecture branch.
