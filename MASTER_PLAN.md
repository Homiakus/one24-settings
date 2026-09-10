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

### AUTOTRACE-002 — удобный редактор алгоритмов

Status: DONE

- Улучшить создание команд: тип блока, вариант, условие, параметры, timeout,
  повтор и понятная диагностика неполной настройки.
- Сохранить обратную совместимость с текущим линейным Modbus sequence API.
- Новые поля нормализуются при импорте и сохраняются в JSON/localStorage.
- Запуск сценария блокируется до устранения ошибок конфигурации; условия
  остаются декларативными и не исполняются в браузере.

### AUTOTRACE-003 — общий граф и параметризованные подграфы

Status: TODO

- Перейти от линейного отображения к графу с соединяемыми блоками, ветвлениями
  и условиями, не обходя backend validation.
- Добавить reusable subgraph definition с входными параметрами и явным
  контрактом выхода; выполнение подграфа допускается только после отдельной
  проверки безопасности Modbus-команд.
- Реализован Canvas graph connectivity: output → input, fan-out, выбор и
  удаление связей, сохранение connections в модели алгоритма.
- Остаются edge-condition editor, полноценный DAG executor и безопасный
  runtime binding параметров подграфа.

### AUDIT-001 — математический и кибернетический аудит

Status: DONE

- Подтверждено, что текущая модель исполнения остаётся линейной: UI-граф,
  `AlgorithmDefinition` и Modbus executor не являются единым контрактом.
- Зафиксированы P0-риски: расхождение кодов команд UI/backend, игнорирование
  условий/вариантов/retry/subgraph при исполнении и отсутствие durable journal
  фактических Modbus-эффектов.
- Зафиксированы математические дефекты: сумма всех времён вместо longest path,
  `prev.time` вместо зависимостей DAG, молчаливый fallback формул и неверное
  определение bottleneck.
- Зафиксированы архитектурные дефекты: глобальный program state, слабая
  семантическая validation, смешение liveness/readiness и отсутствие resource
  model.

### CONTRACT-001 — единый versioned AlgorithmDefinition

Status: TODO

Priority: P0

Prerequisites: AUDIT-001

- Ввести единую схему `AlgorithmDefinition v1`: `nodes`, `edges`, `parameters`,
  `subgraphs`, `resources`, `entry`, `exit`, `metadata`.
- Использовать одну схему в UI, Go API, экспортируемом JSON, Wails и executor;
  запретить silent discard неизвестных полей.
- Добавить явные типы узлов: `command`, `condition`, `variant`, `subgraph`,
  `checkpoint`, `parallel_join`, `loop`.
- Добавить schema version, stable IDs, units и provenance для каждого
  параметра/команды.
- Gate: round-trip export/import без потери полей и contract tests для Go/UI.

### MODBUS-001 — canonical command registry

Status: TODO

Priority: P0

Prerequisites: CONTRACT-001

- Сделать `configurator/modbus` единственным источником кодов команд и их
  свойств; UI должен получать registry через API или generated JSON.
- Устранить расхождения `90/120`, `110/105`, `180/150`, `200/170`,
  `111/999` и добавить тест полного равенства registry UI/backend.
- Для каждой команды описать `idempotency`, допустимые зоны, требуемые
  ресурсы, безопасную отмену, timeout и recovery action.
- Gate: неизвестная или опасная команда не проходит validation; zero hardware
  command mismatch.

### GRAPH-001 — строгая семантическая валидация графа

Status: TODO

Priority: P0

Prerequisites: CONTRACT-001

- Проверять уникальность node/edge/parameter/subgraph IDs, допустимые kinds,
  command registry и корректность port types.
- Проверять entry/exit, достижимость, dead ends, циклы и разрешать циклы только
  через типизированный bounded loop с лимитом итераций.
- Проверять branch completeness: mutually exclusive conditions, fallback branch,
  deterministic priority и поведение `unknown`.
- Проверять принадлежность `nodeIds` и внутренних edges подграфу, отсутствие
  рекурсивных вызовов и корректность entry/exit подграфа.
- Gate: negative/property tests на циклы, dangling edges, duplicate IDs,
  missing fallback, invalid bindings и resource conflicts.

### MATH-001 — корректная модель времени и надёжности

Status: TODO

Priority: P0

Prerequisites: CONTRACT-001, GRAPH-001

- Заменить сумму узлов на DAG longest-path; для parallel branches считать
  `max(branch_duration)`, для join учитывать готовность всех входов.
- Убрать `prev.time`; формулы должны ссылаться на stable node IDs и проходить
  typed expression parser без `Function`/silent fallback.
- Рассчитывать отдельно nominal, worst-case и expected duration с учётом retry,
  timeout, failure probability и recovery action.
