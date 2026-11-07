# Идея 01 — воспроизводимые тесты JavaScript DemoAPI без сервера

Сценарии для фронтендера после реализации [handoff](frontend-handoff.md). Источник требований: [идея № 01](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L52). Интерфейс методов: [demo-api-spec.md](demo-api-spec.md); модель данных: [demo-data-migration.md](demo-data-migration.md); алгоритм: [assignment-engine.md](assignment-engine.md). **Здесь тестовые требования и ожидаемые ответы, а не сообщение об успешно запущенных тестах.**

## A. Фиксированный стенд

- Выполнять через локальный статический сервер и DevTools либо `node:test` с изолированным окружением `window`, `localStorage` и загружаемым `DemoAPI`. Никаких Go/SQL/сети не требуется.
- Каждый тест получает отдельную JSON-копию состояния; очищать только тестовый storage (не данные реального пользователя). Использовать `SiteAPI.importData(fixture)` только с тестовым JSON. В случае отсутствующего storage проверить memory fallback.
- Clock должен быть **инъецируемым** в движок/демо-окружение (`now()`) без замены глобального Date во всём браузере. Основной фиксированный момент: `2030-01-01T00:00:00.000Z`, дату сеансов использовать `2030-01-07` (понедельник); тестировать в `Europe/Moscow` или явно принятом demo-local режиме. Никаких зависящих от текущей даты тестов.
- `settings`: simple, неделя [0,1,2,3,4,5,6] с 09:00–21:00, `step=30`, `breakMinutes=0`; если нужен отдельный кейс — явно переопределить. Одна active комната R1; один active специалист S1 умеет услуги `A,B`, неделя каждый день 09:00–21:00, overrides пусты, approved absence нет. Услуги A и B — 60 минут (остальные поля совпадают с валидной схемой сервиса). `pending_confirmation` и `confirmed` резервируют время.
- Все внешние IDs стабильны, а внутренние `uuid()` допустимо стабилизировать test factory. Сеансы обозначать `J1`, `J2` в ожидаемых таблицах; в реальной fixture их `id` — валидные значения согласно DemoAPI.
- Отчёт теста должен показывать `caseId`, `pass/fail`, ожидаемый/фактический результат, `db.revision`; при тестах privacy — список реально возвращённых JSON ключей.

### Базовая конфигурация для matcher (псевдо-fixture, приводится к полному v4 JSON перед import)

~~~js
const F = {
  date: '2030-01-07',
  now: '2030-01-01T00:00:00.000Z',
  services: [
    {id:'A', durationMinutes:60}, {id:'B', durationMinutes:60}
  ],
  rooms:[{id:'R1',name:'Кабинет 1',isActive:true,sortOrder:0}],
  specialists:[{id:'S1',state:'active',serviceIds:['A','B'],work:'09:00-21:00'}],
  bookings:[],
  breakMinutes:0
};
// ЭТО входная схема тестового генератора, НЕ прямой JSON для SiteAPI.importData.
// В адаптере фикстур создать полный db.version=4 с обязательными fullName,
// currency, price, settings, schedule, specialistServices, accounts, revision и т.д.
~~~

**Условное правило результата solver:** `SAT` → `{feasible:true,uncertain:false}`, `UNSAT` → `{feasible:false,uncertain:false}`, `UNKNOWN` → `{feasible:false,uncertain:true}`. `allocations` — доказательное временное распределение, но **не** автоматическая запись в `bookings[].specialistId/roomId`.

## B. Миграция и сохранность (P0)

