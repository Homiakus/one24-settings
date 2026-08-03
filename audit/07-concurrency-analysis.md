# Concurrency Analysis — modbus-configurator

## 1. Реестр примитивов конкурентности

| Пакет | Область | Примитив | Назначение | Статус проверки Race Detector |
|---|---|---|---|---|
| `ws` | `Hub` | `sync.RWMutex` | Защита `clients map[*client]struct{}` | **FAIL — DATA RACE DETECTED** |
| `modbus` | `Client` | `sync.Mutex` | Монопольный доступ к RS-485 COM-порту | PASS |
| `api` | `ServerState` | `sync.RWMutex` | Защита `Steps`, `Detection`, `Log` | PASS |
| `api` | `rateLimiter` | `sync.Mutex` | Защита `limiters map[string]*rate.Limiter` | PASS |

## 2. Локализация обнаруженной гонки данных (ISSUE-GO-0001)

```text
==================
WARNING: DATA RACE
Write at 0x00c00010c900 by goroutine 35:
  modbus-configurator/ws.(*Hub).remove()
      hub.go:95 -> delete(h.clients, c)
Previous read at 0x00c00010c900 by goroutine 38:
  modbus-configurator/ws.(*Hub).remove()
      hub.go:97 -> log.Printf("[ws] client disconnected (total=%d)", len(h.clients))
==================
```

### Первопричина (Root Cause)
В методах `ServeHTTP` (строка 87) и `remove` (строка 97) файла `configurator/ws/hub.go` считывание размера карты `len(h.clients)` для лога выполнялось ПОСЛЕ снятия мьютекса `h.mu.Unlock()`. В моменты высокого конкурентного трафика подсоединений и отключений параллельные горутины выполняли одновременно запись `delete(h.clients, c)` или `h.clients[c] = struct{}{}` и незащищенное чтение длины хеш-таблицы.

### План устранения
Перенести вызов чтения длины карты `len(h.clients)` внутрь защищенного блока мьютекса `h.mu.Lock() ... h.mu.Unlock()` или считывать локальное значение под защитой мьютекса.
