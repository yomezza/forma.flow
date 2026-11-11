# Проверка архитектуры перед реализацией

Источник: `backend-architecture` @ `160f0df`; frontend @ `f640caa`.
Прочитаны все 25 документов из docs/backend/README.md, включая полные схемы,
SQL и примеры OpenAPI; идея 01 обязательна.

Приложение: Go modular monolith, net/http, pgxpool, goose, slog, Argon2id.
Порядок работ: фазы 0–9 go-implementation-guide.md.

Замечания для архитектора (его ветка не изменяется):

- [go-implementation-guide.md](https://github.com/yomezza/forma.flow/blob/160f0df/docs/backend/go-implementation-guide.md): «77 операций» устарело. OpenAPI содержит 79 операций / 64 пути; используется полный набор.
- [go-service-contracts.md](https://github.com/yomezza/forma.flow/blob/160f0df/docs/backend/go-service-contracts.md) называет VERSION_CONFLICT/CONFLICT_ACK_REQUIRED, [api-contracts.md](https://github.com/yomezza/forma.flow/blob/160f0df/docs/backend/api-contracts.md) уточняет STALE_CAPACITY_REVISION/INDIRECT_ASSIGNMENT_CONFLICT для assignment. Реализация следует более конкретным wire-правилам. Schedule revision использует VERSION_CONFLICT.
- [authorization.md](https://github.com/yomezza/forma.flow/blob/160f0df/docs/backend/authorization.md) требует specialist-контекст для dual-role, но OpenAPI не задаёт транспорта выбора activeRole. Specialist routes сами обозначают контекст операции; права всегда берутся из БД, связь account→specialist обязательна. Никакой передачи прав через заголовок.
- [scheduling-engine.md](https://github.com/yomezza/forma.flow/blob/160f0df/docs/backend/scheduling-engine.md) предписывает отклонять DST gap/fold; README оставляет это открытым. Следуем точному правилу движка. TZ задаётся явно, без предположений о фактическом местонахождении студии.
- SpecialistSchedule PUT использует поле revision (не expectedCapacityRevision): его сравниваем с capacity_revision студии, возвращаем актуальное значение. Нового поля API не добавляем.
- SQL допускает минуту 1440, OpenAPI Interval — только HH:mm до 23:59. HTTP v1 принимает лишь значения контракта; SQL-диапазон шире, нарушения совместимости нет.
- BookingReceipt при replay должен сохраняться даже после смены статуса заявки: добавляется сохранённый JSON ответа в idempotency_keys, без PII. Ссылки на живую booking недостаточно для требования «same receipt».

Архитектура веток frontend/ideas/backend-architecture и принятые бизнес-правила сохраняются.

Дополнительные конкретизации реализации:

- Для исторического простого недельного графика restore создаёт 366 дней
  черновиков от текущей **локальной** даты студии; сохраняет также датированные
  правила из снимка и текущего состояния. Контракт restore не принимает диапазон,
  поэтому это конечный горизонт генерации, а публикация остаётся выборочной.
- Studio SQL lock применяется и при отмене/завершении/undo. Повторная сверка
  capacity_exceptions затрагивает только загруженные транзитивные компоненты;
  исправление одной записи не снимает предупреждения в другом интервале.
- Admin-списки ограничиваются SQL LIMIT/OFFSET; cursor кодирует смещение. GET
  доступности использует один согласованный REPEATABLE READ snapshot и пакетные
  чтения, не выполняет SQL для каждого слота.
- Нормализация названий ошибок сохранена в wire API: VERSION_CONFLICT для
  студийных правил, STALE_CAPACITY_REVISION для назначений/отсутствий,
  INDIRECT_ASSIGNMENT_CONFLICT для подтверждаемого косвенного назначения.
- Дополнительные проверки перед завершением: обязательная смена пароля
  проверяется также в доменном actor gate; повторяющиеся JSON-ключи и слишком
  глубокие вложения отклоняются. Preview отсутствия рассматривает компоненты,
  где отсутствие меняет допустимых исполнителей, и не требует подтверждать
  конфликт в независимом времени.

Уточнение ответов OpenAPI в ветке реализации: исходный openapi.yaml не перечислял
503 для части старых операций, хотя scheduling-engine.md и database-operations.md
явно требуют UNKNOWN → CAPACITY_CHECK_LIMIT (503), а SQL timeout также даёт 503.
В нём также отсутствовал 403 для части публичных POST с проверкой Origin.
В backend/api/openapi.yaml добавлены эти ответы с существующим ErrorEnvelope;
маршруты, operationId и JSON-схемы не изменены (79 операций / 64 пути).
Предложение архитектору — перенести такое же уточнение в исходный контракт;
его ветка этой работой не изменяется. Независимый conformance suite проверяет
и отказ входа с чужим Origin.
