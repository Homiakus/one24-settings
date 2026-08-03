# Test Coverage Matrix — modbus-configurator

| Пакет | API | Unit Tests | Integration | Race Detector | Fuzz | Benchmarks | Allocation Analysis | Stress | Статус |
|---|---|---:|---:|---:|---:|---:|---:|---:|---|
| `main` | Config loading, main loop | PASS | PASS | PASS | N/A | PASS | PASS | PASS | COVERED |
| `modbus-configurator/config` | `LoadConfig`, `DefaultConfig` | PASS | PASS | PASS | N/A | PASS | PASS | PASS | COVERED |
| `modbus-configurator/api` | REST routes, CORS, Auth, Status | PASS | PASS | PASS | N/A | PASS | PASS | PASS | COVERED |
| `modbus-configurator/modbus` | RTU Client, retry, step validation | PASS | PASS | PASS | N/A | PASS | PASS | PASS | COVERED |
| `modbus-configurator/ws` | Hub broadcast, client limit, topics | PASS | PASS | FAIL (RACE) | N/A | PASS | PASS | FAIL | ISSUE_CONFIRMED |

> [!WARNING]
> Пакет `modbus-configurator/ws` выявил DATA RACE при многопоточной работе в `Race Detector` и `Stress` тесте (`ISSUE-GO-0001`).
