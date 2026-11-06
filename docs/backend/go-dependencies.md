# Go dependencies — verified 2026-11-06

| Candidate | Decision | Rationale |
|---|---|---|
| Go 1.27.3 | Target Go toolchain | latest verified Go downloads listing; use Go standard HTTP, testing, slog |
| net/http | Yes | method-based routing and path values are sufficient; chi not necessary |
| github.com/jackc/pgx/v5 v5.12.0 | Yes | native PostgreSQL pool/transactions/types; latest security fixes |
| github.com/pressly/goose/v3 v3.29.0 | Yes, CLI | SQL migrations and schema history |
| sqlc v1.31.1 | Optional later | initially handwritten focused pgx queries for transaction control; no generator requirement |
| log/slog, testing, httptest | Yes | standard library |
| golang.org/x/crypto/argon2 | Yes | Argon2id password hashing for admins; pin exact compatible version in go.mod |
| chi v5.3.2, ORM, Redis, MQ | No | no demonstrated need in MVP |

Sources: https://go.dev/dl/ ; https://github.com/jackc/pgx/blob/master/CHANGELOG.md ; https://github.com/pressly/goose/blob/main/CHANGELOG.md ; https://github.com/sqlc-dev/sqlc/releases ; https://github.com/go-chi/chi/releases . Version claims are a snapshot, not indefinite latest guarantees.

PostgreSQL target 16–18. Use GiST exclusion constraints available in this range, avoid relying on PostgreSQL 18-only temporal syntax for portable tests. Lock dependencies in go.mod and go.sum; run go mod verify, go vet, go test and govulncheck in CI. Keep sqlc out of initial runtime dependencies; when added, query generator output must be committed and reviewed.
