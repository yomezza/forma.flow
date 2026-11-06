# Жизненный цикл бронирования и назначений

[Источник идеи № 01, пункты о статусах](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L45-L52) · [Источник DemoAPI](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L43-L52).

~~~mermaid
stateDiagram-v2
  [*] --> pending_confirmation: публичная заявка
  pending_confirmation --> confirmed: админ подтверждает при linked client
  pending_confirmation --> cancelled: админ отменяет
  confirmed --> cancelled: админ отменяет
  confirmed --> completed: назначенный специалист после конца
  completed --> confirmed: тот же специалист отменяет свою отметку
  cancelled --> pending_confirmation: админ восстанавливает после solver
~~~

**Подтверждение ≠ назначение.** `specialist_id/room_id` оба NULL до назначения; `confirmed` законен без специалиста, но только с клиентской карточкой `client_id`. Дата, время, услуга и первоначальные данные посетителя — snapshots. Телефон не является идентичностью клиента; несколько клиентов могут иметь один номер.

Назначение/переназначение администратором допустимо для pending/confirmed, с выбором активного компетентного специалиста и автоматически или вручную определённого кабинета. Статус не меняется. Непригодный/занятый исполнитель или кабинет — прямой запрет; вытеснение **других неназначенных** можно подтвердить явно с аудитом и запретом новых заявок в проблемном временном компоненте. Отсутствия/архивирование не стирают историю, показывают предупреждения.

Новый `completed` ставит **только назначенный специалист** по окончании `ends_at` и при статусе `confirmed`; админ не имеет этого действия. Завершивший специалист может отменить отметку без срока, оставив событие в append-only history. `completed` и `cancelled` не переназначаются. Старый completed без автора после миграции — исторический legacy, не придумывать исполнителя.

Прямой SQL запрет накладок — два partial EXCLUDE по specialist_id и room_id; глобальный capacity solver плюс `schedule_state FOR UPDATE` защищает **неназначенные** заявки в разных экземплярах Go. Отмена frees resources; восстановление — новая полная проверка (клиент может оставаться связанным или NULL). Подробности в [database-operations.md](database-operations.md) и [scheduling-engine.md](scheduling-engine.md).

Специалист получает свои pending/confirmed/completed записи без телефонов и чужих броней; бывший специалист получает только события своего переназначения, а не чужую текущую карточку.

## Новая модель после идеи 01

`client_id`, `specialist_id`, `room_id` — **три разные связи**. После публичного POST все три NULL. `pending_confirmation → confirmed` требует client_id, но **не** требует specialist_id/room_id. Администратор подтверждает клиента после звонка; назначает или переназначает специалиста и кабинет независимо, до завершения. При закреплении пара specialist_id+room_id устанавливается вместе; без назначенного специалиста нет закреплённого кабинета.

`confirmed → completed`: текущий назначенный специалист, `now >= ends_at`, а не администратор. В одной транзакции сохранить `completed_by_specialist_id`, `completed_at` и completion/status/audit события. `completed → confirmed`: только автор собственной завершённости; сохранить историю события «undone». При повторном завершении должно снова проверяться окончание и фактический назначенный специалист. Отмена и восстановление по-прежнему админские. Восстановление проходит **совместную** проверку вместимости, а не только сравнение одного временного диапазона.

Занятость конкретного специалиста и кабинета защищают две DB EXCLUDE; неназначенные заявки защищает сериализованный solver. `completed` остаётся в историческом GiST EXCLUDE. Факт назначения никогда не меняет `booking.status` сам по себе.


## Идея 01: источник и приоритет

Согласованные требования: [назначения и вместимость](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L26), [личные графики и отсутствия](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L28-L34), [роли](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L36-L43), [завершение и аудит](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L45-L52). Текущий frontend не реализует эти правила; Go/PostgreSQL-проект в данном документе — целевое состояние.
