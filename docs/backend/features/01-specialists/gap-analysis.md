# Проверка существующего frontend и пробелы для идеи 01

Аудит: ветка [`frontend`](https://github.com/yomezza/forma.flow/tree/frontend) на коммите [1bdc8c63](https://github.com/yomezza/forma.flow/commit/1bdc8c63eb0db12445e4a6dc0bf717967df0d9cf). Целевое требование: [идея 01](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md). Go/PostgreSQL намеренно вне объёма задачи.

| Область | Что реально есть сейчас | Что добавить / изменить в демо |
|---|---|---|
| Хранение | [`formaTela.demo.v1`, `version:3`, миграции v1→v3](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L1-L3), [валидация и read/write](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L94-L102) | Без потери данных поднять JSON до v4, сохранить прежний ключ, новые списки/индексы и аудит |
| Доступность | [`available(db,date,id)`](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L226-L239) проверяет время услуги, единый кабинет подразумевается, персонала нет | Проверять **всю совместную задачу размещения** услуг по специалистам и кабинетам, не только свободный кабинет |
| График | [simple/advanced студии](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L122-L132), [публикация и черновики](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L273-L289) | Сохранить оба режима без изменения; дополнить недельным графиком каждого специалиста, исключениями и утверждёнными отсутствиями |
| Бронирование | [`createBookingRequest`](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L314-L321), статусы pending/confirmed/completed/cancelled | Не выбирать специалиста клиентом; отделить назначение специалиста и кабинета от подтверждения; учитывать неназначенные записи |
| Клиенты | [уникальный телефон, автоматический выбор клиента](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L303-L321) | Сохранить согласованное правило **нескольких клиентов на один телефон**; публичная заявка `clientId:null`, админ выбирает/создаёт карточку вручную |
| Админка | [восемь разделов](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L1-L8), [кнопка подтверждения и завершения](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L129-L148) | Раздел «Специалисты», кабинеты, назначения/переназначения, блок «Требуют внимания», согласование отсутствий; завершение только назначенным специалистом |
| Личный кабинет | Отсутствует | Специалист: свои предстоящие/завершённые записи, свой график, заявки отсутствия, история переназначений; **без телефонов** |
| Доступ | [Переключение представления](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L1-L24) — не аутентификация | Демонстрационный выбор аккаунта/роли, первый вход и временные учебные реквизиты; не хранить реальные пароли |
| Черновики UI | [`AdminDraftStore` в sessionStorage](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/draft-store.js#L1-L13) | Ключи для форм специалиста/отсутствий; не смешивать с опубликованными графиками |
| Передача календаря | [Локальный `.ics` / QR](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/calendar-event.js#L41-L85) | Оставить клиентские данные и назначенного исполнителя вне QR/переносимой ссылки |
| Конкуренция | [Web Locks / localStorage](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L241-L245) | Для учебной демонстрации последовательная локальная смена состояния и повторная проверка. **Не выдавать за межустройственную защиту** |

### Принцип совместимости

Существующие названия `getAvailableSlots`, `createBookingRequest`, `confirmBooking`, `getDateStates`, `getServices`, `getSettings`, `updateSettings`, `getPublishedSchedule`, `getDraftSchedule` и т.д. **сохраняются**, но у некоторых расширяется внутренняя проверка. Новые методы вынесены в [demo-api-spec.md](demo-api-spec.md).

Перед созданием специалиста или кабинета нельзя заменять/сбрасывать существующие услуги, записи, клиентов, историю расписания и внешний вид сайта. Не переименовывать существующий ключ `localStorage` без миграции.

### Новые несоответствия целевой демонстрации

1. Текущий `completeBooking` вызывается админкой и не проверяет фактическое окончание — в новом демо завершает **назначенный специалист** после планового времени.
2. Текущая валидация считает телефон уникальным; отменить уникальность и автоматическую склейку клиентов, не меняя старые ID.
3. Текущая проверка требует только непересечения старых бронирований в пределах одной линии; после появления кабинетов пересечения на одной дате **разрешены**, если система может корректно распределить их по ресурсам.
4. Текущие студийные `settings.vacations` действуют только в simple. Не переносить их на специалистов: у каждого отдельный список утверждённых личных отсутствий.
5. Текущий `getBookingWarnings` вычисляет «Вне графика». Добавить **отдельные** коды причин «Требуют внимания», не заменяя старое предупреждение.

Исходное утверждённое правило: [назначения и ограничения](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L26); [роли и завершение](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L36-L52).
