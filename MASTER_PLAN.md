# MASTER_PLAN — ONEPAP.24 settings-modbus

## Цель

Получить единый Windows desktop-продукт для Modbus-конфигуратора: Go backend,
Wails v3/WebView2 shell и durable lifecycle state через Axiom. План является
локальным источником статуса; green локальных тестов не означает hardware,
production или hosted-CI qualification.

## Правила доказательности

- Не включать в commit чужие untracked-файлы и не удалять их автоматически.
- Перед каждым push проверять `git diff --check`, Go tests/vet/race и Wails build.
- Modbus hardware, COM4, WebView2 runtime, installer/signing и hosted CI —
  отдельные внешние gates.
- Axiom хранит durable lifecycle facts; business operations и Modbus single
  worker не дублируются вторым scheduler/runtime.

## Состояние на 2026-09-10

### WAILS-001 — заменить Rust desktop

Status: DONE

- Rust/Tao/Wry desktop удалён из checkout.
- Добавлен Wails v3 проект `desktop/ONEPAP.24_Modbus_Configurator`.
- Go backend встраивается в единый Wails EXE и запускается с runtime-file.
- Сохранены `-config`, `-runtime-file`, `-host`, `-port`, `-server-only`.
- `onepap.ps1 build` использует `wails3 build`, Cargo/Rust не требуются.

### AXIOM-001 — durable lifecycle coordination

Status: DONE

- Axiom `github.com/Homiakus/axiom` закреплён на commit
  `60ea147d4070cf767ebdb9f18372e623c380933b`.
- Lifecycle events `started`, `ready`, `failed`, `stopping` сохраняются через
  synchronous Pebble-backed durable single-node profile.
- Reopen test подтверждает восстановление состояния execution `configurator`.

### WAILS-002 — runtime qualification

Status: DONE

- Реальный локальный Wails EXE запущен в `-ServerOnly` режиме.
- Runtime-file создан, `/healthz` ответил HTTP 200, Axiom сообщил `ready`.
- `onepap.ps1 stop` корректно завершил Wails/backend process tree.
- Fallback HTTP port подтверждён фактическим runtime URL.
- Чистый host/WebView2 и повторный GUI-жизненный цикл требуют отдельного host gate.

### AXIOM-002 — operational observability

Status: DONE

- Восстановленный Axiom lifecycle state выводится в `/healthz`.
- Stale runtime-file фиксируется событием `recovered` с прежним PID.
- Добавлен API-тест наличия orchestrator state в health response.
- Не считать это доказательством Modbus hardware recovery без HIL-теста.

### AUTOTRACE-001 — backend routing integration

Status: DONE

- Обновлён upstream AutoTrace Lab до commit `ba9613ff534d`.
- Go core подключён как versioned dependency и используется через
  `POST /api/v1/autotrace/route`.
- Backend выполняет scene validation, context timeout и deterministic routing;
  браузер получает рассчитанные пути и отображает их в Canvas.
- Добавлены API-тест и UI-кнопка «Рассчитать трассы»; Modbus execution contract
  не изменён.
- Результат локально проверен через `go test ./...`; hardware/visual host gate
  остаётся отдельным доказательством.

### RELEASE-001 — внешняя квалификация

Status: BLOCKED

- Нет подтверждения hosted CI, подписанного installer и чистого hardware/HIL
  прогона на ESP32/Medtechnica.
- Нет основания объявлять production-ready только по локальным Go/Wails gates.

## Локальные gates

1. `pwsh -NoProfile -ExecutionPolicy Bypass -File .\onepap.ps1 build`
2. `Push-Location configurator; go test ./...; go vet ./...; go test -race ./orchestrator ./api ./ws`
3. `Push-Location desktop\ONEPAP.24_Modbus_Configurator; go test ./...; wails3 build`
4. `rg --files . -g '*.rs' -g 'Cargo.toml' -g 'Cargo.lock'` должен быть пустым.
