# DemoAPI / SiteAPI v4 — точный JavaScript-контракт идеи 01

Здесь перечислены **проектируемые, ещё не реализованные** методы демонстрации. Основы: [идея](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L52), [реальный интерфейс DemoAPI](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L246-L345), [существующий список методов](https://github.com/yomezza/forma.flow/blob/frontend/docs/DEMO_API.md#L1-L108). Никаких Go handlers, SQL-запросов, REST URL и production OpenAPI в этой итерации **не проектируем**; серверный mapping будет отдельной задачей.

## 0. Общий контракт

- `SiteAPI = DemoAPI` сохраняется. Все методы вызываются через `SiteAPI` и **возвращают Promise**. Чтение возвращает клон, запись выполняется в существующем `change(fn)`: прочесть актуальный v4 → проверить `revision` при необходимости → построить проектируемое состояние → полный валидатор → `revision++` → одна запись в localStorage → уведомление `forma-change` → Promise результата. В случае ошибки **ничего не изменять**.
- Объекты ID остаются строками, даты `YYYY-MM-DD`, время `HH:mm`, datetime ISO. Nullable ID имеют значение `null`, а не `undefined`. `status` заявки по-прежнему `pending_confirmation|confirmed|completed|cancelled`.
- Входные данные валидируются до действия. Возвращаемые массивы отсортированы стабильно: по `sortOrder,id` (кабинеты), по `fullName,id` (специалисты), `date,time,id` (записи).
- `account` берётся из `db.demoSession.activeAccountId`; переключение `activeRole` не выдаёт дополнительных прав. Ролевой контроль внутри DemoAPI нужен для **демонстрации поведения UI**; localStorage доступен любому браузерному скрипту и **не является защитой**.
- Публичные методы никогда не раскрывают специалиста, назначенного кабинета, другие заявки или телефоны клиентов. Посетитель узнаёт специалиста только от администратора по телефону после подтверждения.
- Общие коды ошибок: `VALIDATION`, `NOT_FOUND`, `ROLE_FORBIDDEN`, `STALE_REVISION`, `SLOT_UNAVAILABLE`, `GLOBAL_CAPACITY_CONFLICT`, `CAPACITY_CHECK_LIMIT`, `DIRECT_RESOURCE_CONFLICT`, `SPECIALIST_INELIGIBLE`, `ROOM_UNAVAILABLE`, `INDIRECT_ASSIGNMENT_CONFLICT`, `CONFLICT_ACK_REQUIRED`, `CLIENT_LINK_REQUIRED`, `ABSENCE_CONFLICT`, `ABSENCE_OVERLAP`, `INVALID_STATUS_TRANSITION`, `NOT_ENDED_YET`, `DEMO_LIMIT`, `VERSION_UNSUPPORTED`, `STORAGE_UNAVAILABLE`. Ошибка — `Object.assign(new Error(message),{code,details})`, где `details` может содержать ID конфликтов для административного интерфейса, но не в публичном ответе.
- В публичной функции `getAvailableSlots` причины внутреннего отказа не раскрываются: просто отсутствие слота; `CAPACITY_CHECK_LIMIT` отдельно объясняется без деталей о персонале.
- Результаты preview включают `revision` исходной базы. Все действия, которым предшествует подтверждение preview, требуют равную `expectedRevision` и пересчёт внутри `change()`.

## 1. Посетитель и существующие публичные методы

| Метод — сигнатура | Ответ Promise | Логика / изменение | Ошибки |
|---|---|---|---|
| `getServices()` | `Service[]` | Без выбора специалиста; активные услуги публично | текущие ошибки чтения |
| `getAvailableSlots(date,serviceId)` | `string[]` HH:mm | Генерация кандидатов из опубликованного студийного графика и **решение общей задачи размещения** по [движку](assignment-engine.md); не изменяет БД | VALIDATION, NOT_FOUND, CAPACITY_CHECK_LIMIT |
| `getDateStates(from,to,serviceId)` | `{[date]: 'past'|'working'|'full'|'off'|'vacation'|'blocked'|'unplanned'}` | Для working ставить full, если у указанной услуги нет ни одного `SAT` слота; если serviceId не указан, не считать день «полным» по условию `!id` | VALIDATION, CAPACITY_CHECK_LIMIT |
| `createBookingRequest({serviceId,date,time,fullName,phone})` | `PublicBookingReceipt` | Повторить проверку `SAT` в `change()`; резервирует статус pending; `specialistId:null,roomId:null,clientId:null`. Не искать клиента по телефону, не создавать клиентскую карточку | SLOT_UNAVAILABLE, GLOBAL_CAPACITY_CONFLICT, CAPACITY_CHECK_LIMIT |
| `getSettings()`, `getContent()` | существующие публичные данные | Студийный график/брендинг прежние, personalSchedules в публичный ответ не включать | текущие |

~~~json
{
  "id":"book-901",
  "serviceId":"classic",
  "name":"Классический массаж",
  "durationMinutes":60,
  "priceAmount":3500,
  "currency":"RUB",
  "date":"2026-11-20",
  "time":"11:00",
  "status":"pending_confirmation"
}
~~~

`PublicBookingReceipt` сохраняет поля, используемые [календарным событием в форме](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/booking-form.js#L46-L54). **Без** clientPhone, clientId, specialistId, roomId и внутренних предупреждений. Поля `clientName/clientPhone` хранятся только во внутреннем `bookings[]`.

## 2. Специалисты и кабинеты — только admin

| Метод | Ответ | Действие / ограничения |
|---|---|---|
| `getSpecialists({includeArchived=false}={})` | `Promise<Specialist[]>` | Чтение. Активные по умолчанию; архивные с флагом |
| `getSpecialist(id)` | `Promise<Specialist>` | Чтение по ID; NOT_FOUND |
| `createSpecialist({fullName,serviceIds,week,inviteAccount=false})` | `Promise<Specialist>` | Сгенерировать ID, создать график/перечень услуг, при invite вызвать учебное создание аккаунта; одна запись и аудит |
| `updateSpecialist(id,{fullName})` | `Promise<Specialist>` | Переименовать, не менять архивную историю |
| `archiveSpecialist(id)` | `Promise<Specialist>` | state=archived; сохранение прошлых назначений, `REQUIRES_REASSIGNMENT` для будущих; лишить специалиста нового назначения |
| `setSpecialistServices(id,serviceIds)` | `Promise<{specialistId,serviceIds,revision}>` | Массив действительных ID услуг, без дубликатов; не переназначать существующие записи; предупреждения пересчитать |
| `getRooms({includeInactive=false}={})` | `Promise<Room[]>` | Список кабинетов, в текущем seed один |
| `createRoom({name,sortOrder=0})` | `Promise<Room>` | Активный кабинет, новый ID; уникальный ID, имя не обязательно уникально |
| `updateRoom(id,{name,sortOrder,isActive})` | `Promise<Room>` | Смена полей, деактивация помечает существующие назначения ROOM_INACTIVE и сохраняет историю |
| `getSpecialistSchedule(specialistId)` | `Promise<SpecialistSchedule>` | Week + overrides + revision |
| `setSpecialistSchedule(id,{week,overrides,expectedRevision})` | `Promise<{specialistId,revision,warnings}>` | Изменяет **только admin**, проверяет диапазоны, сохраняет существующие брони, возвращает вычисленные предупреждения; STA​LE_REVISION при несовпадении |

Возвращаемые специалистам объекты профиля/графика **не содержат** чужих аккаунтов, контактов и массива клиентов. `expectedRevision` сравнивается с `db.revision` для сохранений после предпросмотра; `specialistSchedules[].revision` нужен для UI-индикатора изменений конкретной карточки.

## 3. Предпросмотр и назначение — только admin

| Метод | Promise | Действие / отказ |
|---|---|---|
| `getAssignmentCandidates(bookingId)` | `{revision,candidates:[{specialistId,roomIds,requiresConfirmation,affectedBookingIds}],reasons:string[]}` | Перечислить допустимые по компетенции/времени пары; выполнить глобальный preview для каждой. Прямые конфликты не становятся доступными кандидатами |
| `previewBookingAssignment(bookingId,{specialistId,roomId=null})` | `AssignmentPreview` | Проиграть предложение без сохранения; `roomId:null` = автоматический выбор лучшего свободного кабинета; посмотреть косвенно вытесненные заявки |
| `assignBooking(bookingId,{specialistId,roomId=null,expectedRevision,acknowledgedBookingIds=[]})` | `BookingAdminDTO` | Повторить preview на актуальных данных, reject при direct/STALE; при indirect требовать **точные** ID и явное подтверждение, записать `bookings[].specialistId/roomId`, assignmentEvents, auditEvents, обновить capacityExceptions |
| `getBookingAssignmentEvents(bookingId)` | `AssignmentEvent[]` | Весь журнал изменений исполнителя/кабинета |
| `getBookingAttention(bookingId)` | `AttentionItem[]` | Вычисленные причины по записи, не изменяют статус |
| `getAttentionItems()` | `{revision,items:AttentionItem[],counts:{byCode:{},total:number}}` | Блок «Требуют внимания»: нет специалиста, перепривязать, нет доступного, поданная заявка на отсутствие, вне студийного графика и т. п. |

~~~json
{
  "allowed":true,
  "requiresConfirmation":true,
  "revision":17,
  "specialistId":"sp-001",
  "roomId":"room-001",
  "reason":"INDIRECT_ASSIGNMENT_CONFLICT",
  "affectedBookingIds":["book-902"],
  "candidates":[],
  "message":"Это назначение оставит одну неназначенную заявку без исполнителя."
}
~~~

Строгая политика: `previewBookingAssignment` может вернуть `allowed:false` с `SPECIALIST_INELIGIBLE`, `SPECIALIST_BUSY`, `ROOM_UNAVAILABLE`, `DIRECT_RESOURCE_CONFLICT`, `CAPACITY_CHECK_LIMIT`. Админ **не может подтвердить прямое нарушение**. Косвенный конфликт допускается только когда перечислены пострадавшие **неназначенные** заявки и `acknowledgedBookingIds` точно совпадают. Изменение назначения **никогда не вызывает** `confirmBooking`, не обновляет `booking.status` и не меняет снимок клиента/услуги. Завершённые и отменённые записи не переназначать.

Для `roomId:null` возвращать конкретный автоматически подобранный `roomId` **в preview**; при `assignBooking` повторно его проверить, сохранить выбранный кабинет. Если администратор явно предлагает другой кабинет, проверить его по тем же правилам.

## 4. Отсутствия и личный график

| Метод | Promise | Действие |
|---|---|---|
| `getMyAbsenceRequests()` | `AbsenceRequest[]` | specialist видит только свои запросы |
| `submitMyAbsence({kind,scope,startDate,endDate,startTime=null,endTime=null,reason=''})` | `AbsenceRequest` | specialist, только от собственного `specialistId`; `status:pending` и аудит; ещё **не меняет** график |
| `withdrawMyAbsence(id)` | `AbsenceRequest` | specialist, свой pending → withdrawn, иначе INVALID_STATUS_TRANSITION |
| `getAbsenceRequests({status=null,specialistId=null}={})` | `AbsenceRequest[]` | admin видит все; сортировка pending first, затем requestedAt |
| `previewAbsenceApproval(id)` | `{revision,requestId,conflicts:[{bookingId,date,time}],requiresConfirmation}` | admin проверяет пересечения назначений и последствия capacity; ничего не изменяет |
| `approveAbsence(id,{expectedRevision,acknowledgedBookingIds=[]})` | `AbsenceRequest` | admin; pending→approved **целиком**, после повторной проверки; предупреждение `ABSENCE_CONFLICT` при конфликте; явное подтверждение позволяет сохранить назначенные заявки с REQUIRES_REASSIGNMENT |
| `rejectAbsence(id,{reason,expectedRevision})` | `AbsenceRequest` | admin; pending→rejected, обязательная непустая причина. Не изменяет график |

Пример полной заявки на выходные:
~~~json
{"kind":"day_off","scope":"full_days","startDate":"2026-11-27","endDate":"2026-11-29","startTime":null,"endTime":null,"reason":"Отдых"}
~~~

Пример частичного отсутствия:
~~~json
{"kind":"vacation","scope":"partial_day","startDate":"2026-11-20","endDate":"2026-11-20","startTime":"14:00","endTime":"17:00","reason":"Личные дела"}
~~~

Одобрение **не переносит автоматически** бронирование, не меняет его статус/исполнителя. Список `conflicts` содержит назначения, которые после approval перестали соответствовать графику; помечать `REQUIRES_REASSIGNMENT`. Повторное approve/reject/withdraw после финального решения — INVALID_STATUS_TRANSITION.

## 5. Демо-аккаунты, роли и личное рабочее пространство

| Метод | Promise | Действие |
|---|---|---|
| `getDemoAccounts()` | `{id,login,displayName,roles,state,mustCompleteFirstDemoLogin}[]` | Только учебный список без каких-либо секретов |
| `inviteDemoAccount({displayName,login,roles,specialistId})` | `{accountId,temporaryLogin,temporaryDemoCode}` | admin создаёт account с `mustCompleteFirstDemoLogin:true`, код показать однократно **в оперативной памяти**, не записывать в localStorage |
| `selectDemoAccount(accountId)` | `{accountId,activeRole,firstDemoLoginRequired}` | Учебный выбор профиля. **Не аутентификация** |
| `completeFirstDemoLogin()` | `{firstDemoLoginRequired:false}` | UI-шаг «демонстрационная смена пароля», только снимает флаг, не хранит/сверяет пароль |
| `switchDemoRole(role)` | `{accountId,activeRole}` | Роль обязана входить в account.roles; смена workspace без смены аккаунта |
| `getMyBookings({includeCompleted=true}={})` | `SpecialistBookingDTO[]` | specialist: только `booking.specialistId===account.specialistId`; pending/confirmed/completed; **без телефонов, clientId и чужих записей** |
| `getMySchedule(from,to)` | `{week,overrides,approvedAbsences,assignedBookings}` | specialist: свой график и свои назначения, без чужого |
| `getMyReassignmentEvents()` | `ReassignmentEventDTO[]` | specialist: события собственного прежнего/текущего назначения, без чужих активных бронирований и телефонов |
| `markMyBookingCompleted(bookingId)` | `SpecialistBookingDTO` | Только назначенный активный специалист, status confirmed, `now>=plannedEnd`; обновить completed и добавить audit |
| `undoMyCompletion(bookingId,{reason='' }={})` | `SpecialistBookingDTO` | Только завершивший тот же специалист; completed→confirmed в любой момент; причина необязательна, audit обязателен |

~~~json
{
  "id":"book-901",
  "clientName":"Иван Иванов",
  "serviceName":"Классический массаж",
  "date":"2026-11-20",
  "time":"11:00",
  "durationMinutes":60,
  "status":"pending_confirmation",
  "roomName":"Кабинет № 1"
}
~~~

**Запрещены** `phone`, `clientPhone`, `phoneNormalized`, `clientId`, `unassignedBookingIds`, `allBookings`, другие клиентские карточки в ответе для специалиста. Старый `exportData()` не использовать как источник его workspace: он содержит все контакты.

## 6. Клиенты — согласованное ранее правило нескольких профилей с одним номером

| Метод | Решение DemoAPI v4 |
|---|---|
| `getClients()/getClientById()` | admin как раньше |
| `findClientByPhone(phone)` | **устарел для админского подбора**: нельзя выбирать первого. Совместимый старый метод не должен автоматически связывать |
| `findClientsByPhone(phone)` | Новый admin метод → `Client[]` всех совпадений, включая нескольких родственников |
| `createClient(data),updateClient(id,data)` | Телефон не уникален, отличает только ID. Дубликат не вызывает ошибку |
| `assignBookingClient(bookingId,{clientId}|{newClient})` | admin выбирает/создаёт карточку и привязывает в одном change; аудит, без перезаписи booking.clientName/Phone |
| `getClientBookings(id)` | История **по clientId**, не по телефону |
| `confirmBooking(bookingId)` | admin разрешает `pending_confirmation→confirmed` даже без специалиста, **но только когда clientId связан** |

### Старые методы бронирований и студийного расписания

| Метод | Версия v4 |
|---|---|
| `getBookings(),getPendingBookings(),getBookingsForDate()` | admin читает расширенные booking поля и вычисляемые причины; не передавать в specialist workspace |
| `cancelBooking(id,note)` | admin, pending/confirmed→cancelled; освободить ресурсы и пересчитать предупреждения |
| `restoreBooking(id)` | admin, cancelled→pending после **полной проверки solver**, а не сразу confirmed; специалист/кабинет, если закреплены, проверяются заново |
| `completeBooking(id)` | Старый вызов админки **больше не допустим**; заменить в UI на `markMyBookingCompleted(id)` специалиста. При вызове без specialist роли → ROLE_FORBIDDEN |
| `updateBooking(id,{status})` | Только существующие разрешённые переходы и роли; нельзя использовать для обхода mark/undo |
| `getBookingWarnings()` | Возвращает прежние ID «Вне студийного графика», а персональные предупреждения — через `getAttentionItems()` |
| `getScheduleMode/getPublishedSchedule/getDraftSchedule/setDraftScheduleRange/publishScheduleRange` | Контракты студии остаются; после записи инвалидировать кэш solver, пометить несовместимые назначения |
| `updateSettings({breakMinutes,...})` | Изменения рабочего шага/перерыва автоматически пересчитывают доступность **и** новые предупреждения; брони/назначения не удалять |
| `createBooking(data)` | Наследованный внутренний метод для тестов, **не публичное бронирование**; запретить создавать confirmed без clientId и вне solver, иначе DEPRECATED_DEMO_METHOD |
| `deleteService/deleteBooking/resetServices/clearBookings/clearClientsAndBookings/importData/exportData/resetDemoData` | Только демонстрационное администрирование; при массовом изменении ссылок согласованно очищать/сохранять события и проверки. `importData` проверяет v4 или мигрирует прежние версии; экспорт включает новые коллекции, предупреждать о PII. Не применять как настоящие серверные операции |

## 7. Порядок транзакционной имитации в DemoAPI

~~~js
async function changeWithRevision(expectedRevision, mutate) {
  // Обёртка над текущим change(): свежий db и одна запись в localStorage.
  // При наличии navigator.locks использовать существующий Web Lock.
  return change(db => {
    if (expectedRevision !== undefined && db.revision !== expectedRevision)
      throw Object.assign(new Error('Данные изменились'), {code:'STALE_REVISION'});
    const result = mutate(db);            // только синхронная логика над snapshot
    // recompute derived warnings, validateV4; при ошибке не записывать!
    db.revision += 1;
    return result;
  });
}
~~~

`change()` в текущем коде [не обеспечивает межустройственную согласованность](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L241-L245). Повторная проверка внутри изменения защищает демо от части логических гонок внутри браузера, **но не является обещанием** защиты двух клиентов на разных устройствах.

**Нельзя** передавать в публичное UI внутренние `allocations` solver: они только свидетельствуют, что запись теоретически осуществима; специалист выбирается администратором позже. `getAssignmentCandidates` / `previewBookingAssignment` работают только в учебном административном режиме.

## 8. Пример полного сценария вызовов

~~~js
await SiteAPI.switchDemoRole('admin');
await SiteAPI.createSpecialist({fullName:'Мария Иванова',serviceIds:['classic'],
  week:{0:[],1:[{start:'09:00',end:'18:00'}],2:[],3:[],4:[],5:[],6:[]}});

const slots = await SiteAPI.getAvailableSlots('2026-11-09','classic');
// Клиент выбирает только услугу, дату и время.
const receipt = await SiteAPI.createBookingRequest({
  serviceId:'classic',date:'2026-11-09',time:slots[0],
  fullName:'Иван Иванов',phone:'+79991234567'
});

const all = await SiteAPI.findClientsByPhone('+79991234567');
// Администратор выбирает существующий ID или создаёт нового.
await SiteAPI.assignBookingClient(receipt.id,{newClient:{fullName:'Иван Иванов',phone:'+79991234567'}});
const preview = await SiteAPI.previewBookingAssignment(receipt.id,{specialistId:'sp-001'});
if (!preview.allowed) throw new Error(preview.reason);
await SiteAPI.assignBooking(receipt.id,{
  specialistId:'sp-001',roomId:preview.roomId,
  expectedRevision:preview.revision,
  acknowledgedBookingIds:preview.affectedBookingIds
});
await SiteAPI.confirmBooking(receipt.id);
// Затем назначенный специалист в своей роли увидит запись без телефона.
// Завершить возможно только когда clock >= плановому окончанию.
~~~

Контрольные сценарии: [demo-test-cases.md](demo-test-cases.md); порядок UI: [frontend-handoff.md](frontend-handoff.md).