| ID | Вход/операция | Ожидаемый проверяемый результат |
|---|---|---|
| **M01** | Пустой storage; открыть сайт, вызвать `getServices`, `exportData` | Нет ошибки; `db.version===4`, один активный специалист, один кабинет, три услуги и учебный профиль; нет настоящих паролей. |
| **M02** | Импорт валидной v3 с 2 клиентами, 2 записями, черновиком advanced, опубликованным днём, историей и отпуском студии | Вся прежняя информация и ID сохранены; новые v4 поля и `revision` присутствуют, старые booking assignments NULL, никаких ложных `completedBy`. |
| **M03** | Повторный read/import v4 после M02 | Нет дополнительных специалистов/кабинетов/событий; `version` остаётся 4. |
| **M04** | Импорт v1 и v2 по отдельности | Сначала штатная цепочка v1→v3, затем v3→v4; существующие сервисы/настройки/контент сохранены. |
| **M05** | Импорт `version:5` или невалидных ссылок/JSON | `VERSION_UNSUPPORTED` либо `INVALID_DATABASE`; прежняя база **не перезаписана** пустым seed. |
| **M06** | При `localStorage.setItem` выбросить QuotaExceededError | Memory fallback / `STORAGE_UNAVAILABLE`, показан warning; нет тихого сброса; старые сохранённые данные не испорчены. |
| **M07** | Два разных client UUID с одним `phoneNormalized` + разными ФИО | Валидатор принимает обоих; после перезагрузки обе карточки и разные истории сохраняются. |
| **M08** | Два будущих pending бронирования в разных комнатах при 2 совместимых специалистах | Валидатор v4 принимает пересечение; прямое двойное закрепление одной комнаты или одного специалиста отклоняет. |

## C. Joint matcher, буферы и расписания

| ID | Вход/ситуация | Ожидаемый результат |
|---|---|---|
| **S01** | 1 комната R1, S1; J1=A, 10:00–11:00 уже pending, J2=B на 10:30 | Для J2 `UNSAT`; `getAvailableSlots` не содержит 10:30; публичный POST → `SLOT_UNAVAILABLE`, база неизменна. |
| **S02** | R1,R2 + S1,S2 умеют A; J1=A 10:00 pending без назначения; J2=A 10:00 | `SAT` с двумя **разными** специалистами и комнатами; две pending сохраняют NULL assignment до админа. |
| **S03** | R1,R2; S1 умеет A/B; S2 только A; J1=A 10:00 и J2=B 10:00 | `SAT` только с J1→S2, J2→S1. Алгоритм не отвергает B из-за жадного первого выбора S1 для A. |
| **S04** | R1,R2; только S1 умеет A; две одновременные A | `UNSAT`, хотя комнаты две. |
| **S05** | R1; S1,S2; две одновременные A | `UNSAT`, хотя персонала два. |
| **S06** | J1 10:00–11:00 и J2 11:00–12:00, `breakMinutes=0`, одна пара | `SAT` — полуоткрытые интервалы соприкасаются, не пересекаются. |
| **S07** | Как S06, но `breakMinutes=15` и `step=5` | J2 11:00 и 11:10 `UNSAT`; старт 11:15 `SAT` при подходящем графике. |
| **S08** | J1 [10:00,11:00), J2 [10:45,11:45), J3 [11:30,12:30); A и C не пересекаются напрямую | Solver строит **один связный компонент** J1↔J2↔J3; отсечение только по непосредственным соседям запрещено. |
| **S09** | Личный график S1 09:00–12:00, сеанс 11:30–12:30 | S1 не в домене; при отсутствии иных сотрудников J2 `UNSAT`. |
| **S10** | Студийный график advanced, день не опубликован или только в drafts | Нет публичных слотов, даже если график специалиста открыт. После публикации approved day слоты появляются. |
| **S11** | Simple studio vacation / blocked date | Нет студийных слотов. В advanced старый studio vacation сам по себе не должен незаявленно закрывать день. |
| **S12** | Запрос на отсутствие S1 14:00–17:00 со статусом `pending` | Слоты 14:00–17:00 **не** изменились. После `approved` S1 исключён; после `rejected` прежний график. |
| **S13** | S1 архивирован, S2 активен и компетентен | Новые слоты используют S2; старые J1→S1 сохранены и отмечены `REQUIRES_REASSIGNMENT`. |
| **S14** | В одном компоненте лимит solver `nodeLimit=0`/искусственно мал | Возвращается UNKNOWN/`CAPACITY_CHECK_LIMIT`; не принимается новая заявка и не выдаётся ложный уверенный UNSAT. |
| **S15** | Переставить порядок `specialists`, `rooms`, `bookings` при одинаковых ID/данных | SAT/UNSAT и доступные времена неизменны; witness детерминирован по sortOrder/ID. |
| **S16** | Открытый `capacityException` на 10:00–11:00, кандидат 10:30 | Публичный POST в затронутом связном компоненте запрещён; независимый кандидат 15:00 проходит при наличии ресурсов. |
| **S17** | Старый fixed J1 закреплён на S1/R1, новый J2 на то же время, S2/R2 свободны | Solver сохраняет J1 на S1/R1, J2 может подобрать S2/R2; никогда молча не переназначает J1. |
| **S18** | Два заявки одновременно требуют конкретного S1, но R1/R2 разные | `UNSAT`/ошибка direct conflict: свободные кабинеты не разрешают двойную занятость специалиста. |
| **S19** | `getDateStates(from,to)` без serviceId, опубликованный рабочий день | День **не** получает `full` только из-за отсутствующего параметра (исправить старую ошибку `!id`). |
| **S20** | `createBookingRequest` получает ранее показанный, но уже занятый slot | Повторный matcher **в `change()`** возвращает `SLOT_UNAVAILABLE`; ни второй booking, ни новый клиент автоматически не создаются. |

