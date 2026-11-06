# Authentication, authorization and data protection

Production is NOT the demo admin button. All /admin routes require a server-issued, opaque session cookie; UI visibility is not authorization. Login POST /api/v1/admin/sessions (legacy compatible path; backed by staff_accounts/staff_sessions) with email/password; password stored as Argon2id hash with per-user random salt and versioned parameters. Generic unauthorized error hides account existence; apply rate limits and audit log failed attempts without secrets.

Session token: 256 bits from crypto/rand; persist only SHA-256 hash in staff_sessions, rotate on login, expire/revoke server side. Cookie: HttpOnly, Secure, SameSite=Lax or Strict, Path=/api/v1; // shared across /admin and /specialist routes; do not embed phone in cookie. All state-changing admin requests require Origin check and CSRF token strategy (SameSite alone is not sufficient), HTTPS and no CORS credentials for arbitrary origins. CSRF double-submit or server-side token tied to session; choose explicit implementation before deploy.

Account roles live in `account_roles` (owner/admin/specialist); one `staff_accounts` row may have **both admin and specialist**. Admin manages bookings, staff, rooms, clients and schedules; owner additionally exports full PII and manages accounts. Specialists see only own assigned bookings and own absence requests, can propose leave and finish/undo their own completed visits. Authorization is server middleware plus service check. No guest booking lookup by name/phone; public booking receipt reveals only the own operation response.

Protect PII: TLS, database least-privileged role, encrypted backups, no personal data in logs/QR/GET query strings except admin by-phone lookup (prefer POST search if access-log hygiene is weak), do not cache admin responses; use Cache-Control: no-store. Retention/deletion/anonymization and export need legal review and audit. Historical booking data must not be accidentally cascade-deleted. Separate secrets from repository and rotate safely.

Availability/rate: per-IP and abuse-pattern throttling for POST booking, body size cap, request timeout, phone normalization server-side, verify origin and human-input controls if abuse appears. Limit concurrent DB connections, lock timeout, statement timeout, bounded pagination/date ranges. Do not leak resource schedule drafts or client histories through public routes.

Backups: daily encrypted snapshot plus PITR if operational RPO/RTO require it, periodic restore drill and restricted access. Application-level full import/reset/clear routes are intentionally excluded from production.

Client candidate matching by phone and booking-to-client assignment are administrative operations only. Never publish a client directory to the public visitor. Administrators must explicitly select the actual client profile; automatic phone-based identity matching is forbidden, even when the lookup returns a single candidate. Shared phones are normal; no separate contact-person entity is created.

## Source boundary

The [current admin trigger/view switch](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L1-L24) is frontend-only, and [demo JSON exports/imports](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L312-L329) can include personal data. These linked source fragments explain why the Go server requires independent authentication, audited privileged exports and safe admin-only client matching.

## Новые границы доступа — идея 01

- `staff_accounts`, `staff_sessions`, `account_roles`; `specialists.account_id` привязывает профиль к аккаунту. После приглашения выдать одноразовый временный пароль только по защищённому каналу; Argon2id hash + `must_change_password=true`, до смены пароля запрещать любые другие операции. Архивирование/деактивация отзывают активные сессии.
- Backend проверяет права **по сохранённым account_roles**, а не по `X-Active-Role` как источнику прав. Активная роль — явный контекст операции: при двух ролях `admin` и `specialist` завершение возможно только из specialist-контекста с проверкой своего specialist_id. Нельзя использовать admin-право для завершения чужого сеанса.
- Specialist endpoints используют `WHERE bookings.specialist_id = $currentSpecialistID` и **отдельные DTO** без телефона, client_id, чужих/неназначенных бронирований. Прошлые назначения видны только как собственные события, без текущих чужих деталей. `getClients`, телефон, экспорт и вся административная база запрещены.
- `POST /api/v1/specialist/me/bookings/{id}/complete` проверяет `confirmed`, `now>=ends_at`, текущий assignment и роль. Undo: тот же `completed_by_specialist_id` без срока, аудит обязателен. Никаких телефонных данных в специалистских логах, событиях или QR.


## Идея 01: источник и приоритет

Согласованные требования: [назначения и вместимость](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L26), [личные графики и отсутствия](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L28-L34), [роли](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L36-L43), [завершение и аудит](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L45-L52). Текущий frontend не реализует эти правила; Go/PostgreSQL-проект в данном документе — целевое состояние.
