# Regression Register — modbus-configurator

| Идентификатор тикета | Проверяемый модуль / Метод | Имя регрессионного теста | Файл теста | Описание регрессионного сценария |
|---|---|---|---|---|
| `ISSUE-GO-0001` | `ws/hub.go` (`ServeHTTP`, `remove`, `Broadcast`) | `TestHubConcurrentBroadcasts` | `configurator/ws/hub_test.go` | Запуск 20 паралельных горутин, одновременно вещающих события при постоянных подсоединениях и отсоединениях 5 клиентов под Race Detector |
