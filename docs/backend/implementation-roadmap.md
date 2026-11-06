# Краткий план реализации

Общий порядок — [подробный Go guide](go-implementation-guide.md): инфраструктура → актуальная SQL-модель (студия, специалисты, кабинеты, роли) → API каталог/аутентификация → published studio schedule + личные графики → pure SAT/UNSAT/UNKNOWN matcher → атомарное бронирование по studio-lock → админские назначения/привязка клиента → отсутствия/архивирование → специалистские отметки завершения → интеграция с frontend → эксплуатационные и конкурентные тесты.

Обоснование новой модели: [идея 01](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L52). **Не реализовывать старый единый GiST EXCLUDE по resource_id**: он запрещает два законных параллельных сеанса в разных кабинетах. Вместо него назначенные пары защищены двумя EXCLUDE, а все неназначенные обязательства — общим solver под `schedule_state FOR UPDATE`.

Демонстрационные `importData`, `resetDemoData`, `clearBookings`, `clearClientsAndBookings`, hard deleteBooking не превращаются автоматически в production endpoints. main и frontend не менять без отдельного задания. Каждый этап принимается по [Go tests](go-testing.md).


**Пересмотр для идеи 01:** ранняя фаза booking не должна использовать старое исключение «один сеанс на студию». До публичного бронирования нужны помещения, персонал и joint solver; дальше админ назначает/переназначает, специалист завершает сеанс, а статус подтверждения независим. Рабочий порядок и критерии — [go-implementation-guide.md](go-implementation-guide.md); исходные правила — [идея 01](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L52).
