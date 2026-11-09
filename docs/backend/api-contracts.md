# Wire contracts, errors and HTTP policy

Canonical machine-readable file: openapi.yaml. It contains 79 operations across 64 paths, public and admin, reusable schemas and concrete booking examples. HTTP handler method is operationId; mapping to Go service and tables is documented in api-mapping.md.

## Protocol

Base /api/v1. JSON UTF-8 only; request body capped (e.g. 32 KiB for booking; 256 KiB admin schedule batch with a date-limit cap). IDs: service text (existing classic/relax/back), other entity IDs UUID string. Public dates YYYY-MM-DD interpreted in cabinet resource IANA time zone; clocks HH:mm interpreted as cabinet-local, not device-local. Instants ISO 8601 UTC in admin/audit. Prices are integer whole-RUB values in v1 to preserve existing priceAmount semantics.

Public POST /booking-requests returns 201 + BookingReceipt. Exact replay with matching Idempotency-Key and canonical body returns 200 + same receipt. A different body with the same key returns 409 IDEMPOTENCY_CONFLICT. Availability GET is advisory; booking POST rechecks in DB transaction.

~~~http
POST /api/v1/booking-requests HTTP/1.1
Content-Type: application/json
Idempotency-Key: 390334f1-72da-41e6-8db9-37f854f51fa9

{"serviceId":"classic","date":"2026-11-10","time":"11:00","fullName":"Иванов Иван","phone":"+79991234567","consentAccepted":true,"consentVersion":"v1"}
~~~

~~~json
{"id":"ded1c777-2445-47b1-bbd3-33b74294ad07","serviceId":"classic","name":"Классический массаж","durationMinutes":60,"priceAmount":3500,"currency":"RUB","date":"2026-11-10","time":"11:00","status":"pending_confirmation"}
~~~

Errors follow a single envelope:

~~~json
{"error":{"code":"SLOT_UNAVAILABLE","message":"Это время уже занято.","requestId":"req-123","details":{}}}
~~~

| HTTP | Error code | Semantics |
|---|---|---|
| 400/422 | VALIDATION | malformed JSON / domain validation, invalid IANA time |
| 401 | AUTH_REQUIRED | missing/expired session |
| 403 | FORBIDDEN | insufficient role/CSRF failure |
| 404 | NOT_FOUND | object doesn't exist or is concealed |
| 409 | SLOT_UNAVAILABLE | concurrent or previously reserved interval |
| 409 | VERSION_CONFLICT | stale schedule expectedRevision; reload/preview |
| 409 | VACATION_CONFLICT | list counts/IDs, explicit acknowledgement needed |
| 409 | INVALID_STATUS_TRANSITION | status cannot change |
| 409 | IDEMPOTENCY_CONFLICT | reused key with different payload |
| 429 | RATE_LIMITED | abusive rate |
| 500 | INTERNAL | internal details only in logs; no SQL errors in response |

GET lists return {items, nextCursor}; cursor is opaque, limit <=100. Each admin query is scope-filtered; admin PII responses are Cache-Control: no-store. Store request_id in structured logs (with phone/name redaction) and send X-Request-ID. Limit date range GET to 62 days, schedule mutations to a configured safe maximum (e.g. 366 days). HTTP timeouts: header timeout 5s, read 10s, write 15s, SQL statement 3–5s for normal routes; adjust batch operations separately after measurement.

Admin session cookie (HttpOnly/Secure/SameSite), role middleware, Origin and X-CSRF-Token on state-changing admin calls. GET /site/settings returns no secret settings. Owner-only export explicitly audited. Public booking receipt is not a publicly retrievable booking token.

## Consistency notes
Schedule preview returns revision; every related write must include expectedRevision and fail closed when stale. Hard destructive DemoAPI methods are omitted. Public consentAccepted/consentVersion are additional adapter fields and must be recorded server-side. Legacy unversioned API calls cannot be accepted without adapter changes.

## Shared-phone association

One contact phone may belong to multiple independent client IDs. POST /booking-requests creates a pending booking with **clientId = null** in the administrative representation; submitted name/phone are stored as snapshots. GET /admin/clients/by-phone?phone= returns a page of all matching clients (not a single client). PUT /admin/bookings/{id}/client accepts either {"clientId":"uuid"} or {"newClient":{"fullName":"...","phone":"+7..."}}; backend selects or creates the profile and links it to the booking in a single audited transaction. Confirm without link returns 409 CLIENT_LINK_REQUIRED. Production admin UI requires explicit choice; no contact-person table.

## Existing implementation references

The initial contract was derived from [frontend DemoAPI documentation](https://github.com/yomezza/forma.flow/blob/frontend/docs/DEMO_API.md#L1-L108), [actual method definitions](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L246-L345), and [booking form consumers](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/booking-form.js#L38-L56). Browse the full [cross-branch source map](frontend-source-links.md). The OpenAPI specification is **proposed Go REST behavior**; the linked JavaScript source is not an HTTP API.

## Идея 01: новый контракт размещения и ролей

Одна `resourceId` = **студия**; специалист и кабинет — отдельные IDs. `POST /booking-requests` не принимает specialistId/roomId, возвращает только результат заявки; сервер защищает условно занятую вместимость с global solver под studio lock. `GET /availability/slots` не раскрывает, какой исполнитель может провести сеанс.

Для администратора: `GET /admin/bookings/{id}/assignment/candidates`, `POST /admin/bookings/{id}/assignment/preview`, `PUT /admin/bookings/{id}/assignment`. Запрос назначения содержит `specialistId`, optional `roomId` (сервер выбирает автоматически, если null), `expectedCapacityRevision`, `acknowledgedBookingIds` для косвенно вытесненных **неназначенных** заявок. Возвращать 409 `DIRECT_RESOURCE_CONFLICT` при прямом конфликте без override, 409 `STALE_CAPACITY_REVISION` при устаревшем preview, 409 `INDIRECT_ASSIGNMENT_CONFLICT` с IDs перед явным подтверждением, 503 `CAPACITY_CHECK_LIMIT` если solver не знает результат.

Личный контур `/specialist/me/*`: только свои назначения и заявки на отсутствие. Отдельные `SpecialistBooking` DTO **без телефона и клиентского ID**. Подтверждение осталось admin-действием; завершение/undo перешли к назначенному специалисту. Из OpenAPI удалён `POST /admin/bookings/{id}/complete`; вместо него `POST /specialist/me/bookings/{id}/complete` и `/undo-completion`. `completed → confirmed` возможен тем же человеком в любой момент с аудитом. `staff_accounts` поддерживает совмещённые роли и обязательную смену пароля на первом входе.

[Источник правил идеи](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L52), [полный OpenAPI](openapi.yaml).

## Staff auth finalization (idea 01)

`POST /api/v1/auth/sessions` is the **canonical login for all staff** (owner/admin/specialist); no `/api/v1/admin/sessions` in the final API. `GET /api/v1/auth/sessions/current` returns the staff Session with `accountId`, `roles`, `activeRole`, `mustChangePassword`, `expiresAt` and CSRF token; `DELETE` logs out. `POST /api/v1/admin/staff-invitations` accepts `{email, roles, specialistId}` and returns a one-time `temporaryPassword` only in its protected 201 response. `POST /api/v1/auth/password/change` accepts `{currentPassword,newPassword}`, rotates the session, clears first-login gate and returns the new Session. All writes use CSRF + Origin check, and passwords are never logged. Until first password change, only current-session, logout and password change are permitted. See [authorization](authorization.md) and [OpenAPI](openapi.yaml).
