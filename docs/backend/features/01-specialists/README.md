# Идея 01 — специалисты, назначения и личные графики

**Статус:** спецификация **демонстрационной логики** для будущей реализации frontend-разработчиком. Код `frontend` не изменён; Go, PostgreSQL, SQL и production OpenAPI **не проектируются и не обновляются в этой итерации** по отдельному решению владельца. Документы в `docs/backend/` остаются базовой архитектурой действующей версии до будущей отдельной задачи.

## Источники через GitHub-ссылки между ветками

- [Идея и решения 1–49](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L1-L78) — согласованные требования.
- [Прямые правила распределения и кабинетов](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L26).
- [Графики и согласование отсутствия](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L28-L34).
- [Роли и личный кабинет](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L36-L43).
- [Статусы, завершение и аудит](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L45-L52).
- [Промпт архитектору](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/architect_prompt.md#L1-L134) — из разделов А–В выполняется **демо-часть**, раздел Г (Go/PostgreSQL) осознанно отложен.
- [Текущий DemoAPI](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L1-L345), [договорённости frontend API](https://github.com/yomezza/forma.flow/blob/frontend/docs/DEMO_API.md#L1-L108), [административный UI](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L1-L348).
- [Существующая базовая архитектура](../../README.md) и [карта 57 методов](../../api-mapping.md) — **не изменялись**; расширение размещено отдельно.

## Что читать frontend-разработчику, в этом порядке

1. [gap-analysis.md](gap-analysis.md) — что существует и что нужно заменить/добавить.
2. [domain-rules.md](domain-rules.md) — утверждённые права, состояния, исключения и инварианты демонстрации.
3. [assignment-engine.md](assignment-engine.md) — центральная функция проверки совместной размещаемости заявок.
4. [demo-data-migration.md](demo-data-migration.md) — формат `localStorage`, JSON-примеры, переход с `version:3` без потери данных.
5. [demo-api-spec.md](demo-api-spec.md) — конкретные сигнатуры методов, результаты, ошибки и изменения данных.
6. [frontend-handoff.md](frontend-handoff.md) — порядок реализации UI, интеграции и чек-лист.
7. [demo-test-cases.md](demo-test-cases.md) — воспроизводимые JS тесты, в том числе сложные конфликты.
8. [decisions.md](decisions.md) — инженерные уточнения и границы принятых допущений.

### Нельзя перепутать

- **Утверждено владельцем:** содержание идеи в ветке `ideas`.
- **Реализовано:** только существующий JavaScript DemoAPI в `frontend`, который пока **не поддерживает** специалистов, кабинеты и роли.
- **Спроектировано здесь:** будущая демонстрационная модель. Она не обеспечивает настоящую авторизацию, синхронизацию между устройствами и защиту от конкурентной записи в PostgreSQL.
- Изменять `frontend`, `main` и `ideas` в этой задаче запрещено. Никакого merge.
- Наличие ссылок на старые backend-документы **не означает**, что их схема готова для нескольких специалистов: перенос на Go/PostgreSQL будет отдельным проектированием.

## Результат

Frontend-разработчик может реализовать новую демонстрационную механику по документам выше: данные, алгоритм доступности, методы SiteAPI, сценарии администратора и специалиста. Результаты работ по самой демоверсии не следует выдавать за production-ready backend.
