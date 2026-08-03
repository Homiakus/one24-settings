# Closed Issues Register — modbus-configurator

## ISSUE-GO-0001 — Гонка данных при считывании len(h.clients) для лога в ws/hub.go

Статус:: CLOSED  
Серьёзность:: High  
Категория:: Concurrency  
Пакет:: configurator/ws  
Файл:: configurator/ws/hub.go  
Функция:: ServeHTTP, remove  
Закрыто в итерации:: 1  
Ревьюер:: Independent Code Reviewer (Go Concurrency Specialist)  

### Первопричина
Вызов `len(h.clients)` внутри функций логирования `log.Printf` производился ПОСЛЕ снятия блокировки `h.mu.Unlock()`. В случаях высокой конкурентной нагрузки горутин происходило несинхронизированное чтение длины карты параллельно с модификацией структуры хеш-таблицы в `delete(h.clients, c)`.

### Решение
Считывание текущей длины карты перенесено непосредственно внутрь секции мьютекса: `total := len(h.clients)` под `h.mu.Lock()`.

### Подтверждение закрытия
* [x] Проблема воспроизводилась до исправления (`go test -race ./ws`).
* [x] Исправление реализовано под строгой блокировкой мьютекса.
* [x] `go test ./...` — PASS.
* [x] `go test -race ./...` — PASS (0 warnings).
* [x] Добавлен регрессионный тест `TestHubConcurrentBroadcasts`.
* [x] Бенчмарк `BenchmarkHubBroadcast` показал снижение latency с 738.4 ns/op до 666.1 ns/op без увеличения аллокаций (273 B/op, 3 allocs/op).
* [x] Независимое ревью завершено с вердиктом APPROVED.
