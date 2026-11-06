# DemoAPI → REST → Go handler → service → PostgreSQL

Frontend source: dist/js/demo-api.js at frontend commit 1bdc8c63eb0db12445e4a6dc0bf717967df0d9cf. All 57 exported methods are implemented in the JavaScript demo. SiteAPI is its alias; some methods are not called by current UI. HTTP routes below are new architectural proposals, not existing endpoints. Prefix for all routes: /api/v1. Suffix paths below begin at that prefix.

## Public and catalogue

| # | DemoAPI method | REST route | Go handler → service | SQL/tables |
|---|---|---|---|---|
| 1 | getServices() | GET /services | Public.ListServices → catalog.ListActive | services SELECT active order |
| 2 | getServiceById(id) | GET /services/{id} | Public.GetService → catalog.Get | services SELECT |
| 3 | getAvailableSlots(date,id) | GET /availability/slots?date=&serviceId= | Public.AvailableSlots → schedule.Slots | services, schedule_state, simple_schedule/schedule_days/schedule_intervals, bookings |
| 4 | getDateStates(a,b,id) | GET /availability/dates?from=&to=&serviceId= | Public.DateStates → schedule.DateStates | bulk read published schedule and bookings |
| 5 | createBookingRequest(data) | POST /booking-requests | Public.CreateBooking → booking.CreateRequest | tx idempotency_keys, services, clients, bookings, booking_events |
| 6 | createBooking(data) | NO production endpoint | Demo-only direct confirmed booking | legacy confirmed-without-client is unsafe for public usage |

## Services (admin)

| # | DemoAPI method | REST route | Go handler → service | SQL/tables |
|---|---|---|---|---|
| 7 | createService(data) | POST /admin/services | Admin.CreateService → catalog.Create | services INSERT, audit_log |
| 8 | updateService(id,data) | PATCH /admin/services/{id} | Admin.UpdateService → catalog.Update | services UPDATE, audit_log |
| 9 | deleteService(id) | DELETE /admin/services/{id} | Admin.ArchiveService → catalog.Archive | services UPDATE is_active=false; no hard delete while referenced |
| 10 | resetServices() | NO production endpoint | Demo-only reset | avoid resetting historical catalogue |

## Bookings

| # | DemoAPI method | REST route | Go handler → service | SQL/tables |
|---|---|---|---|---|
| 11 | getBookings() | GET /admin/bookings | Admin.ListBookings → booking.List | bookings SELECT paginated |
| 12 | getPendingBookings() | GET /admin/bookings?status=pending_confirmation | Admin.ListBookings → booking.List | bookings partial pending index |
| 13 | getBookingsForDate(d) | GET /admin/bookings?date= | Admin.ListBookings → booking.List | bookings local_date filter |
| 14 | getBookingWarnings() | GET /admin/schedule/warnings | Admin.BookingWarnings → schedule.Warnings | active bookings + published schedule |
| 15 | confirmBooking(id) | POST /admin/bookings/{id}/confirm | Admin.ConfirmBooking → booking.Transition | tx bookings, booking_events, audit_log |
| 16 | cancelBooking(id,note) | POST /admin/bookings/{id}/cancel | Admin.CancelBooking → booking.Transition | tx bookings, booking_events, audit_log |
| 17 | completeBooking(id) | POST /admin/bookings/{id}/complete | Admin.CompleteBooking → booking.Transition | tx bookings, booking_events, audit_log |
| 18 | restoreBooking(id) | POST /admin/bookings/{id}/restore | Admin.RestoreBooking → booking.Restore | tx schedule_state, bookings EXCLUDE, booking_events |
| 19 | updateBooking(id,{status}) | ADAPTER DISPATCH to status actions above | Admin status dispatcher | no generic status overwrite |
| 20 | deleteBooking(id) | NO production endpoint | Demo-only destructive delete | retain history, use cancel/anonymization |
| 21 | clearBookings() | NO production endpoint | Demo-only wipe | unsafe for production |

## Clients

| # | DemoAPI method | REST route | Go handler → service | SQL/tables |
|---|---|---|---|---|
| 22 | getClients() | GET /admin/clients | Admin.ListClients → client.List | clients SELECT paginated |
| 23 | getClientById(id) | GET /admin/clients/{id} | Admin.GetClient → client.Get | clients SELECT |
| 24 | findClientByPhone(phone) | GET /admin/clients/by-phone?phone= | Admin.FindClient → client.FindPhone | clients unique normalized phone |
| 25 | createClient(data) | POST /admin/clients | Admin.CreateClient → client.Create | clients INSERT |
| 26 | updateClient(id,data) | PATCH /admin/clients/{id} | Admin.UpdateClient → client.Update | clients UPDATE + audit_log |
| 27 | getClientBookings(id) | GET /admin/clients/{id}/bookings | Admin.ClientBookings → booking.ListClient | bookings client history |
| 28 | clearClientsAndBookings() | NO production endpoint | Demo-only wipe | no cascade deletes of history |

## Simple schedule and vacations