## D. Админское назначение, клиенты и «Требуют внимания»

| ID | Вход/операция | Ожидаемое поведение |
|---|---|---|
| **A01** | Pending J1, `clientId:null`, `specialistId:null`; admin вызывает `confirmBooking` | `CLIENT_LINK_REQUIRED`, статус не меняется. |
| **A02** | Admin `assignBookingClient(J1,{newClient:{...}})`, затем confirm, специалист всё ещё NULL | `confirmed`, `specialistId:null`; предупреждение `UNASSIGNED_SPECIALIST` сохраняется. |
| **A03** | Два родственника с одним телефоном; `findClientsByPhone` | Возвращаются **обе** карточки; admin вручную выбирает нужную; история по `clientId` независима. |
| **A04** | Назначить J1 на S1/R1, а пересекающийся J2 уже fixed S1/R2 | `DIRECT_RESOURCE_CONFLICT` / `SPECIALIST_BUSY`, никакого override. |
| **A05** | Назначить J1 на некомпетентного либо отсутствующего специалиста | `SPECIALIST_INELIGIBLE` / `SPECIALIST_OFF`; данные не изменены. |
| **A06** | R1 занята, R2 свободна; admin предлагает S2 без roomId | Preview выбирает R2, итог сохраняет R2, assignmentEvent с новым room ID. |
| **A07** | R1 и R2 свободны, равная ёмкость | Автоматический кабинет выбирается по `sortOrder,id` стабильно. |
| **A08** | S1 умеет A+B, S2 только A; pending J1=A, J2=B одновременно. Админ закрепляет J1 за S1 | Preview `INDIRECT_ASSIGNMENT_CONFLICT` с `affectedBookingIds:[J2]`; без ack → `CONFLICT_ACK_REQUIRED`; при точном ack сохраняется J1→S1, J2 в «Требуют внимания». |
| **A09** | После A08 отправить нового клиента в тот же связный компонент | Новая заявка отвергается из-за незакрытого исключения, хотя прямого двойного назначения может ещё не быть. |
| **A10** | После A08 вручную переназначить J1→S2, вернуть J2 возможность S1 | Событие с before/after; capacityException закрывается, предупреждение исчезает, статус J1 не меняется. |
| **A11** | Preview J1 выдаёт revision=5, иное успешное изменение повышает до 6, затем assign с expectedRevision=5 | `STALE_REVISION`, никакого сохранения и auto-retry. |
| **A12** | `archiveSpecialist(S1)` при active J1→S1 | Профиль архивный, J1 не удалена/не переназначена; `REQUIRES_REASSIGNMENT`. |
| **A13** | Деактивировать R1 при active J1→R1 | J1 сохраняется, видимое `ROOM_INACTIVE`; новых назначений туда нет. |
| **A14** | Переназначение J1 было выполнено из S1 в S2 после подтверждения | История содержит from/to S1,S2 и room changes, подтверждение осталось `confirmed`; бывший S1 видит только событие. |
| **A15** | `getAttentionItems` после J2 unassigned, approved leave overlap и pending absence | Элементы содержат разные коды `UNASSIGNED_SPECIALIST`, `REQUIRES_REASSIGNMENT`, `ABSENCE_REVIEW_PENDING`, ID и причины; без изменения статуса. |

