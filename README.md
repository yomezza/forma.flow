# FORMA TELA

Go-backend для онлайн-записи, нескольких специалистов и кабинетов.
Исходная архитектура: [`backend-architecture`](https://github.com/yomezza/forma.flow/tree/backend-architecture/docs/backend).
Frontend и DemoAPI находятся в отдельной ветке [`frontend`](https://github.com/yomezza/forma.flow/tree/frontend).

Реализация в [`backend/`](backend/): Go 1.27.4, net/http, PostgreSQL 16–18,
pgxpool, goose, Argon2id. Все 79 операций закреплённого OpenAPI реализованы.

- [Запуск, миграции и тесты](backend/README.md).
- [Контракт HTTP](backend/api/openapi.yaml).
- [Интеграция frontend](docs/implementation/frontend-handoff.md).
- [Замечания к исходной архитектуре](docs/implementation/architecture-review.md).
- [Результаты проверок и эксплуатационные ограничения](docs/implementation/verification.md).

Все изменения ведутся в `main`. Ветки `frontend`, `ideas`, `backend-architecture`
этой реализацией не изменяются.
