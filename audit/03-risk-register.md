# Risk Register — modbus-configurator

| ID | Компонент | Описание риска | Категория | Вероятность | Влияние | Статус |
|---|---|---|---|---|---|---|
| RISK-01 | `ws/hub.go` | Гонка данных при чтении/записи карты подписок `client.topics` | Concurrency | High | High | OPEN |
| RISK-02 | `api/middleware.go` | Неограниченный рост карты лимитеров `rateLimiter.limiters` при атаке с разных IP | Memory Leak | Medium | High | OPEN |
| RISK-03 | `modbus/operations.go` | Зависание `WaitReady` и `SendCommand` при разрыве физического связи RS-485 | Reliability | High | Critical | OPEN |
| RISK-04 | `api/programs.go` | Отсутствие синхронизации доступа к полю `reagentResume` при параллельных HTTP-запросах | Concurrency | Medium | High | OPEN |
| RISK-05 | `api/server.go` | Копирование структуры `ServerState` с мьютексом при вызове срезки слайсов лога | Maintainability | Low | Medium | OPEN |
| RISK-06 | `modbus/client.go` | Неудерживание горутины при отказе повторных подключений `Recover()` | Reliability | Low | Medium | OPEN |
