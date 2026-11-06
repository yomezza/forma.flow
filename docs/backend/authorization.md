# Authentication, authorization and data protection

Production is NOT the demo admin button. All /admin routes require a server-issued, opaque session cookie; UI visibility is not authorization. Login POST /api/v1/admin/sessions with email/password; password stored as Argon2id hash with per-user random salt and versioned parameters. Generic unauthorized error hides account existence; apply rate limits and audit log failed attempts without secrets.

Session token: 256 bits from crypto/rand; persist only SHA-256 hash in admin_sessions, rotate on login, expire/revoke server side. Cookie: HttpOnly, Secure, SameSite=Lax or Strict, Path=/api/v1/admin; do not embed phone in cookie. All state-changing admin requests require Origin check and CSRF token strategy (SameSite alone is not sufficient), HTTPS and no CORS credentials for arbitrary origins. CSRF double-submit or server-side token tied to session; choose explicit implementation before deploy.

Roles: admin may manage bookings, services, clients and schedule; owner may additionally export full database/PII and manage administrators. Authorization is server middleware plus service check. No guest booking lookup by name/phone; public booking receipt reveals only the own operation response.

Protect PII: TLS, database least-privileged role, encrypted backups, no personal data in logs/QR/GET query strings except admin by-phone lookup (prefer POST search if access-log hygiene is weak), do not cache admin responses; use Cache-Control: no-store. Retention/deletion/anonymization and export need legal review and audit. Historical booking data must not be accidentally cascade-deleted. Separate secrets from repository and rotate safely.

Availability/rate: per-IP and abuse-pattern throttling for POST booking, body size cap, request timeout, phone normalization server-side, verify origin and human-input controls if abuse appears. Limit concurrent DB connections, lock timeout, statement timeout, bounded pagination/date ranges. Do not leak resource schedule drafts or client histories through public routes.

Backups: daily encrypted snapshot plus PITR if operational RPO/RTO require it, periodic restore drill and restricted access. Application-level full import/reset/clear routes are intentionally excluded from production.

Client candidate matching by phone and booking-to-client assignment are administrative operations only. Never publish a client directory to the public visitor. Administrators must explicitly select the actual client profile; automatic phone-based identity matching is forbidden, even when the lookup returns a single candidate. Shared phones are normal; no separate contact-person entity is created.