## E. Отсутствия, роль и завершение

| ID | Вход/операция | Ожидаемый результат |
|---|---|---|
| **L01** | Specialist подаёт `partial_day` 14:00–17:00, одна дата | `pending`, дата/время нормализованы, публичная доступность ещё прежняя. |
| **L02** | Specialist подаёт `full_days` 7–9 января с `startTime` | `VALIDATION`: у full_days время NULL. |
| **L03** | Specialist отзывает свою `pending` | `withdrawn`; повторно и после approve/reject отзыв невозможен. |
| **L04** | Admin `rejectAbsence` без причины | `VALIDATION`; статус pending сохраняется. |
| **L05** | Admin `approveAbsence` с конфликтом J1→S1 и без списка acknowledged IDs | Preview показывает J1; сохранение требует явного подтверждения. |
| **L06** | Admin утверждает L05 с точным acknowledged ID | `approved` **целиком**; J1 и назначение сохранены, `REQUIRES_REASSIGNMENT`, без автоотмены. |
| **L07** | Аккаунт с `roles:['admin','specialist']` | Role switch сохраняет аккаунт, но разные workspace, собственная заявка специалиста рассматривается только действием admin и не утверждается автоматически. |
| **L08** | `getMyBookings` у S1 с J1→S1 и J2→S2 и J3 unassigned | Только J1, включая pending; **нет ключей `phone`, `clientPhone`, `phoneNormalized`, `clientId`**, нет J2/J3. |
| **L09** | J1 confirmed, назначен S1, сейчас раньше `plannedEnd` | `markMyBookingCompleted` → `NOT_ENDED_YET`; J1 остаётся confirmed. |
| **L10** | J1 confirmed, now ровно `plannedEnd`, active S1 вызывает mark | `completed`, `completion.bySpecialistId==='S1'`, audit event существует. |
| **L11** | После L10 S2 или admin пытается undo | `ROLE_FORBIDDEN`; никакого изменения. |
| **L12** | S1 спустя месяц вызывает undo собственной отметки | `confirmed`, событие undo добавлено, старое событие completion сохранено. |
| **L13** | Cancelled либо pending J1; S1 вызывает mark | `INVALID_STATUS_TRANSITION`; нельзя завершить неподтверждённую заявку. |
| **L14** | Двойная роль admin+specialist, activeRole=admin, попытка mark | Действие недоступно в admin workspace; в specialist workspace доступно для собственных сеансов. |
| **L15** | Возврат `getMyReassignmentEvents` после перевода J1 S1→S2 | S1 видит собственное событие снятия; ни телефон J1, ни детали текущих чужих записей не раскрываются. |
| **L16** | Приглашение сотрудника и первый демонстрационный вход | Одноразовый учебный код показан только в UI-памяти; в экспортированном JSON нет password/hash/demoCode; учебная смена пароля снимает флаг. |

## F. Регресс и UI-проверки

| ID | Проверка | Ожидаемо |
|---|---|---|
| **R01** | Открыть старый простой календарь, изменить рабочий день/шаг, записаться | Правила simple и новые квалификации/комнаты учитываются совместно; публичный интерфейс не содержит selector исполнителя. |
| **R02** | Advanced: черновик, selective publish, restore history to draft | Черновик не меняет слоты, публикация меняет только выбранные даты, история не потеряна; новая personal week отдельно. |
| **R03** | Уменьшить рабочее время/архивировать сотрудника при уже закреплённой записи | История и назначение сохраняются, предупреждение отображается, статус неизменен. |
| **R04** | Перезагрузить после назначения, отпуска и completion/undo | Все коллекции v4, события, selected role, ревизии и состояния сохраняются. |
| **R05** | Экспортировать v4, импортировать обратно | Отношения specialist/room/booking и histories сохранены; PII export warning показан. |
| **R06** | 320/375/768/1280px, длинные названия, клавиатура, Escape, `prefers-reduced-motion` | Ничего не обрезано, формы/диалоги доступны, фокус возвращается, анимации адаптированы. |
| **R07** | Клиентское `.ics` / Google/QR после новой записи | Нет ФИО, телефона или ID работника/клиента в открытых ссылках/QR; календарное время корректно. |
| **R08** | Нажать кнопки `admin` в workspace специалиста / вызвать specialist methods с чужим accountId | UI не показывает чужие данные; DemoAPI выдаёт `ROLE_FORBIDDEN` или формирует только собственный DTO. Это симуляция, не безопасная авторизация. |
| **R09** | Вторая вкладка меняет `revision` во время открытого preview или формы с draft | Первая вкладка не затирает изменения; показывает `STALE_REVISION` и предлагает повторный preview. |
| **R10** | Сломать v4 JSON/поставить неизвестную версию | Данные не стираются автоматически, показана ошибка/возможность ручного импорта тестового backup. |

