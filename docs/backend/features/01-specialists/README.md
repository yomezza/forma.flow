# Идея 01 — специалисты, назначения и личные графики

**Статус:** спецификация **демонстрационной логики** для будущей реализации frontend-разработчиком. Код `frontend` не изменён. **Серверная архитектура для идеи № 01 теперь обновлена непосредственно в `docs/backend/`**: SQL, Go, API, бизнес-правила, миграции и тесты. В этой папке остаются **только спецификации демонстрационного DemoAPI**.

## Источники через GitHub-ссылки между ветками

- [Идея и решения 1–49](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L1-L78) — согласованные требования.
- [Прямые правила распределения и кабинетов](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L26).
- [Графики и согласование отсутствия](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L28-L34).
- [Роли и личный кабинет](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L36-L43).
- [Статусы, завершение и аудит](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L45-L52).
- [Промпт архитектору](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/architect_prompt.md#L1-L134) — исходные требования; демо-спецификация здесь, Go/PostgreSQL — в основных документах.
- [Текущий DemoAPI](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L1-L345), [договорённости frontend API](https://github.com/yomezza/forma.flow/blob/frontend/docs/DEMO_API.md#L1-L108), [административный UI](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L1-L348).
- [Обновлённая Go/PostgreSQL-архитектура](../../README.md), [SQL-схема](../../database-schema.sql), [сервисы Go](../../go-service-contracts.md), [OpenAPI](../../openapi.yaml) и [карта DemoAPI → backend](../../api-mapping.md) — единый серверный источник истины.

## Что читать frontend-разработчику, в этом порядке

1. [gap-analysis.md](gap-analysis.md) — что существует и что нужно заменить/добавить.
2. [domain-rules.md](domain-rules.md) — утверждённые права, состояния, исключения и инварианты демонстрации.
3. [assignment-engine.md](assignment-engine.md) — центральная функция проверки совместной размещаемости заявок.
4. [demo-data-migration.md](demo-data-migration.md) — формат `localStorage`, JSON-примеры, переход с `version:3` без потери данных.
5. [demo-api-spec.md](demo-api-spec.md) — конкретные сигнатуры методов, результаты, ошибки и изменения данных.
6. **[frontend-handoff.md](frontend-handoff.md)** — задания фронтендеру по этапам P0–P8, конкретные UI-экраны, связи методов и критерии приёмки.
7. **[demo-test-cases.md](demo-test-cases.md)** — 69 воспроизводимых проверок от миграции и solver до приватности и мобильной адаптации.
8. [decisions.md](decisions.md) — инженерные уточнения и границы принятых допущений.

**Go/PostgreSQL-архитектура — в [основной документации `docs/backend`](../../README.md)**. Здесь только описание будущего браузерного DemoAPI. Реализация кода, тестов и публикация сайта — отдельная задача разработчика `frontend`; наличие технического задания не означает их готовности.

### Нельзя перепутать

- **Утверждено владельцем:** содержание идеи в ветке `ideas`.
- **Реализовано:** только существующий JavaScript DemoAPI в `frontend`, который пока **не поддерживает** специалистов, кабинеты и роли.
- **Спроектировано здесь:** будущая демонстрационная модель. Она не обеспечивает настоящую авторизацию, синхронизацию между устройствами и защиту от конкурентной записи в PostgreSQL.
- Изменять `frontend`, `main` и `ideas` в этой задаче запрещено. Никакого merge.
- Архитектура Go/PostgreSQL уже спроектирована в основных документах `docs/backend/`, однако серверный код не реализован, миграции не применялись и интеграционные тесты не запускались.

## Результат

Frontend-разработчик может реализовать новую демонстрационную механику по документам выше: данные, алгоритм доступности, методы SiteAPI, сценарии администратора и специалиста. Результаты работ по самой демоверсии не следует выдавать за production-ready backend.

**Проверенный baseline frontend:** [`02be239`](https://github.com/yomezza/forma.flow/commit/02be23938f6861a78781ba4d7039d7e755ccde1d). В этой версии только начат v4, а `validate()` ещё требует v3: см. [P0-блокер](frontend-handoff.md#0-сначала-исправить-сломанный-фундамент-v4-блокер-p0). Начните с исправления миграции и только потом подключайте совместный matcher.
