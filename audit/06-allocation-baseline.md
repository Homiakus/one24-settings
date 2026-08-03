# Allocation Baseline — modbus-configurator

## 1. Сводная матрица аллокаций памяти

| Пакет / Точка вызова | B/op | Allocs/op | Источник аллокаций | Вывод escape-анализа (`go test -gcflags="-m=2"`) |
|---|---:|---:|---|---|
| `configurator/config` | 8,002 B | 95 allocs | TOML AST parsing, map allocation in `toml.DecodeFile` | AST узлы уходят в heap (`escapes to heap`) |
| `configurator/api` | 20,283 B | 166 allocs | `http.Server` context, JSON encoders, `http.ResponseWriter` wrapper | Response buffer & context allocation |
| `configurator/modbus` | 0 B | 0 allocs | stack allocation of uint16 registers & struct fields | `does not escape` (стековые переменные) |
| `configurator/ws` | 274 B | 3 allocs | `json.Marshal(event)`, channel message allocation | JSON byte slice escapes to heap |

## 2. Активы Heap и Escape Analysis

```text
# Escape Analysis Highlights:
- modbus/operations.go: validateStepParams: does not escape
- modbus/client.go: ReadRegister: values escape to heap via return slice if heap allocated
- ws/hub.go: Broadcast: json.Marshal(event) allocates byte slice that escapes to goroutine channel
- api/server.go: jsonOK: map[string]any allocates interface box on heap
```