## G. Минимальный тестовый каркас (скопировать в frontend tests)

Каркас не подменяет реальное `SiteAPI`; `makeFixture` подготавливает **полный валидный v4 объект** из [demo-data-migration.md](demo-data-migration.md). В тестах не мутировать один singleton между кейсами.

~~~js
// tests/idea01/smoke.js — после реализации соответствующих методов
const eq = (actual, expected, msg='') => {
  if (JSON.stringify(actual)!==JSON.stringify(expected))
    throw new Error(msg+' expected='+JSON.stringify(expected)+' got='+JSON.stringify(actual));
};
async function rejectsCode(operation, code) {
  try { await operation(); }
  catch (error) { eq(error.code, code); return; }
  throw new Error('Expected rejection '+code);
}
async function runCase(id, run) {
  try { await run(); console.log('PASS',id); }
  catch (error) { console.error('FAIL',id,error); throw error; }
}
await runCase('M01', async () => {
  const db=await SiteAPI.exportData();
  eq(db.version,4);
  if(db.specialists.filter(x=>x.state==='active').length<1) throw new Error('No active specialist');
  if(db.rooms.filter(x=>x.isActive).length<1) throw new Error('No active room');
});
await runCase('A01', async () => {
  // fixture: admin role, active pending booking J1 with clientId=null
  await SiteAPI.importData(makeFixture('A01'));
  await rejectsCode(() => SiteAPI.confirmBooking('J1'),'CLIENT_LINK_REQUIRED');
});
await runCase('L08', async () => {
  await SiteAPI.importData(makeFixture('L08'));
  const list=await SiteAPI.getMyBookings();
  if(list.some(x=>['clientPhone','phone','phoneNormalized','clientId'].some(k=>k in x)))
    throw new Error('Specialist DTO leaks contact or client identity');
});
// Полный прогон: M01–M08, S01–S20, A01–A15, L01–L16, R01–R10.
// В каждом кейсе fresh fixture + fixed clock; никогда не тестировать на рабочей CRM.
~~~

**Как проверить ручной прогон:** DevTools → Application → localStorage `formaTela.demo.v1`; после каждого write смотреть `version:4`, `revision`, реальные booking/events, отсутствие скрытых данных в specialist возвращаемом DTO; повторно открыть сайт. Четыре класса исходов: `PASS`, `FAIL`, `BLOCKED` (ещё нет метода), `NOT_RUN`. Нельзя записывать `BLOCKED` как `PASS`.

## H. Release gate и передача

Автоматизировать в `frontend` хотя бы M01–M08, S01–S20, A01–A15, L01–L16, R01–R10 (всего **69** сценариев), отдельно ручные responsive/клавиатура и проверку двух вкладок. В CI или скрипте завершать процесс с ненулевым кодом при любом FAIL. Текущая ветка frontend на коммите [`02be239`](https://github.com/yomezza/forma.flow/commit/02be23938f6861a78781ba4d7039d7e755ccde1d) **не соответствует этим условиям**; данное описание не утверждает, что тесты выполнялись.

При передаче заказчику предоставить commit SHA, результат запуска тестов (количество PASS/FAIL/BLOCKED), скриншоты admin/specialist и мобильного отображения, адрес работающей HTTPS-демонстрации **только если действительно опубликован**, краткий список оставшихся ограничений `localStorage`.

Никаких реальных телефонных номеров в тестовых fixture и скриншотах.
