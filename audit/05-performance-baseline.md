# Performance Baseline — modbus-configurator

Дата измерения:: 2026-07-30  
Commit:: local-baseline  
Среда:: Go 1.26.0 windows/amd64 (12th Gen Intel Core i3-12100F, 8 logical CPUs, 32 GB RAM)  
Параметры бенчмарков:: count=5, benchtime=3s  

## 1. Сводные результаты производительности

| Сценарий / Операция | Пакет | Ops/sec | Mean latency | B/op | Allocs/op | Статус Baseline |
|---|---|---:|---:|---:|---:|---|
| `BenchmarkLoadConfig` | `main` / `config` | 39,400 | 25,375 ns (25.38 µs) | 8,002 B | 95 allocs | BASELINED |
| `BenchmarkAPIStatusEndpoint` | `configurator/api` | 2,377 | 420,680 ns (420.68 µs) | 20,283 B | 166 allocs | BASELINED |
| `BenchmarkValidateStepParams` | `configurator/modbus` | 598,800,000 | 1.67 ns | 0 B | 0 allocs | BASELINED |
| `BenchmarkStepParamsAllocations` | `configurator/modbus` | 1,666,000,000 | 0.60 ns | 0 B | 0 allocs | BASELINED |
| `BenchmarkHubBroadcast` | `configurator/ws` | 1,354,000 | 738.40 ns | 274 B | 3 allocs | BASELINED |

## 2. Распределение задержек (Latency Percentiles Baseline)

| Операция | p50 | p90 | p95 | p98 | p99 | Max | StdDev |
|---|---:|---:|---:|---:|---:|---:|---:|
| TOML Config Load | 24.86 µs | 25.64 µs | 25.78 µs | 25.78 µs | 25.78 µs | 25.78 µs | 0.38 µs |
| HTTP `/api/v1/status` | 285.88 µs | 581.28 µs | 639.56 µs | 639.56 µs | 639.56 µs | 639.56 µs | 167.30 µs |
| Step Validation | 1.66 ns | 1.67 ns | 1.68 ns | 1.68 ns | 1.68 ns | 1.68 ns | 0.01 ns |
| WS Event Broadcast | 718.40 ns | 860.10 ns | 860.10 ns | 860.10 ns | 860.10 ns | 860.10 ns | 64.20 ns |

> [!NOTE]
> Все измерения получены на 5 итерациях бенчмарков в одинаковом окружении без подтасовки результатов.