| # | DemoAPI method | REST route | Go handler → service | SQL/tables |
|---|---|---|---|---|
| 29 | getVacations() | GET /admin/vacations | Admin.ListVacations → schedule.ListVacations | vacations SELECT |
| 30 | createVacation(v,confirmedIds) | POST /admin/vacations | Admin.CreateVacation → schedule.SaveVacation | tx schedule_state, vacations, schedule_changes, audit |
| 31 | updateVacation(id,v,confirmedIds) | PATCH /admin/vacations/{id} | Admin.UpdateVacation → schedule.SaveVacation | tx same |
| 32 | deleteVacation(id) | DELETE /admin/vacations/{id} | Admin.DeleteVacation → schedule.DeleteVacation | tx vacations, schedule_changes |
| 33 | isVacationDate(date) | GET /admin/vacations?contains= (or adapter local derivation) | Admin.VacationLookup → schedule.OnVacation | vacations SELECT |
| 34 | getSettings() | GET /site/settings (public safe subset); GET /admin/settings and GET /admin/schedule/simple (admin) | Site.GetSettings/Admin.GetSettings | site_settings, resources, simple_schedule, blocked dates |
| 35 | updateSettings(data) | PATCH /admin/settings and PUT /admin/schedule/simple | Admin.UpdateSettings → admin/site + schedule.UpdateSimple | site_settings, simple_schedule, blocked dates; revision |
| 36 | resetSettings() | NO production endpoint | Demo-only reset | admin may edit fields explicitly |

## Advanced schedule, previews, version history

| # | DemoAPI method | REST route | Go handler → service | SQL/tables |
|---|---|---|---|---|
| 37 | getScheduleMode() | GET /admin/schedule/mode | Admin.GetMode → schedule.Mode | schedule_state |
| 38 | previewScheduleModeChange(mode,a,b) | POST /admin/schedule/mode/preview | Admin.PreviewMode → schedule.PreviewMode | published + bookings |
| 39 | setScheduleMode(mode) | PUT /admin/schedule/mode | Admin.SetMode → schedule.SetMode | tx schedule_state + schedule_changes |
| 40 | getPublishedSchedule(a,b) | GET /admin/schedule/days?stage=published&from=&to= | Admin.ListScheduleDays → schedule.ListDays | schedule_days, intervals |
| 41 | getDraftSchedule(a,b) | GET /admin/schedule/days?stage=draft&from=&to= | Admin.ListScheduleDays → schedule.ListDays | schedule_days, intervals |
| 42 | getDateSchedule(d) | GET /admin/schedule/days/{date} | Admin.GetDay → schedule.DayOverview | simple + published/draft |
| 43 | setDraftSchedule(date,rule) | PUT /admin/schedule/drafts/{date} | Admin.SetDraft → schedule.SaveDraft | tx schedule_days, intervals, changes |
| 44 | setDraftScheduleRange(a,b,data) | PUT /admin/schedule/drafts/range | Admin.SetDraftRange → schedule.SaveDraftRange | tx schedule_days, intervals, changes |
| 45 | copySimpleScheduleToDraft(a,b) | POST /admin/schedule/drafts/copy-simple | Admin.CopySimple → schedule.CopySimple | tx simple_schedule to drafts |
| 46 | previewSchedulePublication(a,b) | POST /admin/schedule/publications/preview | Admin.PreviewPublication → schedule.PreviewPublish | drafts, bookings, revision |
| 47 | publishScheduleRange(a,b) | POST /admin/schedule/publications | Admin.Publish → schedule.Publish | tx schedule_state, schedule_days, changes, audit |
| 48 | unpublishDate(d) | PUT /admin/schedule/drafts/{date} kind=unplanned | Admin.SetDraft → schedule.SaveDraft | draft unpublish marker, NOT immediate publication |
| 49 | getScheduleHistory() | GET /admin/schedule/history | Admin.ListHistory → schedule.History | schedule_changes paginated |
| 50 | previewRestore(id) | POST /admin/schedule/history/{id}/preview | Admin.PreviewRestore → schedule.PreviewRestore | change snapshot, bookings |
| 51 | restoreScheduleVersion(id) | POST /admin/schedule/history/{id}/restore | Admin.RestoreDraft → schedule.RestoreToDraft | tx schedule_changes + drafts + revision |

## Site content and demo maintenance

| # | DemoAPI method | REST route | Go handler → service | SQL/tables |
|---|---|---|---|---|
| 52 | getContent() | GET /site/content | Site.GetContent → admin.ReadContent | site_content |
| 53 | updateContent(data) | PATCH /admin/site/content | Admin.UpdateContent → admin.WriteContent | site_content + audit_log |
| 54 | getWarning() | NO endpoint | local storage warning only | none |
| 55 | exportData() | POST /admin/exports (owner only; explicit permission) | Admin.Export → admin.ExportData | audited read, PII download, no cache |
| 56 | importData(data) | NO production endpoint | offline validated migration/import only | not general-purpose HTTP |
| 57 | resetDemoData() | NO production endpoint | Demo-only reset | none |

Notes: list methods become paginated HTTP responses; current UI expects arrays, so adapter unwraps items and optionally fetches more pages. Write methods that currently accept no expectedRevision gain a version parameter; the API may reject stale writes with 409 VERSION_CONFLICT. POST /booking-requests adds Idempotency-Key header and returns 201 or 200 on replay. Full wire schemas: openapi.yaml.
