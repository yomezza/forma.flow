# DemoAPI → REST → Go handler → service → PostgreSQL

Frontend source: dist/js/demo-api.js at frontend commit 1bdc8c63eb0db12445e4a6dc0bf717967df0d9cf. All 57 exported methods are implemented in the JavaScript demo. SiteAPI is its alias; some methods are not called by current UI. HTTP routes below are new architectural proposals, not existing endpoints. Prefix for all routes: /api/v1. Suffix paths below begin at that prefix.

## Cross-branch source references

The method-name links in all 57 numbered rows above lead **directly into the `frontend` branch** at their actual JavaScript definitions. These describe the **existing demo**, while REST routes and Go operations in the other columns are proposed architecture. For the consumer call sites and known differences, see [frontend source index](frontend-source-links.md) and [frontend integration requirements](frontend-integration.md).

## Public and catalogue

| # | DemoAPI method | REST route | Go handler → service | SQL/tables |
|---|---|---|---|---|
| 1 | [getServices()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L291) | GET /services | Public.ListServices → catalog.ListActive | services SELECT active order |
| 2 | [getServiceById(id)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L292) | GET /services/{id} | Public.GetService → catalog.Get | services SELECT |
| 3 | [getAvailableSlots(date,id)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L297) | GET /availability/slots?date=&serviceId= | Public.AvailableSlots → schedule.Slots | services, schedule_state, simple_schedule/schedule_days/schedule_intervals, bookings |
| 4 | [getDateStates(a,b,id)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L253) | GET /availability/dates?from=&to=&serviceId= | Public.DateStates → schedule.DateStates | bulk read published schedule and bookings |
| 5 | [createBookingRequest(data)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L314) | POST /booking-requests | Public.CreateBooking → booking.CreateRequest | tx idempotency_keys, services, bookings (client_id=NULL), booking_events; admin later assigns |
| 6 | [createBooking(data)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L298) | NO production endpoint | Demo-only direct confirmed booking | legacy confirmed-without-client is unsafe for public usage |

## Services (admin)

| # | DemoAPI method | REST route | Go handler → service | SQL/tables |
|---|---|---|---|---|
| 7 | [createService(data)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L293) | POST /admin/services | Admin.CreateService → catalog.Create | services INSERT, audit_log |
| 8 | [updateService(id,data)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L294) | PATCH /admin/services/{id} | Admin.UpdateService → catalog.Update | services UPDATE, audit_log |
| 9 | [deleteService(id)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L295) | DELETE /admin/services/{id} | Admin.ArchiveService → catalog.Archive | services UPDATE is_active=false; no hard delete while referenced |
| 10 | [resetServices()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L338) | NO production endpoint | Demo-only reset | avoid resetting historical catalogue |

## Bookings

| # | DemoAPI method | REST route | Go handler → service | SQL/tables |
|---|---|---|---|---|
| 11 | [getBookings()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L296) | GET /admin/bookings | Admin.ListBookings → booking.List | bookings SELECT paginated |
| 12 | [getPendingBookings()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L313) | GET /admin/bookings?status=pending_confirmation | Admin.ListBookings → booking.List | bookings partial pending index |
| 13 | [getBookingsForDate(d)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L265) | GET /admin/bookings?date= | Admin.ListBookings → booking.List | bookings local_date filter |
| 14 | [getBookingWarnings()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L264) | GET /admin/schedule/warnings | Admin.BookingWarnings → schedule.Warnings | active bookings + published schedule |
| 15 | [confirmBooking(id)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L322) | POST /admin/bookings/{id}/confirm | Admin.ConfirmBooking → booking.Transition | tx bookings (requires linked client_id), booking_events, audit_log |
| 16 | [cancelBooking(id,note)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L323) | POST /admin/bookings/{id}/cancel | Admin.CancelBooking → booking.Transition | tx bookings, booking_events, audit_log |
| 17 | [completeBooking(id)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L324) | POST /admin/bookings/{id}/complete | Admin.CompleteBooking → booking.Transition | tx bookings, booking_events, audit_log |
| 18 | [restoreBooking(id)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L325) | POST /admin/bookings/{id}/restore | Admin.RestoreBooking → booking.Restore | tx schedule_state, bookings EXCLUDE, booking_events |
| 19 | [updateBooking(id,{status})](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L330) | ADAPTER DISPATCH to status actions above | Admin status dispatcher | no generic status overwrite |
| 20 | [deleteBooking(id)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L332) | NO production endpoint | Demo-only destructive delete | retain history, use cancel/anonymization |
| 21 | [clearBookings()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L339) | NO production endpoint | Demo-only wipe | unsafe for production |

