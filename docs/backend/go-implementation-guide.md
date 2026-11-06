# Реализация Go-приложения по этапам

Основные требования [business-rules.md](business-rules.md) и [идея № 01](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L52). Работы выполняются **backend-разработчиком в main**, это только документация в backend-architecture. Никакой отдельной архитектуры в `features`.

## Фаза 0 — инфраструктура
Go module, config, net/http, pgxpool, goose, slog, /healthz, graceful shutdown, IANA TZ студии, CI. Подготовить SQL migrations по актуальной [database-schema.sql](database-schema.sql), не по старому global EXCLUDE. Приёмка: поднимается на пустой PostgreSQL и корректно останавливается.

## Фаза 1 — БД, учётные записи, кабинеты и персонал
Миграции resources/staff_accounts/account_roles/staff_sessions/services/clients/rooms/specialists/specialist_services/weekly/overrides/absence/bookings/events. Сеять одну студию, один кабинет и **одного специалиста только по явным данным**, без фальсификации исторических назначений. Проверить FK, nonunique phone, два EXCLUDE, nullable pair, audit. Приёмка: 2 специалиста могут иметь разные роли одного аккаунта, две комнаты допускаются.

## Фаза 2 — публичные справочники и авторизация
Catalog/site GET, sessions, смена временного пароля при первом входе, отдельные role checks для admin и specialist, cookies/CSRF. Приёмка: незалогиненный не видит ни администратора, ни специалиста; специалист никогда не получает телефон клиента и неназначенные заявки.

## Фаза 3 — студийные слоты + pure assignment solver
Опубликованный simple/advanced календарь, личные недели, overrides, approved absence, квалификация, rooms. Чистый `assignment.Solve`, проверка транзитивной компоненты, gap/buffer/DST, limit UNKNOWN. GET availability/dates/slots. Приёмка: совместимые два сеанса в разных комнатах видны, неподходящий staff не создаёт ложные слоты.

## Фаза 4 — атомарная заявка
POST booking-requests с Idempotency-Key и **studio-lock FOR UPDATE**, pending с NULL specialist/room/client, повторное вычисление SAT и записи events; 409 UNSAT/ 503 UNKNOWN. Приёмка: 20 конкурентных запросов в один кабинет дают один успех, при двух подходящих ресурсных парах — до двух успехов; один и тот же ключ даёт одну заявку.

## Фаза 5 — клиенты, назначения, статусы
Admin client linking (несколько карточек на номер), preview/assignment specialist + room auto-select, прямые запреты и подтверждаемые косвенные конфликты, booking_assignment_events, capacity_exceptions, admin attention dashboard, cancel/restore/confirm. Приёмка: confirmed не требует специалиста, но требует client_id; конфликты прямого назначения запрещены DB и сервисом.

## Фаза 6 — личные графики и запросы отсутствия
Admin schedule/staff qualifications, specialist submit/withdraw absence, admin preview/approve/reject whole request, archive staff/disable room, warnings and audit. Приёмка: pending absence не влияет на слоты, approved конфликт может оставить назначение с предупреждением, новые брони в проблемном компоненте запрещены.

## Фаза 7 — завершение специалистом и кабинеты
Specialist workspace: my bookings/schedule/reassignment events без телефона; own confirmed completion после ends_at; unlimited undo own completion, full audit. Приёмка: admin/чужой специалист не может завершить, early completion запрещён, отмена отметки возвращает confirmed.

## Фаза 8 — интеграция сайта и публикация
HttpSiteAPI adapter к frontend, режимы расписания и revision preview/publish, site settings, owner export, pagination; calendar QR/.ics без персональных данных и внутренних staff IDs. Приёмка: frontend не выбирает специалиста; админ вручную назначает и клиентскую карточку, не смешивая их состояния.

## Фаза 9 — надёжность
Два Go инстанса, PostgreSQL concurrent tests, solver performance bounds, migration/backup restore, PII audit, HTTP rate-limits, security checks, CI. Приёмка: гарантии вместимости не зависят от mutex, все миграции проверены на целевой PostgreSQL, нет PII в специалистских ответах.

Приоритет задают [go-testing.md](go-testing.md) и [migrations.md](migrations.md). Ни один этап не завершён лишь потому, что handler компилируется. Не переносить в production демонстрационные import/reset/clear endpoints.

## Фазы для идеи 01 — специалисты/кабинеты, обязательный следующий набор задач

Реализовать **после базового старта, но до публичного бронирования**: это **не** самостоятельный будущий модуль, который можно отложить после выпуска.

1. **Миграции и schema cutover.** `resources` трактовать как студию, создать `staff_accounts/account_roles`, `specialists`, `rooms`, их квалификацию и графики, absence_requests, историю назначений/завершений, capacity_exceptions. Удалить несовместимый **глобальный** `bookings_no_overlap`, создать два GiST EXCLUDE на специалиста/кабинет; зафиксировать studio-lock protocol. 
2. **Пакет assignment.** Чистый Go алгоритм совместного поиска по всем пересекающимся обязательствам; SAT/UNSAT/UNKNOWN, фиксированные назначения, поддержка парных ограничений, буферов и личных графиков. Unit-тесты до создания HTTP handlers.
3. **Публичный booking сервис.** Под `schedule_state FOR UPDATE` повторно считать solver, NULL assigned pair, snapshot услуги/клиента, idempotency; 20 конкурентных POST с 1 комнатой → только одна бронь. Две разные комнаты и сотрудники → две одновременные брони.
4. **Admin staff/rooms/absences.** CRUD/архивирование/приглашение, недельный график, approved leave с предупреждениями, admin preview/assign/reassign (room auto), лог событий, блок «Требуют внимания».
5. **RBAC + specialist workspace.** `account_roles`, принудительная смена первого пароля, чтение только своих booked/confirmed/completed без телефона, own absences, own reassignment history.
6. **Завершение специалистом.** POST specialist/me/bookings/{id}/complete после `ends_at`, undo тем же специалистом без срока, аудит. Удалить устаревший `adminCompleteBooking` и admin завершение из UI/REST.
7. **Интеграция и тесты.** Проверить все 77 операций OpenAPI, валидировать фронтенд-адаптер и permissions, запускать migration + многопроцессные тесты; документация `docs/backend` — источник backend-решений, а `ideas` — источник согласованных бизнес-правил.

Подробности в [бизнес-правилах](business-rules.md), [движке](scheduling-engine.md), [SQL](database-schema.sql), [транзакциях](database-operations.md), [контрактах Go](go-service-contracts.md), [Go тестах](go-testing.md) и [OpenAPI](openapi.yaml). Первоисточник: [идея 01](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L52).
