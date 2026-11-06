# DemoAPI v4 — структура localStorage и миграция без потерь

**Только демонстрационная модель**, не PostgreSQL. Основание: [идея 01](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L7-L70), [нынешний JS migrate()](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L94-L102), [начальные демо-данные](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-data.js#L1-L18).

## 1. Ключ, версия и сохранение старых полей

**Ключ не меняется:** `localStorage['formaTela.demo.v1']`. Внутри JSON `version:4`. Название ключа историческое — оно не диктует номер схемы. `migrate(db)` сначала вызывает существующие переходы v1→v2→v3, затем **один раз** v3→v4. Повторный вызов на v4 — no-op. Не вызывать `resetDemoData` при обновлении.

Сохранять побайтно по смыслу (не менять ID/дату/статус/цену/историю) прежние поля: `services`, `clients`, `bookings`, `settings`, `schedule.activeMode/published/drafts/history`, `content`, палитру/тексты. Не терять `settings.vacations`, `settings.blockedDates`, заметки отмен и снимки услуги в старых записях.

Новые обязательные корневые поля:
- `revision:number` — положительный или нулевой безопасный integer; увеличивается после **каждой успешной записи** через `change()`;
- `specialists:Specialist[]`, `rooms:Room[]`, `specialistServices:{specialistId,serviceId}[]`;
- `specialistSchedules:SpecialistSchedule[]`, `absenceRequests:AbsenceRequest[]`;
- `accounts:DemoAccount[]`, `demoSession:{activeAccountId,activeRole}`;
- `assignmentEvents:AssignmentEvent[]`, `auditEvents:AuditEvent[]`, `capacityExceptions:CapacityException[]`.
  
`booking` v4 расширить `specialistId:string|null`, `roomId:string|null`, `assignmentUpdatedAt:string|null`, `completion:null|{bySpecialistId,at}`; `clientId` **может быть null**. Статусы и снимки услуг остаются без изменений.

## 2. Строгие поля и примеры JSON

### Специалист, кабинет, разрешённые услуги

~~~json
{
  "specialists": [
    {
      "id": "sp-001",
      "fullName": "Специалист № 1",
      "state": "active",
      "accountId": "acct-owner",
      "createdAt": "2026-11-07T08:00:00.000Z",
      "archivedAt": null
    }
  ],
  "rooms": [
    {
      "id": "room-001",
      "name": "Кабинет № 1",
      "isActive": true,
      "sortOrder": 0
    }
  ],
  "specialistServices": [
    {"specialistId":"sp-001","serviceId":"classic"},
    {"specialistId":"sp-001","serviceId":"relax"},
    {"specialistId":"sp-001","serviceId":"back"}
  ]
}
~~~

`Specialist.id` и `Room.id` уникальны и неизменны. `fullName` 2..120 символов; `state:active|archived`; `archivedAt` ISO или null. `Room.name` 1..80, `sortOrder` integer 0..9999. Одну пару `specialistId/serviceId` хранить только один раз. Существующая услуга не исчезает при удалении специализации: будущие назначения теряют допустимость, прошлые сохраняются с предупреждением.

### Недельный график, личное исключение, отсутствие

~~~json
{
  "specialistSchedules": [
    {
      "specialistId": "sp-001",
      "revision": 1,
      "week": {
        "0": [],
        "1": [{"start":"09:00","end":"21:00"}],
        "2": [{"start":"09:00","end":"21:00"}],
        "3": [{"start":"09:00","end":"21:00"}],
        "4": [{"start":"09:00","end":"21:00"}],
        "5": [{"start":"09:00","end":"21:00"}],
        "6": []
      },
      "overrides": {
        "2026-11-13": {
          "kind": "working",
          "intervals": [{"start":"10:00","end":"14:00"}]
        },
        "2026-11-14": {"kind":"off","intervals":[]}
      }
    }
  ],
  "absenceRequests": [
    {
      "id": "abs-001",
      "specialistId": "sp-001",
      "kind": "vacation",
      "scope": "partial_day",
      "startDate": "2026-11-20",
      "endDate": "2026-11-20",
      "startTime": "14:00",
      "endTime": "17:00",
      "status": "pending",
      "reason": "Личные дела",
      "requestedAt": "2026-11-07T09:00:00.000Z",
      "reviewedByAccountId": null,
      "reviewedAt": null,
      "reviewNote": ""
    }
  ]
}
~~~

`week`: семь ключей `0..6`, где 0=воскресенье как JS `getDay()`; упорядоченные непересекающиеся интервалы `HH:mm`, `start<end`, все в пределах одних суток. `overrides` — только админские исключения `working|off` (нет третьего личного режима). Отсутствие `override` = недельное правило.

`absenceRequests.kind:day_off|vacation`, `scope:full_days|partial_day`; `status:pending|approved|rejected|withdrawn`. При `full_days`: включительный диапазон, `startTime/endTime:null`. При `partial_day`: `startDate===endDate`, обе строки `HH:mm`, положительный интервал. Ретроактивные даты в демо для новых заявок отклонять, но мигрированные исторические записи не удалять. Одновременные повторяющиеся pending/approved отсутствия одного специалиста на одинаковый период не допускаются (`ABSENCE_OVERLAP`).

### Демо-аккаунт и выбранная роль

~~~json
{
  "accounts": [
    {
      "id":"acct-owner",
      "login":"forma-demo-owner",
      "displayName":"Владелец студии (демо)",
      "roles":["admin","specialist"],
      "specialistId":"sp-001",
      "state":"active",
      "mustCompleteFirstDemoLogin":false
    }
  ],
  "demoSession":{"activeAccountId":"acct-owner","activeRole":"admin"}
}
~~~

Не хранить `password`, `passwordHash`, токены доступа и реальные секреты. В интерфейсе «Пригласить» показывать **одноразовую учебную пару** логин + случайный **DEMO**-код только в памяти диалога, с текстом «Это демонстрация, не защищённый вход». В localStorage сохраняется лишь `login` и `mustCompleteFirstDemoLogin:true`. При выборе нового аккаунта открывается экран имитации первого входа; кнопка «Завершить учебную смену пароля» меняет только флаг. Фактический пароль **не сохраняется и не сверяется**. Демо-переключение доступных аккаунтов — выбор учебного профиля, не аутентификация.

### Бронирование, аудит и явное исключение

~~~json
{
  "bookings": [
    {
      "id":"book-demo-01",
      "serviceId":"classic",
      "name":"Классический массаж",
      "durationMinutes":60,
      "priceAmount":3500,
      "currency":"RUB",
      "date":"2026-11-20",
      "time":"10:00",
      "status":"pending_confirmation",
      "clientId":null,
      "clientName":"Иван Иванов",
      "clientPhone":"+79991234567",
      "specialistId":null,
      "roomId":null,
      "assignmentUpdatedAt":null,
      "completion":null,
      "createdAt":"2026-11-07T09:00:00.000Z",
      "updatedAt":"2026-11-07T09:00:00.000Z"
    }
  ],
  "assignmentEvents":[
    {
      "id":"evt-001",
      "bookingId":"book-demo-01",
      "at":"2026-11-07T09:30:00.000Z",
      "actorAccountId":"acct-owner",
      "fromSpecialistId":null,
      "toSpecialistId":"sp-001",
      "fromRoomId":null,
      "toRoomId":"room-001",
      "affectedBookingIds":[],
      "explicitOverride":false
    }
  ],
  "auditEvents":[
    {
      "id":"audit-001",
      "at":"2026-11-07T09:30:00.000Z",
      "actorAccountId":"acct-owner",
      "activeRole":"admin",
      "action":"booking.assignment_changed",
      "entityType":"booking",
      "entityId":"book-demo-01",
      "metadata":{"oldSpecialistId":null,"newSpecialistId":"sp-001"}
    }
  ],
  "capacityExceptions":[
    {
      "id":"capacity-001",
      "createdByEventId":"evt-010",
      "bookingIds":["book-demo-02"],
      "reason":"INDIRECT_ASSIGNMENT_CONFLICT",
      "createdAt":"2026-11-07T09:40:00.000Z",
      "resolvedAt":null
    }
  ]
}
~~~

`roomId:null` у публичной заявки означает **не закреплён**, а не «заявка не занимает места». `specialistId:null` аналогично. `assignmentEvents` append-only; `auditEvents` append-only по содержанию демо, но локально технически редактируемы владельцем браузера. Аудит не должен содержать телефоны и пароли. `capacityExceptions` хранит решение администратора, его `bookingIds` и источник; актуальные UI предупреждения дополнительно пересчитываются.

## 3. Пошаговая миграция v3 → v4

~~~js
function migrateV3toV4(old, now = new Date().toISOString()) {
  if (old.version !== 3) throw new Error('VERSION');
  const db = structuredClone(old); // не мутировать старые сохранённые данные до проверки
  const ownerId='acct-owner', specialistId='sp-001', roomId='room-001';
  const activeServiceIds=db.services.filter(s=>s.isActive).map(s=>s.id);
  const week=Object.fromEntries(
    Array.from({length:7},(_,day)=>[
      String(day),db.settings.workDays.includes(day)
        ? [{start:db.settings.start,end:db.settings.end}] : []
    ])
  );
  db.specialists=[{id:specialistId,fullName:'Специалист № 1',
    state:'active',accountId:ownerId,createdAt:now,archivedAt:null}];
  db.rooms=[{id:roomId,name:'Кабинет № 1',isActive:true,sortOrder:0}];
  db.specialistServices=activeServiceIds.map(serviceId=>({specialistId,serviceId}));
  db.specialistSchedules=[{specialistId,revision:1,week,overrides:{}}];
  db.absenceRequests=[];
  db.accounts=[{id:ownerId,login:'forma-demo-owner',displayName:'Владелец студии (демо)',
    roles:['admin','specialist'],specialistId,state:'active',mustCompleteFirstDemoLogin:false}];
  db.demoSession={activeAccountId:ownerId,activeRole:'admin'};
  db.assignmentEvents=[];db.auditEvents=[];db.capacityExceptions=[];
  db.revision=0;
  db.bookings=db.bookings.map(b=>({
    ...b, specialistId:null,roomId:null,assignmentUpdatedAt:null,completion:null
  }));
  db.version=4;
  return db; // запись только после полной validateV4
}
~~~

**Нельзя при миграции**: автоматически объявлять, что специалист № 1 провёл все старые сеансы; назначать roomId старым записям; менять старые `clientId` или телефоны; удалять finished/ cancelled/ legacy confirmed. Старые записи без специалиста получают вычисляемое предупреждение `UNASSIGNED_SPECIALIST` только если ещё действующие. Старые completed хранятся как завершённая история без подтверждённого нового `completion.bySpecialistId`; это допустимое **наследованное** исключение.

**Сначала** разобрать JSON, применить v1→v3 при необходимости, сделать v4 в памяти, проверить ссылки/типы/ограничения, **затем** атомарно выполнить один `localStorage.setItem(key,JSON.stringify(db))`. Не стирать старый ключ до успешной записи. При переполненном localStorage сохранить старые данные, показать ошибку `STORAGE_UNAVAILABLE` и не заменять базу пустым seed. Поддерживать текущий fallback-memory режим для недоступного storage.

Если клиентский телефон в старой базе уникален, это не означает, что новый валидатор должен сохранять ограничение: в v4 **допускаются дубликаты** телефона при разных ID клиента.

**Версия выше 4:** fail-closed с `VERSION_UNSUPPORTED`; не перезаписывать неизвестные данные. Импорт и экспорт: JSON v4, возможность импортировать v1–v3 через те же миграции; при ошибке проверок не трогать текущие записи. Для отсутствующего (нового) key seed v1 → standard migrate → v4. При повторном запуске v4 миграция не создаёт дополнительных специалистов/событий.

## 4. Правила валидации

- Уникальны ID в каждой коллекции, но **не телефон**; ссылки на услуги, специалистов, кабинеты, аккаунты должны существовать или быть nullable по контракту.
- Архивные специалисты/неактивные кабинеты не становятся «отсутствующими» в исторических ссылках; просто исключаются из доменов новых назначений.
- Действующие новые двойные *закреплённые* назначения одного специалиста или кабинета с пересечением — ошибка `DIRECT_RESOURCE_CONFLICT`; **старые** данные, которые стали некорректны после утверждённого отсутствия, не удалять и не пытаться «исправить» базовой валидацией, только предупреждение.
- Не запрещать одновременные **неназначенные** записи лишь из-за совпадающего времени; обязан проверять capacity solver на момент создания. После подтверждённого override данные могут содержать открытый `CapacityException` — это допустимое демо-состояние с предупреждением.
- Длина JSON / кол-во сущностей может быть ограничена для работоспособности демо; при превышении `DEMO_LIMIT`, без потери уже сохранённых данных.
- Изменение каждой коллекции только через `change()`, с переоценкой предупреждений и `revision++` после успешной проверки.

## 5. Совместимость с AdminDraftStore

Старый `AdminDraftStore` живёт в `sessionStorage` и хранит незавершённые формы [в `frontend`](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/draft-store.js#L1-L13). Новые ключи: `specialists/profile/<id|new>`, `specialists/schedule/<id>`, `specialists/absence-review/<id>`, `specialist/absence/new`. Они сохраняют только несохранённый UI, не являются публикацией графика или утверждением заявки. Не складывать персональные телефоны, временные демо-коды и пароли в sessionStorage; при изменении исходной `revision` показать сравнение и не затирать чужие изменения молча.