## Clients

| # | DemoAPI method | REST route | Go handler → service | SQL/tables |
|---|---|---|---|---|
| 22 | [getClients()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L303) | GET /admin/clients | Admin.ListClients → client.List | clients SELECT paginated |
| 23 | [getClientById(id)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L304) | GET /admin/clients/{id} | Admin.GetClient → client.Get | clients SELECT |
| 24 | [findClientByPhone(phone)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L305) | GET /admin/clients/by-phone?phone= | Admin.FindClientsByPhone → client.ListByPhone | non-unique clients.phone_normalized index; returns all candidates |
| 25 | [createClient(data)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L306) | POST /admin/clients | Admin.CreateClient → client.Create | clients INSERT (duplicate phone allowed) |
| 26 | [updateClient(id,data)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L307) | PATCH /admin/clients/{id} | Admin.UpdateClient → client.Update | clients UPDATE + audit_log |
| 27 | [getClientBookings(id)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L312) | GET /admin/clients/{id}/bookings | Admin.ClientBookings → booking.ListClient | bookings client history |
| 28 | [clearClientsAndBookings()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L331) | NO production endpoint | Demo-only wipe | no cascade deletes of history |

## Simple schedule and vacations

| # | DemoAPI method | REST route | Go handler → service | SQL/tables |
|---|---|---|---|---|
| 29 | [getVacations()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L247) | GET /admin/vacations | Admin.ListVacations → schedule.ListVacations | vacations SELECT |
| 30 | [createVacation(v,confirmedIds)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L248) | POST /admin/vacations | Admin.CreateVacation → schedule.SaveVacation | tx schedule_state, vacations, schedule_changes, audit |
| 31 | [updateVacation(id,v,confirmedIds)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L249) | PATCH /admin/vacations/{id} | Admin.UpdateVacation → schedule.SaveVacation | tx same |
| 32 | [deleteVacation(id)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L250) | DELETE /admin/vacations/{id} | Admin.DeleteVacation → schedule.DeleteVacation | tx vacations, schedule_changes |
| 33 | [isVacationDate(date)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L251) | GET /admin/vacations?contains= (or adapter local derivation) | Admin.VacationLookup → schedule.OnVacation | vacations SELECT |
| 34 | [getSettings()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L333) | GET /site/settings (public safe subset); GET /admin/settings and GET /admin/schedule/simple (admin) | Site.GetSettings/Admin.GetSettings | site_settings, resources, simple_schedule, blocked dates |
| 35 | [updateSettings(data)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L334) | PATCH /admin/settings and PUT /admin/schedule/simple | Admin.UpdateSettings → admin/site + schedule.UpdateSimple | site_settings, simple_schedule, blocked dates; revision |
| 36 | [resetSettings()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L335) | NO production endpoint | Demo-only reset | admin may edit fields explicitly |

## Advanced schedule, previews, version history