- Разделить machine/operator/wait/transport/recovery time; bottleneck считать
  по ресурсу и очереди, а не по максимальному локальному времени.
- Gate: golden models для serial, branch, join, retry, loop и resource conflict;
  результаты сверяются с ручным эталоном.

### RUNTIME-001 — безопасный DAG executor

Status: TODO

Priority: P0

Prerequisites: CONTRACT-001, GRAPH-001, MODBUS-001

- Исполнять граф по readiness/conditions, а не по `[]SequenceStep` порядку.
- Ввести state machine узла: `pending`, `ready`, `written`, `start_confirmed`,
  `running`, `completed`, `failed`, `unknown`, `quarantined`.
- Требовать подтверждение перехода PLC в busy; отсутствие подтверждения не
  считать успешным завершением.
- Реализовать bounded retry только для идемпотентных действий; ambiguous
  external effect переводить в `unknown` и останавливать опасные продолжения.
- Реализовать checkpoints перед/после external effect и explicit resume policy.
- Gate: deterministic executor tests с fake PLC и fault injection после записи,
  во время polling, при timeout и при потере COM.

### AXIOM-003 — durable execution journal и recovery

Status: TODO

Priority: P0

Prerequisites: RUNTIME-001

- Расширить Axiom за пределы lifecycle: execution ID, graph revision/digest,
  node attempt, input digest, command intent, outcome, evidence ref и fence token.
- Сделать операции idempotent/replay-safe; после restart восстанавливать только
  доказанные checkpoints, неоднозначные команды помещать в quarantine.
- Убрать глобальный `programMu` и перенести execution ownership в runtime
  controller, связанный с Axiom execution.
- Gate: crash/restart/replay test с доказанным отсутствием двойного physical
  effect; состояние `/healthz` показывает execution, node и recovery phase.

### RESOURCE-001 — ресурсная и безопасностная модель аппарата

Status: TODO

Priority: P1

Prerequisites: CONTRACT-001, MODBUS-001

- Описать ресурсы: rotor, selector1, selector2, pump, valves, zones, waste
  tank, operator и их capacity/exclusive locks.
- Для команд задать preconditions, interlocks, postconditions и safe abort.
- Запретить графы, которые требуют несовместимых ресурсов или одновременного
  управления конфликтующими зонами.
- Gate: проверка конфликтов на тестовых алгоритмах и safety matrix для PLC.

### API-001 — разделение liveness/readiness/diagnostics

Status: TODO

Priority: P1

Prerequisites: AXIOM-003

- Разделить `/livez`, `/readyz`, `/healthz` и `/modbus/status`.
- `/livez` не должен выполнять serial I/O; Modbus probe должен иметь отдельный
  timeout и не блокировать диагностику процесса.
- Все ошибки executor/Axiom связывать с operation ID и structured event log.
- Gate: timeout tests для недоступного COM и проверка HTTP-кодов в degraded state.

### TEST-001 — математические, контрактные и fault-injection gates

Status: TODO

Priority: P1

Prerequisites: MATH-001, RUNTIME-001, AXIOM-003, RESOURCE-001

- Добавить property-based tests графа и формул, race tests executor и replay
  tests Axiom journal.
- Добавить fake PLC с моделями busy/complete/error/ambiguous states.
- Добавить real-process smoke: Wails EXE, HTTP, WebSocket, runtime-file и
  restart/recovery sequence.
- Разделять local PASS от HIL/hosted-CI/production evidence; не продвигать
  release без аппаратного подтверждения.

### PRODUCT-001 — разделение Studio и Operator без дублирования ядра

Status: TODO

Priority: P0

Prerequisites: CONTRACT-001, RUNTIME-001

- Разделить приложение на два функциональных режима поверх одного Go/Wails
  backend и одного runtime: `Algorithm Studio` для разработки и `Machine
  Control` для эксплуатации.
- Запретить создание второй бизнес-логики: оба режима используют общий
  `AlgorithmDefinition`, command registry, validator, compiler, executor,
  Axiom journal, machine state и WebSocket events.
- Зафиксировать границу данных: Studio работает с `Draft`, `Validated`,
  `Simulated`, `Published`; Operator получает только immutable
  `Published/Approved` snapshot с digest и revision.
- Определить capability boundary: Studio может редактировать граф и запускать
  симуляцию; Operator может выбирать опубликованный рецепт, запускать,
  приостанавливать, продолжать и безопасно останавливать execution, но не
  может менять граф или отправлять произвольный Modbus command.
- Выбрать один Wails binary с двумя frontend routes/profiles и отдельными
  capability tokens вместо двух разошедшихся приложений; для production
  разрешать только Operator UI.
- Gate: обе UI используют один API-контракт, Operator не видит draft и не
  получает endpoint редактирования, Studio не может обходить runtime и писать
  в Modbus напрямую.