| # | DemoAPI method | REST route | Go handler → service | SQL/tables |
|---|---|---|---|---|
| 37 | [getScheduleMode()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L252) | GET /admin/schedule/mode | Admin.GetMode → schedule.Mode | schedule_state |
| 38 | [previewScheduleModeChange(mode,a,b)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L266) | POST /admin/schedule/mode/preview | Admin.PreviewMode → schedule.PreviewMode | published + bookings |
| 39 | [setScheduleMode(mode)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L272) | PUT /admin/schedule/mode | Admin.SetMode → schedule.SetMode | tx schedule_state + schedule_changes |
| 40 | [getPublishedSchedule(a,b)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L261) | GET /admin/schedule/days?stage=published&from=&to= | Admin.ListScheduleDays → schedule.ListDays | schedule_days, intervals |
| 41 | [getDraftSchedule(a,b)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L262) | GET /admin/schedule/days?stage=draft&from=&to= | Admin.ListScheduleDays → schedule.ListDays | schedule_days, intervals |
| 42 | [getDateSchedule(d)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L263) | GET /admin/schedule/days/{date} | Admin.GetDay → schedule.DayOverview | simple + published/draft |
| 43 | [setDraftSchedule(date,rule)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L273) | PUT /admin/schedule/drafts/{date} | Admin.SetDraft → schedule.SaveDraft | tx schedule_days, intervals, changes |
| 44 | [setDraftScheduleRange(a,b,data)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L274) | PUT /admin/schedule/drafts/range | Admin.SetDraftRange → schedule.SaveDraftRange | tx schedule_days, intervals, changes |
| 45 | [copySimpleScheduleToDraft(a,b)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L275) | POST /admin/schedule/drafts/copy-simple | Admin.CopySimple → schedule.CopySimple | tx simple_schedule to drafts |
| 46 | [previewSchedulePublication(a,b)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L276) | POST /admin/schedule/publications/preview | Admin.PreviewPublication → schedule.PreviewPublish | drafts, bookings, revision |
| 47 | [publishScheduleRange(a,b)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L277) | POST /admin/schedule/publications | Admin.Publish → schedule.Publish | tx schedule_state, schedule_days, changes, audit |
| 48 | [unpublishDate(d)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L286) | PUT /admin/schedule/drafts/{date} kind=unplanned | Admin.SetDraft → schedule.SaveDraft | draft unpublish marker, NOT immediate publication |
| 49 | [getScheduleHistory()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L287) | GET /admin/schedule/history | Admin.ListHistory → schedule.History | schedule_changes paginated |
| 50 | [previewRestore(id)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L288) | POST /admin/schedule/history/{id}/preview | Admin.PreviewRestore → schedule.PreviewRestore | change snapshot, bookings |
| 51 | [restoreScheduleVersion(id)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L289) | POST /admin/schedule/history/{id}/restore | Admin.RestoreDraft → schedule.RestoreToDraft | tx schedule_changes + drafts + revision |

## Site content and demo maintenance

| # | DemoAPI method | REST route | Go handler → service | SQL/tables |
|---|---|---|---|---|
| 52 | [getContent()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L336) | GET /site/content | Site.GetContent → admin.ReadContent | site_content |
| 53 | [updateContent(data)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L337) | PATCH /admin/site/content | Admin.UpdateContent → admin.WriteContent | site_content + audit_log |
| 54 | [getWarning()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L290) | NO endpoint | local storage warning only | none |
| 55 | [exportData()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L341) | POST /admin/exports (owner only; explicit permission) | Admin.Export → admin.ExportData | audited read, PII download, no cache |
| 56 | [importData(data)](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L342) | NO production endpoint | offline validated migration/import only | not general-purpose HTTP |
| 57 | [resetDemoData()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L340) | NO production endpoint | Demo-only reset | none |

Notes: list methods become paginated HTTP responses; current UI expects arrays, so adapter unwraps items and optionally fetches more pages. Write methods that currently accept no expectedRevision gain a version parameter; the API may reject stale writes with 409 VERSION_CONFLICT. POST /booking-requests adds Idempotency-Key header and returns 201 or 200 on replay. Full wire schemas: openapi.yaml.

## New administrative operation (not present in the 57 DemoAPI methods)

| Frontend adapter addition | REST | Go HTTP handler → service | PostgreSQL |
|---|---|---|---|
| assignBookingClient(bookingId, {clientId} or {newClient}) | PUT /admin/bookings/{id}/client | Admin.AssignBookingClient → booking.AssignClient | tx bookings row lock, clients SELECT or INSERT, bookings.client_id UPDATE, audit_log INSERT |

The old findClientByPhone(phone) returns only one client/null in the JavaScript demo. It must become **findClientsByPhone(phone) returning 0..N candidates** in the production administrator workflow, not a silent first result. Existing 57-row inventory still accurately describes current DemoAPI, not necessarily future method signatures. Public booking never creates or selects a client; administrator assignment is mandatory before confirmation. No separate contact-person entity.