### STUDIO-001 — Algorithm Studio: редактор, симулятор и публикация

Status: TODO

Priority: P0

Prerequisites: PRODUCT-001, GRAPH-001, MATH-001

- Выделить отдельный Studio navigation и компоненты: graph canvas, palette
  блоков, property inspector, edge-condition editor, subgraph editor,
  parameter bindings, validation panel и execution preview.
- Поддержать типы блоков `command`, `condition`, `variant`, `subgraph`,
  `checkpoint`, `parallel_join` и bounded `loop`; для каждого блока показывать
  входы, выходы, параметры, ресурсы и безопасную стратегию recovery.
- Добавить операции create/connect/disconnect/clone/delete, undo/redo,
  copy/paste, zoom/pan и сохранение draft с optimistic revision check.
- Реализовать подграфы с формальными входами/выходами, типизированными
  параметрами, default values, required values, mapping call-site и запретом
  рекурсивных вызовов.
- Добавить статический validation report с ошибками, предупреждениями,
  affected nodes, resource conflicts, unreachable branches и unknown paths.
- Добавить simulator без Modbus I/O с trace по узлам, условиям, параметрам,
  времени, ресурсам и fault-injection сценариям.
- Публиковать только graph revision, прошедшую validation и обязательную
  simulation policy; публикация должна быть атомарной и сохранять provenance.
- Gate: round-trip draft, визуальная проверка Studio, negative tests для
  invalid graph/bindings и доказательство отсутствия physical I/O в simulation.

### OPERATOR-001 — Machine Control: меню и пошаговое выполнение

Status: TODO

Priority: P0

Prerequisites: PRODUCT-001, AXIOM-003, RESOURCE-001

- Построить Operator navigation вокруг операций аппарата: System Check,
  Load, Sedimentation, Stain, Wash, Drain и пользовательские опубликованные
  recipes.
- Генерировать меню и пошаговое представление из metadata опубликованного
  алгоритма, а не поддерживать отдельный список команд в JavaScript.
- Показывать оператору только существенную модель: текущий шаг, прогресс,
  ожидаемое условие, оставшееся время, выбранную зону, реагент, sensor state,
  предупреждение и требуемое подтверждение.
- Реализовать состояния execution `idle`, `preflight`, `running`, `waiting`,
  `paused`, `failed`, `recovering`, `completed`, `quarantined` с понятным
  переходом и причиной.
- Дать оператору только безопасные действия: start, pause, resume, safe stop,
  acknowledge и recovery decision; скрыть graph editing, raw register writes,
  arbitrary command execution и draft selection.
- При restart восстанавливать экран из Axiom execution snapshot; не запускать
  повторно команду с неизвестным external effect, а показывать quarantine и
  процедуру подтверждения.
- Gate: реальный process smoke с Wails/HTTP/WebSocket, operator journey по
  System Check и Stain, запрет опасных endpoint-ов и recovery после перезапуска.

### PUBLISH-001 — жизненный цикл алгоритма и совместимость версий

Status: TODO

Priority: P1

Prerequisites: CONTRACT-001, PRODUCT-001, STUDIO-001, OPERATOR-001

- Ввести сущности `AlgorithmDraft`, `AlgorithmRevision`, `PublishedAlgorithm`
  и `ExecutionSnapshot`; draft не должен изменяться после публикации.
- Проверять совместимость command registry, schema version, firmware
  capability и resource model до публикации и до запуска.
- Хранить digest графа, compiled plan digest, author, timestamp, source,
  simulation evidence, validation evidence и changelog.
- Разрешить rollback только на ранее опубликованную совместимую revision;
  нельзя подменять revision во время уже запущенного execution.
- Gate: публикация invalid/draft algorithm отклоняется, запуск старой revision
  после редактирования draft остаётся детерминированным, rollback покрыт
  integration tests.

### SECURITY-001 — профили доступа и безопасная поверхность API

Status: TODO

Priority: P1

Prerequisites: PRODUCT-001, PUBLISH-001, API-001

- Разделить capability scopes: `studio.read`, `studio.edit`, `studio.simulate`,
  `studio.publish`, `operator.read`, `operator.execute`, `operator.recover`,
  `diagnostics.service`.
- Проверять scope на backend endpoint, а не только скрывать кнопки в UI.
- Разнести API на namespaces `/api/v1/studio/*`, `/api/v1/operator/*`,
  `/api/v1/executions/*` и `/api/v1/diagnostics/*`.
- Исключить из Operator API raw Modbus writes и импорт неподтверждённого draft.
- Gate: endpoint authorization matrix, negative tests с каждым scope и
  production profile, в котором Studio capabilities отсутствуют.

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
